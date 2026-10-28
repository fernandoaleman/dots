package filesystem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Only an isolated child with already available mount capabilities may run.
func TestImportSelectedInputMountView(t *testing.T) {
	if os.Getenv("DOTS_COMPLETE_MOUNT_CHILD") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestImportSelectedInputMountView$", "-test.v")
		cmd.Env = append(os.Environ(), "DOTS_COMPLETE_MOUNT_CHILD=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWNS}
		out, err := cmd.CombinedOutput()
		if importMountUnavailable(err) {
			t.Skipf("private mount namespace unavailable: %v", err)
		}
		if err != nil {
			t.Fatalf("isolated mount fixture: %v %s", err, out)
		}
		if strings.Contains(string(out), "--- SKIP") {
			t.Skipf("native mount prerequisite unavailable: %s", out)
		}
		return
	}
	self, err := os.Stat("/proc/self/ns/mnt")
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			t.Skipf("cannot verify child mount namespace through procfs: %v", err)
		}
		t.Fatal(err)
	}
	parent, err := os.Stat(fmt.Sprintf("/proc/%d/ns/mnt", os.Getppid()))
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			t.Skipf("cannot verify parent mount namespace through procfs: %v", err)
		}
		t.Fatal(err)
	}
	if os.SameFile(self, parent) {
		t.Skip("mount namespace is not independent")
	}
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		if importMountUnavailable(err) {
			t.Skipf("private mount propagation unavailable: %v", err)
		}
		t.Fatal(err)
	}
	q := preparationFixture(t, false, 1)
	p := liveImport(t, q)
	original := p.intent.OriginalFile
	alias := filepath.Join(filepath.Dir(q.Root), "input-alias")
	if err := os.Mkdir(alias, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mount(q.Ownership.Target, alias, "", unix.MS_BIND, ""); err != nil {
		if importMountUnavailable(err) {
			t.Skipf("bind mount unavailable: %v", err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = p.Close()
		if err := unix.Unmount(alias, 0); err != nil {
			t.Error("unmount fixture alias", err)
		}
	})
	fd, err := unix.Open(alias, directoryFlags, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the inode-deduplicated parent choosing this equivalent mount.
	old := p.observation.inputParent.fd
	p.observation.inputParent.fd = fd
	if err := unix.Close(old); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(filepath.Dir(q.Root), "other-file")
	fixtureFile(t, replacement)
	var replacementStat unix.Stat_t
	if err := unix.Lstat(replacement, &replacementStat); err != nil {
		t.Fatal(err)
	}
	fired := false
	var registry unix.Stat_t
	var afterInjection map[string]string
	p.ops.checkpoint = func(stage string) {
		if stage == "digest-read" && !fired {
			fired = true
			if err := unix.Mount(replacement, q.Input, "", unix.MS_BIND, ""); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = p.Close()
				if err := unix.Unmount(q.Input, 0); err != nil {
					t.Error("unmount fixture input", err)
				}
			})
			var selected, deduplicated unix.Stat_t
			if err := unix.Lstat(q.Input, &selected); err != nil {
				t.Fatal(err)
			}
			if err := unix.Lstat(filepath.Join(alias, filepath.Base(q.Input)), &deduplicated); err != nil {
				t.Fatal(err)
			}
			if idOf(selected) != idOf(replacementStat) || idOf(deduplicated) != original.Fingerprint.ID || idOf(selected) == idOf(deduplicated) {
				t.Fatal("fixture did not establish different selected and deduplicated file views")
			}
			if err := unix.Lstat(q.Ownership.Registry, &registry); err != nil {
				t.Fatal(err)
			}
			afterInjection = treeState(t, filepath.Dir(q.Root))
		}
	}
	ops := nativeImportCompleteOps()
	renamed := false
	ops.rename = func(int, string, int, string) error { renamed = true; return unix.EIO }
	result, err := p.complete(ops)
	if err == nil || !strings.Contains(err.Error(), "selected input slot changed") || !fired || renamed || result != (importCompletion{}) {
		t.Fatal("selected view bypassed final validation", result, err)
	}
	if _, err := os.Lstat(q.Source); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("source slot modified", err)
	}
	var kept, currentRegistry unix.Stat_t
	if err := unix.Lstat(filepath.Join(alias, filepath.Base(q.Input)), &kept); err != nil || fingerprintOf(kept) != original.Fingerprint {
		t.Fatal("original file through alias changed", err)
	}
	if err := unix.Lstat(replacement, &kept); err != nil || fingerprintOf(kept) != fingerprintOf(replacementStat) {
		t.Fatal("replacement file changed", err)
	}
	if err := unix.Lstat(q.Ownership.Registry, &currentRegistry); err != nil || fingerprintOf(currentRegistry) != fingerprintOf(registry) {
		t.Fatal("pre-relocation refusal changed registry identity or metadata", err)
	}
	if !reflect.DeepEqual(afterInjection, treeState(t, filepath.Dir(q.Root))) {
		t.Fatal("pre-relocation refusal modified registry or fixture contents")
	}
	s := ownershipSnapshot(t, q.Ownership)
	if len(s.Imports) != 1 || !s.Unfinished {
		t.Fatal("pending evidence lost")
	}
}

func importMountUnavailable(err error) bool {
	return errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.ENOTSUP)
}
