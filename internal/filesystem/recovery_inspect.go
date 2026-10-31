package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"golang.org/x/sys/unix"
)

// These copied observations carry no descriptors, lock, or mutation authority.
// A future administrator must select the ID/revision and repeat checks under lock.
type recoveryInspection struct {
	Registry          string
	Exists            bool
	Version           int
	Revision          uint64
	Unfinished        bool
	AuthorityRecords  int
	Intent            *recoveryIntentInspection
	SettlementReasons []string
}

type recoveryIntentInspection struct {
	ID, Profile, Target, Phase string
	Input, Source              recoverySlot
	Checks                     []recoveryCheck
	RetirementObserved         bool // Consistent observations only, never authorization.
}

type recoverySlot struct{ Path, Kind string }
type recoverySelection struct {
	ID       string
	Revision uint64
}
type recoveryCheck struct {
	Name   string
	Passed bool
	Detail string
}

func (i *recoveryIntentInspection) check(name string, err error) {
	detail := "verified"
	if err != nil {
		detail = err.Error()
	}
	i.Checks = append(i.Checks, recoveryCheck{name, err == nil, detail})
}

// inspectRecovery is private and strictly observational. A nil selection requests
// a summary; only an exact stored ID permits visiting recorded file locations.
// checkpoint is a per-call test seam, never a recovery or synchronization hook.
func inspectRecovery(path string, selection *recoverySelection, checkpoint func(string)) (result recoveryInspection, err error) {
	result.Registry = path
	if !validAbsolute(path) || filepath.Base(path) != "ownership.json" {
		return result, fmt.Errorf("invalid recovery registry locator")
	}
	r, err := openRegistryState(Ownership{Registry: path}, false, nativeRegistryOps())
	if err != nil {
		return result, err
	}
	view, err := inspectImportRegistry(r, nil)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, view.Close()) }()
	var evidence *registryPublication
	if r.dir != nil {
		entry, e := observeImportState(r.dir, "ownership.json")
		if e != nil {
			return result, e
		}
		result.Exists = entry.exists
		// Canonical continuity does not require spare future write capacity.
		h := sha256.New()
		encoder := json.NewEncoder(h)
		encoder.SetIndent("", "  ")
		if e := encoder.Encode(r.data); e != nil {
			return result, e
		}
		published := registryPublication{entry: entry, revision: r.data.Revision}
		copy(published.digest[:], h.Sum(nil))
		evidence = &published
	}
	defer func() {
		err = errors.Join(err, view.revalidate())
		if evidence != nil {
			err = errors.Join(err, evidence.check(r))
		}
		if err != nil && result.Intent != nil {
			result.Intent.RetirementObserved = false
			result.Intent.check("final consistency", err)
		}
	}()
	result.Version, result.Revision, result.Unfinished = r.data.Version, r.data.Revision, r.data.Unfinished
	for _, record := range r.data.Records {
		if record.Phase == "owned" || record.Phase == "removing" {
			result.AuthorityRecords++
		}
	}
	if checkpoint != nil {
		checkpoint("snapshot")
	}
	if len(r.data.Imports) == 0 {
		if selection != nil {
			return result, fmt.Errorf("no pending import matches selected identifier %q", selection.ID)
		}
		if !result.Unfinished {
			result.SettlementReasons = []string{"no unfinished barrier to settle"}
		} else {
			result.SettlementReasons = []string{"barrier-only state requires separate locked M3 validation; settlement is not implemented"}
		}
		return result, nil
	}
	i := r.data.Imports[0]
	observed := recoverySummary(i)
	result.Intent = observed
	result.SettlementReasons = []string{"pending import must be explicitly retired before barrier settlement; no public recovery command is available"}
	if selection == nil {
		observed.check("selection", fmt.Errorf("select pending import %s at registry revision %d to inspect recorded locations", i.ID, result.Revision))
		return result, nil
	}
	if selection.ID != i.ID || selection.Revision != r.data.Revision {
		return result, fmt.Errorf("selected import identifier or registry revision does not match pending import %s at revision %d", i.ID, r.data.Revision)
	}
	observed.check("retirement revision budget", revisionCapacity(r.data.Revision, 2))
	_, capacityErr := snapshotBytes(r.data, 0)
	observed.check("publication capacity", capacityErr)
	evidenceFiles, err := inspectRecoveryFiles(i, observed, checkpoint)
	if evidenceFiles != nil {
		defer func() { err = errors.Join(err, evidenceFiles.Close()) }()
	}
	return result, err
}

