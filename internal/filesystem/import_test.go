package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func importInspectionFixture(t *testing.T, nested bool) ImportRequest {
	t.Helper()
	base := t.TempDir()
	root, target := filepath.Join(base, "source"), filepath.Join(base, "target")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	name, source := ".zshrc", "zshrc"
	if nested {
		name, source = ".config/nvim/init.lua", "config/nvim/init.lua"
	}
	request := ImportRequest{
		Input: filepath.Join(target, name), Source: filepath.Join(root, source), Root: root,
		Ownership: Ownership{Registry: filepath.Join(base, "state/dots/ownership.json"),
			Profile: filepath.Join(base, "config.toml"), Target: target, Sources: []string{root}},
	}
	fixtureFile(t, request.Input)
	fixtureFile(t, request.Ownership.Profile)
	request.Entries = []Destination{{Path: request.Input, Source: request.Source}}
	if nested {
		for _, parent := range []string{"config", "config/nvim"} {
			request.Entries = append(request.Entries, Destination{Path: filepath.Join(target, "."+parent),
				Source: filepath.Join(root, parent), Directory: true})
		}
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg-config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "xdg-state"))
	t.Setenv("TMPDIR", base)
	return request
}

func TestInspectImportReadOnly(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprint(nested), func(t *testing.T) {
			request := importInspectionFixture(t, nested)
			base := filepath.Dir(request.Root)
			before := treeState(t, base)
			original := request
			original.Entries = append([]Destination(nil), request.Entries...)
			original.Ownership.Sources = append([]string(nil), request.Ownership.Sources...)
			input, err := os.Lstat(request.Input)
			if err != nil {
				t.Fatal(err)
			}
			report, err := InspectImport(request)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if nested {
				want = []string{request.Root + "/config", request.Root + "/config/nvim"}
			}
			if !reflect.DeepEqual(report.MissingParents, want) {
				t.Fatalf("missing parents: %+v", report)
			}
			again, err := InspectImport(request)
			if err != nil || !reflect.DeepEqual(report, again) {
				t.Fatalf("unstable inspection: %+v, %v", again, err)
			}
			if !reflect.DeepEqual(request, original) || !reflect.DeepEqual(before, treeState(t, base)) {
				t.Fatal("inspection modified request or fixture")
			}
			after, err := os.Lstat(request.Input)
			if err != nil || !os.SameFile(input, after) {
				t.Fatal("input identity changed")
			}
		})
	}
}

func TestInspectImportRejectsInputWithoutOpeningSpecialEntries(t *testing.T) {
	for _, mode := range []string{"missing", "directory", "symlink", "broken link", "fifo", "hard link", "privileged"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			if err := os.Rename(request.Input, request.Input+".preserved"); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "directory":
				if err := os.Mkdir(request.Input, 0o700); err != nil {
					t.Fatal(err)
				}
			case "symlink", "broken link":
				text := request.Input + ".preserved"
				if mode == "broken link" {
					text += ".missing"
				}
				fixtureLink(t, text, request.Input)
			case "fifo":
				if err := unix.Mkfifo(request.Input, 0o600); err != nil {
					t.Fatal(err)
				}
			case "hard link":
				if err := os.Link(request.Input+".preserved", request.Input); err != nil {
					t.Fatal(err)
				}
			case "privileged":
				fixtureFile(t, request.Input)
				if err := os.Chmod(request.Input, 0o600|os.ModeSetuid); err != nil {
					t.Fatal(err)
				}
			}
			ops := nativeImportInspectionOps()
			opened := false
			ops.openInput = func(int, string, int, uint32) (int, error) { opened = true; return -1, unix.EIO }
			_, err := inspectImport(request, ops)
			if err == nil || opened {
				t.Fatalf("unsafe input opened: %v, %v", opened, err)
			}
			data, err := os.ReadFile(request.Input + ".preserved")
			if err != nil || len(data) == 0 {
				t.Fatal("original content lost")
			}
		})
	}
}

