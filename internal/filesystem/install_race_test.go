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

func TestAncestorChangedBeforeApplyPreventsAllWrites(t *testing.T) {
	for _, mode := range []string{"symlink", "replacement directory", "rename only"} {
		t.Run(mode, func(t *testing.T) {
			root, destinations := installationFixture(t, ".a", ".b")
			target := filepath.Join(root, "target")
			if err := os.Mkdir(target, 0o700); err != nil {
				t.Fatal(err)
			}
			fixtureFile(t, filepath.Join(root, "external", "sentinel"))
			prepared, err := Prepare(destinations)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(target, filepath.Join(root, "moved")); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "symlink":
				fixtureLink(t, filepath.Join(root, "external"), target)
			case "replacement directory":
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			afterActor := treeState(t, root)
			report, err := prepared.Apply()
			if err == nil || report.LinksCreated != 0 || report.DirectoriesCreated != 0 || !reflect.DeepEqual(afterActor, treeState(t, root)) {
				t.Fatalf("changed ancestor accepted or modified: %+v, %v", report, err)
			}
		})
	}
}

func TestConcurrentParentCreationAbortsWithoutReplacement(t *testing.T) {
	root, destinations := installationFixture(t, ".a")
	prepared, err := Prepare(destinations)
	if err != nil {
		t.Fatal(err)
	}
	var afterActor map[string]string
	prepared.ops.checkpoint = func(phase string, _ Destination) {
		if phase != "before-mkdir" {
			return
		}
		if err := os.Mkdir(filepath.Join(root, "target"), 0o700); err != nil {
			t.Fatal(err)
		}
		fixtureFile(t, filepath.Join(root, "target", "sentinel"))
		afterActor = treeState(t, root)
	}
	report, err := prepared.Apply()
	if !errors.Is(err, unix.EEXIST) || report.LinksCreated != 0 || report.DirectoriesCreated != 0 || !reflect.DeepEqual(afterActor, treeState(t, root)) {
		t.Fatalf("concurrent parent modified or miscounted: %+v, %v", report, err)
	}
}

func TestApplyReleasesDescriptors(t *testing.T) {
	for _, mode := range []string{"success", "mutation failure", "close failure", "runtime exhaustion"} {
		t.Run(mode, func(t *testing.T) {
			_, destinations := installationFixture(t, ".a")
			ops := nativeInstallOps()
			owned := make(map[int]bool)
			ops.openat = func(fd int, name string, flags int, permissions uint32) (int, error) {
				opened, err := unix.Openat(fd, name, flags, permissions)
				if err == nil {
					owned[opened] = true
				}
				return opened, err
			}
			ops.close = func(fd int) error {
				if !owned[fd] {
					t.Fatal("unowned descriptor closed")
				}
				delete(owned, fd)
				return unix.Close(fd)
			}
			prepared, err := prepare(destinations, ops)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "mutation failure" {
				prepared.ops.symlinkat = func(string, int, string) error { return unix.EIO }
			}
			if mode == "close failure" {
				closeFD := prepared.ops.close
				prepared.ops.close = func(fd int) error { return errors.Join(closeFD(fd), unix.EIO) }
			}
			if mode == "runtime exhaustion" {
				prepared.ops.openat = func(int, string, int, uint32) (int, error) { return -1, unix.EMFILE }
			}
			report, err := prepared.Apply()
			if (err == nil) != (mode == "success") || len(owned) != 0 {
				t.Fatalf("leaked descriptors or wrong error: %+v, %v, %v", report, err, owned)
			}
			if mode != "mutation failure" && mode != "runtime exhaustion" && report.LinksCreated != 1 {
				t.Fatal("completed creation hidden")
			}
			if mode == "runtime exhaustion" && (!errors.Is(err, unix.EMFILE) || report.DirectoriesCreated != 1 || report.LinksCreated != 0) {
				t.Fatalf("runtime exhaustion miscounted: %+v, %v", report, err)
			}
			if err := prepared.Close(); err != nil {
				t.Fatal("repeated close:", err)
			}
			if _, err := prepared.Apply(); err == nil {
				t.Fatal("consumed plan reused")
			}
		})
	}
}

func TestExistingLinkTargetAliasChanges(t *testing.T) {
	for _, timing := range []string{"before apply", "during apply"} {
		for _, target := range []string{"other", "missing"} {
			t.Run(timing+"/"+target, func(t *testing.T) {
				root, destinations := installationFixture(t, ".a", ".b", ".c")
				if err := os.Mkdir(filepath.Join(root, "target"), 0o700); err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(root, "alias")
				fixtureLink(t, "source", alias)
				fixtureLink(t, "../alias/file-1", destinations[1].Path)
				fixtureFile(t, filepath.Join(root, "other", "file-1"))
				prepared, err := Prepare(destinations)
				if err != nil {
					t.Fatal(err)
				}
				retarget := func() {
					if err := os.Remove(alias); err != nil {
						t.Fatal(err)
					}
					fixtureLink(t, target, alias)
				}
				if timing == "before apply" {
					retarget()
				} else {
					prepared.ops.checkpoint = func(phase string, item Destination) {
						if phase == "before-symlink" && item.Path == destinations[0].Path {
							retarget()
						}
					}
				}
				before := treeState(t, root)
				report, err := prepared.Apply()
				if err == nil || !strings.Contains(err.Error(), "destination target changed") || report.DirectoriesCreated != 0 {
					t.Fatalf("changed target accepted: %+v, %v", report, err)
				}
				if timing == "before apply" {
					if report.LinksCreated != 0 || !reflect.DeepEqual(before, treeState(t, root)) {
						t.Fatal("global preflight allowed writes")
					}
				} else if report.LinksCreated != 1 || report.Results[1].Status != "ERROR" || report.Results[2].Status != "SKIP" {
					t.Fatalf("changed target not caught before OK: %+v", report)
				}
				text, err := os.Readlink(destinations[1].Path)
				if err != nil || text != "../alias/file-1" {
					t.Fatal("existing link modified")
				}
			})
		}
	}
}

