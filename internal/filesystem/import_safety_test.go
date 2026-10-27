package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestInspectImportLockIdentity(t *testing.T) {
	for _, mode := range []string{"absent", "distinct", "alias", "unavailable", "late alias", "late error", "early failure"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			lock := importLockPath(request.Ownership.Registry)
			if mode == "distinct" {
				fixtureFile(t, lock)
			}
			before := treeState(t, filepath.Dir(request.Root))
			ops := nativeImportInspectionOps()
			late, calls := false, 0
			ops.statLock = func(path string, stat *unix.Stat_t) error {
				calls++
				if path != lock {
					t.Fatalf("unexpected lock path %q", path)
				}
				if mode == "unavailable" || (late && mode == "late error") {
					return unix.EACCES
				}
				if mode == "alias" || (late && (mode == "late alias" || mode == "early failure")) {
					return unix.Stat(request.Input, stat)
				}
				return unix.Stat(path, stat)
			}
			ops.checkpoint = func() { late = true }
			if mode == "early failure" {
				ops.openInput = func(int, string, int, uint32) (int, error) { late = true; return -1, unix.EIO }
			}
			_, err := inspectImport(request, ops)
			wantError := mode != "absent" && mode != "distinct"
			if (err != nil) != wantError {
				t.Fatalf("lock check: %v", err)
			}
			if wantError && (mode == "unavailable" || mode == "late error") && !errors.Is(err, unix.EACCES) {
				t.Fatalf("lost metadata failure: %v", err)
			}
			if mode == "alias" || mode == "late alias" || mode == "early failure" {
				if err == nil || !strings.Contains(err.Error(), "stable ownership lock") {
					t.Fatalf("lock alias not detected: %v", err)
				}
			}
			if mode == "early failure" && !errors.Is(err, unix.EIO) {
				t.Fatalf("lost original error: %v", err)
			}
			if !wantError && calls != 2 {
				t.Fatalf("lock checked %d times", calls)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("lock inspection wrote to fixture")
			}
		})
	}
}

func TestInspectImportProviderIdentity(t *testing.T) {
	for _, mode := range []string{"distinct", "winning alias", "losing alias", "blocked destination alias", "missing", "unreadable", "late replacement", "late removal", "opaque symlink"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			root2 := filepath.Dir(request.Root) + "/second"
			provider := root2 + "/bashrc"
			fixtureFile(t, provider)
			request.Ownership.Sources = append(request.Ownership.Sources, root2)
			destination := request.Ownership.Target + "/.bashrc"
			request.Entries = append(request.Entries, Destination{Path: destination, Source: provider})
			if mode == "losing alias" {
				winner := request.Root + "/bashrc"
				fixtureFile(t, winner)
				request.Entries = append(request.Entries, Destination{Path: destination, Source: winner})
			}
			if mode == "blocked destination alias" {
				block := request.Ownership.Target + "/.other"
				fixtureFile(t, block)
				request.Entries[len(request.Entries)-1].Path = block + "/child"
			}
			if mode == "missing" {
				if err := os.Rename(provider, provider+".preserved"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "opaque symlink" {
				if err := os.Rename(provider, provider+".preserved"); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, request.Input, provider)
			}
			before := treeState(t, filepath.Dir(request.Root))
			ops := nativeImportInspectionOps()
			ops.statProvider = func(fd int, name string, stat *unix.Stat_t, flags int) error {
				if flags != unix.AT_SYMLINK_NOFOLLOW {
					t.Fatal("provider inspection followed symlink")
				}
				if mode == "unreadable" {
					return unix.EACCES
				}
				err := unix.Fstatat(fd, name, stat, flags)
				if err == nil && name == "bashrc" && (mode == "winning alias" || mode == "losing alias" || mode == "blocked destination alias") {
					var parent, second unix.Stat_t
					if err := unix.Fstat(fd, &parent); err != nil {
						return err
					}
					if err := unix.Stat(root2, &second); err != nil {
						return err
					}
					if identity(parent) == identity(second) {
						return unix.Stat(request.Input, stat)
					}
				}
				return err
			}
			ops.checkpoint = func() {
				if mode == "late replacement" || mode == "late removal" {
					if err := os.Rename(provider, provider+".preserved"); err != nil {
						t.Fatal(err)
					}
					if mode == "late replacement" {
						fixtureFile(t, provider)
					}
				}
			}
			_, err := inspectImport(request, ops)
			wantError := mode != "distinct" && mode != "opaque symlink"
			if (err != nil) != wantError {
				t.Fatalf("provider check: %v", err)
			}
			if strings.Contains(mode, "alias") && (err == nil || !strings.Contains(err.Error(), "aliases existing source provider")) {
				t.Fatalf("provider alias check not reached: %v", err)
			}
			if mode == "losing alias" && (err == nil || !strings.Contains(err.Error(), provider)) {
				t.Fatalf("loser identity not checked: %v", err)
			}
			if strings.HasPrefix(mode, "late") && (err == nil || !strings.Contains(err.Error(), "source provider changed")) {
				t.Fatalf("provider final check not reached: %v", err)
			}
			if mode == "unreadable" && !errors.Is(err, unix.EACCES) {
				t.Fatalf("lost uncertainty: %v", err)
			}
			if !strings.HasPrefix(mode, "late") && !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("provider inspection modified fixture")
			}
		})
	}
}

