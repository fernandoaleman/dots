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

func TestInspectionDestinationStates(t *testing.T) {
	for _, test := range []struct {
		mode, label, counter string
		code                 int
	}{
		{"missing parents", "MISSING", "1 missing", 1},
		{"missing leaf", "MISSING", "1 missing", 1},
		{"correct absolute", "OK", "1 correct", 0},
		{"correct relative", "OK", "1 correct", 0},
		{"wrong", "WRONG", "1 wrong links", 1},
		{"broken", "BROKEN", "1 broken links", 1},
		{"broken non-directory target", "BROKEN", "1 broken links", 1},
		{"file", "FILE", "1 occupied", 1},
		{"directory", "DIRECTORY", "1 occupied", 1},
		{"blocked file", "BLOCKED", "1 blocked", 1},
		{"broken parent", "BLOCKED", "1 blocked", 1},
		{"non-directory parent target", "BLOCKED", "1 blocked", 1},
	} {
		t.Run(test.mode, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "local", "a")
			fixtureFile(t, source, "winner")
			loser := filepath.Join(root, "shared", ".a")
			fixtureFile(t, loser, "loser")
			target := filepath.Join(root, "target")
			destination := filepath.Join(target, ".a")
			if test.mode != "missing parents" && test.mode != "blocked file" && test.mode != "broken parent" && test.mode != "non-directory parent target" {
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			switch test.mode {
			case "correct absolute":
				fixtureLink(t, source, destination)
			case "correct relative":
				fixtureLink(t, "../local/a", destination)
			case "wrong":
				fixtureLink(t, loser, destination)
			case "broken":
				fixtureLink(t, "missing", destination)
			case "broken non-directory target", "non-directory parent target":
				blocker := filepath.Join(root, "external", "file")
				fixtureFile(t, blocker, "preserved")
				link := destination
				if test.mode == "non-directory parent target" {
					link = target
				}
				fixtureLink(t, blocker+"/child", link)
			case "file":
				fixtureFile(t, destination, "preserved")
			case "directory":
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatal(err)
				}
			case "blocked file":
				fixtureFile(t, target, "preserved")
			case "broken parent":
				fixtureLink(t, "missing", target)
			}
			config := installationConfig(t, root, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n")
			before := snapshot(t, root)
			for _, command := range []string{"ls", "status", "up"} {
				args := []string{command, "--config", config}
				label, code := test.label, test.code
				if command == "ls" {
					label, code = "SELECTED", 0
				}
				if command == "up" {
					args = append(args, "--dry-run")
					label = "CONFLICT"
					if test.label == "OK" {
						label = "OK"
					} else if test.label == "MISSING" {
						label, code = "LINK", 0
					}
				}
				var output, stderr bytes.Buffer
				if got := Run(args, &output, &stderr); got != code {
					t.Fatalf("%s: exit %d want %d: %s %s", command, got, code, &output, &stderr)
				}
				mapping := fmt.Sprintf("%s %q -> %q", label, destination, source)
				if !strings.HasPrefix(output.String(), mapping) || !strings.HasSuffix(output.String(), "0 changes made\n") {
					t.Fatalf("%s changed source selection or row: %s", command, &output)
				}
				if command == "status" && !strings.Contains(output.String(), test.counter) {
					t.Fatalf("wrong status counts: %s", &output)
				}
				if !reflect.DeepEqual(before, snapshot(t, root)) {
					t.Fatalf("%s modified the fixture", command)
				}
			}
		})
	}
}

