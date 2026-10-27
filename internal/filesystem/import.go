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

// ImportRequest contains resolver-selected values. Entries must include the
// prospective leaf, all existing nonexcluded providers (including losers), and
// required directories. Input is the selected destination, not a cleaned alias.
type ImportRequest struct {
	Input, Source, Root string
	Ownership           Ownership
	Entries             []Destination
}

// ImportInspection reports point-in-time observations only. It retains no
// handles or mutation authority. MissingParents are source parents, root first.
type ImportInspection struct {
	MissingParents []string
}

type importInspectionOps struct {
	writable   func(int) error
	statFile   func(int, *unix.Stat_t) error
	openInput  func(int, string, int, uint32) (int, error)
	nameRules  func(int) (nameRules, error)
	device     func(int) (uint64, error)
	checkpoint func()
}

func nativeImportInspectionOps() importInspectionOps {
	return importInspectionOps{
		writable: importWritable, statFile: unix.Fstat,
		openInput: unix.Openat, nameRules: directoryNameRulesFD,
		device: func(fd int) (uint64, error) {
			var stat unix.Stat_t
			err := unix.Fstat(fd, &stat)
			return uint64(stat.Dev), err
		},
	}
}

// InspectImport only observes files, permissions and namespace evidence.
// Registry contents and historical ownership claims are not inspected yet.
// It never locks, initializes, synchronizes, recovers or changes paths.
// Execution must repeat authoritative checks under the stable registry lock.
// Successful permission/device checks do not guarantee a later rename can work.
func InspectImport(request ImportRequest) (ImportInspection, error) {
	return inspectImport(request, nativeImportInspectionOps())
}

