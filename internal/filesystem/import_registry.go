package filesystem

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/unix"
)

// Observe existing state without acquiring the lock. Atomic snapshots permit
// previews during cooperative writers, but observations never authorize writes.
type importRegistryInspection struct {
	registry *registry
	entries  []importStateEntry
	lock     int
}

type importStateEntry struct {
	parent *directory
	name   string
	stat   unix.Stat_t
	exists bool
}

func observeImportState(parent *directory, name string) (importStateEntry, error) {
	result := importStateEntry{parent: parent, name: name}
	err := unix.Fstatat(parent.fd, name, &result.stat, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.exists = true
	return result, nil
}

func (s importStateEntry) revalidate() error {
	current, err := observeImportState(s.parent, s.name)
	if err != nil {
		return fmt.Errorf("inspect ownership state binding %q: %w", s.name, err)
	}
	if current.exists != s.exists || (s.exists && (fingerprintOf(current.stat) != fingerprintOf(s.stat) || current.stat.Uid != s.stat.Uid || current.stat.Gid != s.stat.Gid || current.stat.Nlink != s.stat.Nlink)) {
		return fmt.Errorf("ownership state binding changed during import inspection: %q", s.name)
	}
	return nil
}

func openImportRegistry(options Ownership, checkpoint func()) (_ *importRegistryInspection, err error) {
	r, err := openRegistry(options, false, nativeRegistryOps())
	if err != nil {
		return nil, err
	}
	return inspectImportRegistry(r, checkpoint)
}

func inspectImportRegistry(r *registry, checkpoint func()) (_ *importRegistryInspection, err error) {
	view := &importRegistryInspection{registry: r, lock: -1}
	defer func() {
		if err != nil {
			err = errors.Join(err, view.revalidate(), view.Close())
		}
	}()
	if err := view.observe(checkpoint); err != nil {
		return nil, err
	}
	return view, nil
}

// The caller owns the registry and any held exclusive lock. Bootstrap refreshes
// obsolete absence observations here; ordinary inspection never initializes it.
func (view *importRegistryInspection) observe(checkpoint func()) error {
	r := view.registry
	parent, missing, _, err := r.space.walk(parentPath(r.options.Registry))
	if err != nil {
		return err
	}
	if r.dir == nil {
		if len(missing) == 0 {
			return fmt.Errorf("ownership directory appeared during import inspection")
		}
		frontier, err := observeImportState(parent, missing[0])
		if err != nil {
			return err
		}
		if frontier.exists {
			return fmt.Errorf("ownership directory frontier appeared during import inspection")
		}
		view.entries = append(view.entries, frontier)
	} else {
		if len(missing) != 0 || parent.id != r.dir.id {
			return fmt.Errorf("ownership directory changed during import inspection")
		}
		state, err := observeImportState(parent, "ownership.json")
		if err != nil {
			return err
		}
		lock, err := observeImportState(parent, "ownership.lock")
		if err != nil {
			return err
		}
		view.entries = append(view.entries, state, lock)
		if state.exists && !lock.exists {
			return fmt.Errorf("ownership registry exists without its stable lock; manual recovery required")
		}
		if lock.exists {
			fd, err := unix.Openat(parent.fd, "ownership.lock", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			if err != nil {
				return fmt.Errorf("inspect stable ownership lock: %w", err)
			}
			view.lock = fd
			if err := privateFD(fd, false); err != nil {
				return err
			}
			var held unix.Stat_t
			if err := unix.Fstat(fd, &held); err != nil {
				return err
			}
			if fingerprintOf(held) != fingerprintOf(lock.stat) {
				return fmt.Errorf("stable ownership lock changed during import inspection")
			}
		}
		if checkpoint != nil {
			checkpoint()
		}
		// The first load reuses all M3 validation. Reload between named observations
		// so the claim data belongs to the snapshot whose binding is rechecked.
		r.data = snapshot{Version: 1, Records: []ownershipRecord{}}
		if err := r.load(); err != nil {
			return err
		}
	}
	return view.revalidate()
}

func (v *importRegistryInspection) revalidate() error {
	var result error
	for _, entry := range v.entries {
		result = errors.Join(result, entry.revalidate())
	}
	if v.registry.dir != nil {
		result = errors.Join(result, privateFD(v.registry.dir.fd, true))
	}
	if v.lock >= 0 {
		result = errors.Join(result, privateFD(v.lock, false))
	}
	return errors.Join(result, v.registry.validate())
}

func (v *importRegistryInspection) Close() error {
	var err error
	if v.lock >= 0 {
		err = unix.Close(v.lock)
		v.lock = -1
	}
	return errors.Join(err, v.registry.Close())
}

func (v *importRegistryInspection) checkClaims(paths *importPathInspection, entries []preparedEntry, rules func(int) (nameRules, error)) error {
	if err := v.registry.allowMutation(); err != nil {
		return err
	}
	if v.registry.data.Unfinished {
		return fmt.Errorf("unfinished ownership transaction blocks import; read-only inspection performs no recovery")
	}
	for _, record := range v.registry.data.Records {
		if record.Phase == "creating" || record.Phase == "removing" {
			return fmt.Errorf("unresolved %s ownership operation blocks import: %q", record.Phase, record.Destination)
		}
	}
	for _, entry := range entries {
		// Include verified backing ancestry without replacing original locators or
		// visiting historical paths. Missing parents retain their literal suffix.
		projected, err := paths.project(entry.destination.Path, false)
		if err != nil {
			return err
		}
		parent, missing, routes, err := paths.plan.walk(parentPath(projected))
		if err != nil {
			return err
		}
		if parent.id != entry.parent.id || strings.Join(missing, "/") != strings.Join(entry.missing, "/") {
			return fmt.Errorf("import claim ancestry changed during inspection")
		}
		entry.routes = append(append([]binding(nil), entry.routes...), routes...)
		for _, record := range v.registry.data.Records {
			// A literal ancestor/descendant reservation is relevant even if historical
			// route evidence is unavailable. Never clean symlink/.. locators here.
			claimed := string(record.Destination)
			literal := strings.HasPrefix(claimed, strings.TrimRight(entry.destination.Path, "/")+"/") || strings.HasPrefix(entry.destination.Path, strings.TrimRight(claimed, "/")+"/")
			shared, err := recordedClaimOverlap(paths.plan, entry, record, rules)
			if err != nil || shared || literal {
				return errors.Join(fmt.Errorf("import path overlaps recorded ownership claim: %q", record.Destination), err)
			}
		}
	}
	return nil
}
