package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func retirementFixture(t *testing.T, mode string) (Ownership, snapshot, recoverySelection) {
	t.Helper()
	o, _, s := v2Fixture(t, true)
	base := s.Records[0]
	for _, phase := range []string{"creating", "removing", "invalidated"} {
		r := base
		r.Profile, r.Target = pathBytes(o.Profile+"."+phase), pathBytes(o.Target+"-historical-"+phase)
		r.Destination, r.Phase = r.Target+"/."+pathBytes(phase), phase
		s.Records = append(s.Records, r)
	}
	i := &s.Imports[0]
	if mode == "source" || mode == "relocated" {
		if err := os.Rename(string(i.Input), string(i.Source)); err != nil {
			t.Fatal(err)
		}
		if mode == "source" {
			i.OriginalFile.Fingerprint.Changed-- // Force the historical ctime gap.
		} else {
			var stat unix.Stat_t
			if err := unix.Lstat(string(i.Source), &stat); err != nil {
				t.Fatal(err)
			}
			f := importFile{fingerprintOf(stat), stat.Uid, stat.Gid, uint64(stat.Nlink)}
			i.Phase, i.RelocatedFile = "relocated", &f
		}
	}
	if mode == "missing" {
		i.Source, i.SourceBinding = pathBytes(o.Sources[0]+"/new/nested/import"), nil
		i.SourceAnchor.Missing = []pathBytes{"new", "nested"}
	}
	i.Profile = pathBytes(o.Profile + ".missing-live-config")
	sort.Slice(s.Records, func(i, j int) bool { return s.Records[i].Destination < s.Records[j].Destination })
	writeImportSnapshot(t, o, s)
	return o, s, recoverySelection{i.ID, s.Revision}
}

func retirementUserState(t *testing.T, o Ownership) map[string]string {
	t.Helper()
	state := treeState(t, o.Target)
	for k, v := range treeState(t, o.Sources[0]) {
		state[k] = v
	}
	for path, value := range state {
		var stat unix.Stat_t
		if err := unix.Lstat(path, &stat); err != nil {
			t.Fatal(err)
		}
		state[path] = fmt.Sprintf("%s:%v:%d:%d:%d", value, fingerprintOf(stat), stat.Uid, stat.Gid, stat.Nlink)
	}
	return state
}

func assertRetirementHistory(t *testing.T, initial, current snapshot) {
	t.Helper()
	want := append([]ownershipRecord{}, initial.Records...)
	for n := range want {
		if want[n].Phase == "owned" || want[n].Phase == "removing" {
			want[n].Phase = "invalidated"
		}
	}
	if !current.Unfinished || current.Version != 2 || !reflect.DeepEqual(want, current.Records) {
		t.Fatal("barrier/history was lost or authority restored", current)
	}
}

func TestPrivateRetirement(t *testing.T) {
	for _, mode := range []string{"original", "source", "relocated", "missing", "read-only parent", "last two revisions"} {
		t.Run(mode, func(t *testing.T) {
			o, initial, selection := retirementFixture(t, mode)
			if mode == "read-only parent" {
				if err := os.Chmod(o.Target, 0o555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(o.Target, 0o700) })
			}
			if mode == "last two revisions" {
				initial.Revision, selection.Revision = math.MaxUint64-2, math.MaxUint64-2
				writeImportSnapshot(t, o, initial)
			}
			before := retirementUserState(t, o)
			ops := nativeRecoveryRetireOps()
			fds := map[int]bool{}
			ops.registry.sync = func(fd int) error { fds[fd] = true; return unix.Fsync(fd) }
			writes := 0
			ops.registry.write = func(f *os.File, b []byte) (int, error) { writes++; return f.Write(b) }
			ops.checkpoint = func(stage string) {
				if stage == "locked" {
					if other, err := openRegistry(o, true, nativeRegistryOps()); err == nil {
						_ = other.Close()
						t.Fatal("retirement did not retain exclusive lock")
					}
				}
			}
			report, err := retireImport(o.Registry, selection, ops)
			if err != nil || !report.Invalidated || !report.Retired || !report.BarrierPending || writes != 2 || report.Revision != initial.Revision+2 {
				t.Fatal(report, writes, err)
			}
			current := ownershipSnapshot(t, o)
			assertRetirementHistory(t, initial, current)
			if len(current.Imports) != 0 || !reflect.DeepEqual(before, retirementUserState(t, o)) {
				t.Fatal("retirement altered user files or retained an intent")
			}
			assertRetirementClosed(t, o, fds)
			survivor := string(initial.Imports[0].Input)
			if mode == "source" || mode == "relocated" {
				survivor = string(initial.Imports[0].Source)
			}
			if report.Survivor != survivor {
				t.Fatal("wrong surviving location", report)
			}
			unchanged := treeState(t, filepath.Dir(o.Target))
			if _, err := retireImport(o.Registry, selection, nativeRecoveryRetireOps()); err == nil || !reflect.DeepEqual(unchanged, treeState(t, filepath.Dir(o.Target))) {
				t.Fatal("completed retirement was retried or settled")
			}
		})
	}
}

