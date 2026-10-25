package filesystem

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func ownedFixture(t *testing.T, names ...string) (Ownership, []Destination) {
	t.Helper()
	root, destinations := installationFixture(t, names...)
	return Ownership{Registry: filepath.Join(root, "state", "dots", "ownership.json"), Profile: filepath.Join(root, "config.toml"),
		Target: filepath.Join(root, "target"), Sources: []string{filepath.Join(root, "source")}}, destinations
}

func ownershipSnapshot(t *testing.T, options Ownership) snapshot {
	t.Helper()
	r, err := openRegistry(options, false, nativeRegistryOps())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	return r.data
}

func editRegistry(t *testing.T, options Ownership, edit func(*snapshot)) {
	t.Helper()
	r, err := openRegistry(options, true, nativeRegistryOps())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	edit(&r.data)
	if err := r.save(); err != nil {
		t.Fatal(err)
	}
}

func installFixture(t *testing.T, options Ownership, destinations []Destination) {
	t.Helper()
	if report, err := InstallOwned(destinations, options); err != nil || report.LinksCreated != len(destinations) {
		t.Fatalf("install: %+v, %v", report, err)
	}
}

func TestOwnershipPathDiscovery(t *testing.T) {
	for _, xdg := range []string{"", "relative", "/custom/state", "/alias/../state"} {
		want := "/home/example/.local/state/dots/ownership.json"
		if strings.HasPrefix(xdg, "/") {
			want = xdg + "/dots/ownership.json"
		}
		if got := StatePath("/home/example", xdg); got != want {
			t.Errorf("xdg=%q: %q != %q", xdg, got, want)
		}
	}
}

func TestOwnedRemovalAndPreservation(t *testing.T) {
	for _, mode := range []string{"owned", "broken", "file", "directory", "wrong link", "same text replacement", "missing", "untracked", "parent replaced", "parent missing", "parent file", "changed target", "changed profile", "source overlap"} {
		t.Run(mode, func(t *testing.T) {
			options, destinations := ownedFixture(t, ".nested/file")
			installFixture(t, options, destinations)
			dest, source := destinations[0].Path, destinations[0].Source
			wantRemoved, wantErr := 1, false
			switch mode {
			case "broken":
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
			case "file", "directory", "wrong link", "same text replacement":
				// Retain the original inode so identical-text replacement is
				// tested without relying on filesystem inode allocation policy.
				if err := os.Rename(dest, dest+".original"); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "file":
					fixtureFile(t, dest)
				case "directory":
					if err := os.Mkdir(dest, 0o700); err != nil {
						t.Fatal(err)
					}
				case "wrong link":
					fixtureLink(t, "/missing/unmanaged", dest)
				case "same text replacement":
					fixtureLink(t, source, dest)
				}
				wantRemoved, wantErr = 0, true
			case "missing":
				if err := os.Remove(dest); err != nil {
					t.Fatal(err)
				}
				wantRemoved = 0
			case "untracked":
				editRegistry(t, options, func(s *snapshot) { s.Records = []ownershipRecord{} })
				wantRemoved = 0
			case "parent replaced", "parent missing", "parent file":
				parent := filepath.Dir(dest)
				if err := os.Rename(parent, parent+".original"); err != nil {
					t.Fatal(err)
				}
				if mode == "parent replaced" {
					if err := os.Mkdir(parent, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "parent file" {
					fixtureFile(t, parent)
				}
				wantRemoved, wantErr = 0, true
			case "changed target":
				options.Target += "-new"
				wantRemoved = 0
			case "changed profile":
				options.Profile += "-new"
				wantRemoved = 0
			case "source overlap":
				options.Sources = []string{options.Target}
				wantRemoved, wantErr = 0, true
			}
			root := filepath.Dir(options.Profile)
			before := treeState(t, root)
			dry, err := Down(options, true)
			if (err != nil) != wantErr || dry.Removed != 0 || !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatalf("dry-run: %+v, %v", dry, err)
			}
			report, err := Down(options, false)
			if (err != nil) != wantErr || report.Removed != wantRemoved {
				t.Fatalf("down: %+v, %v", report, err)
			}
			if wantRemoved == 1 {
				if _, err := os.Lstat(dest); !os.IsNotExist(err) {
					t.Fatalf("destination remains: %v", err)
				}
				if _, err := os.Stat(filepath.Dir(dest)); err != nil {
					t.Fatal("parent removed:", err)
				}
				if mode != "broken" {
					if _, err := os.Stat(source); err != nil {
						t.Fatal("source removed:", err)
					}
				}
				if again, err := Down(options, false); err != nil || again.Removed != 0 {
					t.Fatalf("repeat: %+v, %v", again, err)
				}
			} else if mode == "file" || mode == "directory" || mode == "wrong link" || mode == "same text replacement" || mode == "untracked" || mode == "changed target" || mode == "changed profile" || mode == "source overlap" {
				if _, err := os.Lstat(dest); err != nil {
					t.Fatal("preserved destination missing:", err)
				}
			}
		})
	}
}

