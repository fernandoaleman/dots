package filesystem

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// Ownership selects a machine-local registry and a configuration/target scope.
// Sources protect configured source contents; removal never scans their trees.
type Ownership struct {
	Registry string
	Profile  string
	Target   string
	Sources  []string
}

// StatePath applies the XDG absolute-path rule without cleaning symlink/.. paths.
func StatePath(home, xdg string) string {
	if !filepath.IsAbs(xdg) {
		xdg = strings.TrimRight(home, "/") + "/.local/state"
	}
	return strings.TrimRight(xdg, "/") + "/dots/ownership.json"
}

// All pathname fields are encoded as bytes, including route names and locators.
// JSON's replacement of invalid UTF-8 must never redirect filesystem authority.
type pathBytes string

func (p pathBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString([]byte(p)))
}

func (p *pathBytes) UnmarshalJSON(data []byte) error {
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return err
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	*p = pathBytes(decoded)
	return err
}

type entryID struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func idOf(stat unix.Stat_t) entryID { return entryID{uint64(stat.Dev), uint64(stat.Ino)} }

type fingerprint struct {
	ID       entryID `json:"id"`
	Mode     uint32  `json:"mode"`
	Size     int64   `json:"size"`
	Modified int64   `json:"modified_seconds"`
	ModNsec  int64   `json:"modified_nanoseconds"`
	Changed  int64   `json:"changed_seconds"`
	ChgNsec  int64   `json:"changed_nanoseconds"`
}

func fingerprintOf(s unix.Stat_t) fingerprint {
	return fingerprint{idOf(s), uint32(s.Mode), s.Size, s.Mtim.Sec, s.Mtim.Nsec, s.Ctim.Sec, s.Ctim.Nsec}
}

type recordedRoute struct {
	Parent entryID   `json:"parent"`
	Name   pathBytes `json:"name"`
	Entry  entryID   `json:"entry"`
	Type   uint32    `json:"type"`
	Text   pathBytes `json:"text"`
	Target entryID   `json:"target"`
}

func recordRoutes(routes []binding) []recordedRoute {
	result := make([]recordedRoute, 0, len(routes))
	for _, r := range routes {
		result = append(result, recordedRoute{idOfDirectory(r.parent), pathBytes(r.name), idOf(r.entry),
			uint32(r.entry.Mode) & unix.S_IFMT, pathBytes(r.text), idOfDirectory(r.target)})
	}
	return result
}

func idOfDirectory(d *directory) entryID { return entryID{d.id.device, d.id.inode} }

type ownershipRecord struct {
	Profile     pathBytes       `json:"profile"`
	Target      pathBytes       `json:"target"`
	Destination pathBytes       `json:"destination"`
	Text        pathBytes       `json:"text"`
	Phase       string          `json:"phase"`
	Parent      entryID         `json:"parent"`
	Routes      []recordedRoute `json:"routes"`
	Leaf        *fingerprint    `json:"leaf"`
}

type snapshot struct {
	Version    int               `json:"version"`
	Revision   uint64            `json:"revision"`
	Unfinished bool              `json:"unfinished"`
	Records    []ownershipRecord `json:"records"`
	Imports    []importIntent    `json:"imports,omitempty"`
}

const registrySizeLimit = 16 << 20

// registryOps exposes narrow per-invocation failure seams, never global hooks.
type registryOps struct {
	nameRules  func(int) (nameRules, error)
	sync       func(int) error
	rename     func(int, string, int, string) error
	write      func(*os.File, []byte) (int, error)
	closeFile  func(*os.File) error
	unlink     func(int, string, int) error
	checkpoint func(string, string)
}

func nativeRegistryOps() registryOps {
	return registryOps{nameRules: directoryNameRulesFD, sync: unix.Fsync, rename: unix.Renameat, write: func(f *os.File, b []byte) (int, error) { return f.Write(b) }, closeFile: func(f *os.File) error { return f.Close() }, unlink: unix.Unlinkat}
}

type registry struct {
	options Ownership
	space   *PreparedPlan
	dir     *directory
	lock    int
	lockID  entryID
	data    snapshot
	ops     registryOps
	warning string
	blocked error // Pending imports loaded from disk cannot be cleared in memory.
	version int   // Loaded wire version; ordinary commands may not change it.
}

