package filesystem

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestImportBootstrapPreparation(t *testing.T) {
	for _, layout := range []string{"fresh", "directory", "lock only", "v1", "v2"} {
		t.Run(layout, func(t *testing.T) {
			q := importInspectionFixture(t, true)
			if layout != "fresh" {
				if err := os.MkdirAll(parentPath(q.Ownership.Registry), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if layout == "lock only" || layout == "v1" || layout == "v2" {
				r, err := openRegistry(q.Ownership, true, nativeRegistryOps())
				if err != nil {
					t.Fatal(err)
				}
				if layout != "lock only" {
					if layout == "v2" {
						r.data.Version, r.data.Imports, r.version = 2, []importIntent{}, 2
					}
					if err := r.save(); err != nil {
						t.Fatal(err)
					}
				}
				if err := r.Close(); err != nil {
					t.Fatal(err)
				}
			}
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(q.Input)
			before := treeState(t, filepath.Dir(q.Root))
			if _, err := InspectImport(q); err != nil || !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
				t.Fatal("preview initialized state", err)
			}
			ops := nativeImportPrepareOps()
			lockSynced, bindingSynced, writes, refreshes := false, false, 0, 0
			barrierVersion := 1
			if layout == "v2" {
				barrierVersion = 2
			}
			ops.registry.sync = func(fd int) error {
				var held, lock, state unix.Stat_t
				if err := unix.Fstat(fd, &held); err != nil {
					return err
				}
				if unix.Lstat(importLockPath(q.Ownership.Registry), &lock) == nil && idOf(held) == idOf(lock) {
					lockSynced = true
				}
				if lockSynced && unix.Lstat(parentPath(q.Ownership.Registry), &state) == nil && idOf(held) == idOf(state) {
					bindingSynced = true
				}
				return unix.Fsync(fd)
			}
			ops.registry.write = func(f *os.File, b []byte) (int, error) {
				if !lockSynced || !bindingSynced {
					t.Fatal("snapshot preceded lock durability")
				}
				var s snapshot
				if err := json.Unmarshal(b, &s); err != nil {
					t.Fatal(err)
				}
				if writes == 0 && (!s.Unfinished || len(s.Imports) != 0 || s.Version != barrierVersion) {
					t.Fatal("first snapshot was not the loaded-version barrier", s)
				}
				writes++
				return f.Write(b)
			}
			p, err := prepareImport(q, func() (ImportRequest, error) {
				refreshes++
				if refreshes == 2 {
					if writes != 0 {
						t.Fatal("snapshot published before authoritative admission")
					}
					assertBootstrapLockHeld(t, q)
					if layout != "v1" && layout != "v2" {
						if _, err := os.Lstat(q.Ownership.Registry); !errors.Is(err, os.ErrNotExist) {
							t.Fatal("bootstrap published an empty registry", err)
						}
					}
				}
				return q, nil
			}, ops)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			state := ownershipSnapshot(t, q.Ownership)
			if refreshes != 2 || writes != 3 || state.Version != 2 || !state.Unfinished || len(state.Imports) != 1 || len(state.Records) != 0 {
				t.Fatal("incorrect prepared bootstrap state", refreshes, writes, state)
			}
			assertPreparationInput(t, q, original, data)
			fds := []int{p.view.registry.lock, p.view.lock, int(p.observation.input.Fd()), p.selectedFD}
			for _, d := range p.view.registry.space.order {
				fds = append(fds, d.fd)
			}
			if err := p.Close(); err != nil {
				t.Fatal(err)
			}
			for _, fd := range fds {
				if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
					t.Fatal("bootstrap descriptor leaked", fd, err)
				}
			}
		})
	}
}

func assertBootstrapLockHeld(t *testing.T, q ImportRequest) {
	t.Helper()
	fd, err := unix.Open(importLockPath(q.Ownership.Registry), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); !errors.Is(err, unix.EWOULDBLOCK) {
		t.Fatal("exclusive lock was not retained", err)
	}
}