func TestInspectionResolutionPolicies(t *testing.T) {
	for _, mode := range []string{"merge", "dotted", "warning", "excluded", "empty", "all missing", "invalid root", "source error", "structural", "canonical XDG"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range []string{"local/gitconfig", "shared/.gitconfig", "local/config/nvim/init.lua", "shared/config/nvim/lua/shared.lua"} {
				fixtureFile(t, filepath.Join(root, name), name)
			}
			body := "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n"
			selected, count, warnings, failure := "local/gitconfig", 3, 0, false
			switch mode {
			case "dotted":
				fixtureFile(t, filepath.Join(root, "local", ".gitconfig"), "dotted winner")
				selected, warnings = "local/.gitconfig", 1
			case "warning":
				body = "dotfiles_dirs = ['absent', 'local', 'shared']\ntarget_dir = 'target'\n"
				warnings = 1
			case "excluded":
				body += "excludes = ['config/nvim']\n"
				count = 1
			case "empty":
				if err := os.Mkdir(filepath.Join(root, "empty"), 0o700); err != nil {
					t.Fatal(err)
				}
				body = "dotfiles_dirs = ['absent', 'empty']\ntarget_dir = 'target'\n"
				count, warnings = 0, 1
			case "all missing":
				body = "dotfiles_dirs = ['absent']\ntarget_dir = 'target'\n"
				count, warnings, failure = 0, 1, true
			case "invalid root":
				body = "dotfiles_dirs = ['local/gitconfig', 'local']\ntarget_dir = 'target'\n"
				count, failure = 2, true
			case "source error":
				fixtureLink(t, "missing", filepath.Join(root, "local", "broken"))
				count, failure = 4, true
			case "structural":
				fixtureFile(t, filepath.Join(root, "local", "config-file"), "unrelated")
				fixtureFile(t, filepath.Join(root, "shared", "config", "nvim", "init.lua", "child"), "collision")
				count, failure = 5, true
			}
			config := installationConfig(t, root, body)
			if mode == "canonical XDG" {
				config = filepath.Join(root, "xdg", "dots", "config.toml")
				fixtureFile(t, config, fmt.Sprintf("dotfiles_dirs = [%q, %q]\ntarget_dir = %q\n", filepath.Join(root, "local"), filepath.Join(root, "shared"), filepath.Join(root, "target")))
			}
			before := snapshot(t, root)
			for _, command := range []string{"ls", "status"} {
				args := []string{"--config", config, command}
				if mode == "canonical XDG" {
					args = []string{command}
				}
				wantCode := 0
				if failure || command == "status" && count != 0 {
					wantCode = 1
				}
				var output, stderr bytes.Buffer
				code := Run(args, &output, &stderr)
				if code != wantCode || !strings.Contains(output.String(), fmt.Sprintf("%d warnings", warnings)) {
					t.Fatalf("%s: code=%d want %d, %s %s", command, code, wantCode, &output, &stderr)
				}
				rows := strings.Split(strings.Split(output.String(), "SUMMARY")[0], "\n")
				actual := 0
				for _, row := range rows {
					if strings.Contains(row, " -> ") {
						actual++
					}
				}
				if actual != count {
					t.Fatalf("%s: %d rows, want %d: %s", command, actual, count, &output)
				}
				if count != 0 && !strings.Contains(output.String(), fmt.Sprintf("%q -> %q", filepath.Join(root, "target", ".gitconfig"), filepath.Join(root, selected))) {
					t.Fatal("source precedence changed:", &output)
				}
				if mode == "structural" && !strings.Contains(output.String(), "structural collision") {
					t.Fatal("structural evidence discarded")
				}
				if mode == "source error" && command == "status" && !strings.Contains(output.String(), "ERROR "+fmt.Sprintf("%q", filepath.Join(root, "target", ".broken"))) {
					t.Fatal("source failure not classified as ERROR:", &output)
				}
				if !reflect.DeepEqual(before, snapshot(t, root)) {
					t.Fatalf("%s wrote files", command)
				}
			}
		})
	}
}

func TestInspectionUsageAndConfigFailures(t *testing.T) {
	for _, command := range []string{"ls", "status"} {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			fixtureFile(t, filepath.Join(root, "source", "a"), "a")
			config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
			fixtureFile(t, filepath.Join(root, "xdg", "dots", "config.toml"), fmt.Sprintf("dotfiles_dirs = [%q]\n", filepath.Join(root, "source")))
			malformed := filepath.Join(root, "malformed.toml")
			fixtureFile(t, malformed, "dotfiles_dirs = [")
			before := snapshot(t, root)
			for _, test := range []struct {
				args []string
				code int
			}{
				{[]string{command, "--help"}, 0},
				{[]string{command, "--dry-run"}, 2},
				{[]string{command, "-n"}, 2},
				{[]string{command, "--json"}, 2},
				{[]string{command, "--config", config, "extra"}, 2},
				{[]string{command, "--config="}, 2},
				{[]string{"--config=", command}, 2},
				{[]string{command, "--config"}, 2},
				{[]string{command, "--config", filepath.Join(root, "missing")}, 1},
				{[]string{"--config", malformed, command}, 1},
			} {
				var output, stderr bytes.Buffer
				if code := Run(test.args, &output, &stderr); code != test.code {
					t.Fatalf("%v: got %d want %d: %s %s", test.args, code, test.code, &output, &stderr)
				}
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("usage or configuration errors wrote files")
			}
		})
	}
}

