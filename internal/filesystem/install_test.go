package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func installationFixture(t *testing.T, names ...string) (string, []Destination) {
	t.Helper()
	root := t.TempDir()
	var destinations []Destination
	for i, name := range names {
		source := filepath.Join(root, "source", fmt.Sprintf("file-%d", i))
		fixtureFile(t, source)
		destinations = append(destinations, Destination{Source: source, Path: filepath.Join(root, "target", name)})
	}
	return root, destinations
}

func treeState(t *testing.T, root string) map[string]string {
	t.Helper()
	state := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%v:%d", info.Mode(), info.ModTime().UnixNano())
		if entry.Type()&os.ModeSymlink != 0 {
			text, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + text
		} else if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += ":" + string(data)
		}
		state[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func assertLink(t *testing.T, path, source string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("not a real symlink at %s: %v", path, err)
	}
	text, err := os.Readlink(path)
	if err != nil || text != source {
		t.Fatalf("link %q = %q, %v; want %q", path, text, err, source)
	}
}

func TestInstallationAndIdempotency(t *testing.T) {
	for _, name := range []string{".gitconfig", ".config/nvim/lua/init.lua"} {
		t.Run(name, func(t *testing.T) {
			root, destinations := installationFixture(t, name)
			sources := treeState(t, filepath.Join(root, "source"))
			prepared, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(root, "target")); !os.IsNotExist(err) {
				t.Fatalf("preparation created target: %v", err)
			}
			// The prepared plan owns its input, not a mutable caller slice.
			chosen := destinations[0]
			destinations[0].Source = "/invalid-caller-mutation"
			report, err := prepared.Apply()
			if err != nil || report.LinksCreated != 1 || report.Results[0].Status != "LINKED" {
				t.Fatalf("install failed: %+v, %v", report, err)
			}
			assertLink(t, chosen.Path, chosen.Source)
			before, err := os.Lstat(chosen.Path)
			if err != nil {
				t.Fatal(err)
			}
			for parent := filepath.Dir(chosen.Path); parent != root; parent = filepath.Dir(parent) {
				info, err := os.Stat(parent)
				if err != nil || info.Mode().Perm()&^os.FileMode(0o700) != 0 {
					t.Fatalf("unexpected created-directory permissions at %s: %v", parent, err)
				}
			}
			again, err := Prepare([]Destination{chosen})
			if err != nil {
				t.Fatal(err)
			}
			second, err := again.Apply()
			after, statErr := os.Lstat(chosen.Path)
			if err != nil || statErr != nil || second.LinksCreated != 0 || second.DirectoriesCreated != 0 ||
				second.Results[0].Status != "OK" || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("idempotency failed: %+v, %v, %v", second, err, statErr)
			}
			if !reflect.DeepEqual(sources, treeState(t, filepath.Join(root, "source"))) {
				t.Fatal("installation modified sources")
			}
		})
	}
}

func TestPrepareFailuresNeverWrite(t *testing.T) {
	for _, mode := range []string{"file", "directory", "wrong link", "broken link", "parent file", "parent link", "source missing", "invalid argument", "structural"} {
		t.Run(mode, func(t *testing.T) {
			root, destinations := installationFixture(t, ".a", ".z/file")
			blocked := destinations[1].Path
			switch mode {
			case "file":
				fixtureFile(t, blocked)
			case "directory":
				if err := os.MkdirAll(blocked, 0o700); err != nil {
					t.Fatal(err)
				}
			case "wrong link", "broken link":
				if err := os.MkdirAll(filepath.Dir(blocked), 0o700); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, "absent")
				if mode == "wrong link" {
					target = destinations[0].Source
				}
				fixtureLink(t, target, blocked)
			case "parent file":
				fixtureFile(t, filepath.Dir(blocked))
			case "parent link":
				if err := os.MkdirAll(filepath.Join(root, "target"), 0o700); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, destinations[0].Source, filepath.Dir(blocked))
			case "source missing":
				destinations[1].Source = filepath.Join(root, "absent")
			case "invalid argument":
				destinations[1].Source += "\x00invalid"
			case "structural":
				destinations = append(destinations, Destination{Path: destinations[0].Path + "/empty", Directory: true})
			}
			before := treeState(t, root)
			ops := nativeInstallOps()
			ops.mkdirat = func(int, string, uint32) error { t.Fatal("mkdir during preparation"); return nil }
			ops.symlinkat = func(string, int, string) error { t.Fatal("symlink during preparation"); return nil }
			if prepared, err := prepare(destinations, ops); err == nil {
				prepared.Close()
				t.Fatal("preflight conflict accepted")
			}
			if !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatal("failed preparation modified fixture")
			}
		})
	}
}

