package filesystem

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Exercise the real executable without making cli a filesystem dependency.
func TestRecoveryCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "dots")
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", binary, "../..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build isolated executable: %v\n%s", err, output)
	}
	run := func(t *testing.T, o Ownership, args ...string) (int, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, append([]string{"recover"}, args...)...)
		root := filepath.Dir(filepath.Dir(filepath.Dir(o.Registry)))
		cmd.Env = append(os.Environ(), "HOME="+root+"/absent-home", "XDG_CONFIG_HOME="+root+"/absent-config", "XDG_STATE_HOME="+filepath.Dir(filepath.Dir(o.Registry)))
		out, err := cmd.CombinedOutput()
		if err == nil {
			return 0, string(out)
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		return exit.ExitCode(), string(out)
	}
	for _, mode := range []string{"original", "source", "relocated", "missing"} {
		t.Run(mode, func(t *testing.T) {
			o, initial, selected := retirementFixture(t, mode)
			user := retirementUserState(t, o)
			root := filepath.Dir(filepath.Dir(filepath.Dir(o.Registry)))
			before := treeState(t, root)
			revision := strconv.FormatUint(selected.Revision, 10)
			held, err := openRegistry(o, true, nativeRegistryOps())
			if err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{}, {"--retire", selected.ID, "--revision", revision, "--dry-run"}} {
				code, output := run(t, o, args...)
				if code != 0 || !strings.Contains(output, selected.ID) || !strings.Contains(output, "no mutation authority") || !strings.Contains(output, "regular file") {
					t.Fatal(code, output)
				}
				if again, repeated := run(t, o, args...); again != code || repeated != output {
					t.Fatal("inspection/preview output is nondeterministic", repeated)
				}
			}
			for _, args := range [][]string{{"--settle", "--revision", revision, "--dry-run"}, {"--retire", selected.ID, "--revision", revision}} {
				if code, output := run(t, o, args...); code != 1 {
					t.Fatal(code, output)
				}
			}
			if err := held.Close(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatal("inspection, previews or lock refusal changed fixture")
			}
			code, output := run(t, o, "--retire", selected.ID, "--revision", revision)
			if code != 0 || !strings.Contains(output, "RETIRED") || !strings.Contains(output, "unfinished barrier remains") || !strings.Contains(output, "surviving file") {
				t.Fatal(code, output)
			}
			retired := ownershipSnapshot(t, o)
			assertRetirementHistory(t, initial, retired)
			if len(retired.Imports) != 0 || !reflect.DeepEqual(user, retirementUserState(t, o)) {
				t.Fatal("retirement changed user data or did not retire")
			}
			if code, output := run(t, o, "--settle", "--revision", revision); code != 1 {
				t.Fatal("stale settlement admitted", code, output)
			}
			revision = strconv.FormatUint(retired.Revision, 10)
			before = treeState(t, root)
			if code, output := run(t, o, "--settle", "--revision", revision, "--dry-run"); code != 0 || !strings.Contains(output, "observed-eligible=true") {
				t.Fatal(code, output)
			}
			if !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatal("settlement preview wrote state")
			}
			if code, output := run(t, o, "--settle", "--revision", revision); code != 0 || !strings.Contains(output, "SETTLED") {
				t.Fatal(code, output)
			}
			settled := ownershipSnapshot(t, o)
			assertSettlementHistory(t, retired, settled)
			if settled.Unfinished || !reflect.DeepEqual(user, retirementUserState(t, o)) {
				t.Fatal("settlement changed user files or left barrier")
			}
		})
	}
	for _, mode := range []string{"both", "neither", "changed", "symlink", "wrong ID", "stale"} {
		t.Run(mode, func(t *testing.T) {
			o, state, selection := retirementFixture(t, "original")
			i := state.Imports[0]
			switch mode {
			case "both":
				fixtureFile(t, string(i.Source))
			case "neither":
				if err := os.Remove(string(i.Input)); err != nil {
					t.Fatal(err)
				}
			case "changed":
				if err := os.WriteFile(string(i.Input), []byte("changed content"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(string(i.Input), string(i.Source)); err != nil {
					t.Fatal(err)
				}
			case "wrong ID":
				selection.ID = strings.Repeat("b", 32)
			case "stale":
				selection.Revision++
			}
			root := filepath.Dir(filepath.Dir(filepath.Dir(o.Registry)))
			before := treeState(t, root)
			for _, dry := range []bool{true, false} {
				args := []string{"--retire", selection.ID, "--revision", fmt.Sprint(selection.Revision)}
				if dry {
					args = append(args, "--dry-run")
				}
				if code, output := run(t, o, args...); code != 1 {
					t.Fatal(code, output)
				}
			}
			if !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatal("refused action wrote fixture")
			}
		})
	}
	t.Run("barrier and clean versions", func(t *testing.T) {
		for _, version := range []int{1, 2} {
			o, _ := settlementFixture(t, version, "invalidated", true)
			s := ownershipSnapshot(t, o)
			if code, output := run(t, o); code != 0 || !strings.Contains(output, "unfinished=true") {
				t.Fatal(code, output)
			}
			if code, output := run(t, o, "--settle", "--revision", fmt.Sprint(s.Revision)); code != 0 {
				t.Fatal(code, output)
			}
			s = ownershipSnapshot(t, o)
			before := treeState(t, filepath.Dir(o.Registry))
			if code, output := run(t, o, "--settle", "--revision", fmt.Sprint(s.Revision)); code != 0 || !strings.Contains(output, "NOOP") {
				t.Fatal(code, output)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Registry))) {
				t.Fatal("clean no-op rewrote state")
			}
		}
	})
	for _, action := range []string{"retire", "settle"} {
		t.Run(action+" output failure", func(t *testing.T) {
			o, _, selection := retirementFixture(t, "original")
			args := []string{"recover", "--retire", selection.ID, "--revision", fmt.Sprint(selection.Revision)}
			steps := uint64(2)
			if action == "settle" {
				o, _ = settlementFixture(t, 2, "invalidated", true)
				selection.Revision = ownershipSnapshot(t, o).Revision
				args = []string{"recover", "--settle", "--revision", fmt.Sprint(selection.Revision)}
				steps = 1
			}
			user := retirementUserState(t, o)
			readOnly, err := os.Open(o.Registry)
			if err != nil {
				t.Fatal(err)
			}
			defer readOnly.Close()
			cmd := exec.Command(binary, args...)
			cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "XDG_STATE_HOME="+filepath.Dir(filepath.Dir(o.Registry)))
			cmd.Stdout = readOnly
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err == nil || !strings.Contains(stderr.String(), "may already have completed") {
				t.Fatal(err, stderr.String())
			}
			if s := ownershipSnapshot(t, o); len(s.Imports) != 0 || s.Unfinished != (action == "retire") || s.Revision != selection.Revision+steps || !reflect.DeepEqual(user, retirementUserState(t, o)) {
				t.Fatal("output failure retried or undid mutation, or altered user files")
			}
		})
	}
	for _, phase := range []string{"owned", "removing"} {
		t.Run("blocked "+phase, func(t *testing.T) {
			o, s := settlementFixture(t, 2, phase, true)
			before := treeState(t, filepath.Dir(o.Registry))
			for _, dry := range []bool{true, false} {
				args := []string{"--settle", "--revision", fmt.Sprint(s.Revision)}
				if dry {
					args = append(args, "--dry-run")
				}
				if code, output := run(t, o, args...); code != 1 {
					t.Fatal(code, output)
				}
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Registry))) {
				t.Fatal("blocked authority changed")
			}
		})
	}
}
