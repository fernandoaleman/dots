package filesystem

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestPersistenceFailuresAtEveryTransactionPhase(t *testing.T) {
	for _, phase := range []string{"creating", "owned", "install finish", "removing", "retirement", "remove finish"} {
		for _, failure := range []string{"write", "file sync", "close", "rename", "published sync"} {
			t.Run(phase+"/"+failure, func(t *testing.T) {
				options, destinations := ownedFixture(t, ".a")
				removal := phase == "removing" || phase == "retirement" || phase == "remove finish"
				if removal {
					installFixture(t, options, destinations)
				}
				ops := nativeRegistryOps()
				armed, published, injected := false, false, false
				ops.write = func(f *os.File, data []byte) (int, error) {
					var s snapshot
					if err := json.Unmarshal(data, &s); err != nil {
						t.Fatal(err)
					}
					armed, published = false, false
					switch phase {
					case "creating", "owned", "removing":
						armed = len(s.Records) == 1 && s.Records[0].Phase == phase
					case "install finish":
						armed = !s.Unfinished && len(s.Records) == 1
					case "retirement":
						armed = s.Unfinished && len(s.Records) == 0
					case "remove finish":
						armed = !s.Unfinished && len(s.Records) == 0
					}
					if armed && failure == "write" {
						injected = true
						return 0, unix.ENOSPC
					}
					return f.Write(data)
				}
				ops.sync = func(fd int) error {
					var stat unix.Stat_t
					if err := unix.Fstat(fd, &stat); err != nil {
						return err
					}
					if armed && ((failure == "file sync" && stat.Mode&unix.S_IFMT == unix.S_IFREG) || (failure == "published sync" && published)) {
						injected = true
						return unix.EIO
					}
					return unix.Fsync(fd)
				}
				ops.closeFile = func(f *os.File) error {
					err := f.Close()
					if armed && failure == "close" {
						injected = true
						return errors.Join(err, unix.EIO)
					}
					return err
				}
				ops.rename = func(a int, b string, c int, d string) error {
					if armed && failure == "rename" {
						injected = true
						return unix.EIO
					}
					err := unix.Renameat(a, b, c, d)
					published = err == nil
					return err
				}
				var operationErr error
				actual := 0
				if removal {
					report, err := down(options, false, ops)
					actual, operationErr = report.Removed, err
				} else {
					report, err := installOwned(destinations, options, ops, nativeInstallOps())
					actual, operationErr = report.LinksCreated, err
				}
				want := 1
				if phase == "creating" || phase == "removing" {
					want = 0
				}
				if !injected || operationErr == nil || actual != want {
					t.Fatalf("phase boundary missed: injected=%v actual=%d want=%d err=%v", injected, actual, want, operationErr)
				}
				state := ownershipSnapshot(t, options)
				// A final clean snapshot may already be visible after rename
				// even when its directory synchronization returned an error.
				cleanVisible := (phase == "install finish" || phase == "remove finish") && failure == "published sync"
				if !state.Unfinished && !cleanVisible {
					t.Fatalf("unfinished barrier lost: %+v", state)
				}
				before := treeState(t, filepath.Dir(options.Profile))
				_, _ = Down(options, true)
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(options.Profile))) {
					t.Fatal("recovery preview wrote")
				}
				if !cleanVisible {
					if report, _ := Down(options, false); report.Removed != 0 {
						t.Fatal("interrupted authority replayed")
					}
				}
			})
		}
	}
}

