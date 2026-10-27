package filesystem

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Additional observations are local to import inspection. Named routes and M3
// operation paths remain unchanged. Backing paths are never mutation authority.
type importPathInspection struct {
	plan  *PreparedPlan
	query func(int) (string, error)
	paths map[*directory]string
	order []*directory
}

func (s *importPathInspection) project(name string, directory bool) (string, error) {
	parent, leaf := name, ""
	if !directory {
		parent, leaf = parentPath(name), filepath.Base(name)
	}
	dir, missing, _, err := s.plan.walk(parent)
	if err != nil {
		return "", err
	}
	path, ok := s.paths[dir]
	if !ok {
		path, err = s.query(dir.fd)
		if err != nil {
			return "", fmt.Errorf("query native import directory path: %w", err)
		}
		if path != "" {
			if !validAbsolute(path) || filepath.Clean(path) != path {
				return "", fmt.Errorf("invalid native import directory path")
			}
			observed, tail, _, err := s.plan.walk(path)
			if err != nil || len(tail) != 0 || observed.id != dir.id {
				return "", errors.Join(fmt.Errorf("native import directory path identity mismatch"), err)
			}
		}
		s.paths[dir] = path
		s.order = append(s.order, dir)
	}
	if path == "" {
		if directory {
			return projectedDirectory(name)
		}
		return physicalDestination(name)
	}
	for _, part := range missing {
		path = filepath.Join(path, part)
	}
	if leaf != "" {
		path = filepath.Join(path, leaf)
	}
	return path, nil
}

func (s *importPathInspection) revalidate() error {
	for _, dir := range s.order {
		path, err := s.query(dir.fd)
		if err != nil || path != s.paths[dir] {
			return errors.Join(fmt.Errorf("native import directory path changed"), err)
		}
		if path != "" {
			var current unix.Stat_t
			if err := unix.Stat(path, &current); err != nil || identity(current) != dir.id {
				return errors.Join(fmt.Errorf("native import directory binding changed"), err)
			}
		}
	}
	for _, route := range s.plan.routes {
		if err := validateBinding(route); err != nil {
			return err
		}
	}
	return nil
}

func (s *importPathInspection) overlap(left, right string) (bool, error) {
	a, err := s.project(left, false)
	if err != nil {
		return false, err
	}
	b, err := s.project(right, false)
	if err != nil {
		return false, err
	}
	return s.plan.importProjectedOverlap(a, b)
}

func (s *importPathInspection) protectedDirectory(path, boundary string) error {
	actual, err := s.project(path, false)
	if err != nil {
		return err
	}
	protected, err := s.project(boundary, true)
	if err != nil {
		return err
	}
	return s.plan.importProtectedDirectory(actual, protected)
}

func (s *importPathInspection) rootRoutes(name string, expected directoryID) ([]binding, error) {
	actual, err := s.project(name, true)
	if err != nil {
		return nil, err
	}
	return s.plan.importRootRoutes(actual, expected)
}

// Inspect source providers independently of destination reachability. Symlink
// leaves remain opaque; regular-file identity includes losing candidates.
func (p *PreparedPlan) importProviders(request ImportRequest, input unix.Stat_t, stat func(int, string, *unix.Stat_t, int) error) (func() error, error) {
	var checks []func() error
	recheck := func() error {
		var result error
		for _, check := range checks {
			result = errors.Join(result, check())
		}
		return result
	}
	seen := make(map[string]bool)
	for _, entry := range request.Entries {
		if entry.Directory || (entry.Path == request.Input && entry.Source == request.Source) || seen[entry.Source] {
			continue
		}
		seen[entry.Source] = true
		var rootName, relative string
		for _, root := range request.Ownership.Sources {
			if rel, err := importRelative(root, entry.Source); err == nil {
				if rootName != "" {
					return recheck, fmt.Errorf("ambiguous source provider root: %q", entry.Source)
				}
				rootName, relative = root, rel
			}
		}
		if rootName == "" {
			return recheck, fmt.Errorf("source provider is outside configured roots: %q", entry.Source)
		}
		root, missing, routes, err := p.walk(rootName)
		if err != nil || len(missing) != 0 {
			return recheck, errors.Join(fmt.Errorf("source provider root is unavailable"), err)
		}
		parent, missing, _, err := p.importParents(root, routes, relative)
		if err != nil || len(missing) != 0 {
			return recheck, errors.Join(fmt.Errorf("source provider parent is unavailable"), err)
		}
		name := filepath.Base(entry.Source)
		var before unix.Stat_t
		if err := stat(parent.fd, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return recheck, fmt.Errorf("inspect source provider: %w", err)
		}
		kind := before.Mode & unix.S_IFMT
		if kind != unix.S_IFREG && kind != unix.S_IFLNK {
			return recheck, fmt.Errorf("source provider changed type: %q", entry.Source)
		}
		if kind == unix.S_IFREG && idOf(input) == idOf(before) {
			return recheck, fmt.Errorf("import input aliases existing source provider: %q", entry.Source)
		}
		checks = append(checks, func() error {
			var after unix.Stat_t
			if err := stat(parent.fd, name, &after, unix.AT_SYMLINK_NOFOLLOW); err != nil || fingerprintOf(before) != fingerprintOf(after) {
				return errors.Join(fmt.Errorf("source provider changed during inspection: %q", entry.Source), err)
			}
			return nil
		})
	}
	return recheck, nil
}

// The registry and stable lock are known protected objects, even through a
// file bind alias. Absence is allowed; unavailable evidence is not absence.
func importProtectedFile(path, label string, input unix.Stat_t, stat func(string, *unix.Stat_t) error) error {
	var object unix.Stat_t
	if err := stat(path, &object); err == nil {
		if idOf(input) == idOf(object) {
			return fmt.Errorf("import input aliases %s", label)
		}
	} else if !errors.Is(err, unix.ENOENT) {
		return fmt.Errorf("inspect %s identity: %w", label, err)
	}
	return nil
}

// Preserve raw parent traversal in locators; the registry basename is fixed by
// the ownership API, while the stable lock is always its sibling.
func importLockPath(registry string) string {
	return strings.TrimRight(parentPath(registry), "/") + "/ownership.lock"
}