func TestInspectImportSourceOccupantsAndParents(t *testing.T) {
	for _, mode := range []string{"file", "directory", "symlink", "broken link", "parent file", "parent symlink", "existing parents"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, true)
			base := filepath.Dir(request.Root)
			blocked := request.Source
			if strings.HasPrefix(mode, "parent") {
				blocked = filepath.Join(request.Root, "config")
			}
			switch mode {
			case "file", "parent file":
				fixtureFile(t, blocked)
			case "directory", "existing parents":
				if mode == "existing parents" {
					blocked = filepath.Dir(request.Source)
				}
				if err := os.MkdirAll(blocked, 0o700); err != nil {
					t.Fatal(err)
				}
			case "symlink", "broken link", "parent symlink":
				fixtureFile(t, filepath.Join(base, "external", "sentinel"))
				if err := os.MkdirAll(filepath.Dir(blocked), 0o700); err != nil {
					t.Fatal(err)
				}
				text := filepath.Join(base, "external")
				if mode == "broken link" {
					text += ".missing"
				}
				fixtureLink(t, text, blocked)
			}
			before := treeState(t, base)
			report, err := InspectImport(request)
			if (err == nil) != (mode == "existing parents") {
				t.Fatalf("source check: %+v, %v", report, err)
			}
			if !reflect.DeepEqual(before, treeState(t, base)) {
				t.Fatal("source inspection modified data")
			}
		})
	}
}

func TestInspectImportBoundaries(t *testing.T) {
	for _, mode := range []string{"outside target", "outside source", "parent traversal", "unconfigured root", "missing root",
		"duplicate root", "source overlap", "configuration", "configuration alias", "state", "missing evidence", "relative", "NUL"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			base := filepath.Dir(request.Root)
			switch mode {
			case "outside target":
				request.Input = filepath.Join(base, "outside")
			case "outside source":
				request.Source = filepath.Join(base, "outside")
			case "parent traversal":
				request.Source = request.Root + "/../outside"
			case "unconfigured root":
				request.Ownership.Sources = nil
			case "missing root":
				request.Root += ".missing"
				request.Source = request.Root + "/zshrc"
				request.Ownership.Sources = []string{request.Root}
			case "duplicate root":
				request.Ownership.Sources = append(request.Ownership.Sources, request.Root)
			case "source overlap":
				child := request.Root + "/child"
				if err := os.Mkdir(child, 0o700); err != nil {
					t.Fatal(err)
				}
				request.Ownership.Sources = append(request.Ownership.Sources, child)
			case "configuration":
				request.Ownership.Profile = request.Input
			case "configuration alias":
				alias := filepath.Join(base, "config-alias")
				fixtureLink(t, request.Input, alias)
				request.Ownership.Profile = alias
			case "state":
				request.Ownership.Registry = filepath.Join(request.Ownership.Target, "dots", "ownership.json")
				request.Input = filepath.Join(request.Ownership.Target, "dots", "private")
				fixtureFile(t, request.Input)
			case "missing evidence":
				request.Entries = nil
			case "relative":
				request.Input = ".zshrc"
			case "NUL":
				request.Input += "\x00"
			}
			before := treeState(t, base)
			if _, err := InspectImport(request); err == nil {
				t.Fatal("invalid boundary accepted")
			}
			if !reflect.DeepEqual(before, treeState(t, base)) {
				t.Fatal("failed boundary check modified fixture")
			}
		})
	}
}

