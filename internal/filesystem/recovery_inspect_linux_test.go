package filesystem

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Use existing capabilities only, in an independently verified private namespace.
func TestRecoverySelectedMountView(t *testing.T) {
	if os.Getenv("DOTS_RECOVERY_MOUNT_CHILD") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRecoverySelectedMountView$", "-test.v")
		cmd.Env = append(os.Environ(), "DOTS_RECOVERY_MOUNT_CHILD=1")
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
		t.Logf("%s", out)
		return
	}
	self, err := os.Stat("/proc/self/ns/mnt")
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			t.Skipf("cannot verify mount isolation: %v", err)
		}
		t.Fatal(err)
	}
	parent, err := os.Stat(fmt.Sprintf("/proc/%d/ns/mnt", os.Getppid()))
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			t.Skipf("cannot verify parent namespace: %v", err)
		}
		t.Fatal(err)
	}
	if os.SameFile(self, parent) {
		t.Skip("mount namespace is not independently isolated")
	}
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		if importMountUnavailable(err) {
			t.Skipf("private propagation unavailable: %v", err)
		}
		t.Fatal(err)
	}
	for _, occupied := range []bool{false, true} {
		t.Run(fmt.Sprintf("competing-view=%t", occupied), func(t *testing.T) {
			o, s, selection := recoveryFixture(t)
			i := &s.Imports[0]
			root := o.Sources[0]
			if err := unix.Mount(o.Target, root, "", unix.MS_BIND, ""); err != nil {
				if importMountUnavailable(err) {
					t.Skipf("bind mount unavailable: %v", err)
				}
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := unix.Unmount(root, 0); err != nil {
					t.Error(err)
				}
			})
			p, err := namespace()
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			dir, missing, routes, err := p.walk(root)
			if err != nil || len(missing) != 0 || idOfDirectory(dir) != i.InputBinding.Parent {
				t.Fatal("fixture did not establish shared directory identity", err)
			}
			binding := importBinding{idOfDirectory(dir), recordRoutes(routes)}
			i.SourceAnchor.importBinding, i.SourceBinding = binding, &binding
			if occupied {
				// The base view holds the original, but the selected source view
				// holds another file at the same basename and parent inode.
				if err := os.Rename(string(i.Input), o.Target+"/"+filepath.Base(string(i.Source))); err != nil {
					t.Fatal(err)
				}
				foreign := filepath.Dir(o.Target) + "/foreign"
				fixtureFile(t, foreign)
				if err := unix.Mount(foreign, string(i.Source), "", unix.MS_BIND, ""); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := unix.Unmount(string(i.Source), 0); err != nil {
						t.Error(err)
					}
				})
			}
			writeImportSnapshot(t, o, s)
			got := requireRecoveryNoWrite(t, o, selection)
			if got.Intent.RetirementObserved == occupied {
				t.Fatal("selected mount view ignored or ordinary alias refused", got.Intent)
			}
		})
	}
}