func TestLateAncestorSubstitutionBeforeMkdir(t *testing.T) {
	root, destinations := installationFixture(t, ".config/nvim/a", ".z")
	target, moved, external := filepath.Join(root, "target"), filepath.Join(root, "moved"), filepath.Join(root, "external")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, filepath.Join(external, "sentinel"))
	before := treeState(t, external)
	prepared, err := Prepare(destinations)
	if err != nil {
		t.Fatal(err)
	}
	prepared.ops.checkpoint = func(phase string, _ Destination) {
		if phase == "before-mkdir" {
			if err := os.Rename(target, moved); err != nil {
				t.Fatal(err)
			}
			fixtureLink(t, external, target)
		}
	}
	report, err := prepared.Apply()
	if err == nil || report.DirectoriesCreated != 1 || report.LinksCreated != 0 || report.Results[1].Status != "SKIP" {
		t.Fatalf("late mkdir substitution mishandled: %+v, %v", report, err)
	}
	if _, err := os.Stat(filepath.Join(moved, ".config")); err != nil {
		t.Fatal("mkdir did not use pinned directory:", err)
	}
	if _, err := os.Lstat(filepath.Join(moved, ".config", "nvim")); !os.IsNotExist(err) {
		t.Fatal("continued after observed substitution")
	}
	if !reflect.DeepEqual(before, treeState(t, external)) {
		t.Fatal("external directory modified")
	}
}

func TestPrepareDeduplicatesAliasedParents(t *testing.T) {
	root, destinations := installationFixture(t, ".a", ".b")
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	fixtureLink(t, target, alias)
	destinations[1].Path = filepath.Join(alias, ".b")
	prepared, err := Prepare(destinations)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	if prepared.entries[0].parent != prepared.entries[1].parent {
		t.Fatal("aliased parents retained separate descriptors")
	}
	seen := make(map[directoryID]bool)
	for _, dir := range prepared.order {
		var stat unix.Stat_t
		if err := unix.Fstat(dir.fd, &stat); err != nil {
			t.Fatal(err)
		}
		if seen[identity(stat)] {
			t.Fatal("duplicate directory descriptor retained")
		}
		seen[identity(stat)] = true
	}
}

func TestCreatedDirectoryMovedToAnotherPlannedName(t *testing.T) {
	root, destinations := installationFixture(t, ".first/a", ".second/b")
	prepared, err := Prepare(destinations)
	if err != nil {
		t.Fatal(err)
	}
	prepared.ops.checkpoint = func(phase string, item Destination) {
		if phase == "before-symlink" && item.Path == destinations[0].Path {
			if err := os.Rename(filepath.Join(root, "target", ".first"), filepath.Join(root, "target", ".second")); err != nil {
				t.Fatal(err)
			}
		}
	}
	report, err := prepared.Apply()
	if err == nil || report.LinksCreated != 1 || report.Results[1].Status != "ERROR" {
		t.Fatalf("directory rename was silently accepted: %+v, %v", report, err)
	}
	assertLink(t, filepath.Join(root, "target", ".second", "a"), destinations[0].Source)
	if _, err := os.Lstat(destinations[1].Path); !os.IsNotExist(err) {
		t.Fatal("later operation used relocated parent")
	}
}

func TestDescriptorNameCapabilitiesMatchInspection(t *testing.T) {
	root := t.TempDir()
	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	want, pathErr := directoryNameRules(root)
	got, descriptorErr := directoryNameRulesFD(int(dir.Fd()))
	if (pathErr == nil) != (descriptorErr == nil) || pathErr == nil && got != want {
		t.Fatalf("descriptor capabilities disagree with inspection: path=%+v %v descriptor=%+v %v", want, pathErr, got, descriptorErr)
	}
}

func TestPrepareNameEquivalencePreventsWrites(t *testing.T) {
	for _, names := range [][2]string{{"Case", "CASE"}, {"é", "e\u0301"}, {"Straße", "STRASSE"}} {
		t.Run(names[0]+" versus "+names[1], func(t *testing.T) {
			root, destinations := installationFixture(t, "."+names[0], "."+names[1])
			fixtureFile(t, filepath.Join(root, "control", names[0]))
			original, err := os.Stat(filepath.Join(root, "control", names[0]))
			if err != nil {
				t.Fatal(err)
			}
			variant, err := os.Stat(filepath.Join(root, "control", names[1]))
			equivalent := err == nil && os.SameFile(original, variant)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			before := treeState(t, root)
			prepared, err := Prepare(destinations)
			if err == nil {
				prepared.Close()
				if equivalent {
					t.Fatal("equivalent destinations accepted")
				}
			} else if !equivalent && !strings.Contains(err.Error(), "unavailable") {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatal("name-capability preflight wrote files")
			}
		})
	}
}