func TestLegacyCorrectLinksAreNeverAdopted(t *testing.T) {
	options, destinations := ownedFixture(t, ".a", ".b")
	if err := os.Mkdir(options.Target, 0o700); err != nil {
		t.Fatal(err)
	}
	fixtureLink(t, destinations[0].Source, destinations[0].Path)
	report, err := InstallOwned(destinations, options)
	if err != nil || report.LinksCreated != 1 {
		t.Fatalf("install: %+v, %v", report, err)
	}
	state := ownershipSnapshot(t, options)
	if len(state.Records) != 1 || state.Records[0].Destination != pathBytes(destinations[1].Path) {
		t.Fatal("legacy link adopted:", state)
	}
	if _, err := InstallOwned(destinations, options); err != nil {
		t.Fatal(err)
	}
	removed, err := Down(options, false)
	if err != nil || removed.Removed != 1 {
		t.Fatalf("down: %+v, %v", removed, err)
	}
	assertLink(t, destinations[0].Path, destinations[0].Source)
}

func TestBrokenConfiguredSourceRootDoesNotBlockRemoval(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	originalSource := options.Sources[0]
	alias := filepath.Join(filepath.Dir(options.Profile), "source-alias")
	fixtureLink(t, options.Sources[0], alias)
	options.Sources = []string{alias}
	destinations[0].Source = filepath.Join(alias, "file-0")
	installFixture(t, options, destinations)
	if err := os.Rename(originalSource, filepath.Join(filepath.Dir(options.Profile), "moved-source")); err != nil {
		t.Fatal(err)
	}
	if report, err := Down(options, false); err != nil || report.Removed != 1 {
		t.Fatalf("missing aliased source: %+v %v", report, err)
	}
}

func TestRegistryRecoveryNeverPromotesOrReactivates(t *testing.T) {
	// Clean "removing" is a synthetic schema-policy case. Actual interrupted
	// operations retain unfinished=true and are tested with child processes.
	for _, phase := range []string{"creating", "owned", "removing", "invalidated"} {
		for _, interrupted := range []bool{false, true} {
			for _, present := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/interrupted=%v/present=%v", phase, interrupted, present), func(t *testing.T) {
					options, destinations := ownedFixture(t, ".a")
					installFixture(t, options, destinations)
					editRegistry(t, options, func(s *snapshot) { s.Records[0].Phase = phase; s.Unfinished = interrupted })
					if !present {
						if err := os.Remove(destinations[0].Path); err != nil {
							t.Fatal(err)
						}
					}
					before := treeState(t, filepath.Dir(options.Profile))
					_, _ = Down(options, true)
					if !reflect.DeepEqual(before, treeState(t, filepath.Dir(options.Profile))) {
						t.Fatal("recovery dry-run wrote")
					}
					report, err := Down(options, false)
					eligible := present && !interrupted && (phase == "owned" || phase == "removing")
					if eligible && (err != nil || report.Removed != 1) {
						t.Fatalf("eligible recovery: %+v, %v", report, err)
					}
					if !eligible && report.Removed != 0 {
						t.Fatal("uncertain entry removed")
					}
					if present && !eligible {
						if err == nil {
							t.Fatal("uncertain ownership accepted")
						}
						assertLink(t, destinations[0].Path, destinations[0].Source)
						if again, _ := Down(options, false); again.Removed != 0 {
							t.Fatal("authority reactivated")
						}
					}
					if !present && err != nil {
						t.Fatal("verified absence not recoverable:", err)
					}
				})
			}
		}
	}
}