func TestPreparedSourceChanges(t *testing.T) {
	for _, mode := range []string{"remove", "replace", "parent alias", "opaque retarget", "edit contents"} {
		t.Run(mode, func(t *testing.T) {
			root, destinations := installationFixture(t, ".gitconfig")
			source := destinations[0].Source
			if mode == "parent alias" {
				fixtureLink(t, filepath.Join(root, "source"), filepath.Join(root, "alias"))
				destinations[0].Source = filepath.Join(root, "alias", filepath.Base(source))
			}
			if mode == "opaque retarget" {
				alias := filepath.Join(root, "opaque")
				fixtureLink(t, source, alias)
				destinations[0].Source = alias
			}
			prepared, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "remove":
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
			case "replace":
				if err := os.Rename(source, source+".old"); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, source)
			case "parent alias":
				if err := os.Remove(filepath.Join(root, "alias")); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, filepath.Join(root, "other", filepath.Base(source)))
				fixtureLink(t, filepath.Join(root, "other"), filepath.Join(root, "alias"))
			case "opaque retarget":
				if err := os.Remove(destinations[0].Source); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, filepath.Join(root, "other"))
				fixtureLink(t, filepath.Join(root, "other"), destinations[0].Source)
			case "edit contents":
				if err := os.WriteFile(source, []byte("edited in place"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			afterActor := treeState(t, root)
			report, err := prepared.Apply()
			if mode == "edit contents" {
				if err != nil || report.LinksCreated != 1 {
					t.Fatalf("content edits should remain allowed: %+v, %v", report, err)
				}
				assertLink(t, destinations[0].Path, destinations[0].Source)
			} else if err == nil || report.LinksCreated != 0 || report.DirectoriesCreated != 0 || !reflect.DeepEqual(afterActor, treeState(t, root)) {
				t.Fatalf("source change accepted or modified fixture: %+v, %v", report, err)
			}
		})
	}
}

func TestLateAncestorSubstitutionCannotRedirectWrites(t *testing.T) {
	for _, level := range []string{"target", "intermediate"} {
		t.Run(level, func(t *testing.T) {
			root, destinations := installationFixture(t, ".config/.a", ".config/.b", ".config/.c")
			parent := filepath.Join(root, "target", ".config")
			if err := os.MkdirAll(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			external := filepath.Join(root, "external")
			fixtureFile(t, filepath.Join(external, "sentinel"))
			before := treeState(t, external)
			prepared, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			moved := filepath.Join(root, "moved")
			actorRan := false
			prepared.ops.checkpoint = func(phase string, _ Destination) {
				if phase != "before-symlink" || actorRan {
					return
				}
				actorRan = true
				swapped := parent
				if level == "target" {
					swapped = filepath.Dir(parent)
				}
				if err := os.Rename(swapped, moved); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, external, swapped)
			}
			report, err := prepared.Apply()
			if !actorRan || err == nil || report.LinksCreated != 1 || report.Results[0].Status != "LINKED" || report.Results[1].Status != "ERROR" || report.Results[2].Status != "SKIP" {
				t.Fatalf("wrong late-race outcome: %+v, %v", report, err)
			}
			movedParent := moved
			if level == "target" {
				movedParent = filepath.Join(moved, ".config")
			}
			assertLink(t, filepath.Join(movedParent, ".a"), destinations[0].Source)
			if !reflect.DeepEqual(before, treeState(t, external)) {
				t.Fatal("write redirected into external sentinel tree")
			}
		})
	}
}

