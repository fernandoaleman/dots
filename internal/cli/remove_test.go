package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func runFixtureCommand(t *testing.T, config, command string, flags ...string) (int, string, string) {
	t.Helper()
	var output, stderr bytes.Buffer
	args := append([]string{"--config", config, command}, flags...)
	code := Run(args, &output, &stderr)
	return code, output.String(), stderr.String()
}

func TestRunDownUsesRecordedSourcesWithoutResolving(t *testing.T) {
	for _, mode := range []string{"unchanged", "missing source", "reordered", "removed entry", "excluded", "legacy", "occupied", "broken", "nested source link"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			fixtureFile(t, filepath.Join(root, "local", "gitconfig"), "personal")
			fixtureFile(t, filepath.Join(root, "shared", ".gitconfig"), "shared")
			fixtureFile(t, filepath.Join(root, "shared", "config", "nvim", "init.lua"), "merged")
			if mode == "nested source link" {
				fixtureFile(t, filepath.Join(root, "referent"), "source contents")
				fixtureLink(t, "../referent", filepath.Join(root, "local", "linked"))
			}
			config := installationConfig(t, root, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\n")
			destination := filepath.Join(root, "target", ".gitconfig")
			if mode == "legacy" {
				if err := os.Mkdir(filepath.Dir(destination), 0o700); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, filepath.Join(root, "local", "gitconfig"), destination)
			}
			if code, output, stderr := runFixtureCommand(t, config, "up"); code != 0 {
				t.Fatalf("install: %d %s %s", code, output, stderr)
			}
			switch mode {
			case "missing source":
				fixtureFile(t, config, "dotfiles_dirs = ['absent']\ntarget_dir = 'target'\n")
			case "reordered":
				fixtureFile(t, config, "dotfiles_dirs = ['shared', 'local']\ntarget_dir = 'target'\n")
			case "excluded":
				fixtureFile(t, config, "dotfiles_dirs = ['local', 'shared']\ntarget_dir = 'target'\nexcludes = ['gitconfig', '.gitconfig', 'config']\n")
			case "removed entry", "broken":
				if err := os.Remove(filepath.Join(root, "local", "gitconfig")); err != nil {
					t.Fatal(err)
				}
			case "occupied":
				if err := os.Rename(destination, destination+".original"); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, destination, "user content")
			}
			before := snapshot(t, root)
			wantCode := 0
			if mode == "occupied" {
				wantCode = 1
			}
			code, dry, stderr := runFixtureCommand(t, config, "down", "--dry-run")
			if code != wantCode || !strings.Contains(dry, "0 changes made") || !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatalf("dry-run: %d %s %s", code, dry, stderr)
			}
			_, again, _ := runFixtureCommand(t, config, "down", "-n")
			if dry != again {
				t.Fatal("dry-run output unstable")
			}
			code, output, stderr := runFixtureCommand(t, config, "down")
			if code != wantCode {
				t.Fatalf("down: %d %s %s", code, output, stderr)
			}
			if mode == "occupied" {
				data, err := os.ReadFile(destination)
				if err != nil || string(data) != "user content" {
					t.Fatal("file changed:", err)
				}
				if !strings.Contains(output, "0 removed") {
					t.Fatal("preflight allowed partial removal:", output)
				}
				return
			}
			if mode == "legacy" {
				if _, err := os.Readlink(destination); err != nil {
					t.Fatal("legacy link removed:", err)
				}
			} else if _, err := os.Lstat(destination); !os.IsNotExist(err) {
				t.Fatal("owned link remains:", err)
			}
			if _, err := os.Stat(filepath.Join(root, "target", ".config", "nvim")); err != nil {
				t.Fatal("parent removed:", err)
			}
			if code, output, stderr := runFixtureCommand(t, config, "down"); code != 0 || !strings.Contains(output, "0 removed") {
				t.Fatalf("idempotency: %d %s %s", code, output, stderr)
			}
		})
	}
}

