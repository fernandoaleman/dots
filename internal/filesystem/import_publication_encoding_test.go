package filesystem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Exercise the public-state encoding contract through the existing checker,
// including the v1 omission and v2 empty-array wire representations.
func TestPublicationCanonicalContinuity(t *testing.T) {
	for _, kind := range []string{"v1 history", "v2 empty", "v2 prepared", "v2 relocated"} {
		t.Run(kind, func(t *testing.T) {
			options, _, state := v2Fixture(t, true)
			switch kind {
			case "v1 history":
				state.Version, state.Imports, state.Unfinished = 1, nil, false
			case "v2 empty":
				state.Imports, state.Unfinished = []importIntent{}, false
			case "v2 relocated":
				i := &state.Imports[0]
				i.Phase = "relocated"
				file := i.OriginalFile
				i.RelocatedFile = &file
			}
			writeImportSnapshot(t, options, state)
			r, err := openRegistry(options, false, nativeRegistryOps())
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			entry, err := observeImportState(r.dir, "ownership.json")
			if err != nil {
				t.Fatal(err)
			}
			expected, err := publicationFor(r.data, entry)
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(options.Registry)
			if err != nil {
				t.Fatal(err)
			}
			// Keep a different prospective revision in memory. A detached reload
			// must leave it, the loaded-version latch and expected evidence intact.
			r.data.Revision += 10
			prospective, err := json.Marshal(r.data)
			if err != nil {
				t.Fatal(err)
			}
			version, blocked := r.version, fmt.Sprint(r.blocked)
			unchanged := expected
			// Repeated checks must validate fresh disk state without advancing evidence.
			for n := 0; n < 2; n++ {
				if err := expected.check(r); err != nil {
					t.Fatal(err)
				}
			}
			if expected != unchanged {
				t.Fatal("checker advanced expected evidence")
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(before, &fields); err != nil {
				t.Fatal(err)
			}
			compact, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(options.Registry, compact, 0o600); err != nil {
				t.Fatal(err)
			}
			// Rebase only metadata in this fixture to isolate the established
			// canonical comparison from separately tested named-binding checks.
			expected.entry, err = observeImportState(r.dir, "ownership.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := expected.check(r); err != nil {
				t.Fatal("canonical formatting/key order changed digest", err)
			}
			before = compact
			changed := expected
			changed.revision++
			if err := changed.check(r); err == nil {
				t.Fatal("mismatched revision accepted")
			}
			changed = expected
			changed.digest[0] ^= 0xff
			if err := changed.check(r); err == nil {
				t.Fatal("mismatched complete-state digest accepted")
			}
			after, err := os.ReadFile(options.Registry)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("continuity checks wrote state", err)
			}
			if err := expected.entry.revalidate(); err != nil {
				t.Fatal(err)
			}
			current, err := json.Marshal(r.data)
			if err != nil || !bytes.Equal(current, prospective) || r.version != version || fmt.Sprint(r.blocked) != blocked {
				t.Fatal("checker changed live prospective state or latches", err)
			}
		})
	}
}

func TestPublicationRecoveryCapacityDiagnostic(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	r, err := openRegistry(options, false, nativeRegistryOps())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	entry, err := observeImportState(r.dir, "ownership.json")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := publicationFor(r.data, entry)
	if err != nil {
		t.Fatal(err)
	}
	state := ownershipSnapshot(t, options)
	original := state.Records[0]
	for n := 0; n < 100; n++ {
		record := original
		record.Destination = pathBytes(fmt.Sprintf("%s/.slot-%d", options.Target, n))
		state.Records = append(state.Records, record)
	}
	base, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	padding := (registrySizeLimit - 256 - len(base)) / 4 * 3
	state.Records[0].Text = pathBytes("/" + strings.Repeat("x", padding))
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if len(data) > registrySizeLimit {
		t.Fatal("fixture exceeds on-disk bound")
	}
	_, capacityError := snapshotBytes(state, 0)
	if capacityError == nil {
		t.Fatal("fixture must exceed recovery capacity")
	}
	// Write a valid-schema but capacity-unsafe change in an isolated fixture.
	if err := os.WriteFile(options.Registry, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// Match metadata only to isolate content/capacity rejection, as in the
	// existing continuity fault tests. Production never refreshes evidence here.
	expected.entry, err = observeImportState(r.dir, "ownership.json")
	if err != nil {
		t.Fatal(err)
	}
	got := expected.check(r)
	if got == nil || got.Error() != capacityError.Error() {
		t.Fatalf("capacity diagnostic changed: got %v; want %v", got, capacityError)
	}
	after, err := os.ReadFile(options.Registry)
	if err != nil || !bytes.Equal(after, data) {
		t.Fatal("rejected snapshot changed", err)
	}
}
