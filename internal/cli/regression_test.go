package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func assertCollisionPlan(t *testing.T, root, mode, output string) {
	t.Helper()
	blocked := [][2]string{
		{"target/.config", "local/config"},
		{"target/.config/nvim/init.lua", "shared/config/nvim/init.lua"},
	}
	if mode == "aliased structural" {
		blocked[0] = [2]string{"target/.other/nvim", "local/other/nvim"}
	}
	for _, entry := range blocked {
		prefix := fmt.Sprintf("CONFLICT %q -> %q",
			filepath.Join(root, entry[0]), filepath.Join(root, entry[1]))
		if !strings.Contains(output, prefix) {
			t.Fatalf("missing blocked participant %s:\n%s", prefix, output)
		}
	}
	unrelated := fmt.Sprintf("LINK %q -> %q",
		filepath.Join(root, "target/.gitconfig"), filepath.Join(root, "local/gitconfig"))
	summary := "SUMMARY 1 planned, 0 correct, 2 conflicts, 0 warnings, 0 errors, 0 changes made\n"
	if !strings.Contains(output, unrelated) || !strings.HasSuffix(output, summary) ||
		strings.Count(output, "CONFLICT ") != 2 || strings.Count(output, "LINK ") != 1 {
		t.Fatalf("incorrect collision rows or counts:\n%s", output)
	}
}

func TestRunPreservesConfiguredTraversal(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		t.Run(fmt.Sprintf("occupied=%v", occupied), func(t *testing.T) {
			root := t.TempDir()
			work := filepath.Join(root, "work")
			actual := filepath.Join(root, "actual")
			for _, dir := range []string{work, filepath.Join(actual, "subdir")} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			fixtureLink(t, filepath.Join(actual, "subdir"), filepath.Join(work, "alias"))
			fixtureFile(t, filepath.Join(actual, "source", "gitconfig"), "selected")
			target := "alias/../missing/target"
			status, expectedCode := "LINK", 0
			if occupied {
				target = "alias/../target"
				fixtureFile(t, filepath.Join(actual, "target", ".gitconfig"), "unmanaged")
				status, expectedCode = "CONFLICT", 1
			}
			config := filepath.Join(work, "config.toml")
			fixtureFile(t, config, fmt.Sprintf("dotfiles_dirs = ['alias/../source']\ntarget_dir = '%s'", target))
			t.Setenv("HOME", work)
			before := snapshot(t, root)
			var stdout, stderr bytes.Buffer
			code := Run([]string{"--config", config, "up", "--dry-run"}, &stdout, &stderr)
			prefix := fmt.Sprintf("%s %q -> %q", status,
				work+"/"+target+"/.gitconfig", work+"/alias/../source/gitconfig")
			if code != expectedCode || !strings.Contains(stdout.String(), prefix) {
				t.Fatalf("wrong traversal: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("path traversal run modified the fixture")
			}
		})
	}
}

func TestRunUnreadableDestinations(t *testing.T) {
	for _, mode := range []string{"destination", "ancestor"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home"))
			source := "config/gitconfig"
			if mode == "ancestor" {
				source = "config/nvim/init.lua"
			}
			fixtureFile(t, filepath.Join(root, "source", source), "source")
			config := filepath.Join(root, "config.toml")
			fixtureFile(t, config, "dotfiles_dirs = ['source']\ntarget_dir = 'target'")
			locked := filepath.Join(root, "target", ".config")
			if err := os.MkdirAll(locked, 0o700); err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, root)
			if err := os.Chmod(locked, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(locked, 0o700); err != nil {
					t.Error(err)
				}
			})
			if _, err := os.Lstat(filepath.Join(locked, "inaccessible")); !os.IsPermission(err) {
				t.Skipf("fixture does not deny path inspection: %v", err)
			}
			var stdout, stderr bytes.Buffer
			code := Run([]string{"--config", config, "up", "--dry-run"}, &stdout, &stderr)
			if code != 1 || !strings.Contains(stdout.String(), "cannot inspect "+mode) {
				t.Fatalf("permission failure became success: code=%d output=%q", code, stdout.String())
			}
			info, err := os.Lstat(locked)
			if err != nil || info.Mode().Perm() != 0 {
				t.Fatalf("inspection changed restricted permissions: %v", err)
			}
			if err := os.Chmod(locked, 0o700); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("permission-error run modified fixture contents")
			}
		})
	}
}

func TestRunCaseCollisions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	fixtureFile(t, filepath.Join(root, "local", "config", "nvim"), "file")
	fixtureFile(t, filepath.Join(root, "shared", ".config", "NVIM", "init.lua"), "nested")
	fixtureFile(t, filepath.Join(root, "local", "zshrc"), "unrelated")
	probe := filepath.Join(root, "target", "CaseProbe")
	fixtureFile(t, probe, "fixture control")
	original, err := os.Lstat(probe)
	if err != nil {
		t.Fatal(err)
	}
	variant, err := os.Lstat(filepath.Join(root, "target", "caseprobe"))
	insensitive := err == nil && os.SameFile(original, variant)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	fixtureFile(t, config, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'")
	before := snapshot(t, root)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config", config, "up", "--dry-run"}, &stdout, &stderr)
	output := stdout.String()
	unknown := strings.Contains(output, "cannot determine target case sensitivity")
	if insensitive || unknown {
		if code != 1 || strings.Count(output, "CONFLICT ") != 2 ||
			!strings.HasSuffix(output, "SUMMARY 1 planned, 0 correct, 2 conflicts, 0 warnings, 0 errors, 0 changes made\n") {
			t.Fatalf("case collision not blocked: code=%d output=%q", code, output)
		}
	} else if code != 0 || strings.Count(output, "LINK ") != 3 {
		t.Fatalf("case-sensitive paths were conflated: code=%d output=%q", code, output)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("case inspection modified the fixture")
	}
}

