package filesystem

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func v2Fixture(t *testing.T, records bool) (Ownership, []Destination, snapshot) {
	t.Helper()
	o, destinations := ownedFixture(t, ".managed", ".new")
	if records {
		installFixture(t, o, destinations[:1])
	} else {
		r, err := openRegistry(o, true, nativeRegistryOps())
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	}
	s := ownershipSnapshot(t, o)
	s.Version, s.Unfinished = 2, true
	input, source := o.Target+"/.import", o.Sources[0]+"/import"
	fixtureFile(t, input)
	p, err := namespace()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	binding := func(path string) importBinding {
		parent, missing, routes, err := p.walk(parentPath(path))
		if err != nil || len(missing) != 0 {
			t.Fatalf("fixture binding: %v %v", missing, err)
		}
		return importBinding{idOfDirectory(parent), recordRoutes(routes)}
	}
	var stat unix.Stat_t
	if err := unix.Lstat(input, &stat); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	sourceBinding := binding(source)
	s.Imports = []importIntent{{ID: strings.Repeat("a", 32), Phase: "prepared", Profile: pathBytes(o.Profile),
		Target: pathBytes(o.Target), Root: pathBytes(o.Sources[0]), Input: pathBytes(input), Source: pathBytes(source),
		InputBinding: binding(input), SourceAnchor: importAnchor{sourceBinding, []pathBytes{}}, SourceBinding: &sourceBinding,
		OriginalFile: importFile{fingerprintOf(stat), stat.Uid, stat.Gid, uint64(stat.Nlink)},
		Digest:       importDigest{"sha256-v1", hex.EncodeToString(digest[:])}}}
	return o, destinations, s
}

func decodeV2Test(data []byte) (snapshot, error) {
	var s snapshot
	if err := strictJSON(data); err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	return s, validateSnapshot(s)
}

func TestRegistryV2Schema(t *testing.T) {
	_, _, good := v2Fixture(t, true)
	tests := []struct {
		name  string
		edit  func(*snapshot)
		valid bool
	}{
		{"prepared", func(*snapshot) {}, true},
		{"empty", func(s *snapshot) { s.Imports = []importIntent{}; s.Unfinished = false }, true},
		{"relocated", func(s *snapshot) {
			i := &s.Imports[0]
			i.Phase = "relocated"
			f := i.OriginalFile
			f.Fingerprint.Changed++
			i.RelocatedFile = &f
		}, true},
		{"barrier", func(s *snapshot) { s.Unfinished = false }, false},
		{"multiple", func(s *snapshot) { s.Imports = append(s.Imports, s.Imports[0]) }, false},
		{"phase", func(s *snapshot) { s.Imports[0].Phase = "owned" }, false},
		{"id", func(s *snapshot) { s.Imports[0].ID = strings.Repeat("A", 32) }, false},
		{"digest", func(s *snapshot) { s.Imports[0].Digest.Value = "00" }, false},
		{"algorithm", func(s *snapshot) { s.Imports[0].Digest.Algorithm = "sha256" }, false},
		{"outside target", func(s *snapshot) { s.Imports[0].Input = "/outside" }, false},
		{"outside root", func(s *snapshot) { s.Imports[0].Source = "/outside" }, false},
		{"traversal", func(s *snapshot) { s.Imports[0].Source += "/../escape" }, false},
		{"nul", func(s *snapshot) { s.Imports[0].Profile += "\x00" }, false},
		{"single link", func(s *snapshot) { s.Imports[0].OriginalFile.Nlink = 2 }, false},
		{"special", func(s *snapshot) { s.Imports[0].OriginalFile.Fingerprint.Mode = unix.S_IFIFO }, false},
		{"privileged", func(s *snapshot) { s.Imports[0].OriginalFile.Fingerprint.Mode |= 0o4000 }, false},
		{"inode", func(s *snapshot) { s.Imports[0].OriginalFile.Fingerprint.ID.Inode = 0 }, false},
		{"size", func(s *snapshot) { s.Imports[0].OriginalFile.Fingerprint.Size = -1 }, false},
		{"nanoseconds", func(s *snapshot) { s.Imports[0].OriginalFile.Fingerprint.ModNsec = 1e9 }, false},
		{"routes", func(s *snapshot) { s.Imports[0].InputBinding.Routes = nil }, false},
		{"parent", func(s *snapshot) { s.Imports[0].InputBinding.Parent.Inode++ }, false},
		{"missing component", func(s *snapshot) { s.Imports[0].SourceAnchor.Missing = []pathBytes{".."} }, false},
		{"source binding", func(s *snapshot) { s.Imports[0].SourceBinding = nil }, false},
		{"prepared relocated", func(s *snapshot) { f := s.Imports[0].OriginalFile; s.Imports[0].RelocatedFile = &f }, false},
		{"relocated incomplete", func(s *snapshot) { s.Imports[0].Phase = "relocated" }, false},
		{"relocated changed", func(s *snapshot) {
			i := &s.Imports[0]
			i.Phase = "relocated"
			f := i.OriginalFile
			f.Fingerprint.ID.Inode++
			i.RelocatedFile = &f
		}, false},
		{"v1 imports", func(s *snapshot) { s.Version = 1 }, false},
		{"unsupported", func(s *snapshot) { s.Version = 3 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(good)
			var s snapshot
			if err := json.Unmarshal(data, &s); err != nil {
				t.Fatal(err)
			}
			tt.edit(&s)
			data, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			_, err = decodeV2Test(data)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v: %v", tt.valid, err)
			}
		})
	}
}

