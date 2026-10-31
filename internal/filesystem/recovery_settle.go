package filesystem

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Shared by read-only preview and locked settlement. Clearing the barrier must
// not revive authority or alter phases; ordinary begin would invalidate them.
func settlementBlockers(s snapshot) []string {
	if len(s.Imports) != 0 {
		return []string{"pending import must be explicitly retired before settlement"}
	}
	if !s.Unfinished {
		return nil
	}
	var reasons []string
	for _, record := range s.Records {
		if record.Phase == "owned" || record.Phase == "removing" {
			reasons = append(reasons, fmt.Sprintf("unfinished %s authority at %q must be invalidated through a separately authorized operation; settlement preserves record states", record.Phase, record.Destination))
		}
	}
	if len(reasons) != 0 {
		return reasons
	}
	if err := revisionCapacity(s.Revision, 1); err != nil {
		return []string{err.Error()}
	}
	future := s
	future.Unfinished, future.Revision = false, s.Revision+1
	if _, err := snapshotBytes(future, 0); err != nil {
		return []string{err.Error()}
	}
	return nil
}

type barrierSettlementOps struct {
	registry   registryOps
	checkpoint func(string)
}

func nativeBarrierSettlementOps() barrierSettlementOps {
	return barrierSettlementOps{registry: nativeRegistryOps()}
}

type barrierSettlement struct {
	Exists, NoOp, Settled, Unfinished bool
	Version                           int
	Revision                          uint64
}

// settleBarrier owns one uninterrupted existing-lock invocation. No user paths
// are inspected or changed. A preview cannot supply authority to this operation.
func settleBarrier(path string, revision uint64, ops barrierSettlementOps) (report barrierSettlement, err error) {
	if !validAbsolute(path) || filepath.Base(path) != "ownership.json" {
		return report, fmt.Errorf("invalid settlement registry locator")
	}
	r, err := openRegistryState(Ownership{Registry: path}, false, ops.registry)
	if err != nil {
		return report, err
	}
	view, err := inspectImportRegistry(r, nil)
	if err != nil {
		return report, err
	}
	attempted := false
	check := func() error {
		if r.publication == nil {
			return view.revalidate()
		}
		var result error
		for _, e := range view.entries {
			if e.name != "ownership.json" { // Own publication advances only snapshot evidence.
				result = errors.Join(result, e.revalidate())
			}
		}
		return errors.Join(result, r.validate(), privateFD(r.dir.fd, true), privateFD(r.lock, false), privateFD(view.lock, false), r.validatePublication())
	}
	defer func() {
		err = errors.Join(err, check(), view.Close())
		if err != nil && attempted {
			err = fmt.Errorf("settlement stopped; barrier clearance may be visible with uncertain durability; preserve registry evidence; no automatic retry or reconstruction: %w", err)
		}
	}()
	step := func(name string) {
		if ops.checkpoint != nil {
			ops.checkpoint(name)
		}
	}
	var entry importStateEntry
	for _, e := range view.entries {
		if e.name == "ownership.json" {
			entry = e
		}
	}
	if !entry.exists {
		if revision != 0 {
			return report, fmt.Errorf("ownership registry missing; expected revision %d cannot be verified", revision)
		}
		report.NoOp = true // Zero authorizes an absent-state no-op, never initialization.
		step("no-op")
		return report, nil
	}
	report.Exists = true
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
	report.Version, report.Revision, report.Unfinished = r.data.Version, r.data.Revision, r.data.Unfinished
	if r.data.Revision != revision {
		return report, fmt.Errorf("expected settlement revision %d does not match registry revision %d", revision, r.data.Revision)
	}
	expected, err := observationFor(r.data, entry)
	if err != nil {
		return report, err
	}
	r.publication = &expected
	step("locked")
	if err := check(); err != nil {
		return report, err
	}
	if err := r.allowMutation(); err != nil {
		return report, err
	}
	if !r.data.Unfinished {
		report.NoOp = true
		step("no-op")
		return report, nil
	}
	if reasons := settlementBlockers(r.data); len(reasons) != 0 {
		return report, fmt.Errorf("settlement refused: %s", strings.Join(reasons, "; "))
	}
	step("before-settlement")
	if err := check(); err != nil {
		return report, err
	}
	attempted = true
	if err := r.finish(); err != nil {
		return report, err
	}
	report.Settled, report.Unfinished, report.Revision = true, false, r.data.Revision
	step("settlement")
	return report, nil
}
