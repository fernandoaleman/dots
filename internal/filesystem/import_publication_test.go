package filesystem

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// Every change is an isolated, deliberately noncooperating fixture writer.
func changeImportRegistry(t *testing.T, p *preparedImport, kind string) {
	t.Helper()
	path := p.request.Ownership.Registry
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "delete":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		return
	case "same contents":
		if err := os.Rename(path, path+".previous"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	case "replacement history":
		s := ownershipSnapshot(t, p.request.Ownership)
		s.Revision++
		s.Imports = []importIntent{}
		s.Unfinished = false
		// Distinct private snapshot preserving whatever history the fixture held.
		writeImportSnapshot(t, p.request.Ownership, s)
		return
	case "revision", "transaction", "malformed":
		if kind == "revision" {
			data = bytes.Replace(data, []byte(fmt.Sprintf("\"revision\": %d", p.view.registry.publication.revision)), []byte(fmt.Sprintf("\"revision\": %d", p.view.registry.publication.revision+1)), 1)
		}
		if kind == "transaction" {
			data = bytes.Replace(data, []byte(p.intent.Digest.Value), []byte(strings.Repeat("f", 64)), 1)
		}
		if kind == "malformed" {
			data = []byte("{broken}")
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		// Simulate indistinguishable metadata to isolate the independent strict
		// content/revision guard. Production never refreshes evidence this way.
		entry, err := observeImportState(p.view.registry.dir, "ownership.json")
		if err != nil {
			t.Fatal(err)
		}
		if idOf(entry.stat) != idOf(p.view.registry.publication.entry.stat) {
			t.Fatal("same-inode fixture replaced registry")
		}
		p.view.registry.publication.entry = entry
	default:
		t.Fatal("unknown registry fault", kind)
	}
}

func TestImportPublicationContinuity(t *testing.T) {
	for _, point := range []string{"handoff", "before-relocated", "before-creating", "before-owned", "before-completion"} {
		for _, kind := range []string{"delete", "same contents", "replacement history", "revision", "transaction", "malformed"} {
			t.Run(point+"/"+kind, func(t *testing.T) {
				q := preparationFixture(t, true, 1)
				data, _ := os.ReadFile(q.Input)
				p := liveImport(t, q)
				original := p.intent.OriginalFile
				var originalStat unix.Stat_t
				if err := unix.Lstat(q.Input, &originalStat); err != nil {
					t.Fatal(err)
				}
				fired := false
				var replacement []byte
				var replacementID entryID
				change := func() {
					fired = true
					changeImportRegistry(t, p, kind)
					replacement, _ = os.ReadFile(q.Ownership.Registry)
					var s unix.Stat_t
					if err := unix.Lstat(q.Ownership.Registry, &s); err == nil {
						replacementID = idOf(s)
					}
				}
				if point == "handoff" {
					change()
				} else {
					p.ops.checkpoint = func(stage string) {
						if stage == point && !fired {
							change()
						}
					}
				}
				ops := nativeImportCompleteOps()
				renames := 0
				native := ops.rename
				ops.rename = func(a int, b string, c int, d string) error { renames++; return native(a, b, c, d) }
				result, err := p.complete(ops)
				if err == nil || !fired || !strings.Contains(err.Error(), "registry") {
					t.Fatal("registry change admitted", result, err)
				}
				linked := point == "before-owned" || point == "before-completion"
				expected := importCompletion{Relocated: point != "handoff", Linked: linked, Owned: point == "before-completion"}
				if result != expected {
					t.Fatal("inaccurate completion result", result, expected)
				}
				if point == "handoff" {
					if renames != 0 {
						t.Fatal("relocated without durable intent")
					}
					assertPreparationInput(t, q, originalStat, data)
				} else {
					assertRelocated(t, q, original, data, linked)
				}
				if kind == "delete" {
					if _, err := os.Lstat(q.Ownership.Registry); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("deleted registry recreated", err)
					}
				} else {
					got, e := os.ReadFile(q.Ownership.Registry)
					var s unix.Stat_t
					if e != nil || !bytes.Equal(got, replacement) || unix.Lstat(q.Ownership.Registry, &s) != nil || idOf(s) != replacementID {
						t.Fatal("replacement history overwritten", e)
					}
				}
				before := treeState(t, filepath.Dir(q.Root))
				if _, err := p.complete(ops); err == nil {
					t.Fatal("failed authority resumed")
				}
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
					t.Fatal("retry modified uncertain state")
				}
			})
		}
	}
}

