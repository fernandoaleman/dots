package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestVersionText(t *testing.T) {
	revision := strings.Repeat("a", 40)
	for _, tt := range []struct{ version, commit, want string }{
		{"v0.1.0", revision, "dots v0.1.0\n"},
		{"v0.1.0", "", "dots v0.1.0\n"},
		{"dev", revision, "dots dev (commit " + revision + ")\n"},
		{"", "", "dots dev (commit unknown)\n"},
		{"dev", "unknown", "dots dev (commit unknown)\n"},
		{"", revision, "dots dev (commit " + revision + ")\n"},
	} {
		if got := versionText(tt.version, tt.commit); got != tt.want {
			t.Fatalf("%q %q: %q", tt.version, tt.commit, got)
		}
	}
}

func TestInformationCommandsAreOfflineAndReadOnly(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "missing-config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "missing-state"))
	before := snapshot(t, base)
	for _, c := range commands {
		var previous string
		for _, args := range [][]string{{"help", c.name}, {c.name, "--help"}, {c.name, "-h"}} {
			var out, err bytes.Buffer
			if code := Run(args, &out, &err); code != 0 {
				t.Fatalf("%v: %d %s", args, code, &err)
			}
			if !strings.Contains(err.String(), "Usage:") || previous != "" && previous != err.String() {
				t.Fatalf("inconsistent help %v: %s", args, &err)
			}
			previous = err.String()
		}
	}
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err); code != 0 || !strings.Contains(err.String(), "Documentation commands:") || !strings.Contains(err.String(), "Recovery commands:") {
			t.Fatal(args, code, &err)
		}
	}
	var canonical string
	for _, args := range [][]string{{"version"}, {"--version"}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err); code != 0 || err.Len() != 0 || out.String() != versionText(version, commit) {
			t.Fatal(args, code, &out, &err)
		}
		if canonical != "" && canonical != out.String() {
			t.Fatal("version aliases differ")
		}
		canonical = out.String()
	}
	var out, err bytes.Buffer
	if code := RunWithLicense([]string{"license"}, &out, &err, "fixture license\n"); code != 0 || out.String() != "fixture license\n" {
		t.Fatal(code, &out, &err)
	}
	if !reflect.DeepEqual(before, snapshot(t, base)) {
		t.Fatal("information command wrote state")
	}
}

func TestHelpFlagsMatchCommands(t *testing.T) {
	for _, c := range commands {
		text, err := helpText(c.name)
		if err != nil {
			t.Fatal(err)
		}
		config := c.name == "add" || c.name == "up" || c.name == "down" || c.name == "list" || c.name == "status"
		dry := c.name == "add" || c.name == "up" || c.name == "down"
		if strings.Contains(text, configHelp) != config || strings.Contains(text, dryHelp) != dry {
			t.Fatalf("wrong applicable flags for %s: %s", c.name, text)
		}
	}
}

func TestInformationErrors(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	before := snapshot(t, base)
	defer func() {
		if !reflect.DeepEqual(before, snapshot(t, base)) {
			t.Fatal("invalid informational arguments wrote state")
		}
	}()
	for _, args := range [][]string{{"-v"}, {"ls"}, {"doctor"}, {"help", "ls"}, {"help", "add", "extra"},
		{"version", "extra"}, {"version", "--build-info"}, {"license", "extra"}, {"--version", "up"}, {"--version=false"},
		{"--config=unused", "version"}, {"-c=unused", "license"},
		{"-c", "first.toml", "--help", "up", "--config", "second.toml"},
		{"--help", "--config", "first.toml", "up", "-c", "first.toml"},
		{"version", "--help", "-c", "config.toml"},
		{"license", "--help", "--config", "config.toml"},
		{"--help", "recover", "-c", "config.toml"}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err); code != 2 || err.Len() == 0 {
			t.Fatal(args, code, &out, &err)
		}
	}
	var err bytes.Buffer
	if code := Run([]string{"license"}, io.Discard, &err); code != 1 {
		t.Fatal("missing license succeeded", code)
	}
	for _, args := range [][]string{{"version"}, {"--version"}, {"license"}} {
		if code := RunWithLicense(args, recoveryShortWriter{}, &err, "license\n"); code != 1 {
			t.Fatal("short information write succeeded", args, code)
		}
	}
	for _, args := range [][]string{{"help"}, {"help", "add"}, {"--help"}, {"add", "-h"}, {"recover", "-h"}, {"version", "-h"}} {
		if code := Run(args, io.Discard, recoveryShortWriter{}); code != 1 {
			t.Fatal("short help write succeeded", args, code)
		}
	}
}