func TestInspectImportObservedFailuresAndCleanup(t *testing.T) {
	for _, mode := range []string{"permission", "read-only mount", "cross-device", "foreign owner", "metadata failure",
		"open failure", "input replacement", "input edit", "source appeared", "parent replacement", "success"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, mode == "parent replacement")
			base := filepath.Dir(request.Root)
			if mode == "parent replacement" {
				if err := os.MkdirAll(filepath.Dir(request.Source), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			ops := nativeImportInspectionOps()
			inputFD, parentFD := -1, -1
			ops.openInput = func(parent int, name string, flags int, permissions uint32) (int, error) {
				parentFD = parent
				if mode == "open failure" {
					return -1, unix.EMFILE
				}
				fd, err := unix.Openat(parent, name, flags, permissions)
				inputFD = fd
				return fd, err
			}
			switch mode {
			case "permission", "read-only mount":
				ops.writable = func(int) error {
					if mode == "permission" {
						return unix.EACCES
					}
					return unix.EROFS
				}
			case "cross-device":
				ops.device = func(int) (uint64, error) { return ^uint64(0), nil }
			case "foreign owner", "metadata failure":
				ops.statFile = func(fd int, stat *unix.Stat_t) error {
					if mode == "metadata failure" {
						return unix.EIO
					}
					if err := unix.Fstat(fd, stat); err != nil {
						return err
					}
					stat.Uid++
					return nil
				}
			case "input replacement", "input edit", "source appeared", "parent replacement":
				ops.checkpoint = func() {
					switch mode {
					case "input replacement":
						if err := os.Rename(request.Input, request.Input+".preserved"); err != nil {
							t.Fatal(err)
						}
						fixtureFile(t, request.Input)
					case "input edit":
						if err := os.WriteFile(request.Input, []byte("changed input bytes"), 0o600); err != nil {
							t.Fatal(err)
						}
					case "source appeared":
						fixtureFile(t, request.Source)
					case "parent replacement":
						if err := os.Rename(request.Root+"/config", request.Root+"/moved"); err != nil {
							t.Fatal(err)
						}
						fixtureFile(t, filepath.Join(base, "external/sentinel"))
						fixtureLink(t, filepath.Join(base, "external"), request.Root+"/config")
					}
				}
			}
			_, err := inspectImport(request, ops)
			if (err == nil) != (mode == "success") {
				t.Fatalf("unexpected result: %v", err)
			}
			if mode == "cross-device" && !strings.Contains(err.Error(), "cross-filesystem import is unsupported") {
				t.Fatalf("device check was not reached: %v", err)
			}
			var stat unix.Stat_t
			for _, fd := range []int{inputFD, parentFD} {
				if fd >= 0 && !errors.Is(unix.Fstat(fd, &stat), unix.EBADF) {
					t.Fatalf("descriptor %d escaped inspection", fd)
				}
			}
		})
	}
}

func TestInspectImportNamespaceAliases(t *testing.T) {
	for _, mode := range []string{"root alias", "target ancestor alias", "raw target traversal", "raw traversal", "raw traversal across root", "provider alias"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, true)
			base := filepath.Dir(request.Root)
			switch mode {
			case "root alias":
				alias := filepath.Join(base, "repo-alias")
				fixtureLink(t, request.Root, alias)
				request.Root, request.Source = alias, alias+"/config/nvim/init.lua"
				request.Ownership.Sources = []string{alias}
			case "target ancestor alias":
				original := request.Ownership.Target + "/.config"
				actual := filepath.Join(base, "external-config")
				if err := os.Rename(original, actual); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, actual, original)
			case "raw traversal":
				child := request.Root + "/child"
				if err := os.Mkdir(child, 0o700); err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(base, "alias")
				fixtureLink(t, child, alias)
				request.Root = alias + "/.."
				request.Source = request.Root + "/config/nvim/init.lua"
				request.Ownership.Sources = []string{request.Root}
			case "raw target traversal":
				actual := filepath.Join(base, "actual")
				if err := os.MkdirAll(actual+"/child", 0o700); err != nil {
					t.Fatal(err)
				}
				oldTarget := request.Ownership.Target
				if err := os.Rename(oldTarget, actual+"/target"); err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(base, "target-alias")
				fixtureLink(t, actual+"/child", alias)
				request.Ownership.Target = alias + "/../target"
				request.Input = request.Ownership.Target + "/.config/nvim/init.lua"
				for i := range request.Entries {
					request.Entries[i].Path = request.Ownership.Target + strings.TrimPrefix(request.Entries[i].Path, oldTarget)
				}
			case "raw traversal across root":
				other := filepath.Join(base, "other")
				if err := os.MkdirAll(other+"/child", 0o700); err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(base, "alias")
				fixtureLink(t, other+"/child", alias)
				request.Root = alias + "/../../source"
				request.Source = request.Root + "/config/nvim/init.lua"
				request.Ownership.Sources = []string{request.Root, other}
			case "provider alias":
				alias := filepath.Join(base, "target-alias")
				fixtureLink(t, request.Ownership.Target, alias)
				request.Entries = append(request.Entries, Destination{Path: alias + "/.config/nvim/init.lua", Source: request.Root + "/other"})
			}
			request.Entries[0].Source = request.Source
			before := treeState(t, base)
			_, err := InspectImport(request)
			if (err != nil) != (mode == "provider alias") {
				t.Fatalf("alias handling: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, base)) {
				t.Fatal("alias inspection changed fixture")
			}
		})
	}
}