func TestRegistryV2WireShape(t *testing.T) {
	_, _, s := v2Fixture(t, false)
	data, _ := json.Marshal(s)
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	intent := object["imports"].([]any)[0].(map[string]any)
	for _, location := range []map[string]any{object, intent, intent["input_binding"].(map[string]any), intent["source_anchor"].(map[string]any), intent["original_file"].(map[string]any), intent["digest"].(map[string]any)} {
		for field, old := range location {
			delete(location, field)
			changed, _ := json.Marshal(object)
			if _, err := decodeV2Test(changed); err == nil {
				t.Fatalf("missing field accepted: %s", field)
			}
			location[field] = nil
			changed, _ = json.Marshal(object)
			if _, err := decodeV2Test(changed); err == nil {
				t.Fatalf("null field accepted: %s", field)
			}
			location[field] = old
		}
		location["unknown"] = true
		changed, _ := json.Marshal(object)
		if _, err := decodeV2Test(changed); err == nil {
			t.Fatal("unknown field accepted")
		}
		delete(location, "unknown")
	}
	for _, changed := range [][]byte{
		bytes.Replace(data, []byte(`"version":2`), []byte(`"version":2,"version":1`), 1),
		bytes.Replace(data, []byte(`"phase":"prepared"`), []byte(`"phase":"prepared","phase":"relocated"`), 1),
		bytes.Replace(data, []byte(`"imports":`), []byte(`"Imports":`), 1),
		append(append([]byte{}, data...), []byte(` {}`)...),
		[]byte(`{"version":2,"revision":0,"unfinished":false,"records":[],"imports":null}`),
		[]byte(`{"version":1,"revision":0,"unfinished":false,"records":[],"imports":[]}`),
		[]byte(`{"version":2,"revision":0,"unfinished":false,"records":[]}`),
		bytes.Replace(data, []byte(`"id":"`), []byte(`"id":3,"old":"`), 1),
	} {
		if _, err := decodeV2Test(changed); err == nil {
			t.Fatalf("bad wire accepted: %.150s", changed)
		}
	}
}

func TestPendingImportMutationGates(t *testing.T) {
	for _, records := range []bool{false, true} {
		for _, operation := range []string{"up", "empty up", "down", "down preview", "revalidate", "other scope", "begin", "finish", "save", "erased in memory"} {
			t.Run(fmt.Sprintf("records=%v/%s", records, operation), func(t *testing.T) {
				o, ds, s := v2Fixture(t, records)
				writeImportSnapshot(t, o, s)
				before := treeState(t, filepath.Dir(o.Profile))
				var err error
				switch operation {
				case "up":
					_, err = InstallOwned(ds, o)
				case "empty up":
					_, err = InstallOwned(nil, o)
				case "down":
					_, err = Down(o, false)
				case "down preview":
					_, err = Down(o, true)
				case "revalidate":
					_, err = RevalidateOwnership(o)
				case "other scope":
					o.Profile += ".other"
					o.Target += ".other"
					_, err = Down(o, false)
				default:
					r, openErr := openRegistry(o, true, nativeRegistryOps())
					if openErr != nil {
						t.Fatal(openErr)
					}
					switch operation {
					case "begin":
						err = r.begin()
					case "finish":
						err = r.finish()
					case "save":
						err = r.save()
					case "erased in memory":
						r.data.Imports = []importIntent{}
						err = r.finish()
					}
					if closeErr := r.Close(); closeErr != nil {
						t.Fatal(closeErr)
					}
				}
				if err == nil || !strings.Contains(err.Error(), "inspect with dots recover") {
					t.Fatalf("missing pending-import diagnostic: %v", err)
				}
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(string(s.Imports[0].Profile)))) {
					t.Fatal("pending-import refusal changed fixture")
				}
			})
		}
	}
}