type shortOutput struct{}

func (shortOutput) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestInspectionOutputFailures(t *testing.T) {
	for _, command := range []string{"ls", "status"} {
		for _, writer := range []io.Writer{failedOutput{}, shortOutput{}} {
			root := t.TempDir()
			fixtureFile(t, filepath.Join(root, "source", "a"), "a")
			config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
			before := snapshot(t, root)
			var stderr bytes.Buffer
			if code := Run([]string{command, "--config", config}, writer, &stderr); code != 1 || !strings.Contains(stderr.String(), "write output") {
				t.Fatalf("%s output failure ignored: code=%d %s", command, code, &stderr)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("failed output caused writes")
			}
		}
	}
}

func TestInspectionPermissionFailure(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "source", "config", "nvim", "init.lua"), "source")
	config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
	locked := filepath.Join(root, "target")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	if _, err := os.Lstat(filepath.Join(locked, ".config")); !os.IsPermission(err) {
		t.Skipf("fixture cannot deny inspection: %v", err)
	}
	for _, command := range []string{"ls", "status"} {
		var output, stderr bytes.Buffer
		want := 0
		if command == "status" {
			want = 1
		}
		if code := Run([]string{command, "--config", config}, &output, &stderr); code != want {
			t.Fatalf("%s: code=%d want %d: %s %s", command, code, want, &output, &stderr)
		}
		if command == "status" && (!strings.HasPrefix(output.String(), "ERROR ") || !strings.Contains(output.String(), "cannot inspect ancestor")) {
			t.Fatal("inspection failure misclassified:", &output)
		}
	}
	info, err := os.Stat(locked)
	if err != nil || info.Mode().Perm() != 0 {
		t.Fatal("commands changed locked permissions")
	}
	if err := os.Chmod(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("permission failure caused writes")
	}
}

func TestInspectionDeterminismAndSummary(t *testing.T) {
	for _, command := range []string{"ls", "status"} {
		var expected string
		for _, reverse := range []bool{false, true} {
			root := t.TempDir()
			names := []string{"a", "b", "c", "d", "e", "f", "g/file"}
			for i := range names {
				index := i
				if reverse {
					index = len(names) - 1 - i
				}
				fixtureFile(t, filepath.Join(root, "source", names[index]), names[index])
			}
			fixtureFile(t, filepath.Join(root, "target", ".d"), "file")
			if err := os.Mkdir(filepath.Join(root, "target", ".e"), 0o700); err != nil {
				t.Fatal(err)
			}
			fixtureLink(t, "../source/a", filepath.Join(root, "target", ".a"))
			fixtureLink(t, "../source/a", filepath.Join(root, "target", ".b"))
			fixtureLink(t, "absent", filepath.Join(root, "target", ".c"))
			fixtureFile(t, filepath.Join(root, "target", ".g"), "blocked parent")
			config := installationConfig(t, root, "dotfiles_dirs = ['absent', 'source']\ntarget_dir = 'target'\n")
			before := snapshot(t, root)
			for range 2 {
				var output, stderr bytes.Buffer
				want := 0
				if command == "status" {
					want = 1
				}
				if code := Run([]string{command, "--config", config}, &output, &stderr); code != want {
					t.Fatalf("%s failed: %d %s %s", command, code, &output, &stderr)
				}
				if command == "status" && !strings.HasSuffix(output.String(), "SUMMARY 1 correct, 1 missing, 1 wrong links, 1 broken links, 2 occupied, 1 blocked, 0 conflicts, 1 warnings, 0 errors, 0 changes made\n") {
					t.Fatal("wrong mixed summary:", &output)
				}
				actual := strings.ReplaceAll(output.String()+stderr.String(), root, "<fixture>")
				if expected != "" && actual != expected {
					t.Fatalf("%s output depends on enumeration order", command)
				}
				expected = actual
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("repeated inspection caused writes")
			}
		}
	}
}

