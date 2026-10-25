package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func installationConfig(t *testing.T, root, body string) string {
	t.Helper()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	path := filepath.Join(root, "config.toml")
	fixtureFile(t, path, body)
	return path
}

func TestDryRunDoesNotRequirePreparedDirectoryHandles(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "source", "a"), "source")
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
	before := snapshot(t, root)
	if err := os.Chmod(target, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o700) })
	if dir, err := os.Open(target); err == nil {
		dir.Close()
		t.Skip("permissions do not prevent read-opening directories")
	} else if !os.IsPermission(err) {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(target, ".a")); !os.IsNotExist(err) {
		t.Fatal("directory is not searchable:", err)
	}
	for _, dryRun := range []bool{true, false} {
		args := []string{"--config", config, "up"}
		want := 1
		if dryRun {
			args = append(args, "--dry-run")
			want = 0
		}
		var output, stderr bytes.Buffer
		if code := Run(args, &output, &stderr); code != want || !strings.Contains(output.String(), "0 changes") {
			t.Fatalf("dry-run=%v: code=%d output=%s stderr=%s", dryRun, code, &output, &stderr)
		}
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0o300 {
		t.Fatal("directory permissions changed")
	}
	if err := os.Chmod(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("commands modified fixture")
	}
}

func TestRunInstallsSelectedSources(t *testing.T) {
	for _, mode := range []string{"merge", "dotted warning", "missing root warning", "canonical config", "excluded", "ancestor alias", "source root alias"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			for _, path := range []string{"local/gitconfig", "shared/.gitconfig", "local/config/nvim/init.lua", "shared/.config/nvim/lua/shared.lua", "shared/config/git/config"} {
				fixtureFile(t, filepath.Join(root, path), path)
			}
			body := "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n"
			winner, warnings, links, directories := "local/gitconfig", 0, 4, 5
			switch mode {
			case "dotted warning":
				fixtureFile(t, filepath.Join(root, "local", ".gitconfig"), "dotted")
				winner, warnings = "local/.gitconfig", 1
			case "missing root warning":
				body = "dotfiles_dirs = ['missing', 'local', 'shared']\ntarget_dir = 'target'\n"
				warnings = 1
			case "excluded":
				body += "excludes = ['config/nvim']\n"
				links = 3
			case "ancestor alias":
				if err := os.MkdirAll(filepath.Join(root, "target"), 0o700); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, filepath.Join(root, "external", "sentinel"), "untouched")
				fixtureLink(t, "../external", filepath.Join(root, "target", ".config"))
				directories = 3
			case "source root alias":
				fixtureLink(t, "local", filepath.Join(root, "alias"))
				body = "dotfiles_dirs = ['alias', 'shared']\ntarget_dir = 'target'\n"
				winner = "alias/gitconfig"
			}
			config := installationConfig(t, root, body)
			args := []string{"--config", config, "up"}
			if mode == "canonical config" {
				canonical := filepath.Join(root, "xdg", "dots", "config.toml")
				fixtureFile(t, canonical, fmt.Sprintf("dotfiles_dirs = [%q, %q]\ntarget_dir = %q\n", filepath.Join(root, "local"), filepath.Join(root, "shared"), filepath.Join(root, "target")))
				args = []string{"up"}
			}
			before := snapshot(t, root)
			var dry, dryErr bytes.Buffer
			if code := Run(append(append([]string(nil), args...), "--dry-run"), &dry, &dryErr); code != 0 {
				t.Fatalf("dry-run failed: %d %s %s", code, &dry, &dryErr)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("dry-run wrote files")
			}
			var output, stderr bytes.Buffer
			if code := Run(args, &output, &stderr); code != 0 {
				t.Fatalf("install failed: %d %s %s", code, &output, &stderr)
			}
			want := fmt.Sprintf("SUMMARY %d linked, 0 correct, 0 conflicts, 0 skipped, %d warnings, 0 errors, %d directories created, %d changes made\n", links, warnings, directories, links+directories)
			if !strings.HasSuffix(output.String(), want) {
				t.Fatalf("wrong creation summary:\n%s\nwant %s", &output, want)
			}
			text, err := os.Readlink(filepath.Join(root, "target", ".gitconfig"))
			if err != nil || text != filepath.Join(root, winner) {
				t.Fatalf("wrong selected source: %q, %v", text, err)
			}
			if _, err := os.Readlink(filepath.Join(root, "target", ".config", "nvim", "lua", "shared.lua")); err != nil {
				t.Fatal("shared tree was not merged:", err)
			}
			if mode == "excluded" {
				if _, err := os.Lstat(filepath.Join(root, "target", ".config", "nvim", "init.lua")); !os.IsNotExist(err) {
					t.Fatal("excluded local subtree was installed")
				}
			}
			for path, state := range before {
				if strings.Contains(path, "/local") || strings.Contains(path, "/shared") || strings.HasSuffix(path, "/sentinel") {
					if snapshot(t, root)[path] != state {
						t.Fatalf("source or sentinel changed: %s", path)
					}
				}
			}
			installed := snapshot(t, root)
			output.Reset()
			if code := Run(args, &output, &stderr); code != 0 {
				t.Fatalf("repeat failed: %d %s", code, &stderr)
			}
			if !strings.HasSuffix(output.String(), fmt.Sprintf("SUMMARY 0 linked, %d correct, 0 conflicts, 0 skipped, %d warnings, 0 errors, 0 directories created, 0 changes made\n", links, warnings)) || !reflect.DeepEqual(installed, snapshot(t, root)) {
				t.Fatalf("repeat changed fixture or counts: %s", &output)
			}
		})
	}
}