func TestRunDownHistoricalTargetsAndProfiles(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "source", "a"), "source")
	config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'old'\n")
	if code, output, _ := runFixtureCommand(t, config, "up"); code != 0 {
		t.Fatal(output)
	}
	fixtureFile(t, config, "dotfiles_dirs = ['source']\ntarget_dir = 'current'\n")
	if code, output, _ := runFixtureCommand(t, config, "up"); code != 0 {
		t.Fatal(output)
	}
	if code, output, _ := runFixtureCommand(t, config, "down"); code != 0 || !strings.Contains(output, "historical target") || !strings.Contains(output, "1 removed") {
		t.Fatalf("scope: %d %s", code, output)
	}
	if _, err := os.Readlink(filepath.Join(root, "old", ".a")); err != nil {
		t.Fatal("historical target removed:", err)
	}
	copyConfig := filepath.Join(root, "copy.toml")
	fixtureFile(t, copyConfig, "dotfiles_dirs = ['source']\ntarget_dir = 'old'\n")
	if code, output, _ := runFixtureCommand(t, copyConfig, "down"); code != 0 || !strings.Contains(output, "no eligible ownership records") {
		t.Fatalf("profile: %d %s", code, output)
	}
	fixtureFile(t, config, "dotfiles_dirs = ['source']\ntarget_dir = 'old'\n")
	if code, output, _ := runFixtureCommand(t, config, "down"); code != 0 || !strings.Contains(output, "1 removed") {
		t.Fatalf("explicit return to target: %d %s", code, output)
	}
}

func TestRunDownWithoutStateAndXDGIsolation(t *testing.T) {
	for _, mode := range []string{"unset", "empty", "relative", "absolute"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			config := installationConfig(t, root, "dotfiles_dirs = ['absent']\ntarget_dir = 'target'\n")
			switch mode {
			case "unset":
				if err := os.Unsetenv("XDG_STATE_HOME"); err != nil {
					t.Fatal(err)
				}
			case "empty":
				t.Setenv("XDG_STATE_HOME", "")
			case "relative":
				t.Setenv("XDG_STATE_HOME", "relative")
			}
			before := snapshot(t, root)
			code, output, stderr := runFixtureCommand(t, config, "down", "--dry-run")
			if code != 0 || !strings.Contains(output, "legacy links are never adopted") || !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatalf("absent state: %d %s %s", code, output, stderr)
			}
		})
	}
}

func TestRunDownPreservesTraversalAndAncestorAliases(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "source", "a"), "source")
	if err := os.MkdirAll(filepath.Join(root, "elsewhere", "deep"), 0o700); err != nil {
		t.Fatal(err)
	}
	fixtureLink(t, "elsewhere/deep", filepath.Join(root, "alias"))
	config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'alias/../target'\n")
	if code, output, _ := runFixtureCommand(t, config, "up"); code != 0 {
		t.Fatal(output)
	}
	if _, err := os.Readlink(filepath.Join(root, "elsewhere", "target", ".a")); err != nil {
		t.Fatal("traversal changed:", err)
	}
	if code, output, _ := runFixtureCommand(t, config, "down"); code != 0 || !strings.Contains(output, "1 removed") {
		t.Fatalf("alias removal: %d %s", code, output)
	}
}

func TestRunDownOutputFailureAndFlags(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "source", "a"), "source")
	config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
	if code, output, _ := runFixtureCommand(t, config, "up"); code != 0 {
		t.Fatal(output)
	}
	before := snapshot(t, root)
	for _, arguments := range [][]string{{"down", "--config="}, {"down", "--force"}, {"down", "--dry-run", "unexpected"}} {
		if code := Run(append([]string{"--config", config}, arguments...), io.Discard, io.Discard); code != 2 {
			t.Fatalf("invalid usage accepted: %v", arguments)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("invalid command wrote")
	}
	var stderr bytes.Buffer
	if code := Run([]string{"down", "--config", config}, failedOutput{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "1 links already removed") {
		t.Fatalf("output failure: %d %s", code, &stderr)
	}
}

func TestRunDownReportsDeterministicOrder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"z", "a", "m"} {
		fixtureFile(t, filepath.Join(root, "source", name), name)
	}
	config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
	if code, output, _ := runFixtureCommand(t, config, "up"); code != 0 {
		t.Fatal(output)
	}
	_, output, _ := runFixtureCommand(t, config, "down", "--dry-run")
	for index, name := range []string{"a", "m", "z"} {
		line := strings.Split(output, "\n")[index]
		if !strings.HasPrefix(line, fmt.Sprintf("REMOVE %q", filepath.Join(root, "target", "."+name))) {
			t.Fatal("unstable order:", output)
		}
	}
}