func TestRemovalFailuresAndRevalidation(t *testing.T) {
	for _, mode := range []string{"unlink", "leaf swap", "ancestor swap", "post unlink sync", "retirement write"} {
		t.Run(mode, func(t *testing.T) {
			options, destinations := ownedFixture(t, ".a", ".b", ".c")
			installFixture(t, options, destinations)
			ops := nativeRegistryOps()
			calls := 0
			ops.unlink = func(fd int, name string, flags int) error {
				calls++
				if mode == "unlink" && calls == 2 {
					return unix.EACCES
				}
				return unix.Unlinkat(fd, name, flags)
			}
			ops.checkpoint = func(_ string, path string) {
				if path != destinations[1].Path {
					return
				}
				switch mode {
				case "leaf swap":
					if err := os.Rename(path, path+".original"); err != nil {
						t.Fatal(err)
					}
					fixtureFile(t, path)
				case "ancestor swap":
					if err := os.Rename(options.Target, options.Target+".original"); err != nil {
						t.Fatal(err)
					}
					fixtureLink(t, options.Sources[0], options.Target)
				}
			}
			// down owns a copied ops value; update its callbacks through a
			// closure flag instead of mutating an already copied struct.
			triggered := false
			checkpoint := ops.checkpoint
			ops.checkpoint = func(phase, path string) {
				checkpoint(phase, path)
				if path == destinations[1].Path {
					triggered = true
				}
			}
			ops.sync = func(fd int) error {
				if mode == "post unlink sync" && triggered {
					return unix.EIO
				}
				return unix.Fsync(fd)
			}
			ops.write = func(f *os.File, b []byte) (int, error) {
				if mode == "retirement write" && triggered {
					return 0, unix.ENOSPC
				}
				return f.Write(b)
			}
			report, err := down(options, false, ops)
			wantRemoved := 1
			if mode == "post unlink sync" || mode == "retirement write" {
				wantRemoved = 2
			}
			if err == nil || report.Removed != wantRemoved || report.Entries[2].Status != "SKIP" {
				t.Fatalf("failure: %+v, %v", report, err)
			}
			state := ownershipSnapshot(t, options)
			if !state.Unfinished || len(state.Records) != 2 {
				t.Fatal("uncertain records lost:", state)
			}
			if mode != "ancestor swap" {
				if _, err := os.Lstat(destinations[2].Path); err != nil {
					t.Fatal("continued after failure:", err)
				}
			}
			for _, source := range destinations {
				if _, err := os.Stat(source.Source); err != nil {
					t.Fatal("source affected:", err)
				}
			}
		})
	}
}

