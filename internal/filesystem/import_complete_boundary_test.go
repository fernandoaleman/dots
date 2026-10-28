package filesystem

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestImportCaptureAndAncestorBoundaries(t *testing.T) {
	for _, kind := range []string{"file at capture", "wrong link at capture", "parent symlink"} {
		t.Run(kind, func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			data, _ := os.ReadFile(q.Input)
			p := liveImport(t, q)
			original := p.intent.OriginalFile
			external := filepath.Join(filepath.Dir(q.Root), "external")
			fixtureFile(t, external+"/"+filepath.Base(q.Input))
			outside := treeState(t, external)
			fired := false
			var injected, registry unix.Stat_t
			var afterInjection map[string]string
			p.ops.checkpoint = func(stage string) {
				if fired {
					return
				}
				if kind == "parent symlink" && stage == "before-link" {
					fired = true
					if err := os.Rename(parentPath(q.Input), parentPath(q.Input)+".saved"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(external, parentPath(q.Input)); err != nil {
						t.Fatal(err)
					}
				} else if kind != "parent symlink" && stage == "link" {
					fired = true
					if err := os.Rename(q.Input, q.Input+".saved"); err != nil {
						t.Fatal(err)
					}
					if kind == "file at capture" {
						fixtureFile(t, q.Input)
					} else if err := os.Symlink("wrong-target", q.Input); err != nil {
						t.Fatal(err)
					}
				}
				if fired {
					if err := unix.Lstat(q.Input, &injected); err != nil {
						t.Fatal(err)
					}
					if err := unix.Lstat(q.Ownership.Registry, &registry); err != nil {
						t.Fatal(err)
					}
					afterInjection = treeState(t, filepath.Dir(q.Root))
				}
			}
			result, err := p.complete(nativeImportCompleteOps())
			expected := importCompletion{Relocated: true, Linked: kind != "parent symlink"}
			if err == nil || !fired || result != expected {
				t.Fatal(result, err)
			}
			got, e := os.ReadFile(q.Source)
			if e != nil || !bytes.Equal(got, data) {
				t.Fatal("source lost", e)
			}
			var moved, kept, currentRegistry unix.Stat_t
			if err := unix.Lstat(q.Source, &moved); err != nil || !sameImportFile(original, importFile{fingerprintOf(moved), moved.Uid, moved.Gid, uint64(moved.Nlink)}) {
				t.Fatal("relocated source identity or metadata changed", err)
			}
			if err := unix.Lstat(q.Input, &kept); err != nil || fingerprintOf(kept) != fingerprintOf(injected) {
				t.Fatal("injected occupant changed", err)
			}
			if err := unix.Lstat(q.Ownership.Registry, &currentRegistry); err != nil || fingerprintOf(currentRegistry) != fingerprintOf(registry) {
				t.Fatal("refusal replaced or modified registry", err)
			}
			if !reflect.DeepEqual(afterInjection, treeState(t, filepath.Dir(q.Root))) {
				t.Fatal("refusal modified fixture contents or bindings")
			}
			if !reflect.DeepEqual(outside, treeState(t, external)) {
				t.Fatal("external tree modified")
			}
			s := ownershipSnapshot(t, q.Ownership)
			if len(s.Imports) != 1 || !s.Unfinished || len(s.Records) != 1 || s.Records[0].Phase != "creating" {
				t.Fatal("uncertain creation granted ownership")
			}
			if _, err := p.complete(nativeImportCompleteOps()); err == nil {
				t.Fatal("capture failure resumed")
			}
			assertImportBlocked(t, q)
		})
	}
}

func TestImportCompletionCapacityBoundary(t *testing.T) {
	q := preparationFixture(t, true, 1)
	managed := Destination{Path: q.Ownership.Target + "/.history", Source: q.Root + "/history"}
	fixtureFile(t, managed.Source)
	installFixture(t, q.Ownership, []Destination{managed})
	q.Entries = append(q.Entries, managed)
	state := ownershipSnapshot(t, q.Ownership)
	if err := os.MkdirAll(parentPath(q.Source), 0o700); err != nil {
		t.Fatal(err)
	}
	// Derive the exact existing-parent intent in this isolated fixture. Restore
	// only its test registry snapshot before exercising admission at the limit.
	preview := liveImport(t, q)
	intent := preview.intent
	if err := preview.Close(); err != nil {
		t.Fatal(err)
	}
	state.Revision = math.MaxUint64 - importRevisionBudget
	state.Records[0].Text = "/"
	bound, _, err := reserveImport(state, intent)
	if err != nil {
		t.Fatal(err)
	}
	state.Records[0].Text = pathBytes("/" + strings.Repeat("x", (registrySizeLimit-bound)/4*3))
	bound, _, err = reserveImport(state, intent)
	if err != nil {
		t.Fatal(err)
	}
	// Decimal metadata widths close the final zero-to-three-byte base64 gap.
	leaf := *state.Records[0].Leaf
	state.Records[0].Leaf = &leaf
	if registrySizeLimit-bound > 3 {
		t.Fatal("unexpected capacity-padding gap", registrySizeLimit-bound)
	}
	for attempts := 0; bound < registrySizeLimit && attempts < 3; attempts++ {
		previous := bound
		leaf.Modified *= 10
		bound, _, err = reserveImport(state, intent)
		if err != nil {
			t.Fatal(err)
		}
		if bound <= previous {
			t.Fatal("capacity padding made no progress")
		}
	}
	if bound != registrySizeLimit {
		t.Fatal("exact bound not reached", bound)
	}
	writeImportSnapshot(t, q.Ownership, state)
	p := liveImport(t, q)
	if p.bound != registrySizeLimit {
		t.Fatal("capacity reservation drift", p.bound)
	}
	before := state.Records[0]
	result, err := p.complete(nativeImportCompleteOps())
	if err != nil || result != (importCompletion{true, true, true}) {
		t.Fatal(result, err)
	}
	done := ownershipSnapshot(t, q.Ownership)
	if done.Revision != math.MaxUint64-5 || p.steps != 5 {
		t.Fatal("recovery revision headroom lost", done.Revision, p.steps)
	}
	found := false
	for _, r := range done.Records {
		found = found || reflect.DeepEqual(r, before)
	}
	if !found {
		t.Fatal("history dropped")
	}
	if len(done.Imports) != 0 || done.Unfinished {
		t.Fatal("completion incomplete")
	}
}