func TestImportBootstrapPreliminaryRefusal(t *testing.T) {
	for _, mode := range []string{"nil resolver", "resolver", "selection", "source occupied", "read only", "state overlap", "unsafe directory", "lock symlink", "lock directory", "lock fifo", "registry without lock"} {
		t.Run(mode, func(t *testing.T) {
			q := importInspectionFixture(t, true)
			ops := nativeImportPrepareOps()
			refresh := func() (ImportRequest, error) { return q, nil }
			switch mode {
			case "nil resolver":
				refresh = nil
			case "resolver":
				refresh = func() (ImportRequest, error) { return q, unix.EIO }
			case "selection":
				refresh = func() (ImportRequest, error) { changed := q; changed.Source += "-changed"; return changed, nil }
			case "source occupied":
				fixtureFile(t, q.Source)
			case "read only":
				ops.inspect.writable = func(int) error { return unix.EROFS }
			case "state overlap":
				q.Ownership.Registry = q.Root + "/dots/ownership.json"
			case "unsafe directory", "lock symlink", "lock directory", "lock fifo", "registry without lock":
				if err := os.MkdirAll(parentPath(q.Ownership.Registry), 0o700); err != nil {
					t.Fatal(err)
				}
				if mode == "unsafe directory" {
					if err := os.Chmod(parentPath(q.Ownership.Registry), 0o755); err != nil {
						t.Fatal(err)
					}
				} else if mode == "lock symlink" {
					fixtureLink(t, q.Input, importLockPath(q.Ownership.Registry))
				} else if mode == "lock directory" {
					if err := os.Mkdir(importLockPath(q.Ownership.Registry), 0o700); err != nil {
						t.Fatal(err)
					}
				} else if mode == "lock fifo" {
					if err := unix.Mkfifo(importLockPath(q.Ownership.Registry), 0o600); err != nil {
						t.Fatal(err)
					}
				} else {
					writeImportSnapshot(t, q.Ownership, snapshot{Version: 1, Records: []ownershipRecord{}})
				}
			}
			ops.registry.sync = func(int) error { t.Fatal("sync during rejected preliminary admission"); return nil }
			ops.registry.write = func(*os.File, []byte) (int, error) {
				t.Fatal("write during rejected preliminary admission")
				return 0, nil
			}
			ops.mkdir = func(int, string, uint32) error {
				t.Fatal("source mkdir during rejected preliminary admission")
				return nil
			}
			before := treeState(t, filepath.Dir(q.Root))
			if p, err := prepareImport(q, refresh, ops); err == nil {
				_ = p.Close()
				t.Fatal("invalid preliminary admission accepted")
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
				t.Fatal("preliminary refusal modified the fixture")
			}
		})
	}
}

