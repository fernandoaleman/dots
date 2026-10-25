package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// Report records successful mutation calls and a result for each selected leaf.
// Results are in destination order; SKIP means the entry was not attempted.
type Report struct {
	Results            []Result
	LinksCreated       int
	DirectoriesCreated int
	Warnings           []string
}

type directoryID struct{ device, inode uint64 }

func identity(stat unix.Stat_t) directoryID {
	return directoryID{uint64(stat.Dev), uint64(stat.Ino)}
}

type directory struct {
	fd int
	id directoryID
}

type binding struct {
	parent *directory
	name   string
	entry  unix.Stat_t
	target *directory
	text   string
}

type sourceState struct {
	entry, parent os.FileInfo
	text          string
}

type preparedEntry struct {
	destination Destination
	initial     Result
	parent      *directory
	missing     []string
	routes      []binding
	source      sourceState
	leaf        unix.Stat_t
	text        string
}

// Per-plan seams schedule real filesystem races and deterministic failures in
// tests. They are private, never global, and default to the real Unix calls.
type installOps struct {
	openat     func(int, string, int, uint32) (int, error)
	mkdirat    func(int, string, uint32) error
	symlinkat  func(string, int, string) error
	close      func(int) error
	checkpoint func(string, Destination)
}

func nativeInstallOps() installOps {
	return installOps{openat: unix.Openat, mkdirat: unix.Mkdirat, symlinkat: unix.Symlinkat, close: unix.Close}
}

// PreparedPlan owns a read-only preparation and its pinned directory handles.
// Apply is single-use. Close is required when abandoning preparation and is
// otherwise called by Apply. A plan must not be used concurrently.
type PreparedPlan struct {
	registry *registry
	entries  []preparedEntry
	dirs     map[directoryID]*directory
	order    []*directory
	paths    map[string]*directory
	routes   []binding
	root     *directory
	ops      installOps
	used     bool
	closed   bool
}

// Prepare validates copied destinations without making filesystem changes.
// Required directories constrain the namespace but do not become link actions.
func Prepare(destinations []Destination) (*PreparedPlan, error) {
	return prepare(destinations, nativeInstallOps())
}

func prepare(destinations []Destination, ops installOps) (_ *PreparedPlan, err error) {
	p := &PreparedPlan{dirs: make(map[directoryID]*directory), paths: make(map[string]*directory), ops: ops}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.Close())
		}
	}()
	destinations = append([]Destination(nil), destinations...)
	sort.SliceStable(destinations, func(i, j int) bool { return destinations[i].Path < destinations[j].Path })
	leaves := 0
	for _, item := range destinations {
		if !filepath.IsAbs(item.Path) || strings.ContainsRune(item.Path, 0) ||
			(!item.Directory && (!filepath.IsAbs(item.Source) || strings.ContainsRune(item.Source, 0))) {
			return nil, fmt.Errorf("invalid source or destination path for %q", item.Path)
		}
		if leaf := filepath.Base(item.Path); leaf == "." || leaf == ".." || leaf == "/" || strings.HasSuffix(item.Path, "/") {
			return nil, fmt.Errorf("invalid destination basename in %q", item.Path)
		}
		if !item.Directory {
			leaves++
			result := Inspect(item.Source, item.Path)
			if result.Status == "CONFLICT" {
				return nil, fmt.Errorf("preflight %q: %s", item.Path, result.Reason)
			}
		}
	}
	if leaves == 0 {
		return p, nil
	}
	p.root, err = p.openDirectory(unix.AT_FDCWD, "/", unix.O_NOFOLLOW)
	if err != nil {
		return nil, fmt.Errorf("open root directory: %w", err)
	}
	p.paths["/"] = p.root
	for _, item := range destinations {
		parent, missing, routes, walkErr := p.walk(parentPath(item.Path))
		if walkErr != nil {
			return nil, fmt.Errorf("prepare parent of %q: %w", item.Path, walkErr)
		}
		if item.Directory {
			continue
		}
		entry := preparedEntry{destination: item, initial: Inspect(item.Source, item.Path), parent: parent, missing: missing, routes: routes}
		entry.source, err = captureSource(item.Source)
		if err != nil {
			return nil, err
		}
		if entry.initial.Status == "CONFLICT" {
			return nil, fmt.Errorf("preflight %q: %s", item.Path, entry.initial.Reason)
		}
		if len(missing) == 0 {
			leafErr := unix.Fstatat(parent.fd, filepath.Base(item.Path), &entry.leaf, unix.AT_SYMLINK_NOFOLLOW)
			if entry.initial.Status == "LINK" && !errors.Is(leafErr, unix.ENOENT) {
				return nil, fmt.Errorf("destination changed during preparation: %q", item.Path)
			}
			if entry.initial.Status == "OK" {
				if leafErr != nil || entry.leaf.Mode&unix.S_IFMT != unix.S_IFLNK {
					return nil, fmt.Errorf("destination changed during preparation: %q", item.Path)
				}
				entry.text, err = readlinkAt(parent.fd, filepath.Base(item.Path))
				if err != nil {
					return nil, err
				}
				if err := entry.validateExistingLink(); err != nil {
					return nil, err
				}
			}
		}
		p.entries = append(p.entries, entry)
	}
	// Name capabilities are queried through pinned descriptors. The existing
	// collision engine retains alias, Unicode, and empty-directory evidence.
	for _, reason := range collisionReasons(destinations, p.rulesAt) {
		if reason != "" {
			return nil, fmt.Errorf("preflight: %s", reason)
		}
	}
	if err = p.validatePreparation(); err != nil {
		return nil, err
	}
	return p, nil
}

const directoryFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC

func (p *PreparedPlan) openDirectory(parent int, name string, flags int) (*directory, error) {
	fd, err := p.ops.openat(parent, name, directoryFlags|flags, 0)
	if err != nil {
		return nil, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, errors.Join(err, p.ops.close(fd))
	}
	id := identity(stat)
	if existing := p.dirs[id]; existing != nil {
		return existing, p.ops.close(fd)
	}
	dir := &directory{fd: fd, id: id}
	p.dirs[id] = dir
	p.order = append(p.order, dir)
	return dir, nil
}

// walk uses one component per Openat. Existing symlinks are followed only
// during read-only preparation; the kernel bounds their expansion and retains
// native symlink/.. semantics. Named bindings are checked against pinned IDs.
func (p *PreparedPlan) walk(path string) (*directory, []string, []binding, error) {
	current, raw := p.root, ""
	var routes []binding
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, name := range parts {
		if name == "" || name == "." {
			continue
		}
		var stat unix.Stat_t
		err := unix.Fstatat(current.fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
		if errors.Is(err, unix.ENOENT) {
			var missing []string
			for _, part := range parts[i:] {
				if part == ".." {
					return nil, nil, nil, fmt.Errorf("unresolved parent traversal beneath missing ancestor")
				}
				if part != "" && part != "." {
					missing = append(missing, part)
				}
			}
			return current, missing, routes, nil
		}
		if err != nil {
			return nil, nil, nil, err
		}
		route := binding{parent: current, name: name, entry: stat}
		flags := unix.O_NOFOLLOW
		if stat.Mode&unix.S_IFMT == unix.S_IFLNK {
			route.text, err = readlinkAt(current.fd, name)
			if err != nil {
				return nil, nil, nil, err
			}
			flags = 0 // An explicitly observed, existing directory symlink.
		}
		next, err := p.openDirectory(current.fd, name, flags)
		if err != nil {
			return nil, nil, nil, err
		}
		route.target = next
		if err := validateBinding(route); err != nil {
			return nil, nil, nil, err
		}
		routes = append(routes, route)
		p.routes = append(p.routes, route)
		current = next
		raw += "/" + name
		canonical, err := filepath.EvalSymlinks(raw)
		if err != nil {
			return nil, nil, nil, err
		}
		var observed unix.Stat_t
		if err := unix.Stat(canonical, &observed); err != nil {
			return nil, nil, nil, err
		}
		if identity(observed) != current.id {
			return nil, nil, nil, fmt.Errorf("directory changed during preparation: %q", raw)
		}
		p.paths[canonical] = current
	}
	return current, nil, routes, nil
}

func (p *PreparedPlan) rulesAt(path string) (nameRules, error) {
	dir := p.paths[path]
	if dir == nil {
		var err error
		dir, _, _, err = p.walk(path)
		if err != nil {
			return nameRules{}, err
		}
	}
	return directoryNameRulesFD(dir.fd)
}

func validateBinding(route binding) error {
	var stat unix.Stat_t
	if err := unix.Fstatat(route.parent.fd, route.name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("ancestor %q changed: %w", route.name, err)
	}
	if identity(stat) != identity(route.entry) || stat.Mode&unix.S_IFMT != route.entry.Mode&unix.S_IFMT {
		return fmt.Errorf("ancestor %q changed", route.name)
	}
	if route.entry.Mode&unix.S_IFMT == unix.S_IFLNK {
		text, err := readlinkAt(route.parent.fd, route.name)
		if err != nil || text != route.text {
			return fmt.Errorf("ancestor symlink %q changed", route.name)
		}
		if err := unix.Fstatat(route.parent.fd, route.name, &stat, 0); err != nil {
			return fmt.Errorf("ancestor symlink %q changed: %w", route.name, err)
		}
	}
	if identity(stat) != route.target.id {
		return fmt.Errorf("ancestor directory %q changed", route.name)
	}
	return nil
}

func captureSource(path string) (sourceState, error) {
	var state sourceState
	var err error
	state.entry, err = os.Lstat(path)
	if err != nil {
		return state, fmt.Errorf("inspect source %q: %w", path, err)
	}
	if !state.entry.Mode().IsRegular() && state.entry.Mode()&os.ModeSymlink == 0 {
		return state, fmt.Errorf("unsupported source type %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		return state, fmt.Errorf("source unavailable %q: %w", path, err)
	}
	state.parent, err = os.Stat(parentPath(path))
	if err != nil {
		return state, fmt.Errorf("inspect source parent %q: %w", path, err)
	}
	if state.entry.Mode()&os.ModeSymlink != 0 {
		state.text, err = os.Readlink(path)
	}
	return state, err
}

func (entry preparedEntry) validateSource() error {
	current, err := captureSource(entry.destination.Source)
	if err != nil {
		return err
	}
	if !os.SameFile(current.entry, entry.source.entry) || !os.SameFile(current.parent, entry.source.parent) ||
		current.entry.Mode().Type() != entry.source.entry.Mode().Type() || current.text != entry.source.text {
		return fmt.Errorf("selected source changed: %q", entry.destination.Source)
	}
	return nil
}

func (p *PreparedPlan) validatePreparation() error {
	for _, route := range p.routes {
		if err := validateBinding(route); err != nil {
			return err
		}
	}
	for _, entry := range p.entries {
		if err := entry.validateSource(); err != nil {
			return err
		}
		name := filepath.Base(entry.destination.Path)
		if len(entry.missing) != 0 {
			name = entry.missing[0]
		}
		var stat unix.Stat_t
		err := unix.Fstatat(entry.parent.fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
		if entry.initial.Status == "LINK" {
			if !errors.Is(err, unix.ENOENT) {
				return fmt.Errorf("destination or parent appeared after preflight: %q", entry.destination.Path)
			}
		} else if err != nil || identity(stat) != identity(entry.leaf) {
			return fmt.Errorf("destination changed after preflight: %q", entry.destination.Path)
		} else if err := entry.validateExistingLink(); err != nil {
			return err
		}
	}
	return nil
}

// An unchanged link inode and text can resolve elsewhere after an alias in its
// stored target changes. Recheck both the link and its selected named entry.
func (entry preparedEntry) validateExistingLink() error {
	var stat unix.Stat_t
	name := filepath.Base(entry.destination.Path)
	statErr := unix.Fstatat(entry.parent.fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	text, linkErr := readlinkAt(entry.parent.fd, name)
	if statErr != nil || linkErr != nil || identity(stat) != identity(entry.leaf) ||
		stat.Mode&unix.S_IFMT != unix.S_IFLNK || text != entry.text {
		return fmt.Errorf("destination changed after preflight: %q", entry.destination.Path)
	}
	target := text
	if !filepath.IsAbs(target) {
		target = parentPath(entry.destination.Path) + "/" + target
	}
	targetEntry, targetErr := canonicalEntry(target)
	sourceEntry, sourceErr := canonicalEntry(entry.destination.Source)
	if targetErr != nil || sourceErr != nil || !sameEntry(targetEntry, sourceEntry) {
		return fmt.Errorf("destination target changed after preflight: %q", entry.destination.Path)
	}
	return nil
}

// Apply creates parents and real symlinks in destination order. It stops on
// failure, retains completed work, and always closes the prepared resources.
// No operation replaces an existing entry or falls back to pathname writes.
func (p *PreparedPlan) Apply() (report Report, err error) {
	for range p.entries {
		report.Results = append(report.Results, Result{Status: "SKIP", Reason: "not attempted"})
	}
	if p.closed || p.used {
		return report, fmt.Errorf("prepared plan is closed or already used")
	}
	p.used = true
	defer func() { err = errors.Join(err, p.Close()) }()
	if err := p.validatePreparation(); err != nil {
		return report, err
	}
	created := make(map[directoryID]binding)
	for i, entry := range p.entries {
		if err := p.applyEntry(entry, created, &report); err != nil {
			report.Results[i] = Result{Status: "ERROR", Reason: err.Error()}
			return report, err
		}
		status := "OK"
		if entry.initial.Status == "LINK" {
			status = "LINKED"
		}
		report.Results[i] = Result{Status: status}
	}
	return report, nil
}

func (p *PreparedPlan) applyEntry(entry preparedEntry, created map[directoryID]binding, report *Report) error {
	check := func() error {
		if p.registry != nil {
			if err := p.registry.validate(); err != nil {
				return err
			}
			if err := p.registry.options.protect(entry.destination.Path); err != nil {
				return err
			}
		}
		for _, route := range entry.routes {
			if err := validateBinding(route); err != nil {
				return err
			}
		}
		return entry.validateSource()
	}
	current := entry.parent
	for _, name := range entry.missing {
		if err := check(); err != nil {
			return err
		}
		var stat unix.Stat_t
		err := unix.Fstatat(current.fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
		if err == nil {
			original, known := created[identity(stat)]
			if stat.Mode&unix.S_IFMT != unix.S_IFDIR || !known || original.parent.id != current.id {
				return fmt.Errorf("unexpected parent %q for %q", name, entry.destination.Path)
			}
			if err := validateBinding(original); err != nil {
				return err
			}
		} else {
			if !errors.Is(err, unix.ENOENT) {
				return fmt.Errorf("inspect parent %q: %w", name, err)
			}
			p.checkpoint("before-mkdir", entry.destination)
			if err := p.ops.mkdirat(current.fd, name, 0o700); err != nil {
				return fmt.Errorf("create parent %q for %q: %w", name, entry.destination.Path, err)
			}
			report.DirectoriesCreated++
			p.checkpoint("after-mkdir", entry.destination)
			if err := unix.Fstatat(current.fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				return fmt.Errorf("inspect created parent %q: %w", name, err)
			}
		}
		p.checkpoint("before-parent-open", entry.destination)
		next, err := p.openDirectory(current.fd, name, unix.O_NOFOLLOW)
		if err != nil {
			return fmt.Errorf("open parent %q for %q: %w", name, entry.destination.Path, err)
		}
		if identity(stat) != next.id {
			return fmt.Errorf("created parent %q changed", name)
		}
		route := binding{parent: current, name: name, entry: stat, target: next}
		if original, known := created[next.id]; known {
			if err := validateBinding(original); err != nil {
				return err
			}
		} else {
			created[next.id] = route
		}
		entry.routes = append(entry.routes, route)
		current = next
	}
	if err := check(); err != nil {
		return err
	}
	name := filepath.Base(entry.destination.Path)
	if entry.initial.Status == "OK" {
		return entry.validateExistingLink()
	}
	if p.registry != nil {
		if err := p.registry.beforeCreate(entry, current); err != nil {
			return err
		}
		if err := check(); err != nil {
			return err
		}
	}
	p.checkpoint("before-symlink", entry.destination)
	if err := p.ops.symlinkat(entry.destination.Source, current.fd, name); err != nil {
		return fmt.Errorf("create symlink %q: %w", entry.destination.Path, err)
	}
	report.LinksCreated++
	if p.registry != nil {
		p.checkpoint("before-ownership", entry.destination)
		if err := p.registry.afterCreate(entry, current); err != nil {
			return err
		}
	}
	return nil
}

func (p *PreparedPlan) checkpoint(phase string, destination Destination) {
	if p.ops.checkpoint != nil {
		p.ops.checkpoint(phase, destination)
	}
}

func readlinkAt(fd int, name string) (string, error) {
	for size := 128; size <= 65536; size *= 2 {
		buffer := make([]byte, size)
		n, err := unix.Readlinkat(fd, name, buffer)
		if err != nil {
			return "", err
		}
		if n < len(buffer) {
			return string(buffer[:n]), nil
		}
	}
	return "", fmt.Errorf("symlink target too long for %q", name)
}

// Close releases prepared resources, including an abandoned preparation.
// Repeated calls are harmless. Close never removes filesystem entries.
func (p *PreparedPlan) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	var errs []error
	for i := len(p.order) - 1; i >= 0; i-- {
		dir := p.order[i]
		if err := p.ops.close(dir.fd); err != nil {
			errs = append(errs, fmt.Errorf("close prepared directory: %w", err))
		}
	}
	return errors.Join(errs...)
}