func assertRetirementClosed(t *testing.T, o Ownership, fds map[int]bool) {
	t.Helper()
	// Snapshot publication intentionally changes the registry inode; compare
	// open counts rather than the zero-valued identity keys of absent FDs.
	for id, count := range recoveryFDs(t, o) {
		if count != 0 {
			t.Fatal("retirement retained descriptor", id, count)
		}
	}
	for fd := range fds {
		if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
			t.Fatal("retirement descriptor escaped", fd, err)
		}
	}
}

func TestRetirementPreliminaryRefusals(t *testing.T) {
	for _, mode := range []string{"id", "revision", "one revision", "v1", "no import", "both", "neither", "symlink", "directory", "fifo", "identity", "digest", "metadata", "hard link", "ancestry", "lock", "lock replacement", "missing state", "malformed", "binding", "frontier", "busy", "capacity"} {
		t.Run(mode, func(t *testing.T) {
			o, s, selection := retirementFixture(t, "original")
			i := &s.Imports[0]
			input, source := string(i.Input), string(i.Source)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			var held *registry
			switch mode {
			case "v1":
				s.Version, s.Imports = 1, nil
				writeImportSnapshot(t, o, s)
			case "no import":
				s.Imports = []importIntent{}
				writeImportSnapshot(t, o, s)
			case "id":
				selection.ID = strings.Repeat("f", 32)
			case "revision":
				selection.Revision++
			case "one revision":
				s.Revision, selection.Revision = math.MaxUint64-1, math.MaxUint64-1
				writeImportSnapshot(t, o, s)
			case "both":
				fixtureFile(t, source)
			case "neither":
				must(os.Remove(input))
			case "symlink", "directory", "fifo", "identity":
				must(os.Rename(input, input+".preserved"))
				switch mode {
				case "symlink":
					must(os.Symlink(source, input))
				case "directory":
					must(os.Mkdir(input, 0o700))
				case "fifo":
					must(unix.Mkfifo(input, 0o600))
				case "identity":
					fixtureFile(t, input)
				}
			case "digest":
				i.Digest.Value = strings.Repeat("0", 64)
				writeImportSnapshot(t, o, s)
			case "metadata":
				must(os.Chmod(input, 0o400))
			case "hard link":
				must(os.Link(input, input+".another"))
			case "ancestry":
				must(os.Rename(o.Target, o.Target+".preserved"))
				must(os.Mkdir(o.Target, 0o700))
			case "lock":
				must(os.Rename(parentPath(o.Registry)+"/ownership.lock", o.Registry+".old-lock"))
			case "missing state":
				o.Registry = filepath.Dir(o.Registry) + "/missing/dots/ownership.json"
			case "malformed":
				must(os.WriteFile(o.Registry, []byte("{"), 0o600))
			case "binding":
				i.SourceBinding = nil
				writeImportSnapshot(t, o, s)
			case "frontier":
				i.Source, i.SourceBinding, i.SourceAnchor.Missing = pathBytes(o.Sources[0]+"/new/import"), nil, []pathBytes{"new"}
				writeImportSnapshot(t, o, s)
				must(os.Mkdir(o.Sources[0]+"/new", 0o700))
			case "busy":
				held, _ = openRegistry(o, true, nativeRegistryOps())
				if held == nil {
					t.Fatal("fixture lock")
				}
				defer held.Close()
			case "capacity":
				o, s = recoveryCapacityFixture(t)
				selection = recoverySelection{s.Imports[0].ID, s.Revision}
			}
			before := treeState(t, filepath.Dir(o.Target))
			ops := nativeRecoveryRetireOps()
			if mode == "lock replacement" {
				ops.checkpoint = func(stage string) {
					if stage == "before-lock" {
						lock := parentPath(o.Registry) + "/ownership.lock"
						must(os.Rename(lock, lock+".preserved"))
						must(os.WriteFile(lock, nil, 0o600))
						before = treeState(t, filepath.Dir(o.Target))
					}
				}
			}
			ops.registry.write = func(*os.File, []byte) (int, error) { t.Fatal("rejected preflight wrote state"); return 0, unix.EIO }
			ops.registry.sync = func(int) error { t.Fatal("rejected preflight synchronized"); return unix.EIO }
			if _, err := retireImport(o.Registry, selection, ops); err == nil || !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Target))) {
				t.Fatal("preliminary refusal wrote files or accepted unsafe evidence", err)
			}
		})
	}
}