func TestRegistryPersistenceFailureBoundaries(t *testing.T) {
	for _, mode := range []string{"short write", "write", "sync", "rename", "published sync", "capture"} {
		t.Run(mode, func(t *testing.T) {
			options, destinations := ownedFixture(t, ".a")
			// Bootstrap once so fault counts concern snapshots, not mkdir.
			r, err := openRegistry(options, true, nativeRegistryOps())
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			ops, install := nativeRegistryOps(), nativeInstallOps()
			published := false
			ops.write = func(f *os.File, b []byte) (int, error) {
				if mode == "short write" {
					return 0, nil
				}
				if mode == "write" {
					return 0, unix.ENOSPC
				}
				return f.Write(b)
			}
			ops.sync = func(fd int) error {
				var s unix.Stat_t
				if err := unix.Fstat(fd, &s); err != nil {
					return err
				}
				if mode == "sync" && s.Mode&unix.S_IFMT == unix.S_IFREG {
					return unix.EIO
				}
				if mode == "published sync" && published {
					return unix.EIO
				}
				return unix.Fsync(fd)
			}
			ops.rename = func(a int, b string, c int, d string) error {
				if mode == "rename" {
					return unix.EIO
				}
				err := unix.Renameat(a, b, c, d)
				published = err == nil
				return err
			}
			install.checkpoint = func(phase string, item Destination) {
				if mode == "capture" && phase == "before-ownership" {
					if err := os.Remove(item.Path); err != nil {
						t.Fatal(err)
					}
					fixtureFile(t, item.Path)
				}
			}
			report, err := installOwned(destinations, options, ops, install)
			if err == nil {
				t.Fatal("failure ignored")
			}
			if mode == "capture" {
				state := ownershipSnapshot(t, options)
				if report.LinksCreated != 1 || len(state.Records) != 1 || state.Records[0].Phase != "creating" || !state.Unfinished {
					t.Fatalf("creation gap adopted: %+v %+v", report, state)
				}
				if again, _ := Down(options, false); again.Removed != 0 {
					t.Fatal("replacement adopted")
				}
			} else if report.LinksCreated != 0 {
				t.Fatal("mutated despite failed intent persistence")
			}
		})
	}
}