func recoverySummary(i importIntent) *recoveryIntentInspection {
	return &recoveryIntentInspection{ID: i.ID, Profile: string(i.Profile), Target: string(i.Target), Phase: i.Phase,
		Input: recoverySlot{string(i.Input), "not inspected"}, Source: recoverySlot{string(i.Source), "not inspected"}}
}

// Pinned observations are not transaction authority. Only the private locked
// retirement invocation may use them for synchronization; inspection closes them.
type recoveryEvidence struct {
	plan     *PreparedPlan
	entries  []importStateEntry
	files    []*os.File
	selected map[*directory]string
	closed   bool
}

func (h *recoveryEvidence) parents() []*directory {
	var dirs []*directory
	for dir := range h.selected {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool { return h.selected[dirs[i]] < h.selected[dirs[j]] })
	return dirs
}

func (h *recoveryEvidence) Close() error {
	if h.closed {
		return nil
	}
	h.closed = true
	var err error
	for _, f := range h.files {
		err = errors.Join(err, f.Close())
	}
	for _, dir := range h.parents() {
		err = errors.Join(err, unix.Close(dir.fd))
	}
	return errors.Join(err, h.plan.Close())
}

func (h *recoveryEvidence) check() error {
	if h.closed {
		return fmt.Errorf("recovery observations already closed")
	}
	var err error
	for _, route := range h.plan.routes {
		err = errors.Join(err, validateBinding(route))
	}
	for _, entry := range h.entries {
		err = errors.Join(err, entry.revalidate())
		// Compare pinned and named mount views, not inode identity alone.
		dir, e := recoveryOpenParent(h.selected[entry.parent], idOfDirectory(entry.parent))
		if e == nil {
			current := entry
			current.parent = dir
			e = errors.Join(current.revalidate(), unix.Close(dir.fd))
		}
		err = errors.Join(err, e)
	}
	return err
}

func inspectRecoveryFiles(i importIntent, observed *recoveryIntentInspection, checkpoint func(string)) (_ *recoveryEvidence, err error) {
	p, err := namespace()
	if err != nil {
		return nil, err
	}
	h := &recoveryEvidence{plan: p, selected: map[*directory]string{}}
	defer func() {
		if checkpoint != nil {
			checkpoint("final")
		}
		err = errors.Join(err, h.check())
		if err != nil {
			observed.RetirementObserved = false
			observed.check("final file consistency", err)
		}
		if err != nil || !observed.RetirementObserved {
			err = errors.Join(err, h.Close())
		}
	}()
	input, _, inputErr := recoveryParent(p, string(i.Input), i.InputBinding, nil, h.selected)
	observed.check("input ancestry", inputErr)
	sourceBinding, missing := i.SourceAnchor.importBinding, i.SourceAnchor.Missing
	if i.SourceBinding != nil {
		sourceBinding, missing = *i.SourceBinding, nil
	}
	source, frontier, sourceErr := recoveryParent(p, string(i.Source), sourceBinding, missing, h.selected)
	if frontier != nil {
		h.entries = append(h.entries, *frontier)
		observed.check("source anchor access", unix.Faccessat(frontier.parent.fd, ".", unix.X_OK, unix.AT_EACCESS))
	}
	observed.check("source ancestry", sourceErr)
	for n, parent := range []*directory{input, source} {
		slot := &observed.Input
		if n == 1 {
			slot = &observed.Source
		}
		if parent == nil {
			if (n == 0 && inputErr != nil) || (n == 1 && sourceErr != nil) {
				slot.Kind = "unverified ancestry"
			} else {
				slot.Kind = "missing ancestors"
			}
			continue
		}
		entry, e := observeImportState(parent, filepath.Base(slot.Path))
		if e != nil {
			slot.Kind = "inspection failed"
			observed.check(slot.Path+" type", e)
			continue
		}
		h.entries = append(h.entries, entry)
		slot.Kind = recoveryKind(entry)
		observed.check(slot.Path+" parent access", unix.Faccessat(parent.fd, ".", unix.X_OK, unix.AT_EACCESS))
		if !entry.exists || slot.Kind != "regular file" {
			continue
		}
		f, e := recoveryFile(parent, entry, i, n == 1, checkpoint)
		if f != nil {
			h.files = append(h.files, f)
		}
		observed.check(slot.Path+" file evidence", e)
	}
	if checkpoint != nil {
		checkpoint("slots")
	}
	absent := func(kind string) bool { return kind == "missing" || kind == "missing ancestors" }
	if !(observed.Input.Kind == "regular file" && absent(observed.Source.Kind) || observed.Source.Kind == "regular file" && absent(observed.Input.Kind)) {
		observed.check("exclusive intact file", fmt.Errorf("retirement requires exactly one intact regular file and a verified absent counterpart; observed input %s, source %s", observed.Input.Kind, observed.Source.Kind))
	} else {
		observed.check("exclusive intact file", nil)
	}
	observed.RetirementObserved = true
	for _, check := range observed.Checks {
		observed.RetirementObserved = observed.RetirementObserved && check.Passed
	}
	return h, nil
}