func TestProcessInterruptionAndConcurrentCommands(t *testing.T) {
	if mode := os.Getenv("DOTS_TRANSACTION_CHILD"); mode != "" {
		var options Ownership
		var destinations []Destination
		if err := json.Unmarshal([]byte(os.Getenv("DOTS_TRANSACTION_OPTIONS")), &options); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(os.Getenv("DOTS_TRANSACTION_DESTINATIONS")), &destinations); err != nil {
			t.Fatal(err)
		}
		pause := func() {
			fmt.Fprintln(os.Stdout, "READY")
			var response [1]byte
			if _, err := os.Stdin.Read(response[:]); err != nil {
				t.Fatal(err)
			}
			// Process exit deliberately bypasses all Go defers. The kernel
			// closes descriptors; no filesystem cleanup or rollback runs.
			os.Exit(23)
		}
		ops, install := nativeRegistryOps(), nativeInstallOps()
		if mode == "creation capture" {
			install.checkpoint = func(phase string, _ Destination) {
				if phase == "before-ownership" {
					pause()
				}
			}
		}
		if mode == "owned publication" {
			owned := false
			ops.write = func(f *os.File, data []byte) (int, error) {
				var s snapshot
				if err := json.Unmarshal(data, &s); err != nil {
					t.Fatal(err)
				}
				owned = s.Unfinished && len(s.Records) == 1 && s.Records[0].Phase == "owned"
				return f.Write(data)
			}
			ops.sync = func(fd int) error {
				err := unix.Fsync(fd)
				var s unix.Stat_t
				if statErr := unix.Fstat(fd, &s); statErr != nil {
					return statErr
				}
				if err == nil && owned && s.Mode&unix.S_IFMT == unix.S_IFDIR {
					pause()
				}
				return err
			}
		}
		if mode == "after unlink" {
			ops.unlink = func(fd int, name string, flags int) error {
				err := unix.Unlinkat(fd, name, flags)
				if err == nil {
					pause()
				}
				return err
			}
		}
		if mode == "before unlink" {
			ops.checkpoint = func(string, string) { pause() }
		}
		if mode == "after unlink" || mode == "before unlink" {
			_, _ = down(options, false, ops)
		} else {
			_, _ = installOwned(destinations, options, ops, install)
		}
		t.Fatal("child failed to reach interruption checkpoint")
	}
	for _, mode := range []string{"creation capture", "owned publication", "before unlink", "after unlink"} {
		t.Run(mode, func(t *testing.T) {
			options, destinations := ownedFixture(t, ".a")
			if mode == "after unlink" || mode == "before unlink" {
				installFixture(t, options, destinations)
			}
			encodedOptions, err := json.Marshal(options)
			if err != nil {
				t.Fatal(err)
			}
			encodedDestinations, err := json.Marshal(destinations)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessInterruptionAndConcurrentCommands$")
			command.Env = append(os.Environ(), "DOTS_TRANSACTION_CHILD="+mode, "DOTS_TRANSACTION_OPTIONS="+string(encodedOptions), "DOTS_TRANSACTION_DESTINATIONS="+string(encodedDestinations))
			output, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			input, err := command.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			command.Stderr = &stderr
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = input.Close(); cancel() }()
			ready, err := bufio.NewReader(output).ReadString('\n')
			if err != nil || ready != "READY\n" {
				_ = command.Wait()
				t.Fatalf("child handshake: %q %v %s", ready, err, &stderr)
			}
			before := treeState(t, filepath.Dir(options.Profile))
			if _, err := InstallOwned(destinations, options); err == nil || !strings.Contains(err.Error(), "busy") {
				t.Fatalf("competing up: %v", err)
			}
			if _, err := Down(options, false); err == nil || !strings.Contains(err.Error(), "busy") {
				t.Fatalf("competing down: %v", err)
			}
			_, _ = Down(options, true)
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(options.Profile))) {
				t.Fatal("competing invocation changed namespace")
			}
			if _, err := input.Write([]byte("X")); err != nil {
				t.Fatal(err)
			}
			err = command.Wait()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 23 {
				t.Fatalf("child interruption failed: %v %s", err, &stderr)
			}
			state := ownershipSnapshot(t, options)
			if !state.Unfinished || len(state.Records) != 1 {
				t.Fatal("interrupted snapshot lost:", state)
			}
			recovery, recoveryErr := Down(options, false)
			if recovery.Removed != 0 {
				t.Fatal("recovery deleted a present interrupted object")
			}
			recovered := ownershipSnapshot(t, options)
			if mode == "after unlink" {
				if recoveryErr != nil || len(recovered.Records) != 0 || recovered.Unfinished {
					t.Fatalf("absent recovery did not complete: %+v %v", recovered, recoveryErr)
				}
			} else {
				wantPhase := "invalidated"
				if mode == "creation capture" {
					wantPhase = "creating"
				}
				if recoveryErr == nil || len(recovered.Records) != 1 || recovered.Records[0].Phase != wantPhase || !recovered.Unfinished {
					t.Fatalf("present recovery not preserved: %+v %v", recovered, recoveryErr)
				}
			}
			if mode != "after unlink" {
				assertLink(t, destinations[0].Path, destinations[0].Source)
			}
		})
	}
}

func TestSnapshotSizeLimitPreservesReadableAuthority(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	before, err := os.ReadFile(options.Registry)
	if err != nil {
		t.Fatal(err)
	}
	r, err := openRegistry(options, true, nativeRegistryOps())
	if err != nil {
		t.Fatal(err)
	}
	r.data.Records[0].Text = pathBytes("/" + strings.Repeat("x", registrySizeLimit))
	if err := r.save(); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized writer: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(options.Registry)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("oversized snapshot replaced authority")
	}
	if len(ownershipSnapshot(t, options).Records) != 1 {
		t.Fatal("previous state unreadable")
	}
}