func TestRegistryValidationAndReadOnlyFailures(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt", "version", "missing barrier", "duplicate key", "duplicate claim", "missing leaf field", "registry symlink", "registry directory", "registry permissions", "directory permissions", "missing lock"} {
		t.Run(mode, func(t *testing.T) {
			options, destinations := ownedFixture(t, ".a")
			if mode != "missing" {
				installFixture(t, options, destinations)
			}
			data := []byte{}
			if mode != "missing" {
				var err error
				data, err = os.ReadFile(options.Registry)
				if err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "corrupt":
				data = []byte("{")
			case "version":
				data = bytes.Replace(data, []byte(`"version": 1`), []byte(`"version": 99`), 1)
			case "missing barrier":
				data = bytes.Replace(data, []byte("  \"unfinished\": false,\n"), nil, 1)
			case "duplicate key":
				data = bytes.Replace(data, []byte(`"unfinished": false`), []byte(`"unfinished": false, "unfinished": true`), 1)
			case "missing leaf field":
				data = bytes.Replace(data, []byte("        \"size\": "), []byte("        \"unrecognized\": "), 1)
			case "duplicate claim":
				state := ownershipSnapshot(t, options)
				state.Records = append(state.Records, state.Records[0])
				var err error
				data, err = json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
			case "registry symlink", "registry directory":
				if err := os.Rename(options.Registry, options.Registry+".original"); err != nil {
					t.Fatal(err)
				}
				if mode == "registry symlink" {
					fixtureLink(t, options.Registry+".original", options.Registry)
				} else if err := os.Mkdir(options.Registry, 0o700); err != nil {
					t.Fatal(err)
				}
			case "registry permissions":
				if err := os.Chmod(options.Registry, 0o644); err != nil {
					t.Fatal(err)
				}
			case "directory permissions":
				if err := os.Chmod(filepath.Dir(options.Registry), 0o755); err != nil {
					t.Fatal(err)
				}
			case "missing lock":
				if err := os.Remove(filepath.Join(filepath.Dir(options.Registry), "ownership.lock")); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "corrupt" || mode == "version" || mode == "missing barrier" || mode == "duplicate key" || mode == "missing leaf field" || mode == "duplicate claim" {
				if err := os.WriteFile(options.Registry, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := treeState(t, filepath.Dir(options.Profile))
			_, dryErr := Down(options, true)
			if mode != "missing" && mode != "missing lock" && dryErr == nil {
				t.Fatal("unsafe state accepted")
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(options.Profile))) {
				t.Fatal("read-only command changed state")
			}
			if mode != "missing" {
				if report, err := Down(options, false); err == nil || report.Removed != 0 {
					t.Fatalf("unsafe mutation: %+v, %v", report, err)
				}
			}
		})
	}
}

func TestSnapshotRejectsDuplicateClaimsAndMissingEvidence(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	state := ownershipSnapshot(t, options)
	state.Records = append(state.Records, state.Records[0])
	if err := validateSnapshot(state); err == nil {
		t.Fatal("duplicate accepted")
	}
	state.Records = state.Records[:1]
	state.Records[0].Leaf = nil
	if err := validateSnapshot(state); err == nil {
		t.Fatal("missing evidence accepted")
	}
}

func TestOwnershipPathBytesRoundTrip(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	editRegistry(t, options, func(s *snapshot) {
		s.Records[0].Destination += "\xff"
		s.Records[0].Profile += "\xfe"
		s.Records[0].Routes[0].Text += "\xfd"
	})
	state := ownershipSnapshot(t, options)
	if state.Records[0].Destination != pathBytes(destinations[0].Path+"\xff") || state.Records[0].Profile != pathBytes(options.Profile+"\xfe") || !strings.HasSuffix(string(state.Records[0].Routes[0].Text), "\xfd") {
		t.Fatal("path bytes lost")
	}
}

func TestDotsProcessLockCoordination(t *testing.T) {
	if os.Getenv("DOTS_LOCK_CHILD") == "1" {
		options := Ownership{Registry: os.Getenv("DOTS_LOCK_REGISTRY"), Profile: "/fixture/config", Target: "/fixture/target"}
		r, err := openRegistry(options, true, nativeRegistryOps())
		if r != nil {
			_ = r.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "busy") {
			t.Fatalf("child acquired held lock: %v", err)
		}
		return
	}
	options, _ := ownedFixture(t, ".a")
	r, err := openRegistry(options, true, nativeRegistryOps())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	command := exec.Command(os.Args[0], "-test.run=^TestDotsProcessLockCoordination$")
	command.Env = append(os.Environ(), "DOTS_LOCK_CHILD=1", "DOTS_LOCK_REGISTRY="+options.Registry)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("independent process coordination: %s %v", output, err)
	}
	before := treeState(t, filepath.Dir(options.Profile))
	if _, err := Down(options, true); err != nil {
		t.Fatal("dry-run requires lock:", err)
	}
	if !reflect.DeepEqual(before, treeState(t, filepath.Dir(options.Profile))) {
		t.Fatal("dry-run wrote while locked")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if next, err := openRegistry(options, true, nativeRegistryOps()); err != nil {
		t.Fatal("lock not released:", err)
	} else {
		_ = next.Close()
	}
}

func TestRegistryWriteErrorsKeepDurableBarrier(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	ops := nativeRegistryOps()
	writes := 0
	ops.write = func(f *os.File, b []byte) (int, error) {
		writes++
		if writes >= 2 {
			return 0, io.ErrClosedPipe
		}
		return f.Write(b)
	}
	ops.checkpoint = func(string, string) { t.Fatal("unexpected removal") }
	if err := os.Rename(destinations[0].Path, destinations[0].Path+".original"); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, destinations[0].Path)
	_, err := down(options, false, ops)
	if !errors.Is(err, io.ErrClosedPipe) || !ownershipSnapshot(t, options).Unfinished {
		t.Fatalf("revocation uncertainty lost: %v", err)
	}
	// Even restoring the exact original object cannot resurrect authority.
	if err := os.Remove(destinations[0].Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(destinations[0].Path+".original", destinations[0].Path); err != nil {
		t.Fatal(err)
	}
	if report, _ := Down(options, false); report.Removed != 0 {
		t.Fatal("authority resurrected")
	}
}

func TestSnapshotJSONEncodingIsValid(t *testing.T) {
	data, err := json.Marshal(snapshot{Version: 1, Records: []ownershipRecord{}})
	if err != nil || strictJSON(data) != nil {
		t.Fatalf("invalid initial snapshot: %s %v", data, err)
	}
}
