package cli

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixtureFile(t *testing.T, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureLink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%s:%d:", info.Mode(), info.ModTime().UnixNano())
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(name)
			if err != nil {
				return err
			}
			value += target
		case info.Mode().IsRegular():
			contents, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			value += string(contents)
		}
		result[name] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRunPlansAreReadOnly(t *testing.T) {
	tests := []struct {
		mode string
		code int
		want string
	}{
		{"missing", 0, "LINK"},
		{"correct", 0, "OK"},
		{"wrong", 1, "another source"},
		{"broken", 1, "broken destination symlink"},
		{"file", 1, "existing regular file"},
		{"directory", 1, "existing directory"},
		{"ancestor", 1, "not a directory"},
		{"warnings", 0, "WARN"},
		{"source error", 1, "invalid source symlink"},
		{"config error", 1, "read configuration"},
		{"structural", 1, "structural collision"},
		{"aliased structural", 1, "aliased destination collision"},
		{"all missing", 1, "no usable source directories"},
	}
	for i, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
			source := filepath.Join(root, "local", "gitconfig")
			destination := filepath.Join(root, "target", ".gitconfig")
			config := filepath.Join(root, "settings.toml")
			fixtureFile(t, source, "selected local source")
			fixtureFile(t, filepath.Join(root, "shared", ".gitconfig"), "losing shared source")
			fixtureFile(t, filepath.Join(root, "shared", "config", "nvim", "init.lua"), "nested")
			body := "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n"
			selected := source
			switch tt.mode {
			case "correct", "wrong", "broken", "file", "directory", "ancestor":
				if err := os.Mkdir(filepath.Dir(destination), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			switch tt.mode {
			case "correct":
				fixtureLink(t, "../local/gitconfig", destination)
			case "wrong":
				fixtureLink(t, filepath.Join(root, "shared", ".gitconfig"), destination)
			case "broken":
				fixtureLink(t, "missing", destination)
			case "file":
				fixtureFile(t, destination, "unmanaged")
			case "directory":
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatal(err)
				}
			case "ancestor":
				fixtureFile(t, filepath.Join(root, "target", ".config"), "blocker")
			case "warnings":
				body = "dotfiles_dirs = ['missing', 'local', 'shared']\ntarget_dir = 'target'\n"
				selected = filepath.Join(root, "local", ".gitconfig")
				fixtureFile(t, selected, "dotted winner")
			case "source error":
				fixtureLink(t, "missing", filepath.Join(root, "local", "broken"))
			case "config error":
				body = "dotfiles_dirs = ["
			case "structural":
				fixtureFile(t, filepath.Join(root, "local", "config"), "structural blocker")
			case "aliased structural":
				fixtureFile(t, filepath.Join(root, "local", "other", "nvim"), "file blocker")
				if err := os.MkdirAll(filepath.Join(root, "target", ".other"), 0o700); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, ".other", filepath.Join(root, "target", ".config"))
			case "all missing":
				body = "dotfiles_dirs = ['missing']\ntarget_dir = 'target'\n"
			}
			fixtureFile(t, config, body)
			before := snapshot(t, root)
			args := []string{"--config", config, "up", "--dry-run"}
			if i%2 != 0 {
				args = []string{"up", "--config", config, "-n"}
			}
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr)
			combined := stdout.String() + stderr.String()
			if code != tt.code || !strings.Contains(combined, tt.want) {
				t.Fatalf("exit = %d, output = %q; want %d containing %q", code, combined, tt.code, tt.want)
			}
			if tt.mode != "config error" && tt.mode != "all missing" {
				if !strings.Contains(stdout.String(), fmt.Sprintf("%q -> %q", destination, selected)) {
					t.Fatalf("destination occupancy changed selected source: %s", stdout.String())
				}
			}
			if tt.mode != "config error" && !strings.Contains(stdout.String(), "0 changes made") {
				t.Fatalf("missing read-only summary: %s", stdout.String())
			}
			var repeatedOut, repeatedErr bytes.Buffer
			if tt.mode == "structural" || tt.mode == "aliased structural" {
				assertCollisionPlan(t, root, tt.mode, stdout.String())
			}
			if again := Run(args, &repeatedOut, &repeatedErr); again != code ||
				repeatedOut.String() != stdout.String() || repeatedErr.String() != stderr.String() {
				t.Fatal("identical runs produced different results")
			}
			if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("dry-run modified the fixture tree")
			}
		})
	}
}

func TestRunUsage(t *testing.T) {
	tests := []struct {
		args []string
		code int
	}{
		{nil, 2},
		{[]string{"--help"}, 0},
		{[]string{"help"}, 0},
		{[]string{"up", "--help"}, 0},
		{[]string{"up"}, 1},
		{[]string{"down"}, 2},
		{[]string{"up", "--force"}, 2},
		{[]string{"up", "--dry-run", "extra"}, 2},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			var stdout, stderr bytes.Buffer
			if got := Run(tt.args, &stdout, &stderr); got != tt.code {
				t.Fatalf("exit = %d; want %d; stderr = %s", got, tt.code, stderr.String())
			}
		})
	}
}

func TestRunRejectsExplicitEmptyConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", "")
	fixtureFile(t, filepath.Join(root, "source", "gitconfig"), "source")
	fixtureFile(t, filepath.Join(root, ".config", "dots", "config.toml"),
		"dotfiles_dirs = ['~/source']\ntarget_dir = '~/target'\n")
	for _, args := range [][]string{
		{"--config=", "up", "--dry-run"},
		{"up", "--config", "", "--dry-run"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 ||
			!strings.Contains(stderr.String(), "must not be empty") || stdout.Len() != 0 {
			t.Fatalf("empty explicit config fell back: exit=%d stdout=%q stderr=%q",
				code, stdout.String(), stderr.String())
		}
	}
}