func TestRunDownAfterSourceDirectoryBecomesFile(t *testing.T) {
	for _, mode := range []string{"source root", "source-root symlink referent", "source ancestor"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			original := filepath.Join(root, "source")
			sourceRoot := original
			configured := "source"
			switch mode {
			case "source-root symlink referent":
				original = filepath.Join(root, "shared")
				sourceRoot = original
				fixtureLink(t, "shared", filepath.Join(root, "source"))
			case "source ancestor":
				sourceRoot = filepath.Join(original, "nested")
				configured = "source/nested"
			}
			fixtureFile(t, filepath.Join(sourceRoot, "a"), "original source")
			fixtureFile(t, filepath.Join(sourceRoot, "config", "nvim", "init.lua"), "nested source")
			config := installationConfig(t, root, fmt.Sprintf("dotfiles_dirs = [%q]\ntarget_dir = 'target'\n", configured))
			if code, output, stderr := runFixtureCommand(t, config, "up"); code != 0 {
				t.Fatalf("install: %d %s %s", code, output, stderr)
			}
			saved := original + ".original"
			if err := os.Rename(original, saved); err != nil {
				t.Fatal(err)
			}
			fixtureFile(t, original, "replacement file must survive")
			replacement, err := os.Lstat(original)
			if err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, root)
			if code, output, stderr := runFixtureCommand(t, config, "down", "--dry-run"); code != 0 || strings.Count(output, "REMOVE ") != 2 || !strings.Contains(output, "0 changes made") {
				t.Fatalf("dry-run: %d %s %s", code, output, stderr)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("dry-run modified source, destination, or state")
			}
			sourceBefore := snapshot(t, saved)
			if code, output, stderr := runFixtureCommand(t, config, "down"); code != 0 || !strings.Contains(output, "2 removed") {
				t.Fatalf("remove: %d %s %s", code, output, stderr)
			}
			for _, destination := range []string{".a", ".config/nvim/init.lua"} {
				if _, err := os.Lstat(filepath.Join(root, "target", destination)); !os.IsNotExist(err) {
					t.Fatalf("owned broken link remains: %s %v", destination, err)
				}
			}
			contents, err := os.ReadFile(original)
			if err != nil || string(contents) != "replacement file must survive" {
				t.Fatal("replacement source file changed:", err)
			}
			after, err := os.Lstat(original)
			if err != nil || !os.SameFile(replacement, after) {
				t.Fatal("replacement source file was replaced:", err)
			}
			if !reflect.DeepEqual(sourceBefore, snapshot(t, saved)) {
				t.Fatal("original source contents changed")
			}
			if mode == "source-root symlink referent" {
				text, err := os.Readlink(filepath.Join(root, "source"))
				if err != nil || text != "shared" {
					t.Fatal("configured root alias changed:", err)
				}
			}
			if _, err := os.Stat(filepath.Join(root, "target", ".config", "nvim")); err != nil {
				t.Fatal("destination parent removed:", err)
			}
			if code, output, stderr := runFixtureCommand(t, config, "down"); code != 0 || !strings.Contains(output, "0 removed") {
				t.Fatalf("repeat: %d %s %s", code, output, stderr)
			}
		})
	}
}

func TestReadOnlyCommandsIgnoreOwnershipState(t *testing.T) {
	for _, mode := range []string{"absent", "corrupt", "unfinished", "permissions", "locked"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			fixtureFile(t, filepath.Join(root, "source", "a"), "source")
			config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
			if code, output, _ := runFixtureCommand(t, config, "up"); code != 0 {
				t.Fatal(output)
			}
			commands := []string{"ls", "status", "up"}
			baseline := make(map[string]string)
			for _, command := range commands {
				var flags []string
				if command == "up" {
					flags = []string{"--dry-run"}
				}
				code, output, _ := runFixtureCommand(t, config, command, flags...)
				if code != 0 {
					t.Fatal(output)
				}
				baseline[command] = output
			}
			state := filepath.Join(root, "state", "dots", "ownership.json")
			data, err := os.ReadFile(state)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "absent":
				if err := os.Remove(state); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(state, []byte("malformed"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unfinished":
				if err := os.WriteFile(state, bytes.Replace(data, []byte(`"unfinished": false`), []byte(`"unfinished": true`), 1), 0o600); err != nil {
					t.Fatal(err)
				}
			case "permissions":
				if err := os.Chmod(state, 0o644); err != nil {
					t.Fatal(err)
				}
			case "locked":
				file, err := os.OpenFile(filepath.Join(filepath.Dir(state), "ownership.lock"), os.O_RDWR, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
					t.Fatal(err)
				}
			}
			fixtureFile(t, filepath.Join(filepath.Dir(state), ".ownership-orphan.tmp"), "never replay")
			before := snapshot(t, root)
			for _, command := range commands {
				var flags []string
				if command == "up" {
					flags = []string{"--dry-run"}
				}
				code, output, stderr := runFixtureCommand(t, config, command, flags...)
				if code != 0 || output != baseline[command] || !reflect.DeepEqual(before, snapshot(t, root)) {
					t.Fatalf("ownership changed read-only behavior: %s %d %s %s", command, code, output, stderr)
				}
			}
		})
	}
}

