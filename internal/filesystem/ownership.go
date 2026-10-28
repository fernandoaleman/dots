package filesystem

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/sys/unix"
)

func projectedDirectory(path string) (string, error) {
	name, err := physicalDestination(strings.TrimRight(path, "/") + "/.dots-boundary")
	return filepath.Dir(name), err
}

// A vanished or broken source tree is not a removal prerequisite. Protect its
// last resolvable named boundary without following a missing referent. Actual
// access/inspection failures still block safety comparisons.
func sourceBoundary(path string) (string, error) {
	root, err := projectedDirectory(path)
	if err == nil {
		return root, nil
	}
	if !missingReferent(err) {
		return "", err
	}
	for {
		root, err = physicalDestination(path)
		if err == nil {
			return root, nil
		}
		if !missingReferent(err) || parentPath(path) == path {
			return "", err
		}
		path = parentPath(path)
	}
}

func overlap(left, right string) (bool, error) {
	if left == "/" || right == "/" {
		return true, nil
	}
	return compareDestinationNames(left, right, directoryNameRules)
}

func (o Ownership) validate() error {
	if !validAbsolute(o.Registry) || filepath.Base(o.Registry) != "ownership.json" ||
		!validAbsolute(o.Profile) || !validAbsolute(o.Target) {
		return fmt.Errorf("invalid ownership registry, configuration, or target locator")
	}
	state, err := projectedDirectory(parentPath(o.Registry))
	if err != nil {
		return fmt.Errorf("resolve ownership state: %w", err)
	}
	for _, source := range o.Sources {
		if !validAbsolute(source) {
			return fmt.Errorf("invalid source root")
		}
		root, err := sourceBoundary(source)
		if err != nil {
			return fmt.Errorf("resolve source/state safety: %w", err)
		}
		shared, err := overlap(state, root)
		if err != nil {
			return err
		}
		if shared {
			return fmt.Errorf("ownership state and source root overlap: %q", source)
		}
	}
	return nil
}

func (o Ownership) protect(destination string) error {
	path, err := physicalDestination(destination)
	if err != nil {
		return err
	}
	state, err := projectedDirectory(parentPath(o.Registry))
	if err != nil {
		return err
	}
	shared, err := overlap(path, state)
	if err != nil {
		return err
	}
	if shared {
		return fmt.Errorf("destination overlaps reserved ownership state: %q", destination)
	}
	for _, source := range o.Sources {
		root, err := sourceBoundary(source)
		if err != nil {
			return fmt.Errorf("resolve source safety: %w", err)
		}
		shared, err := overlap(path, root)
		if err != nil {
			return err
		}
		if shared {
			return fmt.Errorf("destination overlaps source contents: %q", destination)
		}
		// Also protect a configured root symlink's own named entry. Following
		// it protects its contents but would miss deletion of the root alias.
		named, err := physicalDestination(strings.TrimRight(source, "/"))
		if err == nil {
			shared, err = overlap(path, named)
			if err != nil {
				return err
			}
			if shared {
				return fmt.Errorf("destination overlaps configured source root entry: %q", destination)
			}
		} else if !missingReferent(err) {
			return err
		}
	}
	return nil
}

// InstallOwned installs a resolver-selected plan and records only links created
// by this invocation. Legacy correct links never acquire ownership.
func InstallOwned(destinations []Destination, options Ownership) (Report, error) {
	return installOwned(destinations, options, nativeRegistryOps(), nativeInstallOps())
}

// RevalidateOwnership records authority changes after a failed installation
// preflight. It never changes destinations and never initializes absent state.
// A matching target for an existing link cannot promote untracked ownership.
func RevalidateOwnership(options Ownership) (warnings []string, err error) {
	probe, err := openRegistry(options, false, nativeRegistryOps())
	if err != nil {
		return nil, err
	}
	if err := probe.allowMutation(); err != nil {
		return nil, errors.Join(err, probe.Close())
	}
	history := len(probe.data.Records) != 0
	if err := probe.Close(); err != nil {
		return nil, err
	}
	if !history {
		return nil, nil
	}
	r, err := openRegistry(options, true, nativeRegistryOps())
	if err != nil {
		return nil, err
	}
	defer func() {
		if r.warning != "" {
			warnings = append(warnings, r.warning)
		}
		err = errors.Join(err, r.Close())
	}()
	if err := r.reserveRevisions(3); err != nil {
		return nil, err
	}
	if err := r.begin(); err != nil {
		return nil, err
	}
	p, err := namespace()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, p.Close()) }()
	var observations []error
	for index := 0; index < len(r.data.Records); index++ {
		record := &r.data.Records[index]
		if record.Profile != pathBytes(options.Profile) || record.Target != pathBytes(options.Target) {
			continue
		}
		missing, problem := inspectRecord(p, *record)
		if problem != nil {
			if record.Phase != "creating" {
				record.Phase = "invalidated"
			}
			observations = append(observations, problem)
		} else if missing {
			if err := r.syncRecordedParent(p, *record); err != nil {
				return nil, err
			}
			r.data.Records = append(r.data.Records[:index], r.data.Records[index+1:]...)
			index--
		}
	}
	if err := r.save(); err != nil {
		return nil, errors.Join(errors.Join(observations...), fmt.Errorf("ownership revocation uncertain: %w", err))
	}
	if len(observations) != 0 {
		return nil, errors.Join(observations...)
	}
	return nil, r.finish()
}