func TestCreatedParentSubstitution(t *testing.T) {
	for _, mode := range []string{"symlink", "file", "directory before observation", "directory after observation"} {
		t.Run(mode, func(t *testing.T) {
			root, destinations := installationFixture(t, ".a")
			external := filepath.Join(root, "external")
			fixtureFile(t, filepath.Join(external, "sentinel"))
			before := treeState(t, external)
			prepared, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			actorRan := false
			prepared.ops.checkpoint = func(phase string, _ Destination) {
				wanted := "after-mkdir"
				if mode == "directory after observation" {
					wanted = "before-parent-open"
				}
				if phase != wanted || actorRan {
					return
				}
				actorRan = true
				target := filepath.Join(root, "target")
				if err := os.Rename(target, filepath.Join(root, "moved")); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "symlink":
					fixtureLink(t, external, target)
				case "file":
					fixtureFile(t, target)
				default:
					if err := os.Mkdir(target, 0o700); err != nil {
						t.Fatal(err)
					}
					fixtureFile(t, filepath.Join(target, "sentinel"))
				}
			}
			report, err := prepared.Apply()
			if !actorRan || report.DirectoriesCreated != 1 {
				t.Fatalf("missing successful mkdir accounting: %+v, %v", report, err)
			}
			if mode == "directory before observation" {
				if err != nil || report.LinksCreated != 1 {
					t.Fatalf("documented acquisition boundary changed: %+v, %v", report, err)
				}
				assertLink(t, destinations[0].Path, destinations[0].Source)
			} else if err == nil || report.LinksCreated != 0 {
				t.Fatalf("substituted parent accepted: %+v, %v", report, err)
			}
			if !reflect.DeepEqual(before, treeState(t, external)) {
				t.Fatal("external sentinel modified")
			}
		})
	}
}

func TestConcurrentDestinationIsNeverReplaced(t *testing.T) {
	for _, mode := range []string{"file", "directory", "wrong link", "broken link", "correct link"} {
		t.Run(mode, func(t *testing.T) {
			root, destinations := installationFixture(t, ".a")
			if err := os.Mkdir(filepath.Join(root, "target"), 0o700); err != nil {
				t.Fatal(err)
			}
			prepared, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			var afterActor map[string]string
			prepared.ops.checkpoint = func(phase string, destination Destination) {
				if phase != "before-symlink" {
					return
				}
				switch mode {
				case "file":
					fixtureFile(t, destination.Path)
				case "directory":
					if err := os.Mkdir(destination.Path, 0o700); err != nil {
						t.Fatal(err)
					}
				case "wrong link":
					fixtureLink(t, filepath.Join(root, "source"), destination.Path)
				case "broken link":
					fixtureLink(t, filepath.Join(root, "missing"), destination.Path)
				case "correct link":
					fixtureLink(t, destination.Source, destination.Path)
				}
				afterActor = treeState(t, root)
			}
			report, err := prepared.Apply()
			if !errors.Is(err, unix.EEXIST) || report.LinksCreated != 0 || report.DirectoriesCreated != 0 || !reflect.DeepEqual(afterActor, treeState(t, root)) {
				t.Fatalf("concurrent entry changed or miscounted: %+v, %v", report, err)
			}
		})
	}
}