func recoveryParent(p *PreparedPlan, path string, expected importBinding, expectedMissing []pathBytes, selected map[*directory]string) (*directory, *importStateEntry, error) {
	parentPathname := parentPath(path)
	for range expectedMissing {
		parentPathname = parentPath(parentPathname)
	}
	parent, missing, routes, err := p.walk(parentPathname)
	if err != nil {
		return nil, nil, err
	}
	if idOfDirectory(parent) != expected.Parent || !reflect.DeepEqual(recordRoutes(routes), expected.Routes) || len(missing) != 0 {
		return nil, nil, fmt.Errorf("recorded parent or ancestry changed for %q", path)
	}
	parent, err = recoveryOpenParent(parentPathname, expected.Parent)
	if err != nil {
		return nil, nil, err
	}
	selected[parent] = parentPathname
	for _, route := range routes {
		if err := validateBinding(route); err != nil {
			return nil, nil, err
		}
	}
	if len(expectedMissing) != 0 {
		frontier, err := observeImportState(parent, string(expectedMissing[0]))
		if err != nil || frontier.exists {
			return nil, nil, errors.Join(fmt.Errorf("unrecorded source-parent frontier is occupied or unsafe"), err)
		}
		return nil, &frontier, nil
	}
	return parent, nil, nil
}

func recoveryOpenParent(path string, expected entryID) (*directory, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || idOf(stat) != expected {
		return nil, errors.Join(fmt.Errorf("selected recovery parent identity changed"), err, unix.Close(fd))
	}
	return &directory{fd: fd, id: identity(stat)}, nil
}

func recoveryKind(entry importStateEntry) string {
	if !entry.exists {
		return "missing"
	}
	switch entry.stat.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		return "regular file"
	case unix.S_IFLNK:
		return "symlink"
	case unix.S_IFDIR:
		return "directory"
	default:
		return "special file"
	}
}

func recoveryFile(parent *directory, entry importStateEntry, intent importIntent, source bool, checkpoint func(string)) (*os.File, error) {
	if !importRegular(entry.stat) {
		return nil, fmt.Errorf("regular file requires current-user ownership, single-link evidence and nonprivileged permissions")
	}
	file := importFile{fingerprintOf(entry.stat), entry.stat.Uid, entry.stat.Gid, uint64(entry.stat.Nlink)}
	want := intent.OriginalFile
	match := file == want
	if source {
		match = sameImportFile(file, want) && intent.SourceBinding != nil
		if intent.RelocatedFile != nil {
			match = match && file == *intent.RelocatedFile
		}
	}
	if !match {
		return nil, fmt.Errorf("recorded file identity or metadata mismatch")
	}
	fd, err := unix.Openat(parent.fd, entry.name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), entry.name)
	verify := func() error {
		var current unix.Stat_t
		if err := unix.Fstat(fd, &current); err != nil {
			return err
		}
		if fingerprintOf(current) != fingerprintOf(entry.stat) || current.Uid != entry.stat.Uid || current.Gid != entry.stat.Gid || current.Nlink != entry.stat.Nlink {
			return fmt.Errorf("file changed during recovery observation")
		}
		return entry.revalidate()
	}
	if err := verify(); err != nil {
		return f, err
	}
	h := sha256.New()
	_, err = io.Copy(h, io.NewSectionReader(f, 0, entry.stat.Size))
	if checkpoint != nil {
		checkpoint("digest")
	}
	if hex.EncodeToString(h.Sum(nil)) != intent.Digest.Value {
		err = errors.Join(err, fmt.Errorf("recorded SHA-256 digest mismatch"))
	}
	return f, errors.Join(err, verify())
}