func TestStatusPhysicalCollisions(t *testing.T) {
	for _, mode := range []string{"alias", "native names"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			fixtureFile(t, filepath.Join(root, "local", "config", "nvim"), "leaf")
			fixtureFile(t, filepath.Join(root, "local", "zshrc"), "unrelated")
			other := "other/nvim/init.lua"
			if mode == "native names" {
				other = "config/NVIM/init.lua"
			}
			fixtureFile(t, filepath.Join(root, "shared", other), "nested")
			fixtureFile(t, filepath.Join(root, "target", ".other", "sentinel"), "preserved")
			if mode == "alias" {
				fixtureLink(t, ".other", filepath.Join(root, "target", ".config"))
			}
			probe := filepath.Join(root, "target", "CaseProbe")
			fixtureFile(t, probe, "native control")
			info, err := os.Lstat(probe)
			if err != nil {
				t.Fatal(err)
			}
			variant, err := os.Lstat(filepath.Join(root, "target", "caseprobe"))
			insensitive := err == nil && os.SameFile(info, variant)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			config := installationConfig(t, root, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n")
			before := snapshot(t, root)
			var output, stderr bytes.Buffer
			code := Run([]string{"status", "--config", config}, &output, &stderr)
			unknown := strings.Contains(output.String(), "cannot determine target case sensitivity")
			if mode == "alias" || insensitive || unknown {
				label := "CONFLICT "
				if unknown {
					label = "ERROR "
				}
				if code != 1 || strings.Count(output.String(), label) != 2 || strings.Count(output.String(), "MISSING ") != 1 {
					t.Fatalf("wrong physical collision classification: %d %s %s", code, &output, &stderr)
				}
			} else if code != 1 || strings.Count(output.String(), "MISSING ") != 3 {
				t.Fatal("distinct names conflated:", &output)
			}
			output.Reset()
			if code := Run([]string{"ls", "--config", config}, &output, &stderr); code != 0 || strings.Count(output.String(), "SELECTED ") != 3 {
				t.Fatalf("ls inspected physical destinations: %d %s %s", code, &output, &stderr)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("collision inspection modified fixture")
			}
		})
	}
}

func TestStatusSourceErrorAndPhysicalCollision(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "local", "zshrc"), "unrelated")
	fixtureFile(t, filepath.Join(root, "shared", "other", "nvim", "init.lua"), "child")
	source := filepath.Join(root, "local", "config", "nvim")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	fixtureLink(t, "missing", source)
	fixtureFile(t, filepath.Join(root, "target", ".other", "sentinel"), "preserved")
	fixtureLink(t, ".other", filepath.Join(root, "target", ".config"))
	config := installationConfig(t, root, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n")
	before := snapshot(t, root)
	var output, stderr bytes.Buffer
	if code := Run([]string{"status", "--config", config}, &output, &stderr); code != 1 {
		t.Fatalf("combined failure became success: %d %s %s", code, &output, &stderr)
	}
	row := strings.SplitN(output.String(), "\n", 2)[0]
	if !strings.HasPrefix(row, fmt.Sprintf("ERROR %q -> %q", filepath.Join(root, "target", ".config", "nvim"), source)) ||
		!strings.Contains(row, "invalid source symlink") || !strings.Contains(row, "aliased destination collision") ||
		!strings.Contains(output.String(), "\nERROR invalid source symlink") {
		t.Fatal("combined explanations or selected source lost:", &output)
	}
	if !strings.HasSuffix(output.String(), "SUMMARY 0 correct, 1 missing, 0 wrong links, 0 broken links, 0 occupied, 0 blocked, 1 conflicts, 0 warnings, 2 errors, 0 changes made\n") {
		t.Fatal("rows or standalone diagnostics miscounted:", &output)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("combined failure caused writes")
	}
}
