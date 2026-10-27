package filesystem

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func writeImportSnapshot(t *testing.T, options Ownership, state snapshot) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parentPath(options.Registry), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(options.Registry, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func importRegistryFixture(t *testing.T, nested bool) (ImportRequest, ownershipRecord) {
	t.Helper()
	request := importInspectionFixture(t, nested)
	source := request.Root + "/managed"
	fixtureFile(t, source)
	destination := Destination{Path: request.Ownership.Target + "/.managed", Source: source}
	installFixture(t, request.Ownership, []Destination{destination})
	request.Entries = append(request.Entries, destination)
	return request, ownershipSnapshot(t, request.Ownership).Records[0]
}

func TestInspectImportRegistryAdmission(t *testing.T) {
	for _, mode := range []string{"missing directories", "missing files", "lock only", "empty snapshot", "unrelated owned", "unrelated invalidated", "locked clean", "unfinished", "creating", "removing"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			var held *registry
			if mode != "missing directories" {
				if err := os.MkdirAll(parentPath(request.Ownership.Registry), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "missing directories" && mode != "missing files" {
				fixtureFile(t, importLockPath(request.Ownership.Registry))
			}
			if mode != "missing directories" && mode != "missing files" && mode != "lock only" {
				var record ownershipRecord
				request, record = importRegistryFixture(t, false)
				state := ownershipSnapshot(t, request.Ownership)
				switch mode {
				case "empty snapshot", "locked clean":
					state.Records = []ownershipRecord{}
				case "unrelated invalidated":
					state.Records[0].Phase = "invalidated"
				case "unfinished":
					state.Unfinished = true
				case "creating", "removing":
					record.Phase = mode
					state.Records[0] = record
				}
				writeImportSnapshot(t, request.Ownership, state)
				if mode == "locked clean" {
					var err error
					held, err = openRegistry(request.Ownership, true, nativeRegistryOps())
					if err != nil {
						t.Fatal(err)
					}
					defer held.Close()
				}
			}
			before := treeState(t, filepath.Dir(request.Root))
			_, err := InspectImport(request)
			denied := mode == "unfinished" || mode == "creating" || mode == "removing"
			if (err != nil) != denied {
				t.Fatalf("admission %s: %v", mode, err)
			}
			if denied && !strings.Contains(err.Error(), "blocks import") {
				t.Fatalf("wrong refusal: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("registry inspection wrote or recovered state")
			}
			if held != nil {
				fd, err := unix.Open(importLockPath(request.Ownership.Registry), unix.O_RDONLY|unix.O_CLOEXEC, 0)
				if err != nil {
					t.Fatal(err)
				}
				err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
				_ = unix.Close(fd)
				if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
					t.Fatalf("writer lock was disturbed: %v", err)
				}
			}
		})
	}
}

func TestInspectImportRegistryRefusesInvalidState(t *testing.T) {
	for _, mode := range []string{"empty file", "malformed", "duplicate key", "missing field", "null field", "unsupported", "duplicate record", "unknown field", "unsafe state", "symlink state", "unsafe lock", "symlink lock", "directory lock", "missing lock", "oversize", "hardlinked state", "directory state", "broken lock"} {
		t.Run(mode, func(t *testing.T) {
			request, _ := importRegistryFixture(t, false)
			raw, err := os.ReadFile(request.Ownership.Registry)
			if err != nil {
				t.Fatal(err)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, raw); err != nil {
				t.Fatal(err)
			}
			raw = compact.Bytes()
			lock := importLockPath(request.Ownership.Registry)
			switch mode {
			case "empty file":
				raw = nil
			case "malformed":
				raw = []byte("{broken")
			case "duplicate key":
				raw = bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
			case "missing field":
				raw = bytes.Replace(raw, []byte(`"unfinished":false,`), nil, 1)
			case "null field":
				raw = bytes.Replace(raw, []byte(`"unfinished":false`), []byte(`"unfinished":null`), 1)
			case "unsupported":
				raw = bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":2`), 1)
			case "unknown field":
				raw = append([]byte(`{"extra":0,`), raw[1:]...)
			case "duplicate record":
				state := ownershipSnapshot(t, request.Ownership)
				state.Records = append(state.Records, state.Records[0])
				raw, err = json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
			case "oversize":
				raw = bytes.Repeat([]byte(" "), registrySizeLimit+1)
			case "hardlinked state":
				if err := os.Link(request.Ownership.Registry, request.Ownership.Registry+".alias"); err != nil {
					t.Fatal(err)
				}
			case "directory state":
				if err := os.Rename(request.Ownership.Registry, request.Ownership.Registry+".preserved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(request.Ownership.Registry, 0o700); err != nil {
					t.Fatal(err)
				}
			case "broken lock":
				if err := os.Rename(lock, lock+".preserved"); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, lock+".missing", lock)
			case "unsafe state":
				if err := os.Chmod(request.Ownership.Registry, 0o644); err != nil {
					t.Fatal(err)
				}
			case "unsafe lock":
				if err := os.Chmod(lock, 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink state", "symlink lock", "directory lock", "missing lock":
				path := lock
				if mode == "symlink state" {
					path = request.Ownership.Registry
				}
				if err := os.Rename(path, path+".preserved"); err != nil {
					t.Fatal(err)
				}
				if mode == "directory lock" {
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				} else if mode != "missing lock" {
					fixtureLink(t, path+".preserved", path)
				}
			}
			if !strings.Contains(mode, "symlink") && mode != "missing lock" && mode != "directory lock" && mode != "directory state" {
				if err := os.WriteFile(request.Ownership.Registry, raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := treeState(t, filepath.Dir(request.Root))
			if _, err := InspectImport(request); err == nil {
				t.Fatalf("invalid state admitted: %s", mode)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("invalid state modified")
			}
		})
	}
}

func TestInspectImportRecordedClaims(t *testing.T) {
	for _, mode := range []string{"current input", "other profile", "historical target", "historical source", "source missing parents", "ancestor", "descendant", "native alias", "unrelated unavailable history", "invalidated overlap", "capability failure", "unrelated capability failure"} {
		t.Run(mode, func(t *testing.T) {
			request, record := importRegistryFixture(t, mode == "source missing parents")
			state := ownershipSnapshot(t, request.Ownership)
			record.Profile += ".other"
			record.Target = pathBytes(request.Ownership.Target)
			record.Destination = pathBytes(request.Input)
			var parent unix.Stat_t
			if err := unix.Stat(parentPath(request.Input), &parent); err != nil {
				t.Fatal(err)
			}
			record.Parent = idOf(parent)
			ops := nativeImportInspectionOps()
			switch mode {
			case "current input":
				record.Profile = pathBytes(request.Ownership.Profile)
			case "historical target", "unrelated unavailable history", "native alias", "capability failure", "unrelated capability failure":
				record.Target = pathBytes(filepath.Dir(request.Root) + "/unavailable-history")
				name := filepath.Base(request.Input)
				if mode == "unrelated unavailable history" || mode == "unrelated capability failure" {
					name = ".unrelated"
				}
				if mode == "native alias" || mode == "capability failure" {
					name = strings.ToUpper(name)
					ops.nameRules = func(int) (nameRules, error) { return nameRules{caseSensitive: false}, nil }
				}
				if mode == "capability failure" || mode == "unrelated capability failure" {
					ops.nameRules = func(int) (nameRules, error) { return nameRules{}, unix.EIO }
				}
				record.Destination = record.Target + "/" + pathBytes(name)
			case "historical source", "source missing parents":
				record.Target = pathBytes(request.Root)
				record.Destination = pathBytes(request.Source)
				if err := unix.Stat(request.Root, &parent); err != nil {
					t.Fatal(err)
				}
				record.Parent = idOf(parent)
			case "ancestor":
				record.Destination = pathBytes(parentPath(request.Input))
				record.Target = pathBytes(parentPath(request.Ownership.Target))
				record.Routes = []recordedRoute{}
			case "descendant":
				record.Destination = pathBytes(request.Input + "/child")
				record.Routes = []recordedRoute{}
			case "invalidated overlap":
				record.Phase = "invalidated"
			}
			state.Records = append(state.Records, record)
			writeImportSnapshot(t, request.Ownership, state)
			before := treeState(t, filepath.Dir(request.Root))
			_, err := inspectImport(request, ops)
			if mode == "unrelated unavailable history" || mode == "unrelated capability failure" {
				if err != nil {
					t.Fatalf("unrelated history refused: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "recorded ownership claim") {
				t.Fatalf("claim missed or wrong refusal: %v", err)
			}
			if mode == "capability failure" && !errors.Is(err, unix.EIO) {
				t.Fatalf("capability uncertainty lost: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("claim inspection changed state")
			}
		})
	}
}

func TestInspectImportRegistryLateChanges(t *testing.T) {
	for _, mode := range []string{"snapshot replacement", "snapshot disappearance", "lock replacement", "missing state appearance", "lock-only publication", "missing frontier", "during reload", "combined error", "malformed reload"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			existing := mode != "missing state appearance" && mode != "missing frontier" && mode != "lock-only publication"
			if existing {
				request, _ = importRegistryFixture(t, false)
			}
			if mode == "lock-only publication" {
				fixtureFile(t, importLockPath(request.Ownership.Registry))
			}
			ops := nativeImportInspectionOps()
			changed := false
			change := func() {
				changed = true
				if mode == "lock replacement" {
					path := importLockPath(request.Ownership.Registry)
					if err := os.Rename(path, path+".preserved"); err != nil {
						t.Fatal(err)
					}
					fixtureFile(t, path)
				} else if mode == "missing frontier" {
					if err := os.MkdirAll(parentPath(request.Ownership.Registry), 0o700); err != nil {
						t.Fatal(err)
					}
				} else {
					if existing {
						if err := os.Rename(request.Ownership.Registry, request.Ownership.Registry+".preserved"); err != nil {
							t.Fatal(err)
						}
					}
					if mode == "malformed reload" {
						if err := os.WriteFile(request.Ownership.Registry, []byte("{broken"), 0o600); err != nil {
							t.Fatal(err)
						}
						lock := importLockPath(request.Ownership.Registry)
						if err := os.Rename(lock, lock+".preserved"); err != nil {
							t.Fatal(err)
						}
						fixtureFile(t, lock)
					} else if mode != "snapshot disappearance" {
						writeImportSnapshot(t, request.Ownership, snapshot{Version: 1, Revision: 2, Records: []ownershipRecord{}})
					}
				}
			}
			ops.checkpoint = change
			if mode == "during reload" || mode == "malformed reload" {
				ops.checkpoint = nil
				ops.registryCheckpoint = change
			}
			if mode == "combined error" {
				ops.statConfig = func(path string, s *unix.Stat_t) error {
					if changed {
						return unix.EACCES
					}
					return unix.Stat(path, s)
				}
			}
			_, err := inspectImport(request, ops)
			if err == nil || !strings.Contains(err.Error(), "state binding changed") {
				t.Fatalf("late state change missed: %v", err)
			}
			if mode == "malformed reload" && (!strings.Contains(err.Error(), "invalid ownership registry") || !strings.Contains(err.Error(), "ownership.lock")) {
				t.Fatalf("reload or lock-change evidence lost: %v", err)
			}
			if mode == "combined error" && !errors.Is(err, unix.EACCES) {
				t.Fatalf("primary inspection failure lost: %v", err)
			}
			if _, err := os.ReadFile(request.Input); err != nil {
				t.Fatal("input fixture lost")
			}
		})
	}
}

func TestOpenImportRegistryClosesDescriptors(t *testing.T) {
	request, _ := importRegistryFixture(t, false)
	view, err := openImportRegistry(request.Ownership, nil)
	if err != nil {
		t.Fatal(err)
	}
	lock, parent := view.lock, view.registry.dir.fd
	if err := view.Close(); err != nil {
		t.Fatal(err)
	}
	var stat unix.Stat_t
	for _, fd := range []int{lock, parent} {
		if !errors.Is(unix.Fstat(fd, &stat), unix.EBADF) {
			t.Fatalf("descriptor %d escaped", fd)
		}
	}
}

func TestInspectImportPhysicalSourceClaims(t *testing.T) {
	for _, mode := range []string{"missing parent", "source ancestor", "unrelated"} {
		t.Run(mode, func(t *testing.T) {
			request, record := importRegistryFixture(t, true)
			state := ownershipSnapshot(t, request.Ownership)
			record.Profile += ".historical"
			record.Target = pathBytes(filepath.Dir(request.Root) + "/unavailable-history")
			record.Destination = record.Target + "/config"
			var parent unix.Stat_t
			root := request.Root
			if mode == "source ancestor" {
				root = parentPath(root)
				record.Destination = record.Target + "/" + pathBytes(filepath.Base(request.Root))
			}
			if err := unix.Stat(root, &parent); err != nil {
				t.Fatal(err)
			}
			record.Parent = idOf(parent)
			if mode == "unrelated" {
				record.Destination = record.Target + "/unrelated"
			}
			// This unavailable historical locator has no literal overlap with either
			// current path. Only the recorded physical parent/name or source route can
			// reserve the missing frontier or the selected root's ancestor entry.
			state.Records = append(state.Records, record)
			writeImportSnapshot(t, request.Ownership, state)
			before := treeState(t, filepath.Dir(request.Root))
			_, err := InspectImport(request)
			if mode == "unrelated" {
				if err != nil {
					t.Fatalf("unrelated source history refused: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "recorded ownership claim") {
				t.Fatalf("physical source reservation missed: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("source-claim inspection wrote state")
			}
		})
	}
}

func TestInspectImportRegistryDirectoryPrivacy(t *testing.T) {
	for _, mode := range []os.FileMode{0o755, 0o777} {
		t.Run(mode.String(), func(t *testing.T) {
			request, _ := importRegistryFixture(t, false)
			ops := nativeImportInspectionOps()
			var injected map[string]string
			ops.checkpoint = func() {
				if err := os.Chmod(parentPath(request.Ownership.Registry), mode); err != nil {
					t.Fatal(err)
				}
				injected = treeState(t, filepath.Dir(request.Root))
			}
			_, err := inspectImport(request, ops)
			if err == nil || !strings.Contains(err.Error(), "private user-owned directories") {
				t.Fatalf("late directory privacy loss admitted: %v", err)
			}
			if !reflect.DeepEqual(injected, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("inspection repaired or modified injected state")
			}
		})
	}
}