func TestInstallationPartialFailure(t *testing.T) {
	for _, operation := range []string{"mkdir", "symlink", "open after mkdir"} {
		t.Run(operation, func(t *testing.T) {
			second := ".b"
			if operation != "symlink" {
				second = ".b/file"
			}
			root, destinations := installationFixture(t, ".a", second, ".c")
			if err := os.Mkdir(filepath.Join(root, "target"), 0o700); err != nil {
				t.Fatal(err)
			}
			prepared, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "mkdir":
				prepared.ops.mkdirat = func(int, string, uint32) error { return unix.EACCES }
			case "symlink":
				prepared.ops.symlinkat = func(source string, fd int, name string) error {
					if name == ".b" {
						return unix.EIO
					}
					return unix.Symlinkat(source, fd, name)
				}
			case "open after mkdir":
				prepared.ops.openat = func(fd int, name string, flags int, mode uint32) (int, error) { return -1, unix.EMFILE }
			}
			report, err := prepared.Apply()
			if err == nil || report.LinksCreated != 1 || report.Results[1].Status != "ERROR" || report.Results[2].Status != "SKIP" {
				t.Fatalf("incorrect partial result: %+v, %v", report, err)
			}
			wantDirectories := 0
			if operation == "open after mkdir" {
				wantDirectories = 1
			}
			if report.DirectoriesCreated != wantDirectories {
				t.Fatalf("wrong directory count: %+v", report)
			}
			assertLink(t, destinations[0].Path, destinations[0].Source)
			if _, err := os.Lstat(destinations[2].Path); !os.IsNotExist(err) {
				t.Fatalf("later entry attempted: %v", err)
			}
			again, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			secondReport, err := again.Apply()
			if err != nil || secondReport.LinksCreated != 2 || secondReport.Results[0].Status != "OK" {
				t.Fatalf("retry failed: %+v, %v", secondReport, err)
			}
		})
	}
}

func TestPreparedDescriptorsAndLifecycle(t *testing.T) {
	for _, failAt := range []int{0, 1, 4, 8} {
		t.Run(fmt.Sprintf("open failure %d", failAt), func(t *testing.T) {
			root, destinations := installationFixture(t, ".config/file")
			before := treeState(t, root)
			ops := nativeInstallOps()
			openFDs := make(map[int]bool)
			calls := 0
			ops.openat = func(fd int, name string, flags int, mode uint32) (int, error) {
				calls++
				if failAt == calls {
					return -1, unix.EMFILE
				}
				opened, err := unix.Openat(fd, name, flags, mode)
				if err == nil {
					openFDs[opened] = true
				}
				return opened, err
			}
			ops.close = func(fd int) error {
				if !openFDs[fd] {
					t.Fatalf("double-close or unowned descriptor %d", fd)
				}
				delete(openFDs, fd)
				return unix.Close(fd)
			}
			prepared, err := prepare(destinations, ops)
			if err == nil {
				if failAt != 0 && failAt <= calls {
					t.Fatal("injected open error ignored")
				}
				if err := prepared.Close(); err != nil {
					t.Fatal(err)
				}
				if err := prepared.Close(); err != nil {
					t.Fatal(err)
				}
				if _, err := prepared.Apply(); err == nil {
					t.Fatal("closed plan applied")
				}
			} else if !errors.Is(err, unix.EMFILE) {
				t.Fatal(err)
			}
			if len(openFDs) != 0 || !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatal("descriptor leak or preparation wrote files")
			}
		})
	}
}

func TestLateSourceChangeRemainsPathBased(t *testing.T) {
	root, destinations := installationFixture(t, ".a")
	prepared, err := Prepare(destinations)
	if err != nil {
		t.Fatal(err)
	}
	prepared.ops.checkpoint = func(phase string, destination Destination) {
		if phase == "before-symlink" {
			if err := os.Rename(destination.Source, filepath.Join(root, "moved-source")); err != nil {
				t.Fatal(err)
			}
		}
	}
	report, err := prepared.Apply()
	if err != nil || report.LinksCreated != 1 {
		t.Fatalf("wrong path-based boundary: %+v, %v", report, err)
	}
	assertLink(t, destinations[0].Path, destinations[0].Source)
	if _, err := os.Stat(destinations[0].Path); !os.IsNotExist(err) {
		t.Fatalf("source referent was unexpectedly frozen: %v", err)
	}
	if _, err := prepared.Apply(); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatal("prepared plan reused")
	}
}

