package filesystem

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func preparationFixture(t *testing.T, nested bool, version int) ImportRequest {
	t.Helper()
	q := importInspectionFixture(t, nested)
	r, err := openRegistry(q.Ownership, true, nativeRegistryOps())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.save(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if version == 2 {
		s := ownershipSnapshot(t, q.Ownership)
		s.Version, s.Imports = 2, []importIntent{}
		writeImportSnapshot(t, q.Ownership, s)
	}
	return q
}

func assertPreparationInput(t *testing.T, q ImportRequest, before unix.Stat_t, data []byte) {
	t.Helper()
	var after unix.Stat_t
	if err := unix.Lstat(q.Input, &after); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(q.Input)
	if err != nil || !bytes.Equal(got, data) || fingerprintOf(before) != fingerprintOf(after) || before.Nlink != after.Nlink {
		t.Fatal("original import file changed", err)
	}
	if _, err := os.Lstat(q.Source); !errors.Is(err, os.ErrNotExist) && !errors.Is(err, unix.ENOTDIR) {
		t.Fatalf("source file was placed: %v", err)
	}
}

func TestPrivateImportPreparation(t *testing.T) {
	for _, version := range []int{1, 2} {
		for _, layout := range []string{"flat", "missing", "partial", "existing"} {
			t.Run(fmt.Sprintf("v%d/%s", version, layout), func(t *testing.T) {
				q := preparationFixture(t, layout != "flat", version)
				if layout == "partial" {
					if err := os.Mkdir(q.Root+"/config", 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if layout == "existing" {
					if err := os.MkdirAll(parentPath(q.Source), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				// This unrelated M3 regression fixture is not an imported symlink.
				managed := Destination{Path: q.Ownership.Target + "/.managed", Source: q.Root + "/managed"}
				fixtureFile(t, managed.Source)
				installFixture(t, q.Ownership, []Destination{managed})
				q.Entries = append(q.Entries, managed)
				history := ownershipSnapshot(t, q.Ownership).Records
				var original unix.Stat_t
				if err := unix.Lstat(q.Input, &original); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(q.Input)
				if err != nil {
					t.Fatal(err)
				}
				refreshes := 0
				p, err := prepareImport(q, func() (ImportRequest, error) {
					refreshes++
					if refreshes == 2 {
						if other, err := openRegistry(q.Ownership, true, nativeRegistryOps()); err == nil {
							_ = other.Close()
							t.Fatal("refresh ran without exclusive lock")
						}
					}
					return q, nil
				}, nativeImportPrepareOps())
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				s := ownershipSnapshot(t, q.Ownership)
				if refreshes != 2 || s.Version != 2 || !s.Unfinished || len(s.Imports) != 1 || s.Imports[0].SourceBinding == nil || !reflect.DeepEqual(history, s.Records) {
					t.Fatalf("bad prepared state: %+v", s)
				}
				if size, _ := os.Stat(q.Ownership.Registry); size.Size() > int64(p.bound) {
					t.Fatal("reserved bound exceeded")
				}
				// Size a future owned snapshot without creating or publishing a link.
				future := s
				intent := s.Imports[0]
				intent.Phase, intent.RelocatedFile = "relocated", &intent.OriginalFile
				future.Imports = []importIntent{intent}
				leaf := fingerprint{ID: entryID{intent.InputBinding.Parent.Device, math.MaxUint64}, Mode: unix.S_IFLNK | 0o777, Size: int64(len(q.Source)), Changed: math.MinInt64}
				future.Records = append(append([]ownershipRecord{}, s.Records...), ownershipRecord{Profile: intent.Profile, Target: intent.Target, Destination: intent.Input, Text: intent.Source, Phase: "owned", Parent: intent.InputBinding.Parent, Routes: intent.InputBinding.Routes, Leaf: &leaf})
				if err := validateSnapshot(future); err != nil {
					t.Fatal(err)
				}
				if encoded, err := snapshotBytes(future, 0); err != nil || len(encoded) > p.bound || p.steps < 8 {
					t.Fatalf("future transaction not reserved: %v", err)
				}
				fds := []int{p.view.registry.lock, p.view.lock, int(p.observation.input.Fd()), p.selectedFD}
				for _, d := range p.observation.plan.order {
					fds = append(fds, d.fd)
				}
				for _, d := range p.view.registry.space.order {
					fds = append(fds, d.fd)
				}
				if err := p.Close(); err != nil {
					t.Fatal(err)
				}
				if err := p.Close(); err != nil {
					t.Fatal(err)
				}
				for _, fd := range fds {
					if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
						t.Fatalf("descriptor leaked: %d %v", fd, err)
					}
				}
				if err := p.publishPrepared(); err == nil {
					t.Fatal("closed authority reused")
				}
				assertPreparationInput(t, q, original, data)
				before := treeState(t, filepath.Dir(q.Root))
				if _, err := Down(q.Ownership, false); err == nil {
					t.Fatal("pending state allowed removal")
				}
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
					t.Fatal("pending refusal wrote state")
				}
			})
		}
	}
}

func TestImportPreparationRejectedWithoutWrites(t *testing.T) {
	for _, mode := range []string{"missing lock", "pending", "unfinished", "creating", "refresh error", "changed selection", "occupied", "permission", "revision", "busy"} {
		t.Run(mode, func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			ops := nativeImportPrepareOps()
			refresh := func() (ImportRequest, error) { return q, nil }
			var held *registry
			switch mode {
			case "missing lock":
				if err := os.Remove(importLockPath(q.Ownership.Registry)); err != nil {
					t.Fatal(err)
				}
			case "pending":
				_, _, s := v2Fixture(t, false)
				writeImportSnapshot(t, q.Ownership, s)
			case "unfinished":
				s := ownershipSnapshot(t, q.Ownership)
				s.Unfinished = true
				writeImportSnapshot(t, q.Ownership, s)
			case "creating":
				_, _, s := v2Fixture(t, true)
				s.Version, s.Imports, s.Unfinished = 1, nil, false
				s.Records[0].Phase = "creating"
				writeImportSnapshot(t, q.Ownership, s)
			case "refresh error":
				refresh = func() (ImportRequest, error) { return ImportRequest{}, unix.EIO }
			case "changed selection":
				refresh = func() (ImportRequest, error) { fresh := q; fresh.Source += ".changed"; return fresh, nil }
			case "occupied":
				fixtureFile(t, q.Source)
			case "permission":
				ops.inspect.writable = func(int) error { return unix.EROFS }
			case "revision":
				s := ownershipSnapshot(t, q.Ownership)
				s.Revision = math.MaxUint64 - 10
				writeImportSnapshot(t, q.Ownership, s)
			case "busy":
				var err error
				held, err = openRegistry(q.Ownership, true, nativeRegistryOps())
				if err != nil {
					t.Fatal(err)
				}
				defer held.Close()
			}
			writes := 0
			ops.registry.write = func(*os.File, []byte) (int, error) { writes++; return 0, unix.EIO }
			ops.mkdir = func(int, string, uint32) error { t.Fatal("mkdir during refused preflight"); return nil }
			before := treeState(t, filepath.Dir(q.Root))
			if p, err := prepareImport(q, refresh, ops); err == nil {
				_ = p.Close()
				t.Fatal("unsafe preparation admitted")
			}
			if writes != 0 || !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
				t.Fatal("rejected preflight wrote artifacts")
			}
		})
	}
}

func TestImportPreparationPublicationFaults(t *testing.T) {
	for _, stage := range []string{"barrier", "intent", "binding"} {
		for _, fault := range []string{"write", "short write", "file sync", "close", "rename", "directory sync"} {
			t.Run(stage+"/"+fault, func(t *testing.T) {
				q := preparationFixture(t, true, 1)
				var original unix.Stat_t
				if err := unix.Lstat(q.Input, &original); err != nil {
					t.Fatal(err)
				}
				data, _ := os.ReadFile(q.Input)
				ops := nativeImportPrepareOps()
				current := ""
				fired := false
				var directory unix.Stat_t
				if err := unix.Stat(parentPath(q.Ownership.Registry), &directory); err != nil {
					t.Fatal(err)
				}
				ops.registry.write = func(f *os.File, b []byte) (int, error) {
					var s snapshot
					if err := json.Unmarshal(b, &s); err != nil {
						t.Fatal(err)
					}
					current = "barrier"
					if len(s.Imports) != 0 {
						current = "intent"
						if s.Imports[0].SourceBinding != nil {
							current = "binding"
						}
					}
					if current == stage && (fault == "write" || fault == "short write") {
						fired = true
						if fault == "short write" {
							return f.Write(b[:len(b)/2])
						}
						return 0, unix.EIO
					}
					return f.Write(b)
				}
				ops.registry.sync = func(fd int) error {
					var s unix.Stat_t
					if err := unix.Fstat(fd, &s); err != nil {
						return err
					}
					if current == stage && ((fault == "file sync" && s.Mode&unix.S_IFMT == unix.S_IFREG) || (fault == "directory sync" && idOf(s) == idOf(directory))) {
						fired = true
						return unix.EIO
					}
					return unix.Fsync(fd)
				}
				ops.registry.closeFile = func(f *os.File) error {
					err := f.Close()
					if current == stage && fault == "close" {
						fired = true
						return errors.Join(err, unix.EIO)
					}
					return err
				}
				ops.registry.rename = func(a int, x string, b int, y string) error {
					if current == stage && fault == "rename" {
						fired = true
						return unix.EIO
					}
					return unix.Renameat(a, x, b, y)
				}
				if p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, ops); err == nil {
					_ = p.Close()
					t.Fatal("fault accepted")
				}
				if !fired {
					t.Fatal("fault seam not reached")
				}
				assertPreparationInput(t, q, original, data)
				s := ownershipSnapshot(t, q.Ownership)
				if (stage != "barrier" || fault == "directory sync") && !s.Unfinished {
					t.Fatal("barrier lost")
				}
				if stage == "binding" && (len(s.Imports) != 1 || s.Imports[0].Phase != "prepared") {
					t.Fatal("intent lost")
				}
				if stage == "binding" {
					if info, err := os.Stat(parentPath(q.Source)); err != nil || !info.IsDir() {
						t.Fatal("created parents rolled back")
					}
				}
			})
		}
	}
}

