package filesystem

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
)

// Removal describes one recorded entry, without claiming ownership of untracked
// paths. REMOVE is eligible, MISSING needs bookkeeping, PRESERVE is unresolved.
type Removal struct {
	Destination string
	Source      string
	Status      string
	Reason      string
}

// RemovalReport includes actual successful unlink calls even if persistence fails.
type RemovalReport struct {
	Entries  []Removal
	Removed  int
	Warnings []string
}

// Down inspects or removes recorded links under the approved cooperative
// concurrency contract. A dry-run never initializes, locks, or updates state.
func Down(options Ownership, dryRun bool) (RemovalReport, error) {
	return down(options, dryRun, nativeRegistryOps())
}

func down(options Ownership, dryRun bool, ops registryOps) (report RemovalReport, err error) {
	r, err := openRegistry(options, !dryRun, ops)
	if err != nil {
		return report, err
	}
	defer func() {
		if r.warning != "" {
			report.Warnings = append(report.Warnings, r.warning)
		}
		err = errors.Join(err, r.Close())
	}()
	if !dryRun {
		if err := r.begin(); err != nil {
			return report, err
		}
	}
	p, err := namespace()
	if err != nil {
		return report, err
	}
	defer func() { err = errors.Join(err, p.Close()) }()
	var selected []ownershipRecord
	for _, record := range r.data.Records {
		if record.Profile != pathBytes(options.Profile) {
			continue
		}
		if record.Target != pathBytes(options.Target) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("historical target %q retained: %q", record.Target, record.Destination))
			continue
		}
		selected = append(selected, record)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Destination < selected[j].Destination })
	var destinations []Destination
	for _, record := range selected {
		destinations = append(destinations, Destination{Path: string(record.Destination), Source: string(record.Text)})
		report.Entries = append(report.Entries, Removal{Destination: string(record.Destination), Source: string(record.Text), Status: "SKIP", Reason: "not inspected after preflight failure"})
	}
	collisions := collisionResults(destinations, p.rulesAt)
	blocked := false
	for index, record := range selected {
		entry := Removal{Destination: string(record.Destination), Source: string(record.Text), Status: "REMOVE"}
		missing, problem := inspectRecord(p, record)
		if collisions[index].Reason != "" {
			problem = errors.Join(problem, fmt.Errorf("recorded destination collision: %s", collisions[index].Reason))
		}
		if problem == nil {
			problem = r.checkRemovalClaims(p, record)
		}
		if problem == nil {
			problem = options.protect(entry.Destination)
		}
		switch {
		case problem != nil:
			entry.Status, entry.Reason = "PRESERVE", problem.Error()
			report.Entries[index] = entry
			if !dryRun {
				if err := r.invalidate(record.Destination); err != nil {
					report.Entries[index].Reason += "; ownership revocation uncertain: " + err.Error()
					skipRemovalPlans(report.Entries)
					return report, fmt.Errorf("ownership revocation uncertain: %w", err)
				}
			}
		case missing:
			entry.Status = "MISSING"
		case dryRun && r.data.Unfinished:
			entry.Status, entry.Reason = "PRESERVE", "unfinished ownership transaction; automatic removal forbidden"
		case record.Phase != "owned" && record.Phase != "removing":
			entry.Status, entry.Reason = "PRESERVE", "unresolved "+record.Phase+" ownership; manual recovery required"
		}
		blocked = blocked || entry.Status == "PRESERVE"
		report.Entries[index] = entry
	}
	if blocked {
		if !dryRun {
			skipRemovalPlans(report.Entries)
		}
		return report, fmt.Errorf("removal preflight failed; no links removed")
	}
	if dryRun {
		return report, nil
	}
	for index, record := range selected {
		if err := r.removeOne(p, record, &report, index); err != nil {
			report.Entries[index].Status, report.Entries[index].Reason = "ERROR", err.Error()
			for next := index + 1; next < len(report.Entries); next++ {
				report.Entries[next].Status = "SKIP"
				report.Entries[next].Reason = "not attempted after failure"
			}
			return report, err
		}
	}
	return report, r.finish()
}

