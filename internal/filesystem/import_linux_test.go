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

// Native bind checks never request additional privileges. Only the child
// operates on mounts, in a new namespace with propagation made private.
func TestInspectImportLinuxBindConfiguration(t *testing.T) {
	if os.Getenv("DOTS_IMPORT_BIND_CHILD") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInspectImportLinuxBindConfiguration$", "-test.v")
		cmd.Env = append(os.Environ(), "DOTS_IMPORT_BIND_CHILD=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWNS}
		out, err := cmd.CombinedOutput()
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) {
			t.Skipf("private mount namespace unavailable with existing capabilities: %v", err)
		}
		if err != nil {
			t.Fatalf("isolated mount test: %v\n%s", err, out)
		}
		if strings.Contains(string(out), "--- SKIP") {
			t.Skipf("native mount prerequisite unavailable:\n%s", out)
		}
		t.Logf("%s", out)
		return
	}
	self, err := os.Stat("/proc/self/ns/mnt")
	if err != nil {
		t.Skipf("cannot verify mount isolation: %v", err)
	}
	parent, err := os.Stat(fmt.Sprintf("/proc/%d/ns/mnt", os.Getppid()))
	if err != nil || os.SameFile(self, parent) {
		t.Skip("mount namespace is not independently isolated")
	}
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) {
			t.Skipf("private propagation unavailable: %v", err)
		}
		t.Fatal(err)
	}
	for _, kind := range []string{"directory", "file", "blocked provider", "state directory", "source directory", "state file", "lock file", "provider forward", "provider reverse", "losing provider", "unrelated bind", "read-only route"} {
		t.Run(kind, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			profile := filepath.Dir(request.Root) + "/active/config.toml"
			fixtureFile(t, profile)
			request.Ownership.Profile = profile
			from, mountpoint := profile, request.Input
			if kind == "directory" {
				from, mountpoint = filepath.Dir(profile), request.Ownership.Target+"/.cfg"
				if err := os.Mkdir(mountpoint, 0o700); err != nil {
					t.Fatal(err)
				}
				request.Input = mountpoint + "/config.toml"
				request.Source = request.Root + "/cfg/config.toml"
			}
			if kind == "state directory" || kind == "source directory" {
				from = filepath.Dir(request.Ownership.Registry)
				leaf := "ownership.json"
				if kind == "source directory" {
					from, leaf = request.Root, "existing"
				}
				fixtureFile(t, from+"/"+leaf)
				mountpoint = request.Ownership.Target + "/.cfg"
				if err := os.Mkdir(mountpoint, 0o700); err != nil {
					t.Fatal(err)
				}
				request.Input, request.Source = mountpoint+"/"+leaf, request.Root+"/cfg/"+leaf
			}
			if kind == "state file" || kind == "lock file" {
				from = request.Ownership.Registry
				if kind == "lock file" {
					from = importLockPath(request.Ownership.Registry)
				}
				fixtureFile(t, from)
			}
			if kind == "blocked provider" {
				from, mountpoint = request.Input, request.Ownership.Target+"/.config"
				fixtureFile(t, mountpoint)
			}
			if kind == "read-only route" {
				work := filepath.Dir(request.Root) + "/work"
				request.Root = work + "/repo"
				if err := os.MkdirAll(request.Root, 0o700); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, work+"/.zshrc")
				request.Ownership.Sources = []string{request.Root}
				from, mountpoint = work, request.Ownership.Target+"/.view"
				if err := os.Mkdir(mountpoint, 0o700); err != nil {
					t.Fatal(err)
				}
				request.Input = mountpoint + "/.zshrc"
				request.Source = request.Root + "/zshrc"
			}
			provider := ""
			if strings.HasPrefix(kind, "provider") || kind == "losing provider" || kind == "unrelated bind" {
				other := filepath.Dir(request.Root) + "/second"
				provider = other + "/bashrc"
				fixtureFile(t, provider)
				request.Ownership.Sources = append(request.Ownership.Sources, other)
				from, mountpoint = provider, request.Input
				if kind == "provider reverse" {
					from, mountpoint = request.Input, provider
				}
				if kind == "unrelated bind" {
					mountpoint = request.Ownership.Target + "/.unrelated"
					fixtureFile(t, mountpoint)
				}
			}
			if kind == "blocked provider" {
				fixtureFile(t, request.Root+"/other")
			}
			if err := unix.Mount(from, mountpoint, "", unix.MS_BIND, ""); err != nil {
				if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || errors.Is(err, unix.ENODEV) {
					t.Skipf("bind mounts unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer func() {
				if err := unix.Unmount(mountpoint, 0); err != nil {
					t.Error(err)
				}
			}()
			if kind == "read-only route" {
				if err := unix.Mount("", mountpoint, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
					t.Fatal(err)
				}
			}
			request.Entries = []Destination{{Path: request.Input, Source: request.Source}}
			want := "active configuration"
			if provider != "" {
				request.Entries = append(request.Entries, Destination{Path: request.Ownership.Target + "/.bashrc", Source: provider})
				if kind == "losing provider" {
					winner := request.Root + "/bashrc"
					fixtureFile(t, winner)
					request.Entries = append(request.Entries, Destination{Path: request.Ownership.Target + "/.bashrc", Source: winner})
				}
				want = "aliases existing source provider"
			}
			if kind == "lock file" {
				want = "stable ownership lock"
			}
			if kind == "read-only route" {
				want = "not writable and searchable"
			}
			if kind == "state directory" || kind == "source directory" {
				want = "protected namespace"
			}
			if kind == "state file" {
				want = "reserved ownership state"
			}
			if kind == "blocked provider" {
				request.Entries = append(request.Entries, Destination{Path: mountpoint + "/child", Source: request.Root + "/other"})
				want = "import overlap"
			}
			before := treeState(t, filepath.Dir(request.Root))
			_, err := InspectImport(request)
			if kind == "unrelated bind" {
				if err != nil {
					t.Fatalf("unrelated bind refused: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("bind-mounted configuration accepted: %v", err)
			}
			if after := treeState(t, filepath.Dir(request.Root)); !reflect.DeepEqual(before, after) {
				t.Fatal("bind inspection modified fixture")
			}
		})
	}
}
