package filesystem

import (
	"encoding/json"
	"fmt"
	"math"

	"golang.org/x/sys/unix"
)

// Seven forward publications (barrier, intent, binding, relocated, creating,
// owned, completion), plus invalidation, retirement and fresh begin/finish.
// This is finite headroom, not a promise of unlimited failed recovery retries.
const importRevisionBudget = 11

func reserveImport(state snapshot, intent importIntent) (int, uint64, error) {
	if err := revisionCapacity(state.Revision, importRevisionBudget); err != nil {
		return 0, 0, err
	}
	state.Version, state.Unfinished = 2, true
	state.Imports = []importIntent{intent}
	if err := validateSnapshot(state); err != nil {
		return 0, 0, err
	}
	maxID := entryID{math.MaxUint64, math.MaxUint64}
	routes := append([]recordedRoute{}, intent.SourceAnchor.Routes...)
	for _, name := range intent.SourceAnchor.Missing {
		routes = append(routes, recordedRoute{Parent: maxID, Name: name, Entry: maxID, Type: unix.S_IFDIR, Target: maxID})
	}
	intent.SourceBinding = &importBinding{maxID, routes}
	intent.Phase = "relocated"
	file := intent.OriginalFile
	file.Fingerprint.Changed, file.Fingerprint.ChgNsec = math.MinInt64, 999999999
	intent.RelocatedFile = &file
	state.Imports, state.Revision = []importIntent{intent}, math.MaxUint64
	state.Records = append([]ownershipRecord{}, state.Records...)
	for n := range state.Records {
		state.Records[n].Phase = "invalidated"
	}
	// These numeric upper bounds are sizing evidence only, never published or
	// interpreted as a claim. Signed seconds include the longest negative value.
	leaf := &fingerprint{ID: maxID, Mode: math.MaxUint32, Size: math.MaxInt64, Modified: math.MinInt64, ModNsec: 999999999, Changed: math.MinInt64, ChgNsec: 999999999}
	state.Records = append(state.Records, ownershipRecord{Profile: intent.Profile, Target: intent.Target, Destination: intent.Input, Text: intent.Source, Phase: "invalidated", Parent: intent.InputBinding.Parent, Routes: intent.InputBinding.Routes, Leaf: leaf})
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return 0, 0, err
	}
	if len(encoded) >= registrySizeLimit {
		return 0, 0, fmt.Errorf("complete import transaction exceeds 16 MiB including future ownership and recovery")
	}
	return len(encoded) + 1, importRevisionBudget, nil
}