func TestExactSchemaRejectsCaseVariants(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	data, err := os.ReadFile(options.Registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"unfinished", "phase", "device", "mode", "name"} {
		changed := bytes.Replace(data, []byte(`"`+field+`":`), []byte(`"`+strings.ToUpper(field)+`":`), 1)
		if err := strictJSON(changed); err == nil {
			t.Fatalf("case variant accepted: %s", field)
		}
	}
	changed := bytes.Replace(data, []byte(`"unfinished": false`), []byte(`"unfinished": true, "Unfinished": false`), 1)
	if err := os.WriteFile(options.Registry, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if report, err := Down(options, false); err == nil || report.Removed != 0 {
		t.Fatalf("barrier bypass: %+v %v", report, err)
	}
	assertLink(t, destinations[0].Path, destinations[0].Source)
}

func TestOwnershipProtectionReportsEveryEntry(t *testing.T) {
	options, destinations := ownedFixture(t, ".a", ".b", ".c")
	destinations[0].Path = filepath.Join(filepath.Dir(options.Registry), "reserved")
	before := treeState(t, filepath.Dir(options.Profile))
	report, err := InstallOwned(destinations, options)
	if err == nil || len(report.Results) != 3 || report.LinksCreated != 0 {
		t.Fatalf("incomplete preflight report: %+v %v", report, err)
	}
	for _, result := range report.Results {
		if result.Status != "SKIP" {
			t.Fatal("unexpected preflight action:", result)
		}
	}
	if !reflect.DeepEqual(before, treeState(t, filepath.Dir(options.Profile))) {
		t.Fatal("protection wrote state")
	}
}

func TestHistoricalUnavailableTargetDoesNotBlockNewInstallation(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	old := options.Target
	if err := os.Rename(old, old+".original"); err != nil {
		t.Fatal(err)
	}
	fixtureLink(t, "/missing/old-target", old)
	options.Target += "-new"
	destinations[0].Path = filepath.Join(options.Target, ".a")
	installFixture(t, options, destinations)
	if report, err := Down(options, false); err != nil || report.Removed != 1 {
		t.Fatalf("current scope: %+v %v", report, err)
	}
	assertLink(t, filepath.Join(old+".original", ".a"), destinations[0].Source)
}

func TestRemovalAncestorSubstitutionPreservesExternalSentinels(t *testing.T) {
	options, destinations := ownedFixture(t, ".a", ".b", ".c")
	installFixture(t, options, destinations)
	external := filepath.Join(filepath.Dir(options.Profile), "external")
	for _, name := range []string{".a", ".b", ".c", "nested/sentinel"} {
		fixtureFile(t, filepath.Join(external, name))
	}
	fixtureLink(t, "missing", filepath.Join(external, "broken"))
	before := treeState(t, external)
	ops := nativeRegistryOps()
	ops.checkpoint = func(_ string, destination string) {
		if destination != destinations[1].Path {
			return
		}
		if err := os.Rename(options.Target, options.Target+".original"); err != nil {
			t.Fatal(err)
		}
		fixtureLink(t, external, options.Target)
	}
	report, err := down(options, false, ops)
	if err == nil || report.Removed != 1 {
		t.Fatalf("ancestor substitution: %+v %v", report, err)
	}
	if !reflect.DeepEqual(before, treeState(t, external)) {
		t.Fatal("external namespace modified")
	}
	for _, destination := range destinations[1:] {
		assertLink(t, filepath.Join(options.Target+".original", filepath.Base(destination.Path)), destination.Source)
	}
}

func TestStateTrustBootstrapAndHandleCleanup(t *testing.T) {
	for _, mode := range []string{"lock symlink", "lock hard link", "lock permissions", "registry hard link", "private directory symlink", "bootstrap sync"} {
		t.Run(mode, func(t *testing.T) {
			options, destinations := ownedFixture(t, ".a")
			ops := nativeRegistryOps()
			if mode != "bootstrap sync" {
				installFixture(t, options, destinations)
			}
			lock := filepath.Join(filepath.Dir(options.Registry), "ownership.lock")
			switch mode {
			case "lock symlink":
				if err := os.Rename(lock, lock+".original"); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, lock+".original", lock)
			case "lock hard link":
				if err := os.Link(lock, lock+".another"); err != nil {
					t.Fatal(err)
				}
			case "lock permissions":
				if err := os.Chmod(lock, 0o644); err != nil {
					t.Fatal(err)
				}
			case "registry hard link":
				if err := os.Link(options.Registry, options.Registry+".another"); err != nil {
					t.Fatal(err)
				}
			case "private directory symlink":
				dir := filepath.Dir(options.Registry)
				if err := os.Rename(dir, dir+".original"); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, dir+".original", dir)
			case "bootstrap sync":
				ops.sync = func(int) error { return unix.EIO }
			}
			before := treeState(t, options.Sources[0])
			if report, err := down(options, false, ops); err == nil || report.Removed != 0 {
				t.Fatalf("unsafe state trust: %+v %v", report, err)
			}
			if !reflect.DeepEqual(before, treeState(t, options.Sources[0])) {
				t.Fatal("source modified")
			}
		})
	}
	options, _ := ownedFixture(t, ".a")
	for range 20 {
		r, err := openRegistry(options, true, nativeRegistryOps())
		if err != nil {
			t.Fatal(err)
		}
		fds := []int{r.lock}
		for _, directory := range r.space.order {
			fds = append(fds, directory.fd)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if err := r.Close(); err != nil {
			t.Fatal("repeated close:", err)
		}
		for _, fd := range fds {
			if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
				t.Fatalf("descriptor remains open: %d %v", fd, err)
			}
		}
	}
}