func TestRetirementChangedEvidence(t *testing.T) {
	for _, stage := range []string{"before-invalidation", "invalidation", "before-retirement"} {
		for _, change := range []string{"registry missing", "registry replacement", "registry revision", "lock", "contents", "counterpart", "parent"} {
			t.Run(stage+"/"+change, func(t *testing.T) {
				o, initial, selection := retirementFixture(t, "original")
				ops := nativeRecoveryRetireOps()
				var injected map[string]string
				writes := 0
				ops.registry.write = func(f *os.File, b []byte) (int, error) { writes++; return f.Write(b) }
				ops.checkpoint = func(name string) {
					if name != stage {
						return
					}
					var err error
					switch change {
					case "registry missing":
						err = os.Remove(o.Registry)
					case "registry replacement", "registry revision":
						s := ownershipSnapshot(t, o)
						if change == "registry replacement" {
							err = os.Rename(o.Registry, o.Registry+".preserved")
						} else {
							s.Revision++
						}
						if err == nil {
							writeImportSnapshot(t, o, s)
						}
					case "lock":
						err = os.Rename(parentPath(o.Registry)+"/ownership.lock", o.Registry+".preserved-lock")
					case "contents":
						err = os.WriteFile(string(initial.Imports[0].Input), []byte("foreign edit"), 0o600)
					case "counterpart":
						err = os.WriteFile(string(initial.Imports[0].Source), []byte("foreign occupant"), 0o600)
					case "parent":
						err = os.Rename(o.Target, o.Target+".preserved")
					}
					if err != nil {
						t.Fatal(err)
					}
					injected = treeState(t, filepath.Dir(o.Target))
				}
				report, err := retireImport(o.Registry, selection, ops)
				wantWrites := 1
				if stage == "before-invalidation" {
					wantWrites = 0
				}
				if err == nil || report.Retired || writes != wantWrites || injected == nil || !reflect.DeepEqual(injected, treeState(t, filepath.Dir(o.Target))) {
					t.Fatal("changed evidence accepted or overwritten", report, writes, err)
				}
			})
		}
	}
}

func TestRetirementRejectsFreshCtimeGap(t *testing.T) {
	o, initial, selection := retirementFixture(t, "source")
	ops := nativeRecoveryRetireOps()
	var injected map[string]string
	ops.checkpoint = func(stage string) {
		if stage != "invalidation" {
			return
		}
		path := string(initial.Imports[0].Source)
		var before, after unix.Stat_t
		if err := unix.Lstat(path, &before); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, os.FileMode(before.Mode).Perm()); err != nil {
			t.Fatal(err)
		}
		if err := unix.Lstat(path, &after); err != nil {
			t.Fatal(err)
		}
		a, b := importFile{fingerprintOf(before), before.Uid, before.Gid, uint64(before.Nlink)}, importFile{fingerprintOf(after), after.Uid, after.Gid, uint64(after.Nlink)}
		if !sameImportFile(a, b) || a == b {
			t.Fatal("fixture must change only current ctime")
		}
		injected = retirementUserState(t, o)
	}
	report, err := retireImport(o.Registry, selection, ops)
	if err == nil || !report.Invalidated || report.Retired || injected == nil || !reflect.DeepEqual(injected, retirementUserState(t, o)) {
		t.Fatal(report, err)
	}
	current := ownershipSnapshot(t, o)
	assertRetirementHistory(t, initial, current)
	if len(current.Imports) != 1 {
		t.Fatal("fresh ctime change was accepted as historical rename gap")
	}
}

func TestRetirementPersistenceFailures(t *testing.T) {
	for _, publication := range []int{1, 2} {
		for _, failure := range []string{"write", "short write", "file sync", "close", "rename", "directory sync"} {
			t.Run(fmt.Sprintf("publication%d/%s", publication, failure), func(t *testing.T) {
				o, initial, selection := retirementFixture(t, "original")
				before := retirementUserState(t, o)
				ops := nativeRecoveryRetireOps()
				active, published, fired, writes := false, false, false, 0
				ops.registry.write = func(f *os.File, b []byte) (int, error) {
					writes++
					active, published = writes == publication, false
					if active && (failure == "write" || failure == "short write") {
						fired = true
						if failure == "write" {
							return 0, unix.ENOSPC
						}
						return 0, nil
					}
					return f.Write(b)
				}
				ops.registry.sync = func(fd int) error {
					var s unix.Stat_t
					if err := unix.Fstat(fd, &s); err != nil {
						return err
					}
					if active && (failure == "file sync" && s.Mode&unix.S_IFMT == unix.S_IFREG || failure == "directory sync" && published) {
						fired = true
						return unix.EIO
					}
					return unix.Fsync(fd)
				}
				ops.registry.closeFile = func(f *os.File) error {
					err := f.Close()
					if active && failure == "close" {
						fired = true
						return errors.Join(err, unix.EIO)
					}
					return err
				}
				ops.registry.rename = func(a int, b string, c int, d string) error {
					if active && failure == "rename" {
						fired = true
						return unix.EIO
					}
					err := unix.Renameat(a, b, c, d)
					published = err == nil
					return err
				}
				report, err := retireImport(o.Registry, selection, ops)
				if err == nil || !fired || report.Retired || writes != publication || !reflect.DeepEqual(before, retirementUserState(t, o)) {
					t.Fatal(report, writes, fired, err)
				}
				current := ownershipSnapshot(t, o)
				visible := publication - 1
				if failure == "directory sync" {
					visible++
				}
				if visible == 0 {
					if !reflect.DeepEqual(current, initial) {
						t.Fatal("initial snapshot not retained")
					}
				} else {
					assertRetirementHistory(t, initial, current)
				}
				if current.Revision != initial.Revision+uint64(visible) || (len(current.Imports) == 0) != (visible == 2) {
					t.Fatal("wrong visible recovery evidence", current)
				}
			})
		}
	}
}