func TestConfigAliasesAndDuplicateRefusals(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "missing-config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "missing-state"))
	config := filepath.Join(base, "config.toml")
	fixtureFile(t, filepath.Join(base, "source", "gitconfig"), "source")
	fixtureFile(t, filepath.Join(base, "target", ".zshrc"), "original")
	fixtureFile(t, config, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
	before := snapshot(t, base)
	for _, command := range []string{"add", "up", "down", "list", "status"} {
		tail := []string{}
		wantCode := 0
		if command == "up" || command == "down" {
			tail = []string{"-n"}
		}
		if command == "add" {
			tail = []string{"-n", filepath.Join(base, "target", ".zshrc")}
		}
		if command == "status" {
			wantCode = 1
		}
		var previous string
		for _, flag := range []string{"-c", "--config"} {
			for _, args := range [][]string{
				append([]string{flag, config, command}, tail...),
				append([]string{command, flag, config}, tail...),
				append([]string{command, flag + "=" + config}, tail...),
				append([]string{flag + "=" + config, "--", command}, tail...),
			} {
				var out, err bytes.Buffer
				if code := Run(args, &out, &err); code != wantCode || err.Len() != 0 {
					t.Fatal(args, code, &out, &err)
				}
				if previous != "" && previous != out.String() {
					t.Fatal("configuration alias changed output", command)
				}
				previous = out.String()
			}
		}
		for _, a := range []string{"-c", "--config"} {
			for _, b := range []string{"-c", "--config"} {
				for _, value := range []string{config, filepath.Join(base, "other.toml")} {
					for _, args := range [][]string{
						{a, config, b, value, command}, {command, a, config, b, value}, {a, config, command, b, value},
						{a + "=" + config, "--", command, b + "=" + value},
					} {
						args = append(args, tail...)
						var out, err bytes.Buffer
						if code := Run(args, &out, &err); code != 2 || out.Len() != 0 {
							t.Fatal(args, code, &out, &err)
						}
					}
				}
			}
		}
		for _, args := range [][]string{{command, "-c="}, {"-c=", command}, {command, "-c", ""}, {command, "--config", "--help"}, {command, "-c"}, {command, "--help", "-c="}, {command, "--help", "-c", config, "--config", config}} {
			var out, err bytes.Buffer
			if code := Run(args, &out, &err); code != 2 || out.Len() != 0 {
				t.Fatal(args, code, &out, &err)
			}
		}
	}
	for _, args := range [][]string{{"recover", "-c", config}, {"-c", config, "recover"}, {"recover", "--config=" + config}, {"--config", config, "recover"}, {"recover", "--help", "-c", config}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err); code != 2 {
			t.Fatal(args, code, &err)
		}
	}
	for _, command := range []string{"up", "down"} {
		for _, help := range [][]string{{"--help", "--help=false"}, {"--help", "-h=false"}, {"-h=false"}, {"-h", "--help"}} {
			args := append([]string{"-c", config}, help...)
			args = append(args, command)
			var out, err bytes.Buffer
			if code := Run(args, &out, &err); code != 2 {
				t.Fatal("help override reached command", args, code, &out, &err)
			}
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, base)) {
		t.Fatal("configuration preview/refusal wrote files or initialized state")
	}
}