func TestAliasOverlapProtectionBeforeStateInitialization(t *testing.T) {
	for _, mode := range []string{"state inside source", "destination inside state", "destination inside aliased source"} {
		t.Run(mode, func(t *testing.T) {
			options, destinations := ownedFixture(t, ".a")
			root := filepath.Dir(options.Profile)
			switch mode {
			case "state inside source":
				alias := filepath.Join(root, "alias")
				fixtureLink(t, options.Sources[0], alias)
				options.Registry = filepath.Join(alias, "dots", "ownership.json")
			case "destination inside state":
				state := filepath.Join(root, "state")
				if err := os.Mkdir(state, 0o700); err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(root, "alias")
				fixtureLink(t, state, alias)
				destinations[0].Path = filepath.Join(alias, "dots", "reserved")
			case "destination inside aliased source":
				alias := filepath.Join(root, "alias")
				fixtureLink(t, options.Sources[0], alias)
				destinations[0].Path = filepath.Join(alias, "would-change-source")
			}
			before := treeState(t, root)
			if report, err := InstallOwned(destinations, options); err == nil || report.LinksCreated != 0 {
				t.Fatalf("alias overlap accepted: %+v %v", report, err)
			}
			if !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatal("overlap preflight wrote")
			}
		})
	}
}

func TestNativeRemovalPermissionFailure(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	if err := os.Chmod(options.Target, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(options.Target, 0o700) })
	probe, err := os.Create(filepath.Join(options.Target, "permission-control"))
	if err == nil {
		_ = probe.Close()
		_ = os.Remove(probe.Name())
		t.Skip("filesystem permits writes despite restricted permissions")
	}
	if !os.IsPermission(err) {
		t.Fatal(err)
	}
	report, err := Down(options, false)
	if err == nil || report.Removed != 0 || len(ownershipSnapshot(t, options).Records) != 1 {
		t.Fatalf("permission failure lost state: %+v %v", report, err)
	}
	assertLink(t, destinations[0].Path, destinations[0].Source)
}

func TestConfiguredSourceRootAliasItselfIsPreserved(t *testing.T) {
	options, destinations := ownedFixture(t, ".config")
	data := filepath.Join(filepath.Dir(options.Profile), "data")
	fixtureFile(t, filepath.Join(data, "sentinel"))
	if err := os.Remove(destinations[0].Source); err != nil {
		t.Fatal(err)
	}
	fixtureLink(t, data, destinations[0].Source)
	installFixture(t, options, destinations)
	options.Sources = []string{destinations[0].Path}
	before := treeState(t, data)
	if report, err := Down(options, false); err == nil || report.Removed != 0 {
		t.Fatalf("configured root alias removed: %+v %v", report, err)
	}
	assertLink(t, destinations[0].Path, destinations[0].Source)
	if !reflect.DeepEqual(before, treeState(t, data)) {
		t.Fatal("source contents changed")
	}
}

