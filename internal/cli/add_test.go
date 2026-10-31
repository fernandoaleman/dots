package cli

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAddUsage(t *testing.T) {
	for _, args := range [][]string{{}, {""}, {"a", "b"}, {"--root", "", ".zshrc"}, {"--config", "", ".zshrc"},
		{"--dry-run=false", ".zshrc"}, {"-n", "--dry-run=false", ".zshrc"}, {"-n=false", "--dry-run=true", ".zshrc"},
		{"--root", "one", "--root", "two", ".zshrc"}, {"--force", ".zshrc"}, {".zshrc", "--dry-run"}} {
		var out, err bytes.Buffer
		if code := Run(append([]string{"add"}, args...), &out, &err); code != 2 || err.Len() == 0 {
			t.Fatal(args, code, out.String(), err.String())
		}
	}
	for _, args := range [][]string{{"add", "--root", "--dry-run", ".zshrc"}, {"add", "--config", "-n", ".zshrc"}, {"--config", "--dry-run", "add", ".zshrc"}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err); code != 2 || !strings.Contains(err.String(), "requires a path value") {
			t.Fatal(args, code, err.String())
		}
	}
	var out, err bytes.Buffer
	if code := Run([]string{"--config", "", "add", ".zshrc"}, &out, &err); code != 2 {
		t.Fatal(code, err.String())
	}
	if code := Run([]string{"add", "--help"}, &out, &err); code != 0 || !strings.Contains(err.String(), "--root") {
		t.Fatal(code, err.String())
	}
}

func TestAddPreviewReadOnly(t *testing.T) {
	base := t.TempDir()
	home, source, target := filepath.Join(base, "home"), filepath.Join(base, "source"), filepath.Join(base, "target")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg-config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	fixtureFile(t, filepath.Join(source, "README.md"), "excluded repository readme")
	input := filepath.Join(target, ".config/nvim/init.lua")
	fixtureFile(t, input, "config data")
	config := filepath.Join(base, "config.toml")
	fixtureFile(t, config, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\nexcludes = ['README.md']\n")
	before := snapshot(t, base)
	var previous string
	for _, args := range [][]string{{"--config", config, "add", "--dry-run", input}, {"add", "--config", config, "-n", input}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err); code != 0 || err.Len() != 0 || !strings.Contains(out.String(), "PREVIEW 0 changes made") || !strings.Contains(out.String(), filepath.Join(source, "config/nvim/init.lua")) {
			t.Fatal(code, out.String(), err.String())
		}
		if previous != "" && previous != out.String() {
			t.Fatal("preview output is nondeterministic")
		}
		previous = out.String()
	}
	var stderr bytes.Buffer
	if code := Run([]string{"add", "--config", config, "--dry-run", input}, recoveryShortWriter{}, &stderr); code != 1 {
		t.Fatal("short preview write reported success", code)
	}
	if !reflect.DeepEqual(before, snapshot(t, base)) {
		t.Fatal("preview created state or changed fixture")
	}
}

func TestAddRelativeLocatorRefusalIsReadOnly(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg-config"))
	fixtureFile(t, filepath.Join(home, "source", "README.md"), "repository")
	input := filepath.Join(home, ".zshrc")
	fixtureFile(t, input, "must remain at home")
	config := filepath.Join(base, "config.toml")
	fixtureFile(t, config, "dotfiles_dirs = ['home/source']\ntarget_dir = 'home'\n")
	before := snapshot(t, base)
	for _, args := range [][]string{{"./~/.zshrc"}, {"././~/.zshrc"}, {".//" + strings.TrimPrefix(input, "/")}, {"--root", "./~/source", input}, {"--root", ".//" + strings.TrimPrefix(filepath.Join(home, "source"), "/"), input}} {
		var out, err bytes.Buffer
		command := append([]string{"--config", config, "add"}, args...)
		if code := Run(command, &out, &err); code != 1 {
			t.Fatal(args, code, out.String(), err.String())
		}
		if !reflect.DeepEqual(before, snapshot(t, base)) {
			t.Fatal("refused relative locator modified home or state")
		}
	}
}