func TestRegistryVersionPreservation(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			o, ds := ownedFixture(t, ".a", ".b")
			installFixture(t, o, ds[:1])
			s := ownershipSnapshot(t, o)
			history := s.Records[0]
			if version == 2 {
				s.Version = 2
				s.Imports = []importIntent{}
				writeImportSnapshot(t, o, s)
			}
			installFixture(t, o, ds[1:])
			s = ownershipSnapshot(t, o)
			if s.Version != version || !reflect.DeepEqual(history, s.Records[0]) {
				t.Fatal("version or prior ownership changed")
			}
			if _, err := RevalidateOwnership(o); err != nil {
				t.Fatal(err)
			}
			if report, err := Down(o, false); err != nil || report.Removed != 2 {
				t.Fatalf("removal regression: %+v %v", report, err)
			}
			s = ownershipSnapshot(t, o)
			if s.Version != version || len(s.Records) != 0 {
				t.Fatal("version changed after removal")
			}
			data, err := os.ReadFile(o.Registry)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(`"imports"`)) != (version == 2) {
				t.Fatal("wrong wire version")
			}
			r, err := openRegistry(o, true, nativeRegistryOps())
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			r.data.Version = 3 - version
			if err := r.save(); err == nil {
				t.Fatal("ordinary version change accepted")
			}
		})
	}
}

func TestRegistryCapacityBoundaries(t *testing.T) {
	_, _, s := v2Fixture(t, true)
	worst := s
	worst.Revision = math.MaxUint64
	worst.Records = append([]ownershipRecord{}, s.Records...)
	for n := range worst.Records {
		worst.Records[n].Phase = "invalidated"
	}
	data, err := json.MarshalIndent(worst, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	remaining := registrySizeLimit - len(data) - 1
	for _, reserve := range []int{remaining, remaining + 1, -1, math.MaxInt} {
		_, err := snapshotBytes(s, reserve)
		if (err == nil) != (reserve == remaining) {
			t.Fatalf("reserve=%d: %v", reserve, err)
		}
	}
	for _, tt := range []struct {
		revision, steps uint64
		valid           bool
	}{{0, 2, true}, {math.MaxUint64 - 2, 2, true}, {math.MaxUint64 - 1, 2, false}, {math.MaxUint64, 1, false}, {math.MaxUint64, 0, true}} {
		if err := revisionCapacity(tt.revision, tt.steps); (err == nil) != tt.valid {
			t.Fatalf("revision boundary: %+v %v", tt, err)
		}
	}
	o, _, s := v2Fixture(t, false)
	s.Imports = []importIntent{}
	s.Unfinished = false
	s.Revision = math.MaxUint64 - 1
	writeImportSnapshot(t, o, s)
	before := treeState(t, filepath.Dir(o.Profile))
	if _, err := InstallOwned(nil, o); err == nil {
		t.Fatal("exhausted transaction admitted")
	}
	if !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Profile))) {
		t.Fatal("counter refusal wrote state")
	}
}