func inspectImport(request ImportRequest, ops importInspectionOps) (report ImportInspection, err error) {
	request.Entries = append([]Destination(nil), request.Entries...)
	sort.SliceStable(request.Entries, func(i, j int) bool {
		a, b := request.Entries[i], request.Entries[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Directory && !b.Directory
	})
	request.Ownership.Sources = append([]string(nil), request.Ownership.Sources...)
	o := request.Ownership
	relative, err := importRelative(request.Root, request.Source)
	if err != nil {
		return report, err
	}
	if _, err := importRelative(o.Target, request.Input); err != nil {
		return report, err
	}
	configured := 0
	for _, root := range o.Sources {
		if root == request.Root {
			configured++
		}
	}
	if configured != 1 {
		return report, fmt.Errorf("import root must select exactly one configured root")
	}
	if err := o.validate(); err != nil {
		return report, err
	}
	if err := o.protect(request.Input); err != nil {
		return report, err
	}

	p, err := namespace()
	if err != nil {
		return report, err
	}
	defer func() { err = errors.Join(err, p.Close()) }()
	root, missing, rootRoutes, err := p.walk(request.Root)
	if err != nil || len(missing) != 0 {
		return report, errors.Join(fmt.Errorf("selected import root must exist"), err)
	}
	physicalRootRoutes, err := p.importRootRoutes(request.Root, root.id)
	if err != nil {
		return report, err
	}
	for _, name := range o.Sources {
		if name == request.Root {
			continue
		}
		other, missing, _, err := p.walk(name)
		if err != nil {
			return report, fmt.Errorf("inspect configured source root: %w", err)
		}
		if len(missing) != 0 {
			continue
		}
		routes, err := p.importRootRoutes(name, other.id)
		if err != nil {
			return report, err
		}
		if other.id == root.id || importRouteContains(physicalRootRoutes, other.id) || importRouteContains(routes, root.id) {
			return report, fmt.Errorf("configured source roots overlap or alias")
		}
	}
	inputParent, missing, inputRoutes, err := p.walk(parentPath(request.Input))
	if err != nil || len(missing) != 0 {
		return report, errors.Join(fmt.Errorf("input parent must be an accessible directory"), err)
	}
	sourceParent, missing, _, err := p.importParents(root, rootRoutes, relative)
	if err != nil {
		return report, err
	}
	parts := strings.Split(relative, "/")
	existing := len(parts) - 1 - len(missing)
	for i := existing; i < len(parts)-1; i++ {
		report.MissingParents = append(report.MissingParents,
			strings.TrimRight(request.Root, "/")+"/"+strings.Join(parts[:i+1], "/"))
	}
	for _, path := range []string{request.Input, request.Source} {
		config := o.Profile
		named, err := physicalDestination(config)
		if err != nil {
			return report, fmt.Errorf("inspect active configuration: %w", err)
		}
		referent, err := filepath.EvalSymlinks(config)
		if err != nil {
			return report, fmt.Errorf("resolve active configuration: %w", err)
		}
		for _, protected := range []string{named, referent} {
			actual, err := physicalDestination(path)
			if err != nil {
				return report, err
			}
			shared, err := overlap(actual, protected)
			if err != nil || shared {
				return report, errors.Join(fmt.Errorf("import path overlaps active configuration: %q", path), err)
			}
		}
	}

	absent := func() error {
		name := filepath.Base(request.Source)
		if len(missing) != 0 {
			name = missing[0]
		}
		var stat unix.Stat_t
		if err := unix.Fstatat(sourceParent.fd, name, &stat, unix.AT_SYMLINK_NOFOLLOW); !errors.Is(err, unix.ENOENT) {
			return errors.Join(fmt.Errorf("source entry or missing parent is occupied: %q", request.Source), err)
		}
		return nil
	}
	if err := absent(); err != nil {
		return report, err
	}
	var observed unix.Stat_t
	if err := unix.Fstatat(inputParent.fd, filepath.Base(request.Input), &observed, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return report, fmt.Errorf("inspect import input: %w", err)
	}
	if !importRegular(observed) {
		return report, fmt.Errorf("import input must be a user-owned regular file with one link and no privileged bits")
	}
	fd, err := ops.openInput(inputParent.fd, filepath.Base(request.Input),
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return report, fmt.Errorf("open regular import input without following symlinks: %w", err)
	}
	defer func() { err = errors.Join(err, unix.Close(fd)) }()
	var input unix.Stat_t
	if err := ops.statFile(fd, &input); err != nil {
		return report, err
	}
	if !importRegular(input) {
		return report, fmt.Errorf("import input must be a user-owned regular file with one link and no privileged bits")
	}
	if fingerprintOf(observed) != fingerprintOf(input) || observed.Uid != input.Uid ||
		observed.Gid != input.Gid || observed.Nlink != input.Nlink {
		return report, fmt.Errorf("input changed while opening")
	}
	for _, dir := range []*directory{sourceParent, inputParent} {
		if err := ops.writable(dir.fd); err != nil {
			return report, fmt.Errorf("import directory is not writable and searchable: %w", err)
		}
	}
	for _, dir := range []*directory{sourceParent, inputParent} {
		device, err := ops.device(dir.fd)
		if err != nil {
			return report, fmt.Errorf("inspect import filesystem device: %w", err)
		}
		if uint64(input.Dev) != device {
			return report, fmt.Errorf("cross-filesystem import is unsupported")
		}
	}
	prospective := -1
	for i, entry := range request.Entries {
		if !validAbsolute(entry.Path) || !validAbsolute(entry.Source) {
			return report, fmt.Errorf("invalid import collision evidence")
		}
		if _, err := physicalDestination(entry.Path); err != nil {
			return report, fmt.Errorf("inspect import collision evidence: %w", err)
		}
		if !entry.Directory && entry.Path == request.Input && entry.Source == request.Source {
			if prospective >= 0 {
				return report, fmt.Errorf("duplicate prospective import mapping")
			}
			prospective = i
		}
	}
	if prospective < 0 {
		return report, fmt.Errorf("missing prospective import mapping")
	}
	rulesAt := func(path string) (nameRules, error) {
		dir, _, _, err := p.walk(path)
		if err != nil {
			return nameRules{}, err
		}
		return ops.nameRules(dir.fd)
	}
	if reason := collisionReasons(request.Entries, rulesAt)[prospective]; reason != "" {
		return report, fmt.Errorf("import collision: %s", reason)
	}
	entry := preparedEntry{destination: Destination{Path: request.Input}, parent: inputParent, routes: inputRoutes}
	for i, evidence := range request.Entries {
		if i == prospective {
			continue
		}
		otherParent, otherMissing, routes, err := p.walk(parentPath(evidence.Path))
		if err != nil {
			return report, err
		}
		if len(otherMissing) != 0 {
			continue
		}
		claim := ownershipRecord{Destination: pathBytes(evidence.Path), Parent: idOfDirectory(otherParent), Routes: recordRoutes(routes)}
		if evidence.Directory {
			inputEntry, err := canonicalEntry(request.Input)
			if err != nil {
				return report, err
			}
			otherEntry, err := canonicalEntry(evidence.Path)
			if err != nil {
				return report, err
			}
			same, err := compareEntries(inputEntry, otherEntry, rulesAt)
			if err != nil || same {
				return report, errors.Join(fmt.Errorf("import destination overlaps required directory"), err)
			}
		} else if shared, err := recordedClaimOverlap(p, entry, claim, ops.nameRules); err != nil || shared {
			return report, errors.Join(fmt.Errorf("import destination overlaps existing provider"), err)
		}
	}

	if ops.checkpoint != nil {
		ops.checkpoint()
	}
	for _, route := range p.routes {
		if err := validateBinding(route); err != nil {
			return report, err
		}
	}
	var current unix.Stat_t
	if err := unix.Fstatat(inputParent.fd, filepath.Base(request.Input), &current, unix.AT_SYMLINK_NOFOLLOW); err != nil ||
		fingerprintOf(input) != fingerprintOf(current) || input.Uid != current.Uid || input.Gid != current.Gid ||
		input.Nlink != current.Nlink {
		return report, errors.Join(fmt.Errorf("input changed during inspection"), err)
	}
	if err := absent(); err != nil {
		return report, err
	}
	return report, nil
}

func importRelative(root, name string) (string, error) {
	if !validAbsolute(root) || !validAbsolute(name) {
		return "", fmt.Errorf("import paths must be absolute")
	}
	prefix := strings.TrimRight(root, "/") + "/"
	if !strings.HasPrefix(name, prefix) {
		return "", fmt.Errorf("import path must belong to its selected root")
	}
	relative := strings.TrimPrefix(name, prefix)
	for _, component := range strings.Split(relative, "/") {
		if !validBasename(component) {
			return "", fmt.Errorf("import relative path must contain literal file names")
		}
	}
	return relative, nil
}

func importRouteContains(routes []binding, id directoryID) bool {
	for _, route := range routes {
		if route.target.id == id {
			return true
		}
	}
	return false
}

// Compare final resolved directory ancestry, not transient alias/.. traversal.
// Preserve the original named bindings separately for revalidation.
func (p *PreparedPlan) importRootRoutes(name string, expected directoryID) ([]binding, error) {
	actual, err := filepath.EvalSymlinks(name)
	if err != nil {
		return nil, err
	}
	dir, missing, routes, err := p.walk(actual)
	if err != nil || len(missing) != 0 || dir.id != expected {
		return nil, errors.Join(fmt.Errorf("source root changed during ancestry inspection"), err)
	}
	return routes, nil
}

func importWritable(fd int) error {
	if err := unix.Faccessat(fd, ".", unix.W_OK|unix.X_OK, unix.AT_EACCESS); err != nil {
		return err
	}
	var volume unix.Statfs_t
	if err := unix.Fstatfs(fd, &volume); err != nil {
		return err
	}
	if uint64(volume.Flags)&uint64(importReadOnlyFlag) != 0 {
		return unix.EROFS
	}
	return nil
}

func importRegular(s unix.Stat_t) bool {
	return s.Mode&unix.S_IFMT == unix.S_IFREG && s.Uid == uint32(os.Geteuid()) &&
		s.Nlink == 1 && s.Mode&0o7000 == 0
}

// Start at the pinned configured root. Source symlinks below it stay opaque;
// unlike destination ancestors, they must never be followed during inspection.
func (p *PreparedPlan) importParents(root *directory, rootRoutes []binding, relative string) (*directory, []string, []binding, error) {
	current := root
	routes := append([]binding(nil), rootRoutes...)
	parts := strings.Split(relative, "/")
	for i, name := range parts[:len(parts)-1] {
		var s unix.Stat_t
		err := unix.Fstatat(current.fd, name, &s, unix.AT_SYMLINK_NOFOLLOW)
		if errors.Is(err, unix.ENOENT) {
			return current, parts[i : len(parts)-1], routes, nil
		}
		if err != nil {
			return nil, nil, nil, err
		}
		if s.Mode&unix.S_IFMT != unix.S_IFDIR {
			return nil, nil, nil, fmt.Errorf("source parent must be a directory, not a file or symlink: %q", name)
		}
		if err := importExactParent(current.fd, name); err != nil {
			return nil, nil, nil, err
		}
		next, err := p.openDirectory(current.fd, name, unix.O_NOFOLLOW)
		if err != nil {
			return nil, nil, nil, err
		}
		route := binding{parent: current, name: name, entry: s, target: next}
		if err := validateBinding(route); err != nil {
			return nil, nil, nil, err
		}
		p.routes = append(p.routes, route)
		routes = append(routes, route)
		current = next
	}
	return current, nil, routes, nil
}

// Enumeration determines the spelling the resolver will later see. Native
// lookup alone may accept a case/normalization alias with a different mapping.
func importExactParent(parent int, name string) error {
	fd, err := unix.Openat(parent, ".", directoryFlags|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), "source parent")
	names, readErr := f.Readdirnames(-1)
	if err := errors.Join(readErr, f.Close()); err != nil {
		return err
	}
	for _, actual := range names {
		if actual == name {
			return nil
		}
	}
	return fmt.Errorf("source parent spelling differs from import path: %q", name)
}