func TestRunUnresolvedParentTraversal(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	fixtureFile(t, filepath.Join(root, "source", "gitconfig"), "selected")
	fixtureFile(t, filepath.Join(root, "target", ".gitconfig"), "unmanaged")
	config := filepath.Join(root, "config.toml")
	fixtureFile(t, config, "dotfiles_dirs = ['source']\ntarget_dir = 'missing/../target'")
	before := snapshot(t, root)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config", config, "up", "--dry-run"}, &stdout, &stderr)
	output := stdout.String()
	if code != 1 || !strings.Contains(output, "unresolved parent traversal") ||
		strings.Count(output, "CONFLICT ") != 1 || strings.Contains(output, "LINK ") ||
		!strings.HasSuffix(output, "SUMMARY 0 planned, 0 correct, 1 conflicts, 0 warnings, 0 errors, 0 changes made\n") {
		t.Fatalf("unresolved traversal was not blocked: code=%d stdout=%q stderr=%q",
			code, output, stderr.String())
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("unresolved traversal modified the fixture")
	}
}

func TestRunUnicodeCollisions(t *testing.T) {
	for _, names := range [][2]string{{"É", "e\u0301"}, {"é", "e\u0301"}, {"Straße", "STRASSE"}, {"filename", "file\u200bname"}} {
		t.Run(names[0]+" versus "+names[1], func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home"))
			fixtureFile(t, filepath.Join(root, "local", names[0]), "first")
			fixtureFile(t, filepath.Join(root, "shared", names[1]), "second")
			fixtureFile(t, filepath.Join(root, "local", "zshrc"), "unrelated")
			fixtureFile(t, filepath.Join(root, "control", names[0]), "native lookup control")
			original, err := os.Stat(filepath.Join(root, "control", names[0]))
			if err != nil {
				t.Fatal(err)
			}
			variant, err := os.Stat(filepath.Join(root, "control", names[1]))
			equivalent := err == nil && os.SameFile(original, variant)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			config := filepath.Join(root, "config.toml")
			fixtureFile(t, config, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'")
			before := snapshot(t, root)
			var stdout, stderr bytes.Buffer
			code := Run([]string{"--config", config, "up", "--dry-run"}, &stdout, &stderr)
			output := stdout.String()
			if equivalent || strings.Contains(output, "cannot determine target case sensitivity") {
				if code != 1 || strings.Count(output, "CONFLICT ") != 2 ||
					!strings.HasSuffix(output, "SUMMARY 1 planned, 0 correct, 2 conflicts, 0 warnings, 0 errors, 0 changes made\n") {
					t.Fatalf("Unicode collision not blocked: code=%d stdout=%q stderr=%q", code, output, stderr.String())
				}
			} else if code != 0 || strings.Count(output, "LINK ") != 3 {
				t.Fatalf("distinct Unicode names conflated: code=%d output=%q", code, output)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("Unicode inspection modified the fixture")
			}
		})
	}
}

func TestRunEmptyDirectoryCollisions(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		directory string
		probe     string
		aliased   bool
	}{
		{"identical", "config", ".config", "config", false},
		{"case equivalent", "config", ".CONFIG", "CONFIG", false},
		{"ancestor of empty directory", "config", ".CONFIG/empty", "CONFIG", false},
		{"normalization equivalent", "é", ".e\u0301", "e\u0301", false},
		{"aliased parent", "first/config", "second/config", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home"))
			fixtureFile(t, filepath.Join(root, "local", tt.file), "file")
			fixtureFile(t, filepath.Join(root, "local", "zshrc"), "unrelated")
			if err := os.MkdirAll(filepath.Join(root, "shared", tt.directory), 0o700); err != nil {
				t.Fatal(err)
			}
			equivalent := tt.aliased
			if tt.aliased {
				if err := os.MkdirAll(filepath.Join(root, "namespace"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(root, "target"), 0o700); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{".first", ".second"} {
					fixtureLink(t, "../namespace", filepath.Join(root, "target", name))
				}
			} else {
				fixtureFile(t, filepath.Join(root, "control", tt.file), "native lookup control")
				original, err := os.Stat(filepath.Join(root, "control", tt.file))
				if err != nil {
					t.Fatal(err)
				}
				variant, err := os.Stat(filepath.Join(root, "control", tt.probe))
				equivalent = err == nil && os.SameFile(original, variant)
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
			config := filepath.Join(root, "config.toml")
			fixtureFile(t, config, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'")
			before := snapshot(t, root)
			var stdout, stderr bytes.Buffer
			code := Run([]string{"--config", config, "up", "--dry-run"}, &stdout, &stderr)
			output := stdout.String()
			if equivalent || strings.Contains(output, "cannot determine target case sensitivity") {
				if code != 1 || strings.Count(output, "CONFLICT ") != 1 ||
					!strings.Contains(output, filepath.Join(root, "shared", tt.directory)) ||
					!strings.HasSuffix(output, "SUMMARY 1 planned, 0 correct, 1 conflicts, 0 warnings, 0 errors, 0 changes made\n") {
					t.Fatalf("empty directory evidence lost: code=%d stdout=%q stderr=%q", code, output, stderr.String())
				}
			} else if code != 0 || strings.Count(output, "LINK ") != 2 {
				t.Fatalf("distinct paths conflated: code=%d output=%q", code, output)
			}
			if !strings.Contains(output, fmt.Sprintf("LINK %q", filepath.Join(root, "target", ".zshrc"))) {
				t.Fatalf("unrelated entry lost: %s", output)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("empty-directory inspection modified the fixture")
			}
		})
	}
}