func TestRegistryV2SourceParentsAndOwnershipClaims(t *testing.T) {
	_, _, base := v2Fixture(t, true)
	for _, mode := range []string{"missing parents", "recorded parents", "relocated parents", "above root", "wrong suffix", "wrong route", "historical overlap", "physical overlap", "associated creating", "associated owned", "associated invalidated", "associated removing"} {
		t.Run(mode, func(t *testing.T) {
			data, _ := json.Marshal(base)
			var s snapshot
			if err := json.Unmarshal(data, &s); err != nil {
				t.Fatal(err)
			}
			i := &s.Imports[0]
			valid := true
			switch mode {
			case "missing parents", "recorded parents", "relocated parents", "above root", "wrong suffix", "wrong route":
				i.Source = i.Root + "/nested/import"
				i.SourceAnchor.Missing = []pathBytes{"nested"}
				i.SourceBinding = nil
				if mode == "above root" {
					i.SourceAnchor.Missing = []pathBytes{pathBytes(filepath.Base(string(i.Root))), "nested"}
					valid = false
				}
				if mode == "wrong suffix" {
					i.SourceAnchor.Missing[0] = "different"
					valid = false
				}
				if mode == "recorded parents" || mode == "relocated parents" || mode == "wrong route" {
					parent := entryID{i.SourceAnchor.Parent.Device, i.SourceAnchor.Parent.Inode + 100}
					routes := append([]recordedRoute{}, i.SourceAnchor.Routes...)
					routes = append(routes, recordedRoute{Parent: i.SourceAnchor.Parent, Name: "nested", Entry: parent, Type: unix.S_IFDIR, Target: parent})
					i.SourceBinding = &importBinding{parent, routes}
					if mode == "relocated parents" {
						i.Phase = "relocated"
						f := i.OriginalFile
						i.RelocatedFile = &f
					}
					if mode == "wrong route" {
						i.SourceBinding.Routes[len(routes)-1].Name = "wrong"
						valid = false
					}
				}
			case "historical overlap":
				s.Records[0].Profile += ".other"
				s.Records[0].Destination = i.Input
				valid = false
			case "physical overlap":
				s.Records[0].Profile += ".other"
				s.Records[0].Target = "/historical"
				s.Records[0].Destination = "/historical/.import"
				s.Records[0].Parent = i.InputBinding.Parent
				valid = false
			default:
				i.Phase = "relocated"
				f := i.OriginalFile
				i.RelocatedFile = &f
				r := &s.Records[0]
				r.Profile, r.Target, r.Destination, r.Text = i.Profile, i.Target, i.Input, i.Source
				r.Parent, r.Routes = i.InputBinding.Parent, i.InputBinding.Routes
				r.Phase = strings.TrimPrefix(mode, "associated ")
				valid = r.Phase != "removing"
			}
			data, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeV2Test(data); (err == nil) != valid {
				t.Fatalf("valid=%v: %v", valid, err)
			}
		})
	}
}

func TestRegistryV2ReadOnlyInspection(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprintf("pending=%v", pending), func(t *testing.T) {
			request := importInspectionFixture(t, false)
			o, _, s := v2Fixture(t, false)
			request.Ownership.Registry = o.Registry
			if !pending {
				s.Imports = []importIntent{}
				s.Unfinished = false
			}
			writeImportSnapshot(t, o, s)
			held, err := openRegistry(o, true, nativeRegistryOps())
			if err != nil {
				t.Fatal(err)
			}
			defer held.Close()
			before := treeState(t, filepath.Dir(o.Profile))
			_, err = InspectImport(request)
			if (err != nil) != pending {
				t.Fatalf("inspection admission: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Profile))) {
				t.Fatal("inspection changed state")
			}
			if other, err := openRegistry(o, true, nativeRegistryOps()); err == nil {
				_ = other.Close()
				t.Fatal("inspection released writer lock")
			}
		})
	}
}

func TestNonemptyRevisionAdmission(t *testing.T) {
	for _, version := range []int{1, 2} {
		for _, operation := range []string{"up", "down", "revalidate"} {
			t.Run(fmt.Sprintf("v%d/%s", version, operation), func(t *testing.T) {
				o, ds := ownedFixture(t, ".old", ".missing/nested")
				installFixture(t, o, ds[:1])
				s := ownershipSnapshot(t, o)
				s.Revision = math.MaxUint64 - 2
				if version == 2 {
					s.Version = 2
					s.Imports = []importIntent{}
				}
				writeImportSnapshot(t, o, s)
				before := treeState(t, filepath.Dir(o.Profile))
				var err error
				switch operation {
				case "up":
					_, err = InstallOwned(ds[1:], o)
				case "down":
					_, err = Down(o, false)
				case "revalidate":
					_, err = RevalidateOwnership(o)
				}
				if err == nil || !strings.Contains(err.Error(), "counter exhausted") {
					t.Fatalf("counter admission: %v", err)
				}
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Profile))) {
					t.Fatal("revision admission changed fixture")
				}
			})
		}
	}
}