func TestInspectImportNativePathFailures(t *testing.T) {
	for _, mode := range []string{"query failure", "relative path", "unclean path", "missing path", "identity mismatch", "late query failure", "late mismatch", "early error recheck"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, true)
			other := filepath.Dir(request.Root) + "/other"
			if err := os.Mkdir(other, 0o700); err != nil {
				t.Fatal(err)
			}
			before := treeState(t, filepath.Dir(request.Root))
			ops := nativeImportInspectionOps()
			native := ops.directoryPath
			late := false
			ops.directoryPath = func(fd int) (string, error) {
				switch mode {
				case "query failure":
					return "", unix.ENOTSUP
				case "relative path":
					return "relative", nil
				case "unclean path":
					return other + "/..", nil
				case "missing path":
					return other + "/missing", nil
				case "identity mismatch":
					return other, nil
				case "late query failure", "early error recheck":
					if late {
						return "", unix.EIO
					}
				case "late mismatch":
					if late {
						return other, nil
					}
				}
				return native(fd)
			}
			ops.checkpoint = func() { late = true }
			if mode == "early error recheck" {
				ops.openInput = func(int, string, int, uint32) (int, error) { late = true; return -1, unix.EACCES }
			}
			_, err := inspectImport(request, ops)
			if err == nil {
				t.Fatal("invalid native evidence accepted")
			}
			if mode == "late query failure" || mode == "early error recheck" {
				if !errors.Is(err, unix.EIO) {
					t.Fatalf("lost final query failure: %v", err)
				}
			}
			if mode == "early error recheck" && !errors.Is(err, unix.EACCES) {
				t.Fatalf("lost original error: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("path query wrote to fixture")
			}
		})
	}
}

func TestInspectImportCollectedEvidenceOnError(t *testing.T) {
	for _, mode := range []string{"configuration", "earlier provider"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			ops := nativeImportInspectionOps()
			if mode == "configuration" {
				ops.openInput = func(int, string, int, uint32) (int, error) {
					if err := os.Rename(request.Ownership.Profile, request.Ownership.Profile+".preserved"); err != nil {
						t.Fatal(err)
					}
					fixtureFile(t, request.Ownership.Profile)
					return -1, unix.EACCES
				}
			} else {
				first, second := request.Root+"/a", request.Root+"/b"
				fixtureFile(t, first)
				fixtureFile(t, second)
				request.Entries = append(request.Entries, Destination{Path: request.Ownership.Target + "/.a", Source: first}, Destination{Path: request.Ownership.Target + "/.b", Source: second})
				ops.statProvider = func(fd int, name string, stat *unix.Stat_t, flags int) error {
					if name == "b" {
						if err := os.Rename(first, first+".preserved"); err != nil {
							t.Fatal(err)
						}
						fixtureFile(t, first)
						return unix.EACCES
					}
					return unix.Fstatat(fd, name, stat, flags)
				}
			}
			_, err := inspectImport(request, ops)
			want := "active configuration changed"
			if mode == "earlier provider" {
				want = "source provider changed"
			}
			if !errors.Is(err, unix.EACCES) || !strings.Contains(err.Error(), want) {
				t.Fatalf("lost combined failure evidence: %v", err)
			}
		})
	}
}