func namespace() (*PreparedPlan, error) {
	p := &PreparedPlan{dirs: make(map[directoryID]*directory), paths: make(map[string]*directory), ops: nativeInstallOps()}
	var err error
	p.root, err = p.openDirectory(unix.AT_FDCWD, "/", unix.O_NOFOLLOW)
	return p, err
}

func openRegistry(options Ownership, writable bool, ops registryOps) (_ *registry, err error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	p, err := namespace()
	if err != nil {
		return nil, err
	}
	r := &registry{options: options, space: p, lock: -1, ops: ops, version: 1, data: snapshot{Version: 1, Records: []ownershipRecord{}}}
	defer func() {
		if err != nil {
			err = errors.Join(err, r.Close())
		}
	}()
	parent, missing, _, err := p.walk(parentPath(options.Registry))
	if err != nil {
		return nil, fmt.Errorf("open ownership directory: %w", err)
	}
	if !writable && len(missing) != 0 {
		return r, nil
	}
	for _, name := range missing {
		if err := unix.Mkdirat(parent.fd, name, 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
			return nil, fmt.Errorf("create state directory: %w", err)
		}
		next, err := p.openDirectory(parent.fd, name, unix.O_NOFOLLOW)
		if err != nil {
			return nil, err
		}
		if err := privateFD(next.fd, true); err != nil {
			return nil, err
		}
		var stat unix.Stat_t
		if err := unix.Fstatat(parent.fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil || idOf(stat) != idOfDirectory(next) {
			return nil, fmt.Errorf("state directory changed during creation")
		}
		p.routes = append(p.routes, binding{parent: parent, name: name, entry: stat, target: next})
		if err := ops.sync(next.fd); err != nil {
			return nil, fmt.Errorf("sync state directory: %w", err)
		}
		if err := ops.sync(parent.fd); err != nil {
			return nil, fmt.Errorf("sync state ancestor: %w", err)
		}
		parent = next
	}
	r.dir = parent
	if err := privateFD(parent.fd, true); err != nil {
		return nil, err
	}
	// The dedicated dots directory itself cannot be a symlink, even when
	// ordinary HOME/XDG ancestors contain stable directory aliases.
	if len(p.routes) != 0 && p.routes[len(p.routes)-1].entry.Mode&unix.S_IFMT == unix.S_IFLNK {
		return nil, fmt.Errorf("ownership directory must not be a symlink")
	}
	if writable {
		if err := r.acquire(); err != nil {
			return nil, err
		}
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func privateFD(fd int, directory bool) error {
	var s unix.Stat_t
	if err := unix.Fstat(fd, &s); err != nil {
		return err
	}
	want, permissions := uint32(unix.S_IFREG), uint32(0o600)
	if directory {
		want, permissions = unix.S_IFDIR, 0o700
	}
	if uint32(s.Mode)&unix.S_IFMT != want || uint32(s.Mode)&0o7777 != permissions || s.Uid != uint32(os.Geteuid()) || (!directory && s.Nlink != 1) {
		return fmt.Errorf("ownership state requires private user-owned directories (0700) and regular files (0600)")
	}
	return nil
}

func (r *registry) acquire() error {
	fd, err := unix.Openat(r.dir.fd, "ownership.lock", unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, unix.ENOENT) {
		var s unix.Stat_t
		if statErr := unix.Fstatat(r.dir.fd, "ownership.json", &s, unix.AT_SYMLINK_NOFOLLOW); !errors.Is(statErr, unix.ENOENT) {
			return fmt.Errorf("ownership registry exists without its stable lock; manual recovery required")
		}
		fd, err = unix.Openat(r.dir.fd, "ownership.lock", unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
		if errors.Is(err, unix.EEXIST) {
			fd, err = unix.Openat(r.dir.fd, "ownership.lock", unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
		}
	}
	if err != nil {
		return fmt.Errorf("open ownership lock: %w", err)
	}
	r.lock = fd
	if err := privateFD(fd, false); err != nil {
		return err
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("ownership registry is busy or locking failed: %w", err)
	}
	var s unix.Stat_t
	if err := unix.Fstat(fd, &s); err != nil {
		return err
	}
	r.lockID = idOf(s)
	if err := r.ops.sync(r.dir.fd); err != nil {
		return fmt.Errorf("sync ownership lock directory: %w", err)
	}
	return r.validate()
}

func (r *registry) validate() error {
	for _, route := range r.space.routes {
		if err := validateBinding(route); err != nil {
			return fmt.Errorf("ownership state ancestry: %w", err)
		}
	}
	if r.lock >= 0 {
		var s unix.Stat_t
		if err := unix.Fstatat(r.dir.fd, "ownership.lock", &s, unix.AT_SYMLINK_NOFOLLOW); err != nil || idOf(s) != r.lockID {
			return fmt.Errorf("stable ownership lock changed")
		}
	}
	return nil
}

func (r *registry) load() error {
	fd, err := unix.Openat(r.dir.fd, "ownership.json", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open ownership registry: %w", err)
	}
	f := os.NewFile(uintptr(fd), "ownership.json")
	defer f.Close()
	if err := privateFD(fd, false); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(f, registrySizeLimit+1))
	if err != nil {
		return fmt.Errorf("read ownership registry: %w", err)
	}
	if len(data) > registrySizeLimit {
		return fmt.Errorf("ownership registry exceeds 16 MiB size limit")
	}
	if err := strictJSON(data); err != nil {
		return fmt.Errorf("invalid ownership registry: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r.data); err != nil {
		return fmt.Errorf("decode ownership registry: %w", err)
	}
	if err := validateSnapshot(r.data); err != nil {
		return err
	}
	r.version = r.data.Version
	r.blocked = r.data.allowMutation()
	sort.Slice(r.data.Records, func(i, j int) bool { return r.data.Records[i].Destination < r.data.Records[j].Destination })
	return r.validate()
}

func validateSnapshot(s snapshot) error {
	if (s.Version != 1 && s.Version != 2) || s.Records == nil ||
		(s.Version == 1 && s.Imports != nil) || (s.Version == 2 && s.Imports == nil) {
		return fmt.Errorf("unsupported or incomplete ownership registry")
	}
	claims := make(map[string]bool)
	physicalClaims := make(map[struct {
		Parent entryID
		Name   string
	}]bool)
	for _, record := range s.Records {
		if !validAbsolute(string(record.Profile)) || !validAbsolute(string(record.Target)) ||
			!validAbsolute(string(record.Destination)) || !validAbsolute(string(record.Text)) ||
			!strings.HasPrefix(string(record.Destination), strings.TrimRight(string(record.Target), "/")+"/") ||
			!validBasename(filepath.Base(string(record.Destination))) || record.Parent.Inode == 0 || record.Routes == nil {
			return fmt.Errorf("invalid ownership record paths or parent evidence")
		}
		if strings.HasSuffix(string(record.Destination), "/") {
			return fmt.Errorf("invalid ownership destination basename")
		}
		switch record.Phase {
		case "creating", "owned", "removing", "invalidated":
		default:
			return fmt.Errorf("unsupported ownership phase %q", record.Phase)
		}
		if record.Leaf == nil || (record.Phase != "creating" && record.Leaf.ID.Inode == 0) ||
			(record.Phase != "creating" && record.Leaf.Mode&unix.S_IFMT != unix.S_IFLNK) {
			return fmt.Errorf("missing ownership fingerprint")
		}
		for _, route := range record.Routes {
			if (!validBasename(string(route.Name)) && route.Name != "..") || route.Parent.Inode == 0 || route.Entry.Inode == 0 || route.Target.Inode == 0 ||
				(route.Type != unix.S_IFDIR && route.Type != unix.S_IFLNK) || (route.Type == unix.S_IFLNK && route.Text == "") {
				return fmt.Errorf("invalid ownership ancestry evidence")
			}
		}
		key := string(record.Destination)
		if claims[key] {
			return fmt.Errorf("duplicate ownership destination %q", key)
		}
		claims[key] = true
		physical := struct {
			Parent entryID
			Name   string
		}{record.Parent, filepath.Base(key)}
		if physicalClaims[physical] {
			return fmt.Errorf("duplicate recorded physical destination %q", key)
		}
		physicalClaims[physical] = true
	}
	return validateImports(s)
}

func validAbsolute(path string) bool { return filepath.IsAbs(path) && !strings.ContainsRune(path, 0) }
func validBasename(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\x00")
}

func (r *registry) save() error {
	if err := r.allowMutation(); err != nil {
		return err
	}
	if r.data.Version != r.version {
		return fmt.Errorf("registry version changes are not supported by ordinary commands")
	}
	if r.lock < 0 {
		return fmt.Errorf("ownership mutation requires a lock")
	}
	if err := r.validate(); err != nil {
		return err
	}
	if err := revisionCapacity(r.data.Revision, 1); err != nil {
		return err
	}
	r.data.Revision++
	sort.Slice(r.data.Records, func(i, j int) bool { return r.data.Records[i].Destination < r.data.Records[j].Destination })
	if err := validateSnapshot(r.data); err != nil {
		return err
	}
	data, err := snapshotBytes(r.data, 0)
	if err != nil {
		return err
	}
	token, err := randomName()
	if err != nil {
		return err
	}
	name := ".ownership-" + token + ".tmp"
	fd, err := unix.Openat(r.dir.fd, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return fmt.Errorf("create ownership snapshot: %w", err)
	}
	f := os.NewFile(uintptr(fd), name)
	// Failed temporary snapshots are deliberately retained, never replayed.
	// Creation permissions are subject to umask; never publish state that a
	// subsequent invocation would reject as unusable or unsafe.
	if err := privateFD(fd, false); err != nil {
		return fmt.Errorf("validate ownership snapshot: %w", errors.Join(err, r.ops.closeFile(f)))
	}
	n, writeErr := r.ops.write(f, data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = r.ops.sync(fd)
	}
	if err := errors.Join(writeErr, r.ops.closeFile(f)); err != nil {
		return fmt.Errorf("write ownership snapshot: %w", err)
	}
	if err := r.validate(); err != nil {
		return err
	}
	if err := r.ops.rename(r.dir.fd, name, r.dir.fd, "ownership.json"); err != nil {
		return fmt.Errorf("publish ownership snapshot: %w", err)
	}
	if err := r.ops.sync(r.dir.fd); err != nil {
		return fmt.Errorf("sync published ownership snapshot; outcome uncertain: %w", err)
	}
	return nil
}

// Every published snapshot must fit after global invalidation as well as now.
// Reserve the longest phase, counter, and barrier encodings so recovery can
// always revoke authority without first deleting or dropping history.
func snapshotBytes(s snapshot, reserve int) ([]byte, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	worst := s
	worst.Revision, worst.Unfinished = math.MaxUint64, len(s.Imports) != 0
	worst.Records = append([]ownershipRecord{}, s.Records...)
	for i := range worst.Records {
		worst.Records[i].Phase = "invalidated"
	}
	recovery, err := json.MarshalIndent(worst, "", "  ")
	if err != nil {
		return nil, err
	}
	if reserve < 0 || len(data) >= registrySizeLimit || len(recovery) >= registrySizeLimit || reserve > registrySizeLimit-len(recovery)-1 {
		return nil, fmt.Errorf("ownership registry exceeds 16 MiB including recovery capacity; previous snapshot retained")
	}
	return append(data, '\n'), nil
}

func randomName() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", token), nil
}

func (r *registry) begin() error {
	if err := r.allowMutation(); err != nil {
		return err
	}
	if err := revisionCapacity(r.data.Revision, 2); err != nil {
		return err
	}
	if r.data.Unfinished {
		// A durable barrier preceded every authority-revoking observation.
		// Revoke conservatively without visiting historical targets.
		for i := range r.data.Records {
			if r.data.Records[i].Phase == "owned" || r.data.Records[i].Phase == "removing" {
				r.data.Records[i].Phase = "invalidated"
			}
		}
		r.warning = "unfinished ownership transaction: prior ownership cannot authorize removal; existing entries preserved; manual recovery may be required"
	}
	r.data.Unfinished = true
	return r.save()
}

func (r *registry) finish() error {
	if err := r.allowMutation(); err != nil {
		return err
	}
	r.data.Unfinished = false
	return r.save()
}

func (r *registry) Close() error {
	var err error
	if r.lock >= 0 {
		err = unix.Close(r.lock)
		r.lock = -1
	}
	return errors.Join(err, r.space.Close())
}