func TestRecoveryWarningsSurviveEarlyFailures(t *testing.T) {
	for _, mode := range []string{"prepare", "claim", "begin persistence"} {
		t.Run(mode, func(t *testing.T) {
			options, destinations := ownedFixture(t, ".a")
			installFixture(t, options, destinations)
			editRegistry(t, options, func(s *snapshot) { s.Unfinished = true })
			ops, install := nativeRegistryOps(), nativeInstallOps()
			if mode == "prepare" {
				install.openat = func(int, string, int, uint32) (int, error) { return -1, unix.EMFILE }
			}
			if mode == "claim" {
				options.Profile += "-other"
			}
			if mode == "begin persistence" {
				ops.write = func(*os.File, []byte) (int, error) { return 0, unix.ENOSPC }
			}
			report, err := installOwned(destinations, options, ops, install)
			if err == nil || len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "cannot authorize removal") {
				t.Fatalf("recovery warning suppressed: %+v %v", report, err)
			}
			assertLink(t, destinations[0].Path, destinations[0].Source)
		})
	}
}

func TestInterruptedRecoveryInvalidatesAllScopesWithoutHistoricalVisits(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	historical := options
	historical.Target += "-historical"
	historicalDestinations := append([]Destination(nil), destinations...)
	historicalDestinations[0].Path = filepath.Join(historical.Target, ".a")
	installFixture(t, historical, historicalDestinations)
	other := options
	other.Profile += "-other"
	other.Target += "-other"
	otherDestinations := append([]Destination(nil), destinations...)
	otherDestinations[0].Path = filepath.Join(other.Target, ".a")
	installFixture(t, other, otherDestinations)
	editRegistry(t, options, func(s *snapshot) {
		s.Unfinished = true
		for i := range s.Records {
			if s.Records[i].Target == pathBytes(historical.Target) {
				s.Records[i].Phase = "removing"
			}
		}
	})
	if err := os.Rename(historical.Target, historical.Target+".original"); err != nil {
		t.Fatal(err)
	}
	fixtureLink(t, "/missing/historical-target", historical.Target)
	report, err := Down(options, false)
	if err == nil || report.Removed != 0 || len(report.Entries) != 1 || !strings.Contains(report.Entries[0].Reason, "invalidated") {
		t.Fatalf("recovery scope: %+v %v", report, err)
	}
	state := ownershipSnapshot(t, options)
	if len(state.Records) != 3 {
		t.Fatal("history discarded:", state)
	}
	for _, record := range state.Records {
		if record.Phase != "invalidated" {
			t.Fatal("authority retained in another scope:", record)
		}
	}
	assertLink(t, destinations[0].Path, destinations[0].Source)
	assertLink(t, filepath.Join(historical.Target+".original", ".a"), destinations[0].Source)
	assertLink(t, otherDestinations[0].Path, destinations[0].Source)
}

func TestSnapshotCapacityReservesGlobalRecovery(t *testing.T) {
	options, destinations := ownedFixture(t, ".a")
	installFixture(t, options, destinations)
	state := ownershipSnapshot(t, options)
	original := state.Records[0]
	for i := 0; i < 100; i++ {
		record := original
		record.Destination = pathBytes(fmt.Sprintf("%s/.slot-%d", options.Target, i))
		state.Records = append(state.Records, record)
	}
	base, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	padding := ((registrySizeLimit - 256 - len(base)) / 4) * 3
	state.Records[0].Text = pathBytes("/" + strings.Repeat("x", padding))
	current, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(current) >= registrySizeLimit {
		t.Fatal("fixture must fit the ordinary snapshot")
	}
	if _, err := snapshotBytes(state, 0); err == nil {
		t.Fatal("snapshot admitted without recovery capacity")
	}
	if report, err := Down(options, false); err != nil || report.Removed != 1 {
		t.Fatalf("previous authority unreadable: %+v %v", report, err)
	}
}

func TestRevocationFailureReportsEverySelectedDestination(t *testing.T) {
	for _, affected := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(affected), func(t *testing.T) {
			options, destinations := ownedFixture(t, ".a", ".b", ".c")
			installFixture(t, options, destinations)
			if err := os.Rename(destinations[affected].Path, destinations[affected].Path+".original"); err != nil {
				t.Fatal(err)
			}
			fixtureFile(t, destinations[affected].Path)
			ops := nativeRegistryOps()
			ops.write = func(f *os.File, data []byte) (int, error) {
				var state snapshot
				if err := json.Unmarshal(data, &state); err != nil {
					t.Fatal(err)
				}
				for _, record := range state.Records {
					if record.Phase == "invalidated" {
						return 0, unix.ENOSPC
					}
				}
				return f.Write(data)
			}
			report, err := down(options, false, ops)
			if err == nil || report.Removed != 0 || len(report.Entries) != 3 {
				t.Fatalf("incomplete failure: %+v %v", report, err)
			}
			for index, entry := range report.Entries {
				if entry.Destination != destinations[index].Path {
					t.Fatal("destination omitted")
				}
				if index == affected {
					if entry.Status != "PRESERVE" || !strings.Contains(entry.Reason, "revocation uncertain") {
						t.Fatal("failure not explained:", entry)
					}
				} else if entry.Status != "SKIP" {
					t.Fatal("unattempted entry not skipped:", entry)
				}
			}
		})
	}
}