func TestInspectImportNameCapabilities(t *testing.T) {
	for _, unicode := range []bool{false, true} {
		for _, mode := range []string{"sensitive", "equivalent", "unknown"} {
			t.Run(fmt.Sprintf("unicode=%v/%s", unicode, mode), func(t *testing.T) {
				request := importInspectionFixture(t, false)
				name, other := ".zshrc", ".ZSHRC"
				if unicode {
					name, other = ".é", ".e\u0301"
				}
				request.Input = request.Ownership.Target + "/" + name
				request.Source = request.Root + "/" + strings.TrimPrefix(name, ".")
				fixtureFile(t, request.Input)
				request.Entries = []Destination{{Path: request.Input, Source: request.Source}}
				fixtureFile(t, request.Root+"/"+other)
				request.Entries = append(request.Entries, Destination{Path: request.Ownership.Target + "/" + other, Source: request.Root + "/" + other})

				before := treeState(t, filepath.Dir(request.Root))
				ops := nativeImportInspectionOps()
				queries := 0
				ops.nameRules = func(int) (nameRules, error) {
					queries++
					if mode == "unknown" {
						return nameRules{}, unix.EIO
					}
					return nameRules{caseSensitive: mode == "sensitive", normalizationSensitive: mode == "sensitive"}, nil
				}
				_, first := inspectImport(request, ops)
				for i, j := 0, len(request.Entries)-1; i < j; i, j = i+1, j-1 {
					request.Entries[i], request.Entries[j] = request.Entries[j], request.Entries[i]
				}
				_, second := inspectImport(request, ops)
				if (first != nil) != (mode != "sensitive") || fmt.Sprint(first) != fmt.Sprint(second) || queries == 0 {
					t.Fatalf("capability handling: %v, %v, queries=%d", first, second, queries)
				}
				if mode == "unknown" && !strings.Contains(first.Error(), unix.EIO.Error()) {
					t.Fatalf("uncertainty lost: %v", first)
				}
				if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
					t.Fatal("capability check modified fixture")
				}
			})
		}
	}
}

func TestImportParentEnumeration(t *testing.T) {
	request := importInspectionFixture(t, true)
	if err := os.Mkdir(request.Root+"/CONFIG", 0o700); err != nil {
		t.Fatal(err)
	}
	p, err := namespace()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	parent, _, _, err := p.walk(request.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := importExactParent(parent.fd, "CONFIG"); err != nil {
		t.Fatal(err)
	}
	if err := importExactParent(parent.fd, "config"); err == nil {
		t.Fatal("different enumerated spelling accepted")
	}
	if _, err := os.Stat(request.Root + "/config"); os.IsNotExist(err) {
		return
	}
	if _, err := InspectImport(request); err == nil || !strings.Contains(err.Error(), "spelling") {
		t.Fatalf("source-parent alias accepted: %v", err)
	}
}

func TestInspectImportDirectoryPermissions(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprint(blocked), func(t *testing.T) {
			request := importInspectionFixture(t, true)
			parent := filepath.Dir(request.Source)
			if err := os.MkdirAll(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			path := request.Root
			if blocked {
				path = parent
			}
			if err := os.Chmod(path, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(path, 0o700); err != nil {
					t.Error(err)
				}
			})
			if blocked && unix.Faccessat(unix.AT_FDCWD, path, unix.W_OK, unix.AT_EACCESS) == nil {
				t.Skip("filesystem permits writing the restricted fixture")
			}
			before := treeState(t, filepath.Dir(request.Root))
			_, err := InspectImport(request)
			if (err != nil) != blocked {
				t.Fatalf("directory permissions: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("permission checks modified fixture")
			}
		})
	}
}
