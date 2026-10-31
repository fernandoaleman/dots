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
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestImportCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "dots")
	if output, err := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", binary, "../..").CombinedOutput(); err != nil {
		t.Fatalf("build isolated executable: %v\n%s", err, output)
	}
	configure := func(t *testing.T, q ImportRequest, extra string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(q.Ownership.Profile), 0o700); err != nil {
			t.Fatal(err)
		}
		var roots []string
		for _, root := range q.Ownership.Sources {
			roots = append(roots, fmt.Sprintf("%q", root))
		}
		data := fmt.Sprintf("dotfiles_dirs = [%s]\ntarget_dir = %q\n%s", strings.Join(roots, ","), q.Ownership.Target, extra)
		if err := os.WriteFile(q.Ownership.Profile, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	command := func(t *testing.T, q ImportRequest, args ...string) (*exec.Cmd, context.CancelFunc) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, binary, args...)
		base := filepath.Dir(filepath.Dir(filepath.Dir(q.Ownership.Registry)))
		cmd.Dir = base
		cmd.Env = append(cmd.Environ(), "HOME="+base+"/home", "XDG_CONFIG_HOME="+base+"/missing-config", "XDG_STATE_HOME="+filepath.Dir(filepath.Dir(q.Ownership.Registry)), "TMPDIR="+base)
		return cmd, cancel
	}
	run := func(t *testing.T, q ImportRequest, args ...string) (int, string) {
		t.Helper()
		cmd, cancel := command(t, q, args...)
		defer cancel()
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
	add := func(q ImportRequest, flags ...string) []string {
		args := append([]string{"--config", q.Ownership.Profile, "add"}, flags...)
		return append(args, q.Input)
	}
	for _, mode := range []string{"fresh", "nested", "existing", "selected lower root", "unrelated blocked", "configured traversal", "xdg config"} {
		t.Run(mode, func(t *testing.T) {
			q := importInspectionFixture(t, mode == "nested")
			if mode == "xdg config" {
				q.Ownership.Profile = filepath.Dir(q.Root) + "/missing-config/dots/config.toml"
			}
			var history []ownershipRecord
			if mode == "existing" {
				managed := Destination{Path: q.Ownership.Target + "/.managed", Source: q.Root + "/managed"}
				fixtureFile(t, managed.Source)
				installFixture(t, q.Ownership, []Destination{managed})
				history = ownershipSnapshot(t, q.Ownership).Records
			}
			if mode == "selected lower root" {
				higher := filepath.Dir(q.Root) + "/higher"
				if err := os.Mkdir(higher, 0o700); err != nil {
					t.Fatal(err)
				}
				q.Ownership.Sources = []string{higher, q.Root}
			}
			if mode == "unrelated blocked" {
				fixtureFile(t, q.Root+"/other/file")
				fixtureFile(t, q.Ownership.Target+"/.other")
			}
			if mode == "configured traversal" {
				base := filepath.Dir(q.Root)
				if err := os.MkdirAll(base+"/actual/deep", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(base+"/actual/deep", base+"/route"); err != nil {
					t.Fatal(err)
				}
				q.Ownership.Target = base + "/route/.."
				q.Input = q.Ownership.Target + "/.zshrc"
				fixtureFile(t, q.Input)
			}
			configure(t, q, "")
			callAdd := func(flags ...string) []string {
				args := add(q, flags...)
				if mode == "fresh" {
					args[len(args)-1] = "./target/.zshrc"
				}
				if mode == "xdg config" {
					args = args[2:]
				}
				return args
			}
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(q.Input)
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Dir(q.Root)
			before := treeState(t, base)
			var rootFlags []string
			if mode == "selected lower root" {
				rootFlags = []string{"--root", q.Root}
			}
			flags := append(append([]string{}, rootFlags...), "--dry-run")
			code, preview := run(t, q, callAdd(flags...)...)
			if code != 0 || !strings.Contains(preview, "PREVIEW 0 changes made") {
				t.Fatal(code, preview)
			}
			if again, repeated := run(t, q, callAdd(flags...)...); again != 0 || repeated != preview {
				t.Fatal(again, repeated)
			}
			if !reflect.DeepEqual(before, treeState(t, base)) {
				t.Fatal("preview changed fixture")
			}
			code, output := run(t, q, callAdd(rootFlags...)...)
			if code != 0 || !strings.Contains(output, "IMPORTED confirmed-relocated=true confirmed-linked=true confirmed-owned=true") {
				t.Fatal(code, output)
			}
			assertRelocated(t, q, importFile{fingerprintOf(original), original.Uid, original.Gid, uint64(original.Nlink)}, content, true)
			s := ownershipSnapshot(t, q.Ownership)
			if s.Version != 2 || s.Unfinished || len(s.Imports) != 0 {
				t.Fatal(s)
			}
			found := false
			for _, record := range s.Records {
				if record.Destination == pathBytes(q.Input) && record.Profile == pathBytes(q.Ownership.Profile) && record.Phase == "owned" {
					found = true
				}
			}
			if !found {
				t.Fatal("import did not publish owned profile/target record")
			}
			for _, old := range history {
				preserved := false
				for _, current := range s.Records {
					preserved = preserved || reflect.DeepEqual(old, current)
				}
				if !preserved {
					t.Fatal("existing ownership history changed")
				}
			}
			before = treeState(t, base)
			if code, output := run(t, q, callAdd(rootFlags...)...); code != 1 {
				t.Fatal("repeated add adopted link", code, output)
			}
			if !reflect.DeepEqual(before, treeState(t, base)) {
				t.Fatal("repeated add changed state")
			}
			if mode != "unrelated blocked" {
				if code, output := run(t, q, "--config", q.Ownership.Profile, "status"); code != 0 {
					t.Fatal(code, output)
				}
				for _, dry := range []bool{true, false} {
					args := []string{"--config", q.Ownership.Profile, "down"}
					if dry {
						args = append(args, "--dry-run")
					}
					if code, output := run(t, q, args...); code != 0 {
						t.Fatal(code, output)
					}
				}
				if _, err := os.Lstat(q.Input); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("down retained imported link", err)
				}
				got, err := os.ReadFile(q.Source)
				if err != nil || !bytes.Equal(got, content) {
					t.Fatal("down removed source content", err)
				}
				for _, parent := range []string{q.Root, filepath.Dir(q.Source), q.Ownership.Target, filepath.Dir(q.Input)} {
					if info, err := os.Stat(parent); err != nil || !info.IsDir() {
						t.Fatal("down removed parent", parent, err)
					}
				}
			}
		})
	}
	for _, mode := range []string{"source collision", "higher provider", "manual link", "directory", "missing", "hard link", "multiple roots", "excluded", "read-only root", "historical claim"} {
		t.Run(mode, func(t *testing.T) {
			q := importInspectionFixture(t, false)
			extra := ""
			switch mode {
			case "source collision":
				fixtureFile(t, q.Source)
			case "higher provider":
				higher := filepath.Dir(q.Root) + "/higher"
				fixtureFile(t, higher+"/.zshrc")
				q.Ownership.Sources = []string{higher, q.Root}
			case "manual link":
				if err := os.Rename(q.Input, q.Input+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(q.Input+".saved", q.Input); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Rename(q.Input, q.Input+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(q.Input, 0o700); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Rename(q.Input, q.Input+".saved"); err != nil {
					t.Fatal(err)
				}
			case "hard link":
				if err := os.Link(q.Input, q.Input+".hard"); err != nil {
					t.Fatal(err)
				}
			case "multiple roots":
				other := filepath.Dir(q.Root) + "/other"
				if err := os.Mkdir(other, 0o700); err != nil {
					t.Fatal(err)
				}
				q.Ownership.Sources = append(q.Ownership.Sources, other)
			case "excluded":
				extra = "excludes = [\"zshrc\"]\n"
			case "read-only root":
				if os.Geteuid() == 0 {
					t.Skip("permission refusal requires unprivileged user")
				}
				if err := os.Chmod(q.Root, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(q.Root, 0o700) })
			case "historical claim":
				old := Destination{Path: q.Ownership.Target + "/.old", Source: q.Root + "/old"}
				fixtureFile(t, old.Source)
				installFixture(t, q.Ownership, []Destination{old})
				editRegistry(t, q.Ownership, func(s *snapshot) {
					s.Records[0].Profile = pathBytes(q.Ownership.Profile + ".historical")
					s.Records[0].Destination = pathBytes(q.Input)
					s.Records[0].Text = pathBytes(q.Source)
					s.Records[0].Phase = "invalidated"
				})
			}
			configure(t, q, extra)
			base := filepath.Dir(q.Root)
			before := treeState(t, base)
			for _, dry := range []bool{true, false} {
				flags := []string{}
				if mode == "higher provider" {
					flags = append(flags, "--root", q.Root)
				}
				if dry {
					flags = append(flags, "--dry-run")
				}
				if code, output := run(t, q, add(q, flags...)...); code != 1 || mode == "historical claim" && !strings.Contains(output, "overlaps recorded ownership claim") {
					t.Fatal(code, output)
				}
			}
			if !reflect.DeepEqual(before, treeState(t, base)) {
				t.Fatal("rejected import changed data or initialized state")
			}
		})
	}
	t.Run("cross filesystem", func(t *testing.T) {
		q := importInspectionFixture(t, false)
		info, err := os.Stat("/dev/shm")
		if err != nil || !info.IsDir() {
			t.Skip("no existing second-filesystem fixture location")
		}
		other, err := os.MkdirTemp("/dev/shm", "dots-add-cli-")
		if err != nil {
			t.Skipf("second-filesystem fixture unavailable: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(other) })
		var input, root unix.Stat_t
		if err := unix.Stat(q.Input, &input); err != nil {
			t.Fatal(err)
		}
		if err := unix.Stat(other, &root); err != nil {
			t.Fatal(err)
		}
		if input.Dev == root.Dev {
			t.Skip("candidate fixture is on the same filesystem")
		}
		q.Root, q.Source = other, other+"/zshrc"
		q.Ownership.Sources = []string{other}
		configure(t, q, "")
		base := filepath.Dir(filepath.Dir(filepath.Dir(q.Ownership.Registry)))
		before, sourceBefore := treeState(t, base), treeState(t, other)
		for _, dry := range []bool{true, false} {
			flags := []string{}
			if dry {
				flags = append(flags, "--dry-run")
			}
			if code, output := run(t, q, add(q, flags...)...); code != 1 || !strings.Contains(output, "filesystem") {
				t.Fatal(code, output)
			}
		}
		if !reflect.DeepEqual(before, treeState(t, base)) || !reflect.DeepEqual(sourceBefore, treeState(t, other)) {
			t.Fatal("cross-filesystem refusal wrote data")
		}
	})
	for _, relocated := range []bool{false, true} {
		t.Run(fmt.Sprintf("interrupted recovery %t", relocated), func(t *testing.T) {
			q := preparationFixture(t, false, 1)
			configure(t, q, "")
			p := liveImport(t, q)
			if relocated {
				p.ops.checkpoint = func(stage string) {
					if stage == "relocation" {
						p.ops.registry.sync = func(int) error { return unix.EIO }
					}
				}
				if report, err := p.complete(nativeImportCompleteOps()); err == nil || !report.Relocated || report.Linked {
					t.Fatal(report, err)
				}
			} else if err := p.Close(); err != nil {
				t.Fatal(err)
			}
			s := ownershipSnapshot(t, q.Ownership)
			id := s.Imports[0].ID
			before := treeState(t, filepath.Dir(q.Root))
			if code, output := run(t, q, add(q)...); code != 1 || !strings.Contains(output, "dots recover") {
				t.Fatal(code, output)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(q.Root))) {
				t.Fatal("pending import mutated")
			}
			if code, output := run(t, q, "recover"); code != 0 || !strings.Contains(output, id) {
				t.Fatal(code, output)
			}
			if code, output := run(t, q, "recover", "--retire", id, "--revision", fmt.Sprint(s.Revision)); code != 0 {
				t.Fatal(code, output)
			}
			s = ownershipSnapshot(t, q.Ownership)
			if code, output := run(t, q, "recover", "--settle", "--revision", fmt.Sprint(s.Revision)); code != 0 {
				t.Fatal(code, output)
			}
			if !relocated {
				if code, output := run(t, q, add(q)...); code != 0 {
					t.Fatal(code, output)
				}
			} else {
				if _, err := os.Stat(q.Source); err != nil {
					t.Fatal("recovery lost relocated data", err)
				}
			}
		})
	}
	t.Run("output failure", func(t *testing.T) {
		q := importInspectionFixture(t, false)
		configure(t, q, "")
		sink, err := os.Open(q.Ownership.Profile)
		if err != nil {
			t.Fatal(err)
		}
		defer sink.Close()
		cmd, cancel := command(t, q, add(q)...)
		defer cancel()
		cmd.Stdout = sink
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil || !strings.Contains(stderr.String(), "may have completed") {
			t.Fatal(err, stderr.String())
		}
		assertLink(t, q.Input, q.Source)
		if s := ownershipSnapshot(t, q.Ownership); s.Unfinished || len(s.Imports) != 0 || len(s.Records) != 1 || s.Records[0].Phase != "owned" {
			t.Fatal("output failure repeated/undid import", s)
		}
	})
}

func TestRunImportRefreshRefusal(t *testing.T) {
	for _, mode := range []string{"selection", "registry replacement"} {
		t.Run(mode, func(t *testing.T) {
			q := preparationFixture(t, false, 1)
			var original unix.Stat_t
			if err := unix.Lstat(q.Input, &original); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(q.Input)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			var replacement []byte
			report, err := RunImport(q, func() (ImportRequest, error) {
				calls++
				fresh := q
				if calls == 2 {
					if mode == "selection" {
						fresh.Source += ".changed"
					} else {
						var e error
						replacement, e = os.ReadFile(q.Ownership.Registry)
						if e != nil {
							t.Fatal(e)
						}
						if e = os.WriteFile(q.Ownership.Registry+".replacement", replacement, 0o600); e != nil {
							t.Fatal(e)
						}
						if e = os.Rename(q.Ownership.Registry+".replacement", q.Ownership.Registry); e != nil {
							t.Fatal(e)
						}
					}
				}
				return fresh, nil
			})
			if err == nil || calls != 2 || report.Relocated || report.Linked || report.Owned {
				t.Fatal(report, calls, err)
			}
			assertPreparationInput(t, q, original, content)
			if replacement != nil {
				got, e := os.ReadFile(q.Ownership.Registry)
				if e != nil || !bytes.Equal(got, replacement) {
					t.Fatal("replacement registry overwritten", e)
				}
			}
		})
	}
}