func TestImportPreparationCrash(t *testing.T) {
	if point := os.Getenv("DOTS_PREP_CRASH"); point != "" {
		var q ImportRequest
		if err := json.Unmarshal([]byte(os.Getenv("DOTS_PREP_REQUEST")), &q); err != nil {
			t.Fatal(err)
		}
		ops := nativeImportPrepareOps()
		ops.checkpoint = func(stage string) {
			if stage == point {
				os.Exit(79)
			}
		}
		p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, ops)
		if p != nil {
			_ = p.Close()
		}
		t.Fatalf("crash point not reached: %v", err)
	}
	for _, point := range []string{"before-barrier", "barrier", "intent", "mkdir:0", "parent:0", "mkdir:1", "parent:1", "binding"} {
		t.Run(point, func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			data, _ := os.ReadFile(q.Input)
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(q)
			cmd := exec.Command(os.Args[0], "-test.run=^TestImportPreparationCrash$")
			cmd.Env = append(os.Environ(), "DOTS_PREP_CRASH="+point, "DOTS_PREP_REQUEST="+string(encoded))
			output, err := cmd.CombinedOutput()
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != 79 {
				t.Fatalf("child crash: %v %s", err, output)
			}
			assertPreparationInput(t, q, original, data)
			s := ownershipSnapshot(t, q.Ownership)
			if s.Unfinished != (point != "before-barrier") || len(s.Records) != 0 {
				t.Fatal("crash evidence incorrect")
			}
			if point != "before-barrier" && point != "barrier" {
				if s.Version != 2 || len(s.Imports) != 1 || (s.Imports[0].SourceBinding != nil) != (point == "binding") {
					t.Fatal("intent/binding evidence incorrect")
				}
			}
			if point != "before-barrier" {
				before := treeState(t, filepath.Dir(q.Root))
				if p, err := prepareImport(q, func() (ImportRequest, error) { t.Fatal("interrupted work refreshed"); return q, nil }, nativeImportPrepareOps()); err == nil {
					_ = p.Close()
					t.Fatal("interrupted import resumed")
				}
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
					t.Fatal("restart changed interrupted state")
				}
			}
		})
	}
}