func TestImportBootstrapFailures(t *testing.T) {
	for _, mode := range []string{"refresh", "directory sync", "ancestor sync", "lock directory sync", "lock sync", "blocked component", "component symlink", "lock substitution", "registry appearance", "registry recapture"} {
		t.Run(mode, func(t *testing.T) {
			q := importInspectionFixture(t, true)
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(q.Input)
			external := filepath.Join(filepath.Dir(q.Root), "external")
			fixtureFile(t, external+"/keep")
			externalBefore := treeState(t, external)
			ops := nativeImportPrepareOps()
			fired, refreshes := false, 0
			var preserved []byte
			ops.registry.write = func(*os.File, []byte) (int, error) { t.Fatal("failure reached snapshot publication"); return 0, nil }
			ops.mkdir = func(int, string, uint32) error { t.Fatal("failure reached source-parent creation"); return nil }
			ops.checkpoint = func(stage string) {
				if stage == "bootstrap-observe" && mode == "registry recapture" {
					fired = true
					preserved, _ = json.Marshal(snapshot{Version: 1, Records: []ownershipRecord{}})
					if err := os.WriteFile(q.Ownership.Registry, preserved, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if stage == "bootstrap" && mode == "lock substitution" {
					fired = true
					lock := importLockPath(q.Ownership.Registry)
					if err := os.Rename(lock, lock+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(lock, []byte("replacement"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			ops.registry.sync = func(fd int) error {
				var held, state, ancestor, lock unix.Stat_t
				if err := unix.Fstat(fd, &held); err != nil {
					return err
				}
				_ = unix.Lstat(parentPath(q.Ownership.Registry), &state)
				_ = unix.Lstat(filepath.Dir(q.Root), &ancestor)
				lockExists := unix.Lstat(importLockPath(q.Ownership.Registry), &lock) == nil
				match := mode == "directory sync" && !lockExists || mode == "ancestor sync" && idOf(held) == idOf(ancestor) ||
					mode == "lock directory sync" && lockExists && idOf(held) == idOf(state) || mode == "lock sync" && lockExists && idOf(held) == idOf(lock)
				if match && !fired {
					fired = true
					return unix.EIO
				}
				if mode == "blocked component" && !fired {
					fired = true
					if err := os.WriteFile(parentPath(q.Ownership.Registry), []byte("occupied"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "component symlink" && !fired {
					fired = true
					if err := os.Symlink(external, parentPath(q.Ownership.Registry)); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "registry appearance" && lockExists && !fired {
					fired = true
					preserved, _ = json.Marshal(snapshot{Version: 1, Records: []ownershipRecord{}})
					if err := os.WriteFile(q.Ownership.Registry, preserved, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return unix.Fsync(fd)
			}
			_, err := prepareImport(q, func() (ImportRequest, error) {
				refreshes++
				if refreshes == 2 && mode == "refresh" {
					fired = true
					return q, unix.EIO
				}
				return q, nil
			}, ops)
			if err == nil || !fired || !strings.Contains(err.Error(), "bootstrap attempted") {
				t.Fatal("bootstrap failure not reported", mode, err)
			}
			assertPreparationInput(t, q, original, data)
			if !reflect.DeepEqual(externalBefore, treeState(t, external)) {
				t.Fatal("bootstrap followed a substituted component into an external fixture")
			}
			if _, err := os.Lstat(q.Root + "/config"); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("source parents created before intent", err)
			}
			if preserved != nil {
				got, err := os.ReadFile(q.Ownership.Registry)
				if err != nil || !bytes.Equal(got, preserved) {
					t.Fatal("unexpected registry overwritten", err)
				}
			} else if _, err := os.Lstat(q.Ownership.Registry); !errors.Is(err, os.ErrNotExist) && !errors.Is(err, unix.ENOTDIR) {
				t.Fatal("bootstrap published a registry", err)
			}
		})
	}
}

func TestImportBootstrapChangedEvidence(t *testing.T) {
	for _, mode := range []string{"bootstrap ancestor", "ancestor", "registry deleted", "registry replaced"} {
		t.Run(mode, func(t *testing.T) {
			var q ImportRequest
			if mode == "bootstrap ancestor" {
				q = importInspectionFixture(t, true)
				if err := os.Mkdir(parentPath(parentPath(q.Ownership.Registry)), 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				q = preparationFixture(t, true, 1)
			}
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(q.Input)
			ops := nativeImportPrepareOps()
			fired := false
			var preserved []byte
			ops.checkpoint = func(stage string) {
				if fired || mode == "bootstrap ancestor" && stage != "before-bootstrap" || mode != "bootstrap ancestor" && stage != "locked" {
					return
				}
				fired = true
				if mode == "ancestor" || mode == "bootstrap ancestor" {
					dir := parentPath(q.Ownership.Registry)
					if mode == "bootstrap ancestor" {
						dir = parentPath(dir)
					}
					if err := os.Rename(dir, dir+".old"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(dir, 0o700); err != nil {
						t.Fatal(err)
					}
					return
				}
				preserved, _ = os.ReadFile(q.Ownership.Registry)
				if err := os.Rename(q.Ownership.Registry, q.Ownership.Registry+".old"); err != nil {
					t.Fatal(err)
				}
				if mode == "registry replaced" {
					if err := os.WriteFile(q.Ownership.Registry, preserved, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			ops.registry.write = func(*os.File, []byte) (int, error) { t.Fatal("changed registry overwritten"); return 0, nil }
			if p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, ops); err == nil || !fired {
				if p != nil {
					_ = p.Close()
				}
				t.Fatal("changed evidence accepted", err)
			}
			assertPreparationInput(t, q, original, data)
			if mode == "registry replaced" {
				got, err := os.ReadFile(q.Ownership.Registry)
				if err != nil || !bytes.Equal(got, preserved) {
					t.Fatal("replacement registry not preserved", err)
				}
			}
		})
	}
}

func TestImportBootstrapAuthoritativeChanges(t *testing.T) {
	for _, mode := range []string{"selection", "source occupied"} {
		t.Run(mode, func(t *testing.T) {
			q := importInspectionFixture(t, false)
			var original, external unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(q.Input)
			ops := nativeImportPrepareOps()
			ops.registry.write = func(*os.File, []byte) (int, error) {
				t.Fatal("changed locked admission published state")
				return 0, nil
			}
			refreshes := 0
			p, err := prepareImport(q, func() (ImportRequest, error) {
				refreshes++
				if refreshes == 2 {
					assertBootstrapLockHeld(t, q)
					if mode == "selection" {
						changed := q
						changed.Source += "-changed"
						return changed, nil
					}
					if err := os.WriteFile(q.Source, []byte("external configuration"), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := unix.Lstat(q.Source, &external); err != nil {
						t.Fatal(err)
					}
				}
				return q, nil
			}, ops)
			if p != nil {
				_ = p.Close()
			}
			if err == nil || refreshes != 2 {
				t.Fatal("locked change was not refused", refreshes, err)
			}
			var current unix.Stat_t
			got, readErr := os.ReadFile(q.Input)
			if readErr != nil || !bytes.Equal(got, data) || unix.Lstat(q.Input, &current) != nil || fingerprintOf(current) != fingerprintOf(original) {
				t.Fatal("original input changed", readErr)
			}
			if mode == "source occupied" {
				got, readErr := os.ReadFile(q.Source)
				if readErr != nil || string(got) != "external configuration" || unix.Lstat(q.Source, &current) != nil || fingerprintOf(current) != fingerprintOf(external) {
					t.Fatal("external source occupant changed", readErr)
				}
			}
			if _, err := os.Lstat(q.Ownership.Registry); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("locked refusal published a registry", err)
			}
		})
	}
}

func TestImportBootstrapCrash(t *testing.T) {
	for _, point := range []string{"before-bootstrap", "bootstrap", "before-barrier", "barrier", "intent"} {
		t.Run(point, func(t *testing.T) {
			q := importInspectionFixture(t, true)
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(q.Input)
			encoded, _ := json.Marshal(q)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestImportPreparationCrash$")
			cmd.Env = append(os.Environ(), "DOTS_PREP_CRASH="+point, "DOTS_PREP_REQUEST="+string(encoded))
			out, err := cmd.CombinedOutput()
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != 79 {
				t.Fatalf("crash point %s: %v %s", point, err, out)
			}
			assertPreparationInput(t, q, original, data)
			if point == "barrier" || point == "intent" {
				s := ownershipSnapshot(t, q.Ownership)
				version := 1
				if point == "intent" {
					version = 2
				}
				if !s.Unfinished || s.Version != version || (len(s.Imports) == 1) != (point == "intent") {
					t.Fatal("wrong first-use crash evidence", s)
				}
			} else if _, err := os.Lstat(q.Ownership.Registry); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("pre-barrier crash published an empty registry", err)
			}
			if _, err := os.Lstat(q.Root + "/config"); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("source parents created before durable intent", err)
			}
		})
	}
}

func TestImportBootstrapProcesses(t *testing.T) {
	if role := os.Getenv("DOTS_BOOTSTRAP_CHILD"); role != "" {
		var q ImportRequest
		if err := json.Unmarshal([]byte(os.Getenv("DOTS_BOOTSTRAP_REQUEST")), &q); err != nil {
			t.Fatal(err)
		}
		ops := nativeImportPrepareOps()
		ops.checkpoint = func(stage string) {
			if role == "umask" {
				if stage == "before-bootstrap" {
					unix.Umask(0o777)
				}
				return
			}
			if stage == "preliminary" || stage == "bootstrap" && role == "holder" {
				fmt.Fprintln(os.Stdout, stage)
				if _, err := io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
					t.Fatal(err)
				}
			}
		}
		refreshes := 0
		p, err := prepareImport(q, func() (ImportRequest, error) {
			refreshes++
			if role == "holder" && refreshes == 2 {
				return q, unix.EIO
			}
			return q, nil
		}, ops)
		if p != nil {
			_ = p.Close()
		}
		if err == nil {
			t.Fatal("child unexpectedly completed preparation")
		}
		return
	}
	t.Run("restrictive umask", func(t *testing.T) {
		q := importInspectionFixture(t, true)
		var original unix.Stat_t
		if err := unix.Lstat(q.Input, &original); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(q.Input)
		encoded, _ := json.Marshal(q)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestImportBootstrapProcesses$")
		cmd.Env = append(os.Environ(), "DOTS_BOOTSTRAP_CHILD=umask", "DOTS_BOOTSTRAP_REQUEST="+string(encoded))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("umask child: %v %s", err, out)
		}
		state := parentPath(parentPath(q.Ownership.Registry))
		info, err := os.Lstat(state)
		if err != nil || info.Mode().Perm() != 0 {
			t.Fatal("restrictive bootstrap did not preserve actual permissions", err)
		}
		// Repair only the known disposable fixture so testing cleanup can visit it.
		if err := os.Chmod(state, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(q.Ownership.Registry); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("umask failure published state", err)
		}
		assertPreparationInput(t, q, original, data)
	})
	t.Run("simultaneous first use", func(t *testing.T) {
		q := importInspectionFixture(t, true)
		var original unix.Stat_t
		if err := unix.Lstat(q.Input, &original); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(q.Input)
		encoded, _ := json.Marshal(q)
		start := func(role string) (*exec.Cmd, io.WriteCloser, *bufio.Reader) {
			t.Helper()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestImportBootstrapProcesses$")
			cmd.Env = append(os.Environ(), "DOTS_BOOTSTRAP_CHILD="+role, "DOTS_BOOTSTRAP_REQUEST="+string(encoded))
			input, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			output, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { cancel(); _ = input.Close(); _ = cmd.Wait() })
			reader := bufio.NewReader(output)
			if line, err := reader.ReadString('\n'); err != nil || line != "preliminary\n" {
				t.Fatal("child did not reach preliminary admission", role, line, err)
			}
			return cmd, input, reader
		}
		a, aInput, aOutput := start("holder")
		b, bInput, bOutput := start("contender")
		if _, err := aInput.Write([]byte("a")); err != nil {
			t.Fatal(err)
		}
		if line, err := aOutput.ReadString('\n'); err != nil || line != "bootstrap\n" {
			t.Fatal("holder did not acquire bootstrap lock", line, err)
		}
		assertBootstrapLockHeld(t, q)
		lock, err := os.Lstat(importLockPath(q.Ownership.Registry))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bInput.Write([]byte("b")); err != nil {
			t.Fatal(err)
		}
		if out, err := io.ReadAll(bOutput); err != nil {
			t.Fatal(err, string(out))
		}
		if err := b.Wait(); err != nil {
			t.Fatal("contender did not refuse safely", err)
		}
		if p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, nativeImportPrepareOps()); err == nil || !strings.Contains(err.Error(), "busy") {
			if p != nil {
				_ = p.Close()
			}
			t.Fatal("held bootstrap lock did not block another preparer", err)
		}
		if _, err := aInput.Write([]byte("a")); err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadAll(aOutput); err != nil {
			t.Fatal(err)
		}
		if err := a.Wait(); err != nil {
			t.Fatal("holder failed its refusal assertion", err)
		}
		after, err := os.Lstat(importLockPath(q.Ownership.Registry))
		if err != nil || !os.SameFile(lock, after) {
			t.Fatal("first-use processes replaced the stable lock", err)
		}
		if _, err := os.Lstat(q.Ownership.Registry); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("failed first use published an empty snapshot", err)
		}
		p, err := prepareImport(q, func() (ImportRequest, error) { return q, nil }, nativeImportPrepareOps())
		if err != nil {
			t.Fatal("fresh retry could not acquire retained lock", err)
		}
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		assertPreparationInput(t, q, original, data)
	})
}