func TestFailedUpPreflightCannotResurrectObservedOwnership(t *testing.T) {
	for _, mode := range []string{"parent replacement", "leaf replacement", "missing leaf with unrelated conflict"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			fixtureFile(t, filepath.Join(root, "source", "a"), "source")
			fixtureFile(t, filepath.Join(root, "source", "z"), "source")
			config := installationConfig(t, root, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\n")
			if code, output, _ := runFixtureCommand(t, config, "up"); code != 0 {
				t.Fatal(output)
			}
			target, leaf := filepath.Join(root, "target"), filepath.Join(root, "target", ".a")
			switch mode {
			case "parent replacement":
				if err := os.Rename(target, target+".original"); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, leaf, "unmanaged conflict")
			case "leaf replacement":
				if err := os.Rename(leaf, leaf+".original"); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, leaf, "unmanaged conflict")
			case "missing leaf with unrelated conflict":
				if err := os.Remove(leaf); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(target, ".z"), filepath.Join(target, ".z.original")); err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, filepath.Join(target, ".z"), "conflict")
			}
			before := snapshot(t, target)
			if code, output, _ := runFixtureCommand(t, config, "up"); code != 1 || !strings.Contains(output, "0 changes made") {
				t.Fatalf("conflict accepted: %d %s", code, output)
			}
			if !reflect.DeepEqual(before, snapshot(t, target)) {
				t.Fatal("failed preflight changed destinations")
			}
			data, err := os.ReadFile(filepath.Join(root, "state", "dots", "ownership.json"))
			if err != nil {
				t.Fatal(err)
			}
			var registry struct {
				Unfinished bool `json:"unfinished"`
				Records    []struct {
					Destination []byte `json:"destination"`
					Phase       string `json:"phase"`
				} `json:"records"`
			}
			if err := json.Unmarshal(data, &registry); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, record := range registry.Records {
				if string(record.Destination) != leaf {
					continue
				}
				found = true
				if mode == "missing leaf with unrelated conflict" || record.Phase != "invalidated" {
					t.Fatalf("stale authority persisted: %+v", record)
				}
			}
			if !registry.Unfinished || (mode != "missing leaf with unrelated conflict" && !found) {
				t.Fatal("revocation/retirement was not durably recorded")
			}
			switch mode {
			case "parent replacement":
				if err := os.Rename(target, target+".conflict"); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(target+".original", target); err != nil {
					t.Fatal(err)
				}
			case "leaf replacement":
				if err := os.Remove(leaf); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(leaf+".original", leaf); err != nil {
					t.Fatal(err)
				}
			case "missing leaf with unrelated conflict":
				fixtureLink(t, filepath.Join(root, "source", "a"), leaf)
				// Remove the unrelated fixture conflict so it cannot mask a
				// mistaken attempt to remove the newly created untracked link.
				if err := os.Remove(filepath.Join(target, ".z")); err != nil {
					t.Fatal(err)
				}
			}
			code, output, _ := runFixtureCommand(t, config, "down")
			wantCode := 1
			if mode == "missing leaf with unrelated conflict" {
				wantCode = 0
			}
			if code != wantCode || !strings.Contains(output, "0 removed") {
				t.Fatal("stale authority replayed:", output)
			}
			if _, err := os.Lstat(leaf); err != nil {
				t.Fatal("restored/untracked link removed:", err)
			}
		})
	}
}