func TestRegistryV2EndpointAliases(t *testing.T) {
	for _, phase := range []string{"prepared", "relocated"} {
		for _, tt := range []struct {
			name, input, source string
			parent              string
			valid               bool
		}{
			{"exact alias", "file", "file", "same", false},
			{"case alias", "File", "file", "same", false},
			{"unicode alias", "é", "e\u0301", "same", false},
			{"ignorable alias", "file", "fi\u200ble", "same", false},
			{"distinct names", "file", "other", "same", true},
			{"different inode", "file", "file", "inode", true},
			{"different device", "file", "file", "device", true},
		} {
			t.Run(phase+"/"+tt.name, func(t *testing.T) {
				o, _, s := v2Fixture(t, false)
				i := &s.Imports[0]
				i.Input, i.Source = i.Target+"/"+pathBytes(tt.input), i.Root+"/"+pathBytes(tt.source)
				parent := i.InputBinding.Parent
				if tt.parent == "inode" {
					parent.Inode++
				}
				if tt.parent == "device" {
					parent.Device++
				}
				i.SourceAnchor.importBinding = importBinding{Parent: parent, Routes: []recordedRoute{}}
				binding := i.SourceAnchor.importBinding
				i.SourceBinding = &binding
				i.Phase = phase
				if phase == "relocated" {
					f := i.OriginalFile
					i.RelocatedFile = &f
				}
				data, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := decodeV2Test(data); (err == nil) != tt.valid {
					t.Fatalf("valid=%v: %v", tt.valid, err)
				}
				writeImportSnapshot(t, o, s)
				before := treeState(t, filepath.Dir(o.Profile))
				if _, err := Down(o, false); err == nil {
					t.Fatal("snapshot authorized mutation")
				}
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Profile))) {
					t.Fatal("snapshot rejection changed fixture")
				}
			})
		}
	}
	_, _, s := v2Fixture(t, false)
	i := &s.Imports[0]
	i.Source = i.Root + "/nested/file"
	i.Input = i.Target + "/Nested"
	i.SourceAnchor.importBinding = i.InputBinding
	i.SourceAnchor.Missing = []pathBytes{"nested"}
	i.SourceBinding = nil
	if err := validateSnapshot(s); err == nil {
		t.Fatal("input accepted as prospective source parent")
	}
}

func TestRegistryV2EndpointAncestryAliases(t *testing.T) {
	for _, phase := range []string{"prepared", "relocated"} {
		for _, direction := range []string{"input in source", "source in input"} {
			for _, tt := range []struct {
				name, endpoint, route string
				kind                  uint32
				valid                 bool
			}{
				{"directory", "file", "file", unix.S_IFDIR, false},
				{"symlink", "file", "file", unix.S_IFLNK, false},
				{"case", "File", "file", unix.S_IFDIR, false},
				{"unicode", "é", "e\u0301", unix.S_IFLNK, false},
				{"distinct", "file", "other", unix.S_IFDIR, true},
			} {
				t.Run(phase+"/"+direction+"/"+tt.name, func(t *testing.T) {
					o, _, s := v2Fixture(t, false)
					i := &s.Imports[0]
					parent, target := i.InputBinding.Parent, i.SourceBinding.Parent
					if direction == "source in input" {
						parent, target = target, parent
					}
					entry := target
					text := pathBytes("")
					if tt.kind == unix.S_IFLNK {
						entry.Inode += 1000
						text = "/recorded-alias"
					}
					binding := importBinding{target, []recordedRoute{{Parent: parent, Name: pathBytes(tt.route), Entry: entry, Type: tt.kind, Text: text, Target: target}}}
					if direction == "input in source" {
						i.Input = i.Target + "/" + pathBytes(tt.endpoint)
						i.SourceAnchor.importBinding, i.SourceBinding = binding, &binding
					} else {
						i.Source = i.Root + "/" + pathBytes(tt.endpoint)
						i.InputBinding = binding
					}
					i.Phase = phase
					if phase == "relocated" {
						f := i.OriginalFile
						i.RelocatedFile = &f
					}
					data, err := json.Marshal(s)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := decodeV2Test(data); (err == nil) != tt.valid {
						t.Fatalf("valid=%v: %v", tt.valid, err)
					}
					writeImportSnapshot(t, o, s)
					before := treeState(t, filepath.Dir(o.Profile))
					if _, err := Down(o, false); err == nil {
						t.Fatal("snapshot authorized mutation")
					}
					if !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Profile))) {
						t.Fatal("snapshot rejection changed fixture")
					}
				})
			}
		}
	}
}