func TestImportReservationBoundaries(t *testing.T) {
	_, _, state := v2Fixture(t, true)
	intent := state.Imports[0]
	state.Version, state.Imports, state.Unfinished = 1, nil, false
	leaf := *state.Records[0].Leaf
	state.Records[0].Leaf = &leaf
	leaf.Modified = 1
	state.Records[0].Text = "/"
	bound, steps, err := reserveImport(state, intent)
	if err != nil || steps != importRevisionBudget {
		t.Fatal(bound, steps, err)
	}
	padding := (registrySizeLimit - bound) / 4 * 3
	state.Records[0].Text = pathBytes("/" + strings.Repeat("x", padding))
	bound, _, err = reserveImport(state, intent)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < registrySizeLimit-bound; n++ {
		leaf.Modified *= 10
	}
	bound, _, err = reserveImport(state, intent)
	if err != nil || bound != registrySizeLimit {
		t.Fatalf("at limit: %d %v", bound, err)
	}
	leaf.Modified *= 10
	if _, _, err := reserveImport(state, intent); err == nil {
		t.Fatal("one-byte shortage admitted")
	}
	state.Records = []ownershipRecord{}
	state.Revision = math.MaxUint64 - importRevisionBudget
	if _, _, err := reserveImport(state, intent); err != nil {
		t.Fatal(err)
	}
	state.Revision++
	if _, _, err := reserveImport(state, intent); err == nil {
		t.Fatal("one-revision shortage admitted")
	}
}