func installOwned(destinations []Destination, options Ownership, ops registryOps, install installOps) (report Report, err error) {
	for _, destination := range destinations {
		if !destination.Directory {
			report.Results = append(report.Results, Result{Status: "SKIP", Reason: "ownership preflight failed; no destination changes made"})
		}
	}
	for _, destination := range destinations {
		if !destination.Directory {
			if err := options.protect(destination.Path); err != nil {
				return report, err
			}
		}
	}
	r, err := openRegistry(options, true, ops)
	if err != nil {
		return report, err
	}
	defer func() {
		if r.warning != "" {
			report.Warnings = append(report.Warnings, r.warning)
		}
		err = errors.Join(err, r.Close())
	}()
	// Begin, possible claim retirement, two publications per link, finish.
	if err := r.reserveRevisions(3 + 2*uint64(len(destinations))); err != nil {
		return report, err
	}
	if err := r.begin(); err != nil {
		return report, err
	}
	p, err := prepare(destinations, install)
	if err != nil {
		return report, err
	}
	defer p.Close()
	if err := r.checkInstallation(p); err != nil {
		return report, err
	}
	p.registry = r
	report, err = p.Apply()
	if err == nil {
		err = r.finish()
	}
	return report, err
}

func (r *registry) checkInstallation(p *PreparedPlan) error {
	changed := false
	for _, entry := range p.entries {
		for index := 0; index < len(r.data.Records); index++ {
			record := &r.data.Records[index]
			same, err := recordedClaimOverlap(p, entry, *record, r.ops.nameRules)
			if err != nil {
				return err
			}
			if !same {
				continue
			}
			if record.Profile != pathBytes(r.options.Profile) || record.Target != pathBytes(r.options.Target) || record.Destination != pathBytes(entry.destination.Path) {
				return fmt.Errorf("destination overlaps another ownership scope: %q", entry.destination.Path)
			}
			missing, err := inspectRecord(p, *record)
			if err != nil {
				record.Phase = "invalidated"
				if saveErr := r.save(); saveErr != nil {
					return errors.Join(err, fmt.Errorf("ownership revocation uncertain: %w", saveErr))
				}
				return err
			}
			if missing {
				if err := r.syncRecordedParent(p, *record); err != nil {
					return err
				}
				r.data.Records = append(r.data.Records[:index], r.data.Records[index+1:]...)
				index--
				changed = true
			} else if record.Phase == "invalidated" && entry.initial.Status == "OK" {
				// Satisfied is not owned. Retain the revocation while allowing
				// unrelated missing entries to be installed on an explicit retry.
				continue
			} else if record.Phase != "owned" {
				return fmt.Errorf("unresolved %s ownership for %q; preserve entry and recover manually", record.Phase, record.Destination)
			}
		}
	}
	if changed {
		return r.save()
	}
	return nil
}

// Compare recorded physical slots with the current pinned namespace. Never
// traverse an unrelated historical target merely to exclude its claim.
func recordedClaimOverlap(p *PreparedPlan, entry preparedEntry, record ownershipRecord, rulesAt func(int) (nameRules, error)) (bool, error) {
	oldParent, oldName := record.Parent, filepath.Base(string(record.Destination))
	parent, name := idOfDirectory(entry.parent), filepath.Base(entry.destination.Path)
	if len(entry.missing) != 0 {
		name = entry.missing[0]
	}
	sameSlot := func(a entryID, x string, b entryID, y string) (bool, error) {
		if a != b {
			return false, nil
		}
		if x == y {
			return true, nil
		}
		if potentialNameKey(x) != potentialNameKey(y) {
			return false, nil
		}
		dir := p.dirs[directoryID{a.Device, a.Inode}]
		if dir == nil {
			return false, fmt.Errorf("ownership claim directory unavailable")
		}
		rules, err := rulesAt(dir.fd)
		return equivalentName(x, y, rules), err
	}
	if record.Destination == pathBytes(entry.destination.Path) {
		return true, nil
	}
	if same, err := sameSlot(parent, name, oldParent, oldName); same || err != nil {
		return same, err
	}
	for _, route := range entry.routes {
		if same, err := sameSlot(idOfDirectory(route.parent), route.name, oldParent, oldName); same || err != nil {
			return same, err
		}
	}
	for _, route := range record.Routes {
		if same, err := sameSlot(parent, name, route.Parent, string(route.Name)); same || err != nil {
			return same, err
		}
	}
	return false, nil
}

