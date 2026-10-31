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
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func settlementFixture(t *testing.T, version int, phase string, unfinished bool) (Ownership, snapshot) {
	t.Helper()
	o, _, s := v2Fixture(t, true)
	s.Version, s.Unfinished, s.Imports = version, unfinished, []importIntent{}
	if version == 1 {
		s.Imports = nil
	}
	if phase == "empty" {
		s.Records = []ownershipRecord{}
	} else {
		s.Records[0].Phase = phase
		if phase == "mixed" {
			s.Records[0].Phase = "invalidated"
			r := s.Records[0]
			r.Phase, r.Profile, r.Target = "creating", pathBytes(o.Profile+".unavailable"), pathBytes(o.Target+"-history")
			r.Destination = r.Target + "/.reserved"
			s.Records = append(s.Records, r)
		}
	}
	sort.Slice(s.Records, func(i, j int) bool { return s.Records[i].Destination < s.Records[j].Destination })
	writeImportSnapshot(t, o, s)
	return o, s
}

func assertSettlementHistory(t *testing.T, before, after snapshot) {
	t.Helper()
	if before.Version != after.Version || !reflect.DeepEqual(before.Records, after.Records) || !reflect.DeepEqual(before.Imports, after.Imports) {
		t.Fatal("settlement changed registry version or historical fields/states", after)
	}
}

func TestPrivateBarrierSettlement(t *testing.T) {
	for _, version := range []int{1, 2} {
		for _, phase := range []string{"empty", "creating", "invalidated", "mixed", "last revision"} {
			t.Run(fmt.Sprintf("v%d/%s", version, phase), func(t *testing.T) {
				p := phase
				if p == "last revision" {
					p = "invalidated"
				}
				o, s := settlementFixture(t, version, p, true)
				if phase == "last revision" {
					s.Revision = math.MaxUint64 - 1
					writeImportSnapshot(t, o, s)
				}
				before := retirementUserState(t, o)
				preview := requireRecoveryNoWrite(t, o, nil)
				if !preview.Unfinished || len(preview.SettlementReasons) != 0 {
					t.Fatal("eligible preview blocked", preview)
				}
				ops := nativeBarrierSettlementOps()
				writes := 0
				fds := map[int]bool{}
				ops.registry.write = func(f *os.File, b []byte) (int, error) { writes++; return f.Write(b) }
				ops.registry.sync = func(fd int) error { fds[fd] = true; return unix.Fsync(fd) }
				ops.checkpoint = func(stage string) {
					if stage == "locked" {
						if other, err := openRegistry(o, true, nativeRegistryOps()); err == nil {
							_ = other.Close()
							t.Fatal("exclusive lock not retained")
						}
					}
				}
				report, err := settleBarrier(o.Registry, s.Revision, ops)
				if err != nil || !report.Settled || report.NoOp || !report.Exists || report.Unfinished || report.Version != version || report.Revision != s.Revision+1 || writes != 1 {
					t.Fatal(report, writes, err)
				}
				current := ownershipSnapshot(t, o)
				assertSettlementHistory(t, s, current)
				if current.Unfinished || !reflect.DeepEqual(before, retirementUserState(t, o)) {
					t.Fatal("barrier not cleared or user entries changed")
				}
				assertRetirementClosed(t, o, fds)
				removal, _ := Down(o, true)
				for _, entry := range removal.Entries {
					if entry.Status == "REMOVE" {
						t.Fatal("settlement restored removal authority", entry)
					}
				}
				if !reflect.DeepEqual(before, retirementUserState(t, o)) {
					t.Fatal("removal preview changed user paths")
				}
				unchanged := treeState(t, filepath.Dir(o.Target))
				again, err := settleBarrier(o.Registry, current.Revision, nativeBarrierSettlementOps())
				if err != nil || !again.NoOp || again.Settled || !reflect.DeepEqual(unchanged, treeState(t, filepath.Dir(o.Target))) {
					t.Fatal("clean retry rewrote state", again, err)
				}
			})
		}
	}
}