// Historical locators need not remain accessible. Compare their recorded slots
// and routes against the selected entry's pinned namespace, without visiting
// historical targets or inferring authority from their current occupants.
func (r *registry) checkRemovalClaims(p *PreparedPlan, record ownershipRecord) error {
	parent, missing, routes, err := p.walk(parentPath(string(record.Destination)))
	if err != nil {
		return err
	}
	entry := preparedEntry{destination: Destination{Path: string(record.Destination)}, parent: parent, missing: missing, routes: routes}
	for _, other := range r.data.Records {
		if other.Destination == record.Destination {
			continue
		}
		shared, err := recordedClaimOverlap(p, entry, other, r.ops.nameRules)
		if err != nil {
			return fmt.Errorf("compare recorded ownership claim %q: %w", other.Destination, err)
		}
		if shared {
			return fmt.Errorf("destination overlaps another recorded ownership claim: %q", other.Destination)
		}
	}
	return nil
}

func skipRemovalPlans(entries []Removal) {
	for i := range entries {
		if entries[i].Status == "REMOVE" {
			entries[i].Status = "SKIP"
			entries[i].Reason = "preflight failed; no links removed"
		}
	}
}

func (r *registry) invalidate(destination pathBytes) error {
	for i := range r.data.Records {
		if r.data.Records[i].Destination == destination && r.data.Records[i].Phase != "creating" {
			r.data.Records[i].Phase = "invalidated"
		}
	}
	return r.save()
}

func (r *registry) removeOne(p *PreparedPlan, record ownershipRecord, report *RemovalReport, index int) error {
	if err := r.validate(); err != nil {
		return err
	}
	missing, err := inspectRecord(p, record)
	if err != nil {
		return errors.Join(err, r.invalidate(record.Destination))
	}
	if err := r.checkRemovalClaims(p, record); err != nil {
		return errors.Join(err, r.invalidate(record.Destination))
	}
	if err := r.options.protect(string(record.Destination)); err != nil {
		return err
	}
	if !missing {
		for i := range r.data.Records {
			if r.data.Records[i].Destination == record.Destination {
				r.data.Records[i].Phase = "removing"
			}
		}
		if err := r.save(); err != nil {
			return err
		}
		if r.ops.checkpoint != nil {
			r.ops.checkpoint("before-remove-validation", string(record.Destination))
		}
		if err := r.validate(); err != nil {
			return err
		}
		missing, err = inspectRecord(p, record)
		if err != nil || missing {
			return errors.Join(fmt.Errorf("destination changed before removal: %q", record.Destination), err, r.invalidate(record.Destination))
		}
		if err := r.checkRemovalClaims(p, record); err != nil {
			return errors.Join(err, r.invalidate(record.Destination))
		}
		if err := r.options.protect(string(record.Destination)); err != nil {
			return err
		}
		// Use exactly the directory just validated, never a fresh pathname
		// walk that could observe a replacement after that validation.
		parent := p.dirs[directoryID{record.Parent.Device, record.Parent.Inode}]
		if parent == nil {
			return fmt.Errorf("validated parent handle unavailable")
		}
		// No portable compare-and-unlink exists. External destination/ancestor
		// writers must cooperate through this final validation/unlink window.
		if err := r.ops.unlink(parent.fd, filepath.Base(string(record.Destination)), 0); err != nil {
			return fmt.Errorf("remove symlink: %w", err)
		}
		report.Removed++
		report.Entries[index].Status = "REMOVED"
	}
	if err := r.syncRecordedParent(p, record); err != nil {
		return err
	}
	for i, entry := range r.data.Records {
		if entry.Destination == record.Destination {
			r.data.Records = append(r.data.Records[:i], r.data.Records[i+1:]...)
			break
		}
	}
	return r.save()
}