func TestInstallationExistingLayouts(t *testing.T) {
	for _, mode := range []string{"relative correct", "opaque correct", "ancestor alias", "source traversal", "target traversal"} {
		t.Run(mode, func(t *testing.T) {
			root, destinations := installationFixture(t, ".a")
			if err := os.Mkdir(filepath.Join(root, "target"), 0o755); err != nil {
				t.Fatal(err)
			}
			targetBefore, err := os.Stat(filepath.Join(root, "target"))
			if err != nil {
				t.Fatal(err)
			}
			wantStatus := "LINKED"
			actual := destinations[0].Path
			var original os.FileInfo
			switch mode {
			case "relative correct", "opaque correct":
				text := "../source/file-0"
				if mode == "opaque correct" {
					fixtureLink(t, destinations[0].Source, filepath.Join(root, "opaque"))
					destinations[0].Source = filepath.Join(root, "opaque")
					text = "../opaque"
				}
				fixtureLink(t, text, actual)
				var err error
				original, err = os.Lstat(actual)
				if err != nil {
					t.Fatal(err)
				}
				wantStatus = "OK"
			case "ancestor alias":
				if err := os.Mkdir(filepath.Join(root, "external"), 0o700); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, "../external", filepath.Join(root, "target", ".config"))
				destinations[0].Path = filepath.Join(root, "target", ".config", "file")
				actual = filepath.Join(root, "external", "file")
			case "source traversal", "target traversal":
				if err := os.MkdirAll(filepath.Join(root, "actual", "subdir"), 0o700); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, filepath.Join(root, "actual", "subdir"), filepath.Join(root, "alias"))
				if mode == "source traversal" {
					fixtureFile(t, filepath.Join(root, "actual", "source"))
					destinations[0].Source = root + "/alias/../source"
				} else {
					destinations[0].Path = root + "/alias/../new-target/.a"
					actual = filepath.Join(root, "actual", "new-target", ".a")
				}
			}
			prepared, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			report, err := prepared.Apply()
			if err != nil || report.Results[0].Status != wantStatus {
				t.Fatalf("layout failed: %+v, %v", report, err)
			}
			if wantStatus == "OK" {
				after, err := os.Lstat(actual)
				if err != nil || !os.SameFile(original, after) || report.LinksCreated != 0 || report.DirectoriesCreated != 0 {
					t.Fatalf("correct link replaced: %+v, %v", report, err)
				}
			} else {
				assertLink(t, actual, destinations[0].Source)
			}
			parent, err := os.Stat(filepath.Join(root, "target"))
			if err != nil || parent.Mode().Perm() != targetBefore.Mode().Perm() {
				t.Fatalf("existing permissions changed: %v", err)
			}
		})
	}
}

func TestInstallationPermissionFailures(t *testing.T) {
	for _, phase := range []string{"prepare", "apply"} {
		t.Run(phase, func(t *testing.T) {
			root, destinations := installationFixture(t, ".a")
			parent := filepath.Join(root, "target")
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(parent, 0o700); err != nil {
					t.Error(err)
				}
			})
			deny := func(mode os.FileMode) {
				if err := os.Chmod(parent, mode); err != nil {
					t.Fatal(err)
				}
				probe := filepath.Join(parent, "permission-control")
				file, err := os.OpenFile(probe, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
				if err == nil {
					file.Close()
					if err := os.Remove(probe); err != nil {
						t.Fatal(err)
					}
					t.Skip("filesystem permits writes despite restricted fixture permissions")
				}
				if !os.IsPermission(err) {
					t.Fatal(err)
				}
			}
			if phase == "prepare" {
				deny(0)
				if prepared, err := Prepare(destinations); err == nil {
					prepared.Close()
					t.Fatal("unreadable target accepted")
				}
			} else {
				prepared, err := Prepare(destinations)
				if err != nil {
					t.Fatal(err)
				}
				prepared.ops.checkpoint = func(stage string, _ Destination) {
					if stage == "before-symlink" {
						deny(0o500)
					}
				}
				report, err := prepared.Apply()
				if !errors.Is(err, unix.EACCES) || report.LinksCreated != 0 || report.DirectoriesCreated != 0 {
					t.Fatalf("permission failure hidden: %+v, %v", report, err)
				}
			}
		})
	}
}