func TestSettlementNoOpsAndRefusals(t *testing.T) {
	for _, mode := range []string{"missing", "missing file", "clean owned", "clean exhausted", "stale clean", "stale missing", "stale unfinished", "owned", "removing", "cross-scope owned", "pending", "exhausted", "privacy", "lock missing", "lock symlink", "registry symlink", "malformed", "busy"} {
		t.Run(mode, func(t *testing.T) {
			o, s := settlementFixture(t, 2, "mixed", true)
			revision := s.Revision
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			var held *registry
			wantNoOp := mode == "missing" || mode == "missing file" || mode == "clean owned" || mode == "clean exhausted"
			switch mode {
			case "missing", "stale missing":
				o.Registry = filepath.Dir(o.Registry) + "/absent/dots/ownership.json"
				revision = 0
				if mode == "stale missing" {
					revision = 9
				}
			case "missing file":
				must(os.Rename(o.Registry, o.Registry+".preserved"))
				revision = 0
			case "clean owned", "clean exhausted", "stale clean":
				s.Unfinished = false
				s.Records[0].Phase = "owned"
				if mode == "clean exhausted" {
					s.Revision = math.MaxUint64
				}
				revision = s.Revision
				if mode == "stale clean" {
					revision++
				}
				writeImportSnapshot(t, o, s)
			case "stale unfinished":
				revision++
			case "owned", "removing", "cross-scope owned":
				phase := mode
				index := len(s.Records) - 1
				if mode == "cross-scope owned" {
					phase = "owned"
					index = 0
				}
				s.Records[index].Phase = phase
				writeImportSnapshot(t, o, s)
			case "pending":
				o, s, _ = recoveryFixture(t)
				revision = s.Revision
			case "exhausted":
				s.Revision = math.MaxUint64
				revision = s.Revision
				writeImportSnapshot(t, o, s)
			case "privacy":
				must(os.Chmod(parentPath(o.Registry), 0o755))
			case "lock missing", "lock symlink":
				lock := parentPath(o.Registry) + "/ownership.lock"
				must(os.Rename(lock, lock+".preserved"))
				if mode == "lock symlink" {
					must(os.Symlink(lock+".preserved", lock))
				}
			case "registry symlink":
				must(os.Rename(o.Registry, o.Registry+".preserved"))
				must(os.Symlink(o.Registry+".preserved", o.Registry))
			case "malformed":
				must(os.WriteFile(o.Registry, []byte("{"), 0o600))
			case "busy":
				var err error
				held, err = openRegistry(o, true, nativeRegistryOps())
				must(err)
				defer held.Close()
			}
			before := treeState(t, filepath.Dir(o.Target))
			preview, previewErr := inspectRecovery(o.Registry, nil, nil)
			if !wantNoOp && mode != "stale clean" && mode != "stale missing" && mode != "stale unfinished" && mode != "busy" && previewErr == nil && len(preview.SettlementReasons) == 0 {
				t.Fatal("preview omitted blocker", preview)
			}
			ops := nativeBarrierSettlementOps()
			ops.registry.write = func(*os.File, []byte) (int, error) { t.Fatal("no-op/refusal wrote state"); return 0, unix.EIO }
			ops.registry.sync = func(int) error { t.Fatal("no-op/refusal synchronized"); return unix.EIO }
			report, err := settleBarrier(o.Registry, revision, ops)
			if (err == nil) != wantNoOp || report.Settled || wantNoOp && !report.NoOp || !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Target))) {
				t.Fatal(report, err)
			}
		})
	}
}