func TestRunInstallPreflightIsAllOrNothing(t *testing.T) {
	for _, mode := range []string{"file", "directory", "wrong link", "broken link", "parent file", "source error", "structural"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			fixtureFile(t, filepath.Join(root, "local", "a"), "first valid entry")
			fixtureFile(t, filepath.Join(root, "local", "z", "file"), "selected local")
			fixtureFile(t, filepath.Join(root, "shared", "z", "file"), "lower priority")
			target := filepath.Join(root, "target", ".z", "file")
			switch mode {
			case "file":
				fixtureFile(t, target, "unmanaged")
			case "directory":
				if err := os.MkdirAll(target, 0o700); err != nil {
					t.Fatal(err)
				}
			case "wrong link", "broken link":
				if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(root, "absent")
				if mode == "wrong link" {
					source = filepath.Join(root, "shared", "z", "file")
				}
				fixtureLink(t, source, target)
			case "parent file":
				fixtureFile(t, filepath.Dir(target), "occupied parent")
			case "source error":
				fixtureLink(t, "absent", filepath.Join(root, "local", "broken"))
			case "structural":
				fixtureFile(t, filepath.Join(root, "local", "config"), "leaf")
				if err := os.MkdirAll(filepath.Join(root, "shared", ".config", "empty"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			config := installationConfig(t, root, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n")
			before := snapshot(t, root)
			var output, stderr bytes.Buffer
			code := Run([]string{"--config", config, "up"}, &output, &stderr)
			if code != 1 || !strings.Contains(output.String(), "SKIP ") || !strings.HasSuffix(output.String(), "0 directories created, 0 changes made\n") || !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatalf("preflight mutated or misreported: code=%d stdout=%s stderr=%s", code, &output, &stderr)
			}
			if !strings.Contains(output.String(), filepath.Join(root, "local", "z", "file")) {
				t.Fatal("conflict changed source selection")
			}
		})
	}
}

func TestRunInstallPartialFailureAndRetry(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b/file", "c"} {
		fixtureFile(t, filepath.Join(root, "local", name), name)
	}
	parent := filepath.Join(root, "target", ".b")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(parent, 0o700); err != nil {
			t.Error(err)
		}
	})
	probe := filepath.Join(parent, "permission-control")
	file, err := os.Create(probe)
	if err == nil {
		file.Close()
		if err := os.Remove(probe); err != nil {
			t.Fatal(err)
		}
		t.Skip("filesystem permits creation despite restricted fixture permissions")
	} else if !os.IsPermission(err) {
		t.Fatal(err)
	}
	config := installationConfig(t, root, "dotfiles_dirs = ['local']\ntarget_dir = 'target'\n")
	var output, stderr bytes.Buffer
	code := Run([]string{"--config", config, "up"}, &output, &stderr)
	want := "SUMMARY 1 linked, 0 correct, 0 conflicts, 1 skipped, 0 warnings, 1 errors, 0 directories created, 1 changes made\n"
	if code != 1 || !strings.HasSuffix(output.String(), want) {
		t.Fatalf("wrong partial result: %d %s %s", code, &output, &stderr)
	}
	first := filepath.Join(root, "target", ".a")
	before, err := os.Lstat(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "target", ".c")); !os.IsNotExist(err) {
		t.Fatal("continued after failure")
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if code := Run([]string{"--config", config, "up"}, &output, &stderr); code != 0 {
		t.Fatalf("retry failed: %d %s %s", code, &output, &stderr)
	}
	after, err := os.Lstat(first)
	if err != nil || !os.SameFile(before, after) || !strings.Contains(output.String(), "SUMMARY 2 linked, 1 correct") {
		t.Fatalf("retry replaced completed link: %s, %v", &output, err)
	}
}

type failedOutput struct{}

func (failedOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestRunInstallOutputFailureReportsCompletedChanges(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "local", "a"), "source")
	config := installationConfig(t, root, "dotfiles_dirs = ['local']\ntarget_dir = 'target'\n")
	var stderr bytes.Buffer
	if code := Run([]string{"--config", config, "up"}, failedOutput{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "1 links and 1 directories already created") {
		t.Fatalf("completed writes hidden by output failure: %d, %s", code, &stderr)
	}
	if _, err := os.Readlink(filepath.Join(root, "target", ".a")); err != nil {
		t.Fatal("completed link was rolled back:", err)
	}
}

func TestRunInstallationReportingIsDeterministic(t *testing.T) {
	paths := []string{"local/gitconfig", "local/.gitconfig", "local/config/nvim/init.lua", "shared/.config/git/config"}
	var expected string
	for _, reversed := range []bool{false, true} {
		root := t.TempDir()
		for i := range paths {
			index := i
			if reversed {
				index = len(paths) - 1 - i
			}
			fixtureFile(t, filepath.Join(root, paths[index]), paths[index])
		}
		config := installationConfig(t, root, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n")
		var output, stderr bytes.Buffer
		if code := Run([]string{"--config", config, "up"}, &output, &stderr); code != 0 {
			t.Fatalf("install failed: %d %s %s", code, &output, &stderr)
		}
		actual := strings.ReplaceAll(output.String(), root, "<fixture>")
		if reversed && actual != expected {
			t.Fatalf("creation order changed report:\n%s\n%s", expected, actual)
		}
		expected = actual
	}
}
