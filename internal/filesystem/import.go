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
	writable           func(int) error
	statFile           func(int, *unix.Stat_t) error
	statConfig         func(string, *unix.Stat_t) error
	statState          func(string, *unix.Stat_t) error
	statLock           func(string, *unix.Stat_t) error
	statProvider       func(int, string, *unix.Stat_t, int) error
	directoryPath      func(int) (string, error)
	openInput          func(int, string, int, uint32) (int, error)
	nameRules          func(int) (nameRules, error)
	device             func(int) (uint64, error)
	registryCheckpoint func()
	checkpoint         func()
}

func nativeImportInspectionOps() importInspectionOps {
	return importInspectionOps{
		writable: importWritable, statFile: unix.Fstat, statConfig: unix.Stat, statState: unix.Stat,
		openInput: unix.Openat, nameRules: directoryNameRulesFD,
		statLock: unix.Stat, statProvider: unix.Fstatat, directoryPath: importDirectoryPath,
		device: func(fd int) (uint64, error) {
			var stat unix.Stat_t
			err := unix.Fstat(fd, &stat)
			return uint64(stat.Dev), err
		},
	}
}

// InspectImport only observes files, permissions and namespace evidence.
// Existing v1 registry contents and historical claims constrain admission.
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
	paths := &importPathInspection{plan: p, query: ops.directoryPath, paths: make(map[*directory]string)}
	defer func() { err = errors.Join(err, paths.revalidate()) }()
	root, missing, rootRoutes, err := p.walk(request.Root)
	if err != nil || len(missing) != 0 {
		return report, errors.Join(fmt.Errorf("selected import root must exist"), err)
	}
	physicalRootRoutes, err := paths.rootRoutes(request.Root, root.id)
	if err != nil {
		return report, err
	}
	inputParent, missing, inputRoutes, err := p.walk(parentPath(request.Input))
	if err != nil || len(missing) != 0 {
		return report, errors.Join(fmt.Errorf("input parent must be an accessible directory"), err)
	}
	sourceParent, missing, sourceRoutes, err := p.importParents(root, rootRoutes, relative)
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
			// Missing roots still reserve their projected namespace. Importing a
			// leaf over, above, or below one would change future source scanning.
			boundary, err := projectedDirectory(name)
			if err != nil {
				return report, fmt.Errorf("inspect missing configured source root: %w", err)
			}
			source, err := physicalDestination(request.Source)
			if err != nil {
				return report, err
			}
			if _, _, _, err := p.walk(parentPath(source)); err != nil {
				return report, err
			}
			if shared, err := paths.overlap(source, boundary); err != nil || shared {
				return report, errors.Join(fmt.Errorf("import source overlaps missing configured source root: %q", name), err)
			}
			continue
		}
		routes, err := paths.rootRoutes(name, other.id)
		if err != nil {
			return report, err
		}
		if other.id == root.id || importRouteContains(physicalRootRoutes, other.id) || importRouteContains(routes, root.id) {
			return report, fmt.Errorf("configured source roots overlap or alias")
		}
	}
	parts := strings.Split(relative, "/")
	existing := len(parts) - 1 - len(missing)
	for i := existing; i < len(parts)-1; i++ {
		report.MissingParents = append(report.MissingParents,
			strings.TrimRight(request.Root, "/")+"/"+strings.Join(parts[:i+1], "/"))
	}
	if _, _, _, err := p.walk(parentPath(o.Profile)); err != nil {
		return report, fmt.Errorf("inspect named configuration ancestry: %w", err)
	}
	var namedConfig unix.Stat_t
	if err := unix.Lstat(o.Profile, &namedConfig); err != nil {
		return report, err
	}
	for _, path := range []string{request.Input, request.Source} {
		if err := paths.protectedDirectory(path, parentPath(o.Registry)); err != nil {
			return report, err
		}
	}
	for _, source := range o.Sources {
		if err := paths.protectedDirectory(request.Input, source); err != nil {
			return report, err
		}
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
			for _, parent := range []string{parentPath(actual), parentPath(protected)} {
				if _, _, _, err := p.walk(parent); err != nil {
					return report, fmt.Errorf("inspect protected configuration ancestry: %w", err)
				}
			}
			shared, err := paths.overlap(actual, protected)
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
	var configuration unix.Stat_t
	if err := ops.statConfig(o.Profile, &configuration); err != nil {
		return report, fmt.Errorf("inspect active configuration identity: %w", err)
	}
	if idOf(observed) == idOf(configuration) {
		return report, fmt.Errorf("import input aliases active configuration")
	}
	checkConfiguration := func() error {
		var currentConfig, currentNamed unix.Stat_t
		if err := ops.statConfig(o.Profile, &currentConfig); err != nil || fingerprintOf(configuration) != fingerprintOf(currentConfig) {
			return errors.Join(fmt.Errorf("active configuration changed during inspection"), err)
		}
		if err := unix.Lstat(o.Profile, &currentNamed); err != nil || fingerprintOf(namedConfig) != fingerprintOf(currentNamed) {
			return errors.Join(fmt.Errorf("named configuration changed during inspection"), err)
		}
		return nil
	}
	defer func() { err = errors.Join(err, checkConfiguration()) }()
	checkStateIdentity := func() error {
		return errors.Join(
			importProtectedFile(o.Registry, "reserved ownership state", observed, ops.statState),
			importProtectedFile(importLockPath(o.Registry), "stable ownership lock", observed, ops.statLock))
	}
	if err := checkStateIdentity(); err != nil {
		return report, err
	}
	// Keep the reserved-object proof current on success and later failures.
	// Cleanup still closes the namespace after this metadata-only recheck.
	defer func() { err = errors.Join(err, checkStateIdentity()) }()
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
	// Inode deduplication is useful for alias comparisons, but permissions and
	// read-only mount flags belong to the selected route, not just its inode.
	sourceParentName := strings.TrimRight(request.Root, "/")
	if existing != 0 {
		sourceParentName += "/" + strings.Join(parts[:existing], "/")
	}
	for _, candidate := range []struct {
		name     string
		expected directoryID
	}{
		{sourceParentName, sourceParent.id}, {parentPath(request.Input), inputParent.id},
	} {
		if err := importCheckRoute(candidate.name, candidate.expected, input, ops); err != nil {
			return report, err
		}
	}

	prospective := -1
	for i, entry := range request.Entries {
		if !validAbsolute(entry.Path) || !validAbsolute(entry.Source) {
			return report, fmt.Errorf("invalid import collision evidence")
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
	checkProviders, err := p.importProviders(request, input, ops.statProvider)
	defer func() { err = errors.Join(err, checkProviders()) }()
	if err != nil {
		return report, err
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
	entry := preparedEntry{destination: Destination{Path: request.Input}, parent: inputParent, routes: inputRoutes, leaf: input}
	var blockerChecks []func() error
	for i, evidence := range request.Entries {
		if i == prospective {
			continue
		}
		otherParent, otherMissing, routes, err := p.walk(parentPath(evidence.Path))
		if err != nil {
			check, err := paths.blockedEvidence(entry, evidence.Path, request.Source, ops.nameRules)
			if err != nil {
				return report, err
			}
			if check != nil {
				blockerChecks = append(blockerChecks, check)
			}
			continue
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

	view, err := openImportRegistry(o, ops.registryCheckpoint)
	if err != nil {
		return report, err
	}
	defer func() { err = errors.Join(err, view.Close()) }()
	defer func() { err = errors.Join(err, view.revalidate()) }()
	if err := view.checkClaims(paths, []preparedEntry{entry, {destination: Destination{Path: request.Source}, parent: sourceParent, missing: missing, routes: sourceRoutes}}, ops.nameRules); err != nil {
		return report, err
	}
	if ops.checkpoint != nil {
		ops.checkpoint()
	}
	for _, route := range p.routes {
		if err := validateBinding(route); err != nil {
			return report, err
		}
	}
	for _, check := range blockerChecks {
		if err := check(); err != nil {
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

func (p *PreparedPlan) importProtectedDirectory(path, boundary string) error {
	actual, err := physicalDestination(path)
	if err != nil {
		return err
	}
	protected, missing, _, err := p.walk(boundary)
	if err != nil {
		return err
	}
	if _, _, _, err := p.walk(parentPath(actual)); err != nil {
		return err
	}
	if len(missing) == 0 {
		for name := parentPath(actual); ; name = parentPath(name) {
			if dir := p.paths[name]; dir != nil && dir.id == protected.id {
				return fmt.Errorf("import path occupies protected namespace: %q", path)
			}
			if name == "/" {
				break
			}
		}
	}
	projected, err := projectedDirectory(boundary)
	if err != nil {
		return err
	}
	if shared, err := p.importProjectedOverlap(actual, projected); err != nil || shared {
		return errors.Join(fmt.Errorf("import path overlaps protected namespace: %q", path), err)
	}
	return nil
}

// A nontraversable provider may be ignored only after its first obstruction
// is proved unrelated to the selected input and its pinned ancestry.

func (s *importPathInspection) blockedEvidence(entry preparedEntry, path, source string, rules func(int) (nameRules, error)) (func() error, error) {
	p := s.plan
	parent, routes := p.root, []binding(nil)
	prefix := ""
	for _, name := range strings.Split(strings.TrimPrefix(parentPath(path), "/"), "/") {
		if name == "" || name == "." {
			continue
		}
		prefix += "/" + name
		next, missing, observed, walkErr := p.walk(prefix)
		if walkErr == nil {
			if len(missing) != 0 {
				return nil, nil
			}
			parent, routes = next, observed
			continue
		}
		if !errors.Is(walkErr, unix.ENOENT) && !errors.Is(walkErr, unix.ENOTDIR) {
			return nil, walkErr
		}
		claim := ownershipRecord{Destination: pathBytes(prefix), Parent: idOfDirectory(parent), Routes: recordRoutes(routes)}
		if shared, err := recordedClaimOverlap(p, entry, claim, rules); err != nil || shared {
			return nil, errors.Join(fmt.Errorf("import destination overlaps blocked provider: %q", path), err)
		}
		var named, referent unix.Stat_t
		if err := unix.Fstatat(parent.fd, name, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return nil, err
		}
		refErr := unix.Fstatat(parent.fd, name, &referent, 0)
		missingReferent := errors.Is(refErr, unix.ENOENT)
		var frontierParent *directory
		var frontierName string
		if missingReferent {
			text, err := readlinkAt(parent.fd, name)
			if err != nil {
				return nil, err
			}
			if !filepath.IsAbs(text) {
				text = parentPath(prefix) + "/" + text
			}
			var tail []string
			frontierParent, tail, _, err = p.walk(parentPath(text))
			if err != nil {
				return nil, err
			}
			frontierName = filepath.Base(text)
			if len(tail) != 0 {
				frontierName = tail[0]
			}
			var absent unix.Stat_t
			if err := unix.Fstatat(frontierParent.fd, frontierName, &absent, unix.AT_SYMLINK_NOFOLLOW); !errors.Is(err, unix.ENOENT) {
				return nil, errors.Join(fmt.Errorf("cannot establish dangling referent frontier"), err)
			}
			projected, err := projectedDirectory(text)
			if err != nil {
				return nil, err
			}
			actualSource, err := physicalDestination(source)
			if err != nil {
				return nil, err
			}
			if _, _, _, err := p.walk(parentPath(projected)); err != nil {
				return nil, err
			}
			if _, _, _, err := p.walk(parentPath(actualSource)); err != nil {
				return nil, err
			}
			if shared, err := s.overlap(projected, actualSource); err != nil || shared {
				return nil, errors.Join(fmt.Errorf("blocked provider depends on import source parents: %q", path), err)
			}
		} else if refErr != nil || referent.Mode&unix.S_IFMT == unix.S_IFDIR || idOf(referent) == idOf(entry.leaf) {
			return nil, errors.Join(fmt.Errorf("cannot exclude import overlap with blocked provider: %q", path), walkErr, refErr)
		}
		return func() error {
			var current, currentRef unix.Stat_t
			err := unix.Fstatat(parent.fd, name, &current, unix.AT_SYMLINK_NOFOLLOW)
			if err != nil || fingerprintOf(current) != fingerprintOf(named) {
				return errors.Join(fmt.Errorf("provider obstruction changed during inspection: %q", path), err)
			}
			err = unix.Fstatat(parent.fd, name, &currentRef, 0)
			if missingReferent && errors.Is(err, unix.ENOENT) {
				err := unix.Fstatat(frontierParent.fd, frontierName, &currentRef, unix.AT_SYMLINK_NOFOLLOW)
				if errors.Is(err, unix.ENOENT) {
					return nil
				}
				return errors.Join(fmt.Errorf("dangling referent frontier changed during inspection: %q", path), err)
			}
			if !missingReferent && err == nil && fingerprintOf(currentRef) == fingerprintOf(referent) {
				return nil
			}
			return errors.Join(fmt.Errorf("provider referent changed during inspection: %q", path), err)
		}, nil
	}
	return nil, fmt.Errorf("provider ancestry changed during inspection: %q", path)
}

// Compare projected tails beneath every shared pinned directory identity.
// Different bind-mount paths can describe the same future filesystem slot.
func (p *PreparedPlan) importProjectedOverlap(left, right string) (bool, error) {
	for a := parentPath(left); ; a = parentPath(a) {
		leftDir := p.paths[a]
		if a == "/" {
			leftDir = p.root
		}
		if leftDir != nil {
			for b := parentPath(right); ; b = parentPath(b) {
				rightDir := p.paths[b]
				if b == "/" {
					rightDir = p.root
				}
				if rightDir != nil && leftDir.id == rightDir.id {
					x := strings.TrimPrefix(left, strings.TrimRight(a, "/")+"/")
					y := strings.TrimPrefix(right, strings.TrimRight(b, "/")+"/")
					same, err := compareDestinationNames("/"+x, "/"+y, func(name string) (nameRules, error) {
						return p.rulesAt(strings.TrimRight(a, "/") + name)
					})
					if err != nil {
						return false, err
					}
					if same {
						// A nested mount can give shared ancestors different
						// descendant namespaces. Prefer observed lower bindings.
						xParts, yParts := strings.Split(x, "/"), strings.Split(y, "/")
						leftPath, rightPath := a, b
						for i := 0; i < min(len(xParts), len(yParts)); i++ {
							leftPath = filepath.Join(leftPath, xParts[i])
							rightPath = filepath.Join(rightPath, yParts[i])
							lowerLeft, lowerRight := p.paths[leftPath], p.paths[rightPath]
							if lowerLeft != nil && lowerRight != nil && lowerLeft.id != lowerRight.id {
								same = false
								break
							}
						}
						if same {
							return true, nil
						}
					}
				}
				if b == "/" {
					break
				}
			}
		}
		if a == "/" {
			return false, nil
		}
	}
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

// Reopen the selected named route without inode deduplication. Distinct bind
// mounts of the same directory can have different read-only mount flags.
func importCheckRoute(name string, expected directoryID, input unix.Stat_t, ops importInspectionOps) (err error) {
	fd, err := unix.Open(name, directoryFlags, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, unix.Close(fd)) }()
	var current unix.Stat_t
	if err := unix.Fstat(fd, &current); err != nil {
		return err
	}
	if identity(current) != expected {
		return fmt.Errorf("import directory route changed")
	}
	if err := ops.writable(fd); err != nil {
		return fmt.Errorf("import directory is not writable and searchable: %w", err)
	}
	device, err := ops.device(fd)
	if err != nil {
		return fmt.Errorf("inspect import filesystem device: %w", err)
	}
	if uint64(input.Dev) != device {
		return fmt.Errorf("cross-filesystem import is unsupported")
	}
	return nil
}