func newRecord(options Ownership, entry preparedEntry, parent *directory) ownershipRecord {
	return ownershipRecord{Profile: pathBytes(options.Profile), Target: pathBytes(options.Target),
		Destination: pathBytes(entry.destination.Path), Text: pathBytes(entry.destination.Source), Phase: "creating",
		Parent: idOfDirectory(parent), Routes: recordRoutes(entry.routes), Leaf: &fingerprint{}}
}

func (r *registry) beforeCreate(entry preparedEntry, parent *directory) error {
	if err := r.validate(); err != nil {
		return err
	}
	if err := r.options.protect(entry.destination.Path); err != nil {
		return err
	}
	r.data.Records = append(r.data.Records, newRecord(r.options, entry, parent))
	// Reserve room for the captured numeric fingerprint before publishing a
	// new destination, not just before publishing its owned snapshot.
	if _, err := snapshotBytes(r.data, 512); err != nil {
		return err
	}
	return r.save()
}

func (r *registry) afterCreate(entry preparedEntry, parent *directory) error {
	for index := range r.data.Records {
		record := &r.data.Records[index]
		if record.Destination != pathBytes(entry.destination.Path) {
			continue
		}
		leaf, err := r.captureCreated(entry, parent)
		if err != nil {
			return err
		}
		record.Leaf, record.Phase = &leaf, "owned"
		return r.save()
	}
	return fmt.Errorf("missing live creation intent")
}

// captureCreated is shared only by callers that just created this link live.
// It neither grants ownership to observed links nor publishes registry state.
func (r *registry) captureCreated(entry preparedEntry, parent *directory) (fingerprint, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(parent.fd, filepath.Base(entry.destination.Path), &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fingerprint{}, err
	}
	text, err := readlinkAt(parent.fd, filepath.Base(entry.destination.Path))
	if err != nil || text != entry.destination.Source || stat.Mode&unix.S_IFMT != unix.S_IFLNK {
		return fingerprint{}, fmt.Errorf("created symlink changed before ownership capture: %q", entry.destination.Path)
	}
	for _, route := range entry.routes {
		if err := validateBinding(route); err != nil {
			return fingerprint{}, err
		}
	}
	if err := r.ops.sync(parent.fd); err != nil {
		return fingerprint{}, fmt.Errorf("sync installed symlink parent: %w", err)
	}
	leaf := fingerprintOf(stat)
	return leaf, nil
}

func inspectRecord(p *PreparedPlan, record ownershipRecord) (bool, error) {
	parent, missing, routes, err := p.walk(parentPath(string(record.Destination)))
	if err != nil {
		return false, err
	}
	if len(missing) != 0 || idOfDirectory(parent) != record.Parent || !reflect.DeepEqual(recordRoutes(routes), record.Routes) {
		return false, fmt.Errorf("recorded destination ancestry changed: %q", record.Destination)
	}
	for _, route := range routes {
		if err := validateBinding(route); err != nil {
			return false, err
		}
	}
	var stat unix.Stat_t
	err = unix.Fstatat(parent.fd, filepath.Base(string(record.Destination)), &stat, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if record.Phase == "creating" || record.Phase == "invalidated" {
		return false, nil // Non-authorizing records may describe arbitrary occupants.
	}
	if record.Leaf == nil || fingerprintOf(stat) != *record.Leaf || stat.Mode&unix.S_IFMT != unix.S_IFLNK {
		return false, fmt.Errorf("recorded symlink identity changed: %q", record.Destination)
	}
	text, err := readlinkAt(parent.fd, filepath.Base(string(record.Destination)))
	if err != nil || text != string(record.Text) {
		return false, fmt.Errorf("recorded symlink contents changed: %q", record.Destination)
	}
	return false, nil
}

func (r *registry) syncRecordedParent(p *PreparedPlan, record ownershipRecord) error {
	parent, missing, routes, err := p.walk(parentPath(string(record.Destination)))
	if err != nil {
		return err
	}
	if len(missing) != 0 || idOfDirectory(parent) != record.Parent || !reflect.DeepEqual(recordRoutes(routes), record.Routes) {
		return fmt.Errorf("recorded parent unavailable")
	}
	if err := r.ops.sync(parent.fd); err != nil {
		return fmt.Errorf("sync recorded parent: %w", err)
	}
	return nil
}