func TestImportPublicationPersistenceBoundary(t *testing.T) {
	for _, phase := range []string{"relocated", "creating", "owned", "completion"} {
		for _, when := range []string{"before snapshot rename", "temporary substitution", "after directory sync", "sync failure"} {
			t.Run(phase+"/"+when, func(t *testing.T) {
				q := preparationFixture(t, true, 1)
				p := liveImport(t, q)
				old := *p.view.registry.publication
				native := p.view.registry.ops
				active, fired := false, false
				var replacement []byte
				p.ops.checkpoint = func(stage string) {
					if stage == "before-"+phase {
						active = true
					}
				}
				change := func() {
					fired = true
					changeImportRegistry(t, p, "same contents")
					replacement, _ = os.ReadFile(q.Ownership.Registry)
				}
				injected := errors.New("fixture registry sync failed")
				p.view.registry.ops.closeFile = func(f *os.File) error {
					err := native.closeFile(f)
					if active && !fired && when == "before snapshot rename" {
						change()
					}
					if active && !fired && when == "temporary substitution" {
						fired = true
						replacement, _ = os.ReadFile(q.Ownership.Registry)
						temporary := filepath.Join(parentPath(q.Ownership.Registry), f.Name())
						if err := os.Rename(temporary, temporary+".previous"); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(temporary, replacement, 0o600); err != nil {
							t.Fatal(err)
						}
					}
					return err
				}
				p.view.registry.ops.sync = func(fd int) error {
					err := native.sync(fd)
					if active && !fired && fd == p.view.registry.dir.fd && when != "before snapshot rename" {
						change()
						if when == "sync failure" {
							return errors.Join(err, injected)
						}
					}
					return err
				}
				// The tracker may advance before the selected phase; retain its exact
				// expected value immediately before that phase's publication begins.
				p.ops.checkpoint = func(stage string) {
					if stage == "before-"+phase {
						active = true
						old = *p.view.registry.publication
					}
				}
				result, err := p.complete(nativeImportCompleteOps())
				if err == nil || !fired || !result.Relocated {
					t.Fatal("persistence mutation not detected", result, err)
				}
				if when == "sync failure" && !errors.Is(err, injected) {
					t.Fatal("original sync error lost", err)
				}
				if *p.view.registry.publication != old {
					t.Fatal("uncertain publication refreshed expected evidence")
				}
				got, e := os.ReadFile(q.Ownership.Registry)
				if e != nil || !bytes.Equal(got, replacement) {
					t.Fatal("replacement snapshot overwritten", e)
				}
				if !strings.Contains(err.Error(), "registry") {
					t.Fatal("missing continuity diagnostic", err)
				}
			})
		}
	}
}

func TestImportPreparationPublicationContinuity(t *testing.T) {
	for _, point := range []string{"barrier", "intent", "before-mkdir:0", "binding"} {
		t.Run(point, func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			data, _ := os.ReadFile(q.Input)
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			ops := nativeImportPrepareOps()
			fired := false
			var replaced []byte
			ops.checkpoint = func(stage string) {
				if stage == point && !fired {
					fired = true
					bytes, err := os.ReadFile(q.Ownership.Registry)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(q.Ownership.Registry, q.Ownership.Registry+".previous"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(q.Ownership.Registry, bytes, 0o600); err != nil {
						t.Fatal(err)
					}
					replaced = bytes
				}
			}
			p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, ops)
			if p != nil {
				_ = p.Close()
			}
			if err == nil || !fired || !strings.Contains(err.Error(), "registry") {
				t.Fatal("preparation lost publication continuity", err)
			}
			assertPreparationInput(t, q, original, data)
			got, e := os.ReadFile(q.Ownership.Registry)
			if e != nil || !bytes.Equal(got, replaced) {
				t.Fatal("preparation overwrote changed registry", e)
			}
		})
	}
}

func TestImportPublicationLockOnlyState(t *testing.T) {
	for _, appear := range []bool{false, true} {
		t.Run(fmt.Sprint(appear), func(t *testing.T) {
			q := preparationFixture(t, false, 1)
			if err := os.Remove(q.Ownership.Registry); err != nil {
				t.Fatal(err)
			}
			ops := nativeImportPrepareOps()
			fired := false
			if appear {
				ops.checkpoint = func(stage string) {
					if stage == "before-barrier" {
						fired = true
						writeImportSnapshot(t, q.Ownership, snapshot{Version: 1, Records: []ownershipRecord{}})
					}
				}
			}
			p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, ops)
			if appear {
				if p != nil {
					_ = p.Close()
				}
				if err == nil || !fired {
					t.Fatal("unexpected snapshot appeared without refusal", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			if p.view.registry.publication == nil || !p.view.registry.publication.entry.exists {
				t.Fatal("first durable publication not retained")
			}
			if _, err := p.complete(nativeImportCompleteOps()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