func TestRegistryV2MissingSourceClaims(t *testing.T) {
	for _, missing := range [][]pathBytes{{"nested"}, {"nested", "deeper"}} {
		for _, tt := range []struct {
			name, claim, phase, location string
			valid                        bool
		}{
			{"first missing", "nested", "prepared", "anchor", false},
			{"case alias", "Nested", "prepared", "anchor", false},
			{"unrelated final basename", "file", "prepared", "anchor", true},
			{"unrelated slot", "other", "prepared", "anchor", true},
			{"historical route", "other", "prepared", "route", false},
			{"realized final slot", "file", "prepared", "final", false},
			{"relocated final slot", "file", "relocated", "final", false},
			{"relocated anchor slot", "nested", "relocated", "anchor", false},
			{"relocated unrelated basename", "file", "relocated", "anchor", true},
		} {
			t.Run(fmt.Sprintf("depth=%d/%s", len(missing), tt.name), func(t *testing.T) {
				o, _, s := v2Fixture(t, true)
				i := &s.Imports[0]
				i.Source = i.Root
				for _, name := range missing {
					i.Source += "/" + name
				}
				i.Source += "/file"
				i.SourceAnchor.Missing = missing
				i.SourceBinding = nil
				if tt.location == "final" || tt.phase == "relocated" {
					binding := i.SourceAnchor.importBinding
					binding.Routes = append([]recordedRoute{}, binding.Routes...)
					for n, name := range missing {
						next := entryID{binding.Parent.Device, binding.Parent.Inode + uint64(n) + 1000}
						binding.Routes = append(binding.Routes, recordedRoute{Parent: binding.Parent, Name: name, Entry: next, Type: unix.S_IFDIR, Target: next})
						binding.Parent = next
					}
					i.SourceBinding = &binding
				}
				i.Phase = tt.phase
				if tt.phase == "relocated" {
					f := i.OriginalFile
					i.RelocatedFile = &f
				}
				r := &s.Records[0]
				r.Profile, r.Target, r.Destination = "/old-profile", "/historical", pathBytes("/historical/"+tt.claim)
				r.Parent, r.Routes = i.SourceAnchor.Parent, []recordedRoute{}
				if tt.location == "final" {
					r.Parent = i.SourceBinding.Parent
				}
				if tt.location == "route" {
					next := entryID{r.Parent.Device, r.Parent.Inode + 1000}
					r.Routes = []recordedRoute{{Parent: r.Parent, Name: missing[0], Entry: next, Type: unix.S_IFDIR, Target: next}}
					r.Parent = next
				}
				data, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := decodeV2Test(data); (err == nil) != tt.valid {
					t.Fatalf("valid=%v: %v", tt.valid, err)
				}
				writeImportSnapshot(t, o, s)
				before := treeState(t, filepath.Dir(o.Profile))
				reader, err := openRegistry(o, false, nativeRegistryOps())
				if reader != nil {
					defer reader.Close()
				}
				if (err == nil) != tt.valid {
					t.Fatalf("registry admission valid=%v: %v", tt.valid, err)
				}
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(o.Profile))) {
					t.Fatal("claim inspection changed fixture")
				}
			})
		}
	}
}

func TestImportAnchorProjectionPreservesTraversal(t *testing.T) {
	i := importIntent{Root: "/alias/../source", Source: "/alias/../source/nested/deeper/file", SourceAnchor: importAnchor{Missing: []pathBytes{"nested", "deeper"}}}
	if got := i.sourceAnchorPath(); got != "/alias/../source/nested" {
		t.Fatalf("changed raw traversal: %q", got)
	}
}