func TestImportPreparationParentFailures(t *testing.T) {
	for _, mode := range []string{"mkdir", "occupied", "permissions", "substitution", "symlink substitution", "directory sync", "binding sync:0", "binding sync:1", "original sync", "retained check"} {
		t.Run(mode, func(t *testing.T) {
			q := preparationFixture(t, true, 1)
			ops := nativeImportPrepareOps()
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(q.Input)
			before := treeState(t, filepath.Dir(q.Root))
			sentinel := t.TempDir()
			fixtureFile(t, sentinel+"/keep/nested")
			protected := treeState(t, sentinel)
			inputFD := -1
			ops.inspect.openInput = func(parent int, name string, flags int, mode uint32) (int, error) {
				fd, err := unix.Openat(parent, name, flags, mode)
				inputFD = fd
				return fd, err
			}
			switch mode {
			case "mkdir":
				ops.mkdir = func(int, string, uint32) error { return unix.EACCES }
			case "occupied":
				ops.checkpoint = func(point string) {
					if point == "before-mkdir:0" {
						fixtureFile(t, q.Root+"/config")
					}
				}
			case "permissions":
				ops.checkpoint = func(point string) {
					if point == "mkdir:0" {
						if err := os.Chmod(q.Root+"/config", 0o755); err != nil {
							t.Fatal(err)
						}
					}
				}
			case "substitution":
				ops.checkpoint = func(point string) {
					if point == "parent:0" {
						if err := os.Rename(q.Root+"/config", q.Root+"/moved"); err != nil {
							t.Fatal(err)
						}
						if err := os.Mkdir(q.Root+"/config", 0o700); err != nil {
							t.Fatal(err)
						}
					}
				}
			case "symlink substitution":
				ops.checkpoint = func(point string) {
					if point == "mkdir:0" {
						if err := os.Rename(q.Root+"/config", q.Root+"/moved"); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(sentinel, q.Root+"/config"); err != nil {
							t.Fatal(err)
						}
					}
				}
			case "binding sync:0", "binding sync:1":
				created := q.Root + "/config"
				if mode == "binding sync:1" {
					created += "/nvim"
				}
				ops.registry.sync = func(fd int) error {
					var current, parent unix.Stat_t
					if err := unix.Fstat(fd, &current); err != nil {
						return err
					}
					if _, err := os.Stat(created); err == nil {
						if err := unix.Stat(parentPath(created), &parent); err != nil {
							return err
						}
						if idOf(current) == idOf(parent) {
							return unix.EIO
						}
					}
					return unix.Fsync(fd)
				}
			case "directory sync":
				ops.registry.sync = func(fd int) error {
					var stat unix.Stat_t
					if err := unix.Fstat(fd, &stat); err != nil {
						return err
					}
					if stat.Mode&unix.S_IFMT == unix.S_IFDIR {
						var created unix.Stat_t
						if err := unix.Stat(q.Root+"/config", &created); err == nil && idOf(created) == idOf(stat) {
							return unix.EIO
						}
					}
					return unix.Fsync(fd)
				}
			case "original sync":
				ops.registry.sync = func(fd int) error {
					if fd == inputFD {
						return unix.EIO
					}
					return unix.Fsync(fd)
				}
			case "retained check":
				// Deferred configuration validation fails after descriptor transfer.
				ops.inspect.statConfig = func(name string, stat *unix.Stat_t) error {
					err := unix.Stat(name, stat)
					if inputFD >= 0 {
						return unix.EIO
					}
					return err
				}
			}
			if p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, ops); err == nil {
				_ = p.Close()
				t.Fatal("parent/input failure accepted")
			}
			if inputFD >= 0 {
				if _, err := unix.FcntlInt(uintptr(inputFD), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
					t.Fatal("input descriptor leaked")
				}
			}
			assertPreparationInput(t, q, original, data)
			if !reflect.DeepEqual(protected, treeState(t, sentinel)) {
				t.Fatal("outside sentinel modified")
			}
			if mode == "binding sync:0" {
				if _, err := os.Stat(q.Root + "/config/nvim"); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("continued after containing-parent sync failure")
				}
			}
			s := ownershipSnapshot(t, q.Ownership)
			if mode == "retained check" || mode == "original sync" {
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
					t.Fatal("rejected preflight wrote state")
				}
			} else if !s.Unfinished || len(s.Imports) != 1 || s.Imports[0].SourceBinding != nil {
				t.Fatal("failed parent preparation lost intent")
			}
		})
	}
}