func TestCreatedAncestorBindingsSyncBeforeOwnership(t *testing.T) {
	for _, failureAt := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprint(failureAt), func(t *testing.T) {
			options, destinations := ownedFixture(t, ".config/nvim/init.lua")
			ops, install := nativeRegistryOps(), nativeInstallOps()
			dirty := make(map[entryID]bool)
			bindingSyncs := 0
			install.mkdirat = func(fd int, name string, mode uint32) error {
				if err := unix.Mkdirat(fd, name, mode); err != nil {
					return err
				}
				var stat unix.Stat_t
				if err := unix.Fstat(fd, &stat); err != nil {
					t.Fatal(err)
				}
				dirty[idOf(stat)] = true
				return nil
			}
			ops.sync = func(fd int) error {
				var stat unix.Stat_t
				if err := unix.Fstat(fd, &stat); err != nil {
					return err
				}
				if stat.Mode&unix.S_IFMT == unix.S_IFDIR && dirty[idOf(stat)] {
					bindingSyncs++
					if failureAt != 0 && bindingSyncs == failureAt {
						return unix.EIO
					}
				}
				if err := unix.Fsync(fd); err != nil {
					return err
				}
				delete(dirty, idOf(stat))
				return nil
			}
			install.checkpoint = func(phase string, _ Destination) {
				if phase == "before-symlink" && len(dirty) != 0 {
					t.Fatal("symlink created before ancestor bindings were synchronized")
				}
			}
			report, err := installOwned(destinations, options, ops, install)
			state := ownershipSnapshot(t, options)
			if failureAt == 0 {
				if err != nil || bindingSyncs != 3 || report.LinksCreated != 1 || state.Unfinished || len(state.Records) != 1 || state.Records[0].Phase != "owned" {
					t.Fatalf("ancestor sync: %+v %+v %v", report, state, err)
				}
			} else {
				if !errors.Is(err, unix.EIO) || bindingSyncs != failureAt || report.DirectoriesCreated != failureAt || report.LinksCreated != 0 || !state.Unfinished || len(state.Records) != 0 {
					t.Fatalf("failed binding sync committed authority: %+v %+v %v", report, state, err)
				}
			}
		})
	}
}

func TestAncestorSyncFailureRetainsEarlierInstallation(t *testing.T) {
	options, destinations := ownedFixture(t, ".a", ".nested/file", ".z")
	ops, install := nativeRegistryOps(), nativeInstallOps()
	var failingParent *entryID
	install.mkdirat = func(fd int, name string, mode uint32) error {
		if err := unix.Mkdirat(fd, name, mode); err != nil {
			return err
		}
		if name == ".nested" {
			var stat unix.Stat_t
			if err := unix.Fstat(fd, &stat); err != nil {
				t.Fatal(err)
			}
			id := idOf(stat)
			failingParent = &id
		}
		return nil
	}
	ops.sync = func(fd int) error {
		var stat unix.Stat_t
		if err := unix.Fstat(fd, &stat); err != nil {
			return err
		}
		if failingParent != nil && idOf(stat) == *failingParent {
			return unix.EIO
		}
		return unix.Fsync(fd)
	}
	report, err := installOwned(destinations, options, ops, install)
	if !errors.Is(err, unix.EIO) || report.LinksCreated != 1 || report.DirectoriesCreated != 2 || report.Results[1].Status != "ERROR" || report.Results[2].Status != "SKIP" {
		t.Fatalf("partial ancestor sync: %+v %v", report, err)
	}
	state := ownershipSnapshot(t, options)
	if !state.Unfinished || len(state.Records) != 1 || state.Records[0].Destination != pathBytes(destinations[0].Path) {
		t.Fatal("partial authority lost or falsely finalized:", state)
	}
	assertLink(t, destinations[0].Path, destinations[0].Source)
}