func TestSettlementChangedEvidence(t *testing.T) {
	for _, stage := range []string{"before-lock", "locked", "before-settlement", "no-op"} {
		for _, change := range []string{"delete", "replace", "revision", "contents", "lock", "directory", "privacy"} {
			t.Run(stage+"/"+change, func(t *testing.T) {
				o, s := settlementFixture(t, 2, "mixed", stage != "no-op")
				ops := nativeBarrierSettlementOps()
				var injected map[string]string
				ops.checkpoint = func(name string) {
					if name != stage {
						return
					}
					var err error
					switch change {
					case "delete":
						err = os.Remove(o.Registry)
					case "replace":
						err = os.Rename(o.Registry, o.Registry+".preserved")
						if err == nil {
							writeImportSnapshot(t, o, s)
						}
					case "revision":
						s.Revision++
						writeImportSnapshot(t, o, s)
					case "contents":
						s.Records[0].Text += "-changed"
						writeImportSnapshot(t, o, s)
					case "lock":
						lock := parentPath(o.Registry) + "/ownership.lock"
						err = os.Rename(lock, lock+".preserved")
						if err == nil {
							err = os.WriteFile(lock, nil, 0o600)
						}
					case "directory":
						dir := parentPath(o.Registry)
						err = os.Rename(dir, dir+".preserved")
						if err == nil {
							err = os.Mkdir(dir, 0o700)
						}
					case "privacy":
						err = os.Chmod(parentPath(o.Registry), 0o755)
					}
					if err != nil {
						t.Fatal(err)
					}
					injected = treeState(t, filepath.Dir(o.Target))
				}
				ops.registry.write = func(*os.File, []byte) (int, error) { t.Fatal("changed evidence overwritten"); return 0, unix.EIO }
				ops.registry.sync = func(int) error { t.Fatal("changed evidence synchronized"); return unix.EIO }
				report, err := settleBarrier(o.Registry, s.Revision, ops)
				if err == nil || report.Settled || injected == nil || !reflect.DeepEqual(injected, treeState(t, filepath.Dir(o.Target))) {
					t.Fatal(report, err)
				}
			})
		}
	}
}

func TestSettlementPersistenceFailures(t *testing.T) {
	for _, version := range []int{1, 2} {
		for _, fault := range []string{"write", "short write", "file sync", "close", "rename", "directory sync", "temporary replacement"} {
			t.Run(fmt.Sprintf("v%d/%s", version, fault), func(t *testing.T) {
				o, s := settlementFixture(t, version, "mixed", true)
				before := retirementUserState(t, o)
				ops := nativeBarrierSettlementOps()
				fired, published := false, false
				var encoded []byte
				ops.registry.write = func(f *os.File, b []byte) (int, error) {
					encoded = append([]byte(nil), b...)
					if fault == "write" {
						fired = true
						return 0, unix.ENOSPC
					}
					if fault == "short write" {
						fired = true
						return 0, nil
					}
					return f.Write(b)
				}
				ops.registry.sync = func(fd int) error {
					var stat unix.Stat_t
					if err := unix.Fstat(fd, &stat); err != nil {
						return err
					}
					if fault == "file sync" && stat.Mode&unix.S_IFMT == unix.S_IFREG || fault == "directory sync" && published {
						fired = true
						return unix.EIO
					}
					return unix.Fsync(fd)
				}
				ops.registry.closeFile = func(f *os.File) error {
					err := f.Close()
					if fault == "close" {
						fired = true
						return errors.Join(err, unix.EIO)
					}
					if fault == "temporary replacement" {
						fired = true
						name := parentPath(o.Registry) + "/" + f.Name()
						if e := os.Rename(name, name+".preserved"); e != nil {
							t.Fatal(e)
						}
						if e := os.WriteFile(name, encoded, 0o600); e != nil {
							t.Fatal(e)
						}
					}
					return err
				}
				ops.registry.rename = func(a int, b string, c int, d string) error {
					if fault == "rename" {
						fired = true
						return unix.EIO
					}
					err := unix.Renameat(a, b, c, d)
					published = err == nil
					return err
				}
				report, err := settleBarrier(o.Registry, s.Revision, ops)
				if err == nil || !fired || report.Settled || !reflect.DeepEqual(before, retirementUserState(t, o)) {
					t.Fatal(report, err)
				}
				current := ownershipSnapshot(t, o)
				assertSettlementHistory(t, s, current)
				visible := fault == "directory sync"
				if current.Unfinished == visible || current.Revision != s.Revision+uint64(boolInt(visible)) {
					t.Fatal("wrong publication evidence", current)
				}
				if visible && !strings.Contains(err.Error(), "uncertain") {
					t.Fatal("uncertain durability not reported", err)
				}
				assertRetirementClosed(t, o, nil)
			})
		}
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestSettlementCapacity(t *testing.T) {
	o, s := settlementFixture(t, 2, "invalidated", true)
	base, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	old, err := json.Marshal(s.Records[0].Text)
	if err != nil {
		t.Fatal(err)
	}
	padding := (registrySizeLimit + 32 - len(base) + len(old) + 3) / 4 * 3
	s.Records[0].Text = pathBytes("/" + strings.Repeat("x", padding))
	compact, err := json.Marshal(s)
	if err != nil || len(compact) > registrySizeLimit {
		t.Fatal("invalid bounded capacity fixture", err)
	}
	if _, err := snapshotBytes(s, 0); err == nil {
		t.Fatal("fixture must exceed publication capacity")
	}
	if err := os.WriteFile(o.Registry, compact, 0o600); err != nil {
		t.Fatal(err)
	}
	before := treeState(t, filepath.Dir(o.Target))
	preview, err := inspectRecovery(o.Registry, nil, nil)
	if err != nil || len(preview.SettlementReasons) == 0 || !strings.Contains(strings.Join(preview.SettlementReasons, " "), "capacity") {
		t.Fatal("valid capacity-limited state not observable", preview, err)
	}
	ops := nativeBarrierSettlementOps()
	ops.registry.write = func(*os.File, []byte) (int, error) { t.Fatal("capacity refusal wrote state"); return 0, unix.EIO }
	ops.registry.sync = func(int) error { t.Fatal("capacity refusal synchronized"); return unix.EIO }
	if _, err := settleBarrier(o.Registry, s.Revision, ops); err == nil || !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Target))) {
		t.Fatal("capacity refusal changed state", err)
	}
}

