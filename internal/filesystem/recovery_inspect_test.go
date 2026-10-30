package filesystem

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func recoveryFixture(t *testing.T) (Ownership, snapshot, *recoverySelection) {
	t.Helper()
	o, _, s := v2Fixture(t, false)
	writeImportSnapshot(t, o, s)
	return o, s, &recoverySelection{s.Imports[0].ID, s.Revision}
}

func requireRecoveryNoWrite(t *testing.T, o Ownership, selection *recoverySelection) recoveryInspection {
	t.Helper()
	root := filepath.Dir(o.Target)
	before := treeState(t, root)
	got, err := inspectRecovery(o.Registry, selection, nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := inspectRecovery(o.Registry, selection, nil)
	if err != nil || !reflect.DeepEqual(got, again) || !reflect.DeepEqual(before, treeState(t, root)) {
		t.Fatal("inspection wrote files or produced unstable observations", err)
	}
	return got
}

func TestRecoveryInspectionSummary(t *testing.T) {
	for _, mode := range []string{"missing directories", "lock only", "v1", "v2", "v1 barrier", "v2 barrier", "pending", "history"} {
		t.Run(mode, func(t *testing.T) {
			o, _, s := v2Fixture(t, mode == "history")
			imports := s.Imports
			s.Imports = []importIntent{}
			s.Unfinished = strings.Contains(mode, "barrier")
			if strings.HasPrefix(mode, "v1") {
				s.Version = 1
			}
			if mode == "missing directories" {
				o.Registry = filepath.Dir(o.Registry) + "/absent/dots/ownership.json"
			} else if mode == "pending" {
				s.Imports, s.Unfinished = imports, true
				writeImportSnapshot(t, o, s)
			} else if mode != "lock only" {
				writeImportSnapshot(t, o, s)
			}
			got := requireRecoveryNoWrite(t, o, nil)
			if got.Registry != o.Registry || got.Exists != (mode != "missing directories" && mode != "lock only") {
				t.Fatal(got)
			}
			if mode == "pending" {
				if got.Intent == nil || got.Intent.Input.Kind != "not inspected" || got.Intent.RetirementObserved {
					t.Fatal("summary inferred authority", got)
				}
			} else if got.Intent != nil || len(got.SettlementReasons) == 0 {
				t.Fatal("missing settlement diagnostic", got)
			}
			if mode == "history" && got.AuthorityRecords != 1 {
				t.Fatal("historical authority not reported", got)
			}
		})
	}
}

func TestRecoveryInspectionSlots(t *testing.T) {
	for _, mode := range []string{"original", "prepared rename gap", "relocated", "relocated ctime", "both", "neither", "correct symlink", "broken symlink", "directory", "fifo", "replaced file", "content", "digest", "hard link", "metadata", "ancestry", "missing ancestry", "exhausted", "one revision", "read-only parent", "no config", "missing root"} {
		t.Run(mode, func(t *testing.T) {
			o, s, selection := recoveryFixture(t)
			i := &s.Imports[0]
			input, source := string(i.Input), string(i.Source)
			switch mode {
			case "prepared rename gap", "relocated", "relocated ctime":
				if err := os.Rename(input, source); err != nil {
					t.Fatal(err)
				}
				if mode == "prepared rename gap" {
					// Force a ctime-only difference independently of native rename behavior.
					i.OriginalFile.Fingerprint.Changed--
				} else {
					var stat unix.Stat_t
					if err := unix.Lstat(source, &stat); err != nil {
						t.Fatal(err)
					}
					i.Phase = "relocated"
					f := importFile{fingerprintOf(stat), stat.Uid, stat.Gid, uint64(stat.Nlink)}
					i.RelocatedFile = &f
					if mode == "relocated ctime" {
						i.RelocatedFile.Fingerprint.Changed--
					}
				}
			case "both":
				fixtureFile(t, source)
			case "neither":
				if err := os.Remove(input); err != nil {
					t.Fatal(err)
				}
			case "correct symlink", "broken symlink", "directory", "fifo", "replaced file":
				if err := os.Rename(input, input+".preserved"); err != nil {
					t.Fatal(err)
				}
				var err error
				switch mode {
				case "correct symlink":
					fixtureFile(t, source)
					err = os.Symlink(source, input)
				case "broken symlink":
					err = os.Symlink(source, input)
				case "directory":
					err = os.Mkdir(input, 0o700)
				case "fifo":
					err = unix.Mkfifo(input, 0o600)
				case "replaced file":
					err = os.WriteFile(input, []byte("replacement"), 0o600)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "content":
				if err := os.WriteFile(input, []byte("changed"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "digest":
				i.Digest.Value = strings.Repeat("0", 64)
			case "hard link":
				if err := os.Link(input, input+".another"); err != nil {
					t.Fatal(err)
				}
			case "metadata":
				if err := os.Chmod(input, 0o400); err != nil {
					t.Fatal(err)
				}
			case "ancestry", "missing ancestry":
				if err := os.Rename(o.Target, o.Target+".preserved"); err != nil {
					t.Fatal(err)
				}
				if mode == "ancestry" {
					if err := os.Mkdir(o.Target, 0o700); err != nil {
						t.Fatal(err)
					}
				}
			case "exhausted":
				s.Revision, selection.Revision = math.MaxUint64, math.MaxUint64
			case "one revision":
				s.Revision, selection.Revision = math.MaxUint64-1, math.MaxUint64-1
			case "read-only parent":
				if err := os.Chmod(o.Target, 0o555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(o.Target, 0o700) })
			case "no config":
				// The stored locator is intentionally absent, not a live config input.
				i.Profile = pathBytes(filepath.Dir(o.Profile) + "/never-configured.toml")
			case "missing root":
				if err := os.Rename(o.Sources[0], o.Sources[0]+".preserved"); err != nil {
					t.Fatal(err)
				}
			}
			writeImportSnapshot(t, o, s)
			got := requireRecoveryNoWrite(t, o, selection)
			want := mode == "original" || mode == "prepared rename gap" || mode == "relocated" || mode == "no config" || mode == "read-only parent"
			if got.Intent == nil || got.Intent.RetirementObserved != want {
				t.Fatalf("wanted observational candidate %v: %+v", want, got.Intent)
			}
			if got.Intent.ID != i.ID || got.Intent.Profile != string(i.Profile) || got.Intent.Phase != i.Phase || got.Revision != s.Revision {
				t.Fatal("lost stored metadata", got)
			}
			if mode == "digest" && !strings.Contains(recoveryDetails(got), "digest mismatch") {
				t.Fatal("missing specific digest diagnostic")
			}
		})
	}
}

func recoveryDetails(r recoveryInspection) string {
	var text string
	if r.Intent != nil {
		for _, check := range r.Intent.Checks {
			text += check.Detail + "\n"
		}
	}
	return text
}

func TestRecoveryInspectionMissingParents(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(strconv.FormatBool(partial), func(t *testing.T) {
			q := preparationFixture(t, true, 2)
			ops := nativeImportPrepareOps()
			ops.mkdir = func(int, string, uint32) error { return unix.EACCES }
			if p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, ops); err == nil {
				_ = p.Close()
				t.Fatal("fixture must stop before source parents")
			}
			s := ownershipSnapshot(t, q.Ownership)
			if len(s.Imports) != 1 || s.Imports[0].SourceBinding != nil {
				t.Fatal("missing durable anchor", s)
			}
			if partial {
				if err := os.Mkdir(q.Root+"/config", 0o700); err != nil {
					t.Fatal(err)
				}
			}
			got := requireRecoveryNoWrite(t, q.Ownership, &recoverySelection{s.Imports[0].ID, s.Revision})
			if got.Intent.RetirementObserved == partial {
				t.Fatal("unrecorded parents inferred as safe", got.Intent)
			}
		})
	}
}

func TestRecoveryInspectionDirectoryAlias(t *testing.T) {
	for _, mode := range []string{"ordinary", "missing frontier", "occupied frontier"} {
		t.Run(mode, func(t *testing.T) {
			o, s, selection := recoveryFixture(t)
			root := o.Sources[0]
			if err := os.Rename(root, root+".preserved"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(o.Target, root); err != nil {
				t.Fatal(err)
			}
			p, err := namespace()
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			dir, _, routes, err := p.walk(root)
			if err != nil {
				t.Fatal(err)
			}
			i := &s.Imports[0]
			binding := importBinding{idOfDirectory(dir), recordRoutes(routes)}
			i.SourceAnchor.importBinding, i.SourceBinding = binding, &binding
			if mode != "ordinary" {
				i.Source, i.SourceBinding = pathBytes(root+"/new/import"), nil
				i.SourceAnchor.Missing = []pathBytes{"new"}
				if mode == "occupied frontier" {
					if err := os.Mkdir(root+"/new", 0o700); err != nil {
						t.Fatal(err)
					}
				}
			}
			writeImportSnapshot(t, o, s)
			got := requireRecoveryNoWrite(t, o, selection)
			if got.Intent.RetirementObserved == (mode == "occupied frontier") {
				t.Fatal("ordinary alias rejected or unknown parents accepted", got.Intent)
			}
		})
	}
}

func TestRecoveryInspectionSelectionAndInvalidState(t *testing.T) {
	for _, mode := range []string{"wrong id", "stale revision", "empty", "malformed", "duplicate", "unsupported", "missing lock", "lock symlink", "registry symlink", "privacy"} {
		t.Run(mode, func(t *testing.T) {
			o, _, selection := recoveryFixture(t)
			switch mode {
			case "wrong id":
				selection.ID = strings.Repeat("b", 32)
			case "stale revision":
				selection.Revision++
			case "empty", "malformed", "duplicate", "unsupported":
				data := map[string]string{"empty": "", "malformed": "{", "duplicate": `{"version":2,"version":2}`, "unsupported": `{"version":3}`}[mode]
				if err := os.WriteFile(o.Registry, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			case "missing lock", "lock symlink":
				lock := parentPath(o.Registry) + "/ownership.lock"
				if err := os.Rename(lock, lock+".preserved"); err != nil {
					t.Fatal(err)
				}
				if mode == "lock symlink" {
					if err := os.Symlink(lock+".preserved", lock); err != nil {
						t.Fatal(err)
					}
				}
			case "registry symlink":
				if err := os.Rename(o.Registry, o.Registry+".preserved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(o.Registry+".preserved", o.Registry); err != nil {
					t.Fatal(err)
				}
			case "privacy":
				if err := os.Chmod(o.Registry, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := treeState(t, filepath.Dir(o.Target))
			got, err := inspectRecovery(o.Registry, selection, nil)
			if err == nil || got.Intent != nil && got.Intent.RetirementObserved || !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Target))) {
				t.Fatal("unsafe state accepted or changed", got, err)
			}
			if (mode == "wrong id" || mode == "stale revision") && got.Intent.Input.Kind != "not inspected" {
				t.Fatal("invalid selection visited paths")
			}
		})
	}
}

func TestRecoveryInspectionChanges(t *testing.T) {
	for _, mode := range []string{"deleted registry", "replaced registry", "revision", "contents", "lock", "lock privacy", "directory privacy", "input", "counterpart", "ancestry", "during digest", "joined errors"} {
		t.Run(mode, func(t *testing.T) {
			o, s, selection := recoveryFixture(t)
			i := s.Imports[0]
			if mode == "joined errors" {
				s.Imports[0].Digest.Value = strings.Repeat("0", 64)
				writeImportSnapshot(t, o, s)
			}
			var expected map[string]string
			hook := func(stage string) {
				if (mode == "during digest" && stage != "digest") || (mode != "during digest" && stage != "final") {
					return
				}
				var err error
				switch mode {
				case "deleted registry":
					err = os.Remove(o.Registry)
				case "replaced registry", "joined errors":
					err = os.Rename(o.Registry, o.Registry+".preserved")
					if err == nil {
						writeImportSnapshot(t, o, s)
					}
				case "revision":
					s.Revision++
					writeImportSnapshot(t, o, s)
				case "contents":
					s.Imports[0].Digest.Value = strings.Repeat("f", 64)
					writeImportSnapshot(t, o, s)
				case "lock":
					lock := parentPath(o.Registry) + "/ownership.lock"
					err = os.Rename(lock, lock+".preserved")
				case "lock privacy":
					err = os.Chmod(parentPath(o.Registry)+"/ownership.lock", 0o644)
				case "directory privacy":
					err = os.Chmod(parentPath(o.Registry), 0o755)
				case "input", "during digest":
					err = os.WriteFile(string(i.Input), []byte("changed by external writer"), 0o600)
				case "counterpart":
					err = os.WriteFile(string(i.Source), []byte("external occupant"), 0o600)
				case "ancestry":
					err = os.Rename(o.Target, o.Target+".preserved")
				}
				if err != nil {
					t.Fatal(err)
				}
				expected = treeState(t, filepath.Dir(o.Target))
			}
			got, err := inspectRecovery(o.Registry, selection, hook)
			// Hash-time changes may surface as structured evidence failures; late
			// changes must also invalidate the final consistency result.
			if got.Intent.RetirementObserved || err == nil {
				t.Fatal("changed evidence accepted", got, err)
			}
			if !reflect.DeepEqual(expected, treeState(t, filepath.Dir(o.Target))) {
				t.Fatal("inspection changed injected foreign evidence")
			}
			if mode == "joined errors" && !strings.Contains(recoveryDetails(got), "digest mismatch") {
				t.Fatal("earlier evidence error lost", got)
			}
		})
	}
}

func TestRecoveryInspectionHeldLockAndCleanup(t *testing.T) {
	o, _, selection := recoveryFixture(t)
	held, err := openRegistry(o, true, nativeRegistryOps())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	before := recoveryFDs(t, o)
	for range 20 {
		got := requireRecoveryNoWrite(t, o, selection)
		if !got.Intent.RetirementObserved {
			t.Fatal(got)
		}
		if _, err := inspectRecovery(o.Registry, &recoverySelection{"wrong", 0}, nil); err == nil {
			t.Fatal("wrong selection accepted")
		}
	}
	if after := recoveryFDs(t, o); !reflect.DeepEqual(before, after) {
		t.Fatal("inspection leaked descriptors", before, after)
	}
	if other, err := openRegistry(o, true, nativeRegistryOps()); err == nil {
		_ = other.Close()
		t.Fatal("inspection disturbed existing exclusive lock")
	}
}

func TestRecoveryInspectionCapacity(t *testing.T) {
	o, _, s := v2Fixture(t, true)
	original := s.Records[0]
	for n := 0; n < 100; n++ {
		record := original
		record.Destination = pathBytes(fmt.Sprintf("%s/.slot-%d", o.Target, n))
		s.Records = append(s.Records, record)
	}
	base, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	padding := (registrySizeLimit - 256 - len(base)) / 4 * 3
	s.Records[0].Text = pathBytes("/" + strings.Repeat("x", padding))
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil || len(data) >= registrySizeLimit || validateSnapshot(s) != nil {
		t.Fatal("invalid at-capacity fixture", err)
	}
	if _, err := snapshotBytes(s, 0); err == nil {
		t.Fatal("fixture requires insufficient recovery-expanded capacity")
	}
	if err := os.WriteFile(o.Registry, data, 0o600); err != nil {
		t.Fatal(err)
	}
	summary, err := inspectRecovery(o.Registry, nil, nil)
	if err != nil || summary.Intent == nil || summary.Revision != s.Revision {
		t.Fatal("capacity must not prevent reading valid state", err)
	}
	got := requireRecoveryNoWrite(t, o, &recoverySelection{s.Imports[0].ID, s.Revision})
	if got.Intent.RetirementObserved || !strings.Contains(recoveryDetails(got), "recovery capacity") {
		t.Fatal("missing capacity refusal", got)
	}
}

func recoveryFDs(t *testing.T, o Ownership) map[entryID]int {
	t.Helper()
	ids := map[entryID]int{}
	for _, path := range []string{"/", o.Registry, parentPath(o.Registry), parentPath(o.Registry) + "/ownership.lock", o.Target, o.Sources[0], o.Target + "/.import"} {
		var stat unix.Stat_t
		if err := unix.Lstat(path, &stat); err == nil {
			ids[idOf(stat)] = 0
		}
	}
	dir, err := os.Open("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	// Read names while the directory stays open. Go 1.23 ReadDir may stat
	// entries after closing its own descriptor, which is unsafe for /dev/fd.
	fds, err := dir.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range fds {
		fd, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		var stat unix.Stat_t
		if err := unix.Fstat(fd, &stat); err != nil && !errors.Is(err, unix.EBADF) {
			t.Fatal(err)
		} else if err == nil {
			if _, ok := ids[idOf(stat)]; ok {
				ids[idOf(stat)]++
			}
		}
	}
	return ids
}