func TestRetirementSurvivorSyncFailures(t *testing.T) {
	for _, point := range []string{"file", "parent1", "parent2"} {
		t.Run(point, func(t *testing.T) {
			o, initial, selection := retirementFixture(t, "missing")
			before := retirementUserState(t, o)
			ops := nativeRecoveryRetireOps()
			armed, parents, fired := false, 0, false
			ops.checkpoint = func(stage string) {
				if stage == "before-file-sync" && point == "file" {
					armed = true
				}
				if stage == "before-parent-sync" {
					parents++
					armed = point == fmt.Sprintf("parent%d", parents)
				}
			}
			ops.registry.sync = func(fd int) error {
				if armed {
					fired = true
					return unix.EIO
				}
				return unix.Fsync(fd)
			}
			report, err := retireImport(o.Registry, selection, ops)
			if err == nil || !fired || !report.Invalidated || report.Retired || !reflect.DeepEqual(before, retirementUserState(t, o)) {
				t.Fatal(report, err)
			}
			current := ownershipSnapshot(t, o)
			assertRetirementHistory(t, initial, current)
			if len(current.Imports) != 1 || current.Revision != initial.Revision+1 {
				t.Fatal("invalidation evidence not retained", current)
			}
		})
	}
}

func TestRetirementCrash(t *testing.T) {
	if path := os.Getenv("DOTS_RETIRE_TEST_REGISTRY"); path != "" {
		var selection recoverySelection
		if err := json.Unmarshal([]byte(os.Getenv("DOTS_RETIRE_TEST_SELECTION")), &selection); err != nil {
			t.Fatal(err)
		}
		ops := nativeRecoveryRetireOps()
		point := os.Getenv("DOTS_RETIRE_TEST_POINT")
		ops.checkpoint = func(stage string) {
			if stage == point {
				os.Exit(81)
			}
		}
		renamed := 0
		ops.registry.rename = func(a int, b string, c int, d string) error {
			err := unix.Renameat(a, b, c, d)
			renamed++
			if err == nil && renamed == 2 && point == "retirement-visible" {
				os.Exit(81)
			}
			return err
		}
		_, err := retireImport(path, selection, ops)
		t.Fatal("expected abrupt process exit", err)
	}
	for _, point := range []string{"before-invalidation", "invalidation", "file-sync", "parent-sync", "before-retirement", "retirement-visible", "retirement"} {
		t.Run(point, func(t *testing.T) {
			o, initial, selection := retirementFixture(t, "original")
			before := retirementUserState(t, o)
			b, err := json.Marshal(selection)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRetirementCrash$")
			cmd.Env = append(os.Environ(), "DOTS_RETIRE_TEST_REGISTRY="+o.Registry, "DOTS_RETIRE_TEST_SELECTION="+string(b), "DOTS_RETIRE_TEST_POINT="+point)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 81 {
				t.Fatal("crash boundary not reached", err, string(output))
			}
			if !reflect.DeepEqual(before, retirementUserState(t, o)) {
				t.Fatal("process crash changed user data")
			}
			current := ownershipSnapshot(t, o)
			visible := uint64(1)
			if point == "before-invalidation" {
				visible = 0
			}
			if point == "retirement-visible" || point == "retirement" {
				visible = 2
			}
			if visible == 0 {
				if !reflect.DeepEqual(current, initial) {
					t.Fatal("prepublication crash changed state")
				}
			} else {
				assertRetirementHistory(t, initial, current)
			}
			if current.Revision != initial.Revision+visible || (len(current.Imports) == 0) != (visible == 2) {
				t.Fatal("crash evidence lost", current)
			}
			r, err := openRegistry(o, true, nativeRegistryOps())
			if err != nil {
				t.Fatal("crash leaked lock", err)
			}
			_ = r.Close()
		})
	}
}