func TestBarrierSettlementCrash(t *testing.T) {
	if path := os.Getenv("DOTS_SETTLE_TEST_REGISTRY"); path != "" {
		revision, err := strconv.ParseUint(os.Getenv("DOTS_SETTLE_TEST_REVISION"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		point := os.Getenv("DOTS_SETTLE_TEST_POINT")
		ops := nativeBarrierSettlementOps()
		ops.checkpoint = func(stage string) {
			if stage == point {
				os.Exit(83)
			}
		}
		ops.registry.rename = func(a int, b string, c int, d string) error {
			err := unix.Renameat(a, b, c, d)
			if err == nil && point == "visible" {
				os.Exit(83)
			}
			return err
		}
		_, err = settleBarrier(path, revision, ops)
		t.Fatal("expected process exit", err)
	}
	for _, point := range []string{"before-settlement", "visible", "settlement"} {
		t.Run(point, func(t *testing.T) {
			o, s := settlementFixture(t, 2, "mixed", true)
			before := retirementUserState(t, o)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBarrierSettlementCrash$")
			cmd.Env = append(os.Environ(), "DOTS_SETTLE_TEST_REGISTRY="+o.Registry, "DOTS_SETTLE_TEST_REVISION="+strconv.FormatUint(s.Revision, 10), "DOTS_SETTLE_TEST_POINT="+point)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 83 {
				t.Fatal("crash boundary not reached", err, string(output))
			}
			current := ownershipSnapshot(t, o)
			assertSettlementHistory(t, s, current)
			visible := point != "before-settlement"
			if current.Unfinished == visible || current.Revision != s.Revision+uint64(boolInt(visible)) || !reflect.DeepEqual(before, retirementUserState(t, o)) {
				t.Fatal("crash lost evidence or changed user paths", current)
			}
			r, err := openRegistry(o, true, nativeRegistryOps())
			if err != nil {
				t.Fatal("crash retained lock", err)
			}
			_ = r.Close()
		})
	}
}
