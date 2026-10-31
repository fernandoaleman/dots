package filesystem

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

type recoveryRetireOps struct {
	registry   registryOps
	checkpoint func(string)
}

func nativeRecoveryRetireOps() recoveryRetireOps {
	return recoveryRetireOps{registry: nativeRegistryOps()}
}

// Confirmed durable publications only. Errors may describe uncertain visibility.
type recoveryRetirement struct {
	Survivor             string
	Revision             uint64
	Invalidated, Retired bool
	BarrierPending       bool
}

// retireImport consumes authority only inside this uninterrupted locked call.
// It never grants continuation authority, settles a barrier or alters user files.
func retireImport(path string, selection recoverySelection, ops recoveryRetireOps) (report recoveryRetirement, err error) {
	if !validAbsolute(path) || filepath.Base(path) != "ownership.json" {
		return report, fmt.Errorf("invalid recovery registry locator")
	}
	r, err := openRegistryState(Ownership{Registry: path}, false, ops.registry)
	if err != nil {
		return report, err
	}
	view, err := inspectImportRegistry(r, nil)
	if err != nil {
		return report, err
	}
	var held *recoveryEvidence
	attempted := false
	defer func() {
		if held != nil {
			err = errors.Join(err, held.Close())
		}
		err = errors.Join(err, r.validatePublication(), view.Close())
		if err != nil && attempted {
			err = fmt.Errorf("retirement stopped; registry publication may be visible or uncertain; preserve surviving file %q and ownership evidence; prior authority may be invalidated; no automatic retry or recovery; barrier settlement remains separate: %w", report.Survivor, err)
		}
	}()
	step := func(name string) {
		if ops.checkpoint != nil {
			ops.checkpoint(name)
		}
	}
	if r.dir == nil || view.lock < 0 {
		return report, fmt.Errorf("retirement requires an existing private state directory, registry and stable lock; no initialization performed")
	}
	step("before-lock")
	if err := r.acquireExisting(); err != nil {
		return report, err
	}
	if err := view.revalidate(); err != nil {
		return report, err
	}
	r.data = snapshot{}
	if err := r.load(); err != nil {
		return report, err
	}
	if err := view.revalidate(); err != nil {
		return report, err
	}
	if r.data.Version != 2 || !r.data.Unfinished || len(r.data.Imports) != 1 || r.data.Imports[0].ID != selection.ID || r.data.Revision != selection.Revision {
		return report, fmt.Errorf("retirement requires the exact pending import identifier and expected registry revision")
	}
	intent := r.data.Imports[0]
	report.Revision, report.BarrierPending = r.data.Revision, true
	entry, err := observeImportState(r.dir, "ownership.json")
	if err != nil || !entry.exists {
		return report, errors.Join(fmt.Errorf("selected recovery snapshot is missing"), err)
	}
	expected, err := publicationFor(r.data, entry)
	if err != nil {
		return report, err
	}
	r.publication = &expected // Never clear the ordinary pending-import latch.
	check := func() error {
		if r.lock < 0 || r.data.Version != 2 || !r.data.Unfinished || len(r.data.Imports) != 1 || !reflect.DeepEqual(r.data.Imports[0], intent) {
			return fmt.Errorf("no live retirement authority for selected import")
		}
		var lockErr error
		for _, e := range view.entries {
			if e.name == "ownership.lock" {
				lockErr = errors.Join(lockErr, e.revalidate())
			}
		}
		return errors.Join(lockErr, r.validate(), privateFD(r.dir.fd, true), privateFD(r.lock, false), r.validatePublication())
	}
	verify := func() error {
		if held != nil {
			if err := held.check(); err != nil {
				return err
			}
			if err := held.Close(); err != nil {
				return err
			}
		}
		observed := recoverySummary(intent)
		var err error
		held, err = inspectRecoveryFiles(intent, observed, nil)
		if err != nil {
			return err
		}
		if !observed.RetirementObserved || held == nil || len(held.files) != 1 {
			var reasons []string
			for _, c := range observed.Checks {
				if !c.Passed {
					reasons = append(reasons, c.Name+": "+c.Detail)
				}
			}
			return fmt.Errorf("retirement refused: %s", strings.Join(reasons, "; "))
		}
		if observed.Input.Kind == "regular file" {
			report.Survivor = string(intent.Input)
		} else {
			report.Survivor = string(intent.Source)
		}
		return check()
	}
	step("locked")
	if err := check(); err != nil {
		return report, err
	}
	if err := verify(); err != nil {
		return report, err
	}
	if err := revisionCapacity(r.data.Revision, 2); err != nil {
		return report, err
	}
	invalidated := r.data
	invalidated.Records = append([]ownershipRecord{}, r.data.Records...)
	for n := range invalidated.Records {
		if invalidated.Records[n].Phase == "owned" || invalidated.Records[n].Phase == "removing" {
			invalidated.Records[n].Phase = "invalidated"
		}
	}
	// Size both complete prospective snapshots, including recovery expansion and
	// the actual revision increments, before the first publication is attempted.
	for n := uint64(1); n <= 2; n++ {
		future := invalidated
		future.Revision += n
		if n == 2 {
			future.Imports = []importIntent{}
		}
		if err := validateSnapshot(future); err != nil {
			return report, err
		}
		if _, err := snapshotBytes(future, 0); err != nil {
			return report, err
		}
	}
	step("before-invalidation")
	if err := errors.Join(held.check(), check()); err != nil {
		return report, err
	}
	r.data = invalidated
	attempted = true
	if err := r.publishSnapshot(); err != nil {
		return report, err
	}
	report.Invalidated, report.BarrierPending, report.Revision = true, true, r.data.Revision
	step("invalidation")
	if err := verify(); err != nil {
		return report, err
	}
	step("before-file-sync")
	if err := ops.registry.sync(int(held.files[0].Fd())); err != nil {
		return report, fmt.Errorf("sync surviving recovery file: %w", err)
	}
	step("file-sync")
	for _, parent := range held.parents() {
		step("before-parent-sync")
		if err := ops.registry.sync(parent.fd); err != nil {
			return report, fmt.Errorf("sync recovery parent %q: %w", held.selected[parent], err)
		}
		step("parent-sync")
	}
	step("before-retirement")
	if err := errors.Join(held.check(), check()); err != nil {
		return report, err
	}
	r.data.Imports = []importIntent{}
	if err := r.publishSnapshot(); err != nil {
		return report, err
	}
	report.Retired, report.Revision = true, r.data.Revision
	step("retirement")
	return report, nil
}
