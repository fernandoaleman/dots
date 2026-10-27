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

func TestInspectImportMissingConfiguredRoots(t *testing.T) {
	for _, mode := range []string{"exact", "ancestor", "descendant", "alias", "raw traversal", "case alias", "sibling", "unrelated"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, mode == "descendant")
			other, rejected := request.Source, true
			switch mode {
			case "ancestor":
				other += "/repository"
			case "descendant":
				other = request.Root + "/config"
			case "alias", "raw traversal":
				alias := filepath.Dir(request.Root) + "/alias"
				fixtureLink(t, request.Root, alias)
				other = alias + "/zshrc"
				if mode == "raw traversal" {
					fixtureLink(t, request.Root, alias+"-child")
					other = alias + "-child/../source/zshrc"
				}
			case "case alias":
				other = request.Root + "/ZSHRC"
				rules, err := directoryNameRules(request.Root)
				if err != nil {
					t.Fatal(err)
				}
				rejected = !rules.caseSensitive
			case "sibling":
				other, rejected = request.Root+"/other", false
			case "unrelated":
				other, rejected = filepath.Dir(request.Root)+"/missing/repository", false
			}
			request.Ownership.Sources = append(request.Ownership.Sources, other)
			before := treeState(t, filepath.Dir(request.Root))
			_, err := InspectImport(request)
			if (err != nil) != rejected {
				t.Fatalf("missing configured root %q: %v", other, err)
			}
			if rejected && !strings.Contains(err.Error(), "missing configured source root") {
				t.Fatalf("missing root protection was not reached: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("missing-root inspection modified fixture")
			}
		})
	}
}

func TestImportProjectedIdentityAliases(t *testing.T) {
	request := importInspectionFixture(t, false)
	p, err := namespace()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	dir, _, _, err := p.walk(request.Root)
	if err != nil {
		t.Fatal(err)
	}
	// Model a second mount path naming the same pinned directory, without
	// requiring mounts or modifying system configuration in fixture tests.
	sourceRoot, err := filepath.EvalSymlinks(request.Root)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Dir(sourceRoot) + "/bind-alias"
	p.paths[alias] = dir
	fixtureFile(t, sourceRoot+"/sub/nested/sentinel")
	fixtureFile(t, filepath.Dir(sourceRoot)+"/distinct/sentinel")
	nested, _, _, err := p.walk(sourceRoot + "/sub/nested")
	if err != nil {
		t.Fatal(err)
	}
	distinct, _, _, err := p.walk(filepath.Dir(sourceRoot) + "/distinct")
	if err != nil {
		t.Fatal(err)
	}
	// Shared higher ancestry can diverge at a nested mount and converge
	// again below it; only the pinned bindings establish that distinction.
	p.paths[alias+"/sub"] = distinct
	p.paths[alias+"/sub/nested"] = nested
	for _, tc := range []struct {
		left, right string
		overlap     bool
	}{
		{"zshrc", "zshrc", true},
		{"zshrc", "zshrc/repository", true},
		{"config/nvim/init.lua", "config", true},
		{"config/nvim/init.lua", "config/nvim", true},
		{"config/nvim/init.lua", "config/other", false},
		{"zshrc", "other", false},
		{"sub/file", "sub/file", false},
		{"sub/nested/file", "sub/nested/file", true},
		{"sub/nested/file", "sub/nested/other", false},
	} {
		t.Run(tc.left+"-"+tc.right, func(t *testing.T) {
			before := treeState(t, filepath.Dir(request.Root))
			same, err := p.importProjectedOverlap(sourceRoot+"/"+tc.left, alias+"/"+tc.right)
			if err != nil || same != tc.overlap {
				t.Fatalf("identity-alias overlap: %v, %v", same, err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("identity-alias comparison modified fixture")
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

func TestInspectImportConfigurationIdentity(t *testing.T) {
	for _, mode := range []string{"alias", "unavailable", "distinct"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			ops := nativeImportInspectionOps()
			ops.statConfig = func(path string, stat *unix.Stat_t) error {
				switch mode {
				case "alias":
					return unix.Stat(request.Input, stat)
				case "unavailable":
					return unix.EACCES
				default:
					return unix.Stat(path, stat)
				}
			}
			before := treeState(t, filepath.Dir(request.Root))
			_, err := inspectImport(request, ops)
			if (err != nil) != (mode != "distinct") {
				t.Fatalf("configuration identity: %v", err)
			}
			if mode == "alias" && !strings.Contains(err.Error(), "aliases active configuration") {
				t.Fatalf("identity protection not reached: %v", err)
			}
			if mode == "unavailable" && !errors.Is(err, unix.EACCES) {
				t.Fatalf("inspection uncertainty lost: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("configuration inspection modified fixture")
			}
		})
	}
}

func TestInspectImportBlockedProviderScope(t *testing.T) {
	for _, mode := range []string{"file", "broken link", "file alias", "related", "related alias", "uncertain alias", "unknown names", "required directory", "future parents"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, mode == "future parents")
			blocker := request.Ownership.Target + "/.config"
			path := blocker + "/nvim/init.lua"
			rejected := false
			switch mode {
			case "file":
				fixtureFile(t, blocker)
			case "broken link":
				fixtureLink(t, filepath.Dir(request.Root)+"/missing", blocker)
			case "file alias":
				other := filepath.Dir(request.Root) + "/obstacle"
				fixtureFile(t, other)
				fixtureLink(t, other, blocker)
			case "related":
				path, rejected = request.Input+"/child", true
			case "related alias", "uncertain alias":
				text := request.Input
				if mode == "uncertain alias" {
					text += "/child"
				}
				fixtureLink(t, text, blocker)
				rejected = true
			case "unknown names":
				path, rejected = request.Ownership.Target+"/.ZSHRC/child", true
				fixtureFile(t, request.Ownership.Target+"/.ZSHRC")
			case "required directory":
				path, rejected = request.Input, true
			case "future parents":
				blocker = request.Ownership.Target + "/.other"
				path, rejected = blocker+"/nvim/init.lua", true
				fixtureLink(t, request.Root+"/config", blocker)
			}
			provider := request.Root + "/other/nvim/init.lua"
			fixtureFile(t, provider)
			request.Entries = append(request.Entries, Destination{
				Path: path, Source: provider, Directory: mode == "required directory",
			})
			evidence := append([]Destination(nil), request.Entries...)
			before := treeState(t, filepath.Dir(request.Root))
			ops := nativeImportInspectionOps()
			if mode == "unknown names" {
				ops.nameRules = func(int) (nameRules, error) { return nameRules{}, unix.EIO }
			}
			_, err := inspectImport(request, ops)
			if (err != nil) != rejected {
				t.Fatalf("blocked provider scope: %v", err)
			}
			if !reflect.DeepEqual(evidence, request.Entries) || !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("inspection changed evidence or filesystem")
			}
		})
	}
}

func TestInspectImportLateProtectionChanges(t *testing.T) {
	for _, mode := range []string{"configuration", "blocker"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			path := request.Ownership.Profile
			if mode == "blocker" {
				path = request.Ownership.Target + "/.config"
				fixtureFile(t, path)
				fixtureFile(t, request.Root+"/other")
				request.Entries = append(request.Entries, Destination{Path: path + "/child", Source: request.Root + "/other"})
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			ops := nativeImportInspectionOps()
			ops.checkpoint = func() {
				if err := os.Rename(path, path+".preserved"); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, request.Input, path)
			}
			if _, err := inspectImport(request, ops); err == nil || !strings.Contains(err.Error(), "changed") {
				t.Fatalf("late protection change accepted: %v", err)
			}
			after, err := os.ReadFile(path + ".preserved")
			if err != nil || string(after) != string(content) {
				t.Fatal("original fixture content lost")
			}
		})
	}
}

func TestInspectImportBlockedProviderPermission(t *testing.T) {
	request := importInspectionFixture(t, false)
	blocker := request.Ownership.Target + "/.private"
	fixtureFile(t, blocker+"/child")
	fixtureFile(t, request.Root+"/other")
	request.Entries = append(request.Entries, Destination{Path: blocker + "/child", Source: request.Root + "/other"})
	before := treeState(t, filepath.Dir(request.Root))
	if err := os.Chmod(blocker, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocker, 0o700) })
	if fd, err := unix.Open(blocker, directoryFlags, 0); err == nil {
		_ = unix.Close(fd)
		t.Skip("existing privileges permit opening the restricted fixture")
	}
	_, err := InspectImport(request)
	if restore := os.Chmod(blocker, 0o700); restore != nil {
		t.Fatal(restore)
	}
	if !errors.Is(err, unix.EACCES) {
		t.Fatalf("permission uncertainty suppressed: %v", err)
	}
	if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
		t.Fatal("permission inspection changed fixture")
	}
}

func TestInspectImportDanglingReferentChanges(t *testing.T) {
	for _, mode := range []string{"alias", "frontier"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, true)
			base := filepath.Dir(request.Root)
			external, alias := base+"/external", base+"/alias"
			if err := os.Mkdir(external, 0o700); err != nil {
				t.Fatal(err)
			}
			fixtureLink(t, external, alias)
			blocker := request.Ownership.Target + "/.other"
			fixtureLink(t, alias+"/config/nvim", blocker)
			fixtureFile(t, request.Root+"/other")
			request.Entries = append(request.Entries, Destination{Path: blocker + "/child", Source: request.Root + "/other"})
			before, err := os.ReadFile(request.Input)
			if err != nil {
				t.Fatal(err)
			}
			ops := nativeImportInspectionOps()
			ops.checkpoint = func() {
				if mode == "alias" {
					if err := os.Rename(alias, alias+".preserved"); err != nil {
						t.Fatal(err)
					}
					fixtureLink(t, request.Root, alias)
				} else {
					fixtureLink(t, request.Root+"/config", external+"/config")
				}
			}
			if _, err := inspectImport(request, ops); err == nil || !strings.Contains(err.Error(), "changed") {
				t.Fatalf("changed dangling referent accepted: %v", err)
			}
			after, err := os.ReadFile(request.Input)
			if err != nil || string(after) != string(before) {
				t.Fatal("input content changed")
			}
		})
	}
}

func TestImportProtectedDirectoryAliases(t *testing.T) {
	for _, kind := range []string{"source", "state"} {
		t.Run(kind, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			boundary := request.Root
			if kind == "state" {
				boundary = filepath.Dir(request.Ownership.Registry)
				fixtureFile(t, boundary+"/ownership.json")
			}
			alias := request.Ownership.Target + "/.alias"
			fixtureLink(t, boundary, alias)
			p, err := namespace()
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			before := treeState(t, filepath.Dir(request.Root))
			err = p.importProtectedDirectory(alias+"/file", boundary)
			if err == nil || !strings.Contains(err.Error(), "protected namespace") {
				t.Fatalf("protected directory alias accepted: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("directory protection changed fixture")
			}
		})
	}
}

func TestInspectImportRegistryIdentityRecheck(t *testing.T) {
	for _, mode := range []string{"absent alias", "existing alias", "distinct replacement", "stat failure", "early failure"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			if err := os.MkdirAll(filepath.Dir(request.Ownership.Registry), 0o700); err != nil {
				t.Fatal(err)
			}
			existing := mode == "existing alias" || mode == "distinct replacement"
			if existing {
				writeImportSnapshot(t, request.Ownership, snapshot{Version: 1, Revision: 1, Records: []ownershipRecord{}})
				fixtureFile(t, importLockPath(request.Ownership.Registry))
			}
			before, err := os.ReadFile(request.Input)
			if err != nil {
				t.Fatal(err)
			}
			ops := nativeImportInspectionOps()
			calls, changed := 0, false
			ops.statState = func(path string, stat *unix.Stat_t) error {
				calls++
				if mode == "stat failure" && changed {
					return unix.EACCES
				}
				return unix.Stat(path, stat)
			}
			change := func() {
				changed = true
				if mode == "stat failure" {
					return
				}
				if existing {
					if err := os.Rename(request.Ownership.Registry, request.Ownership.Registry+".preserved"); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "distinct replacement" {
					writeImportSnapshot(t, request.Ownership, snapshot{Version: 1, Revision: 2, Records: []ownershipRecord{}})
				} else {
					fixtureLink(t, request.Input, request.Ownership.Registry)
				}
			}
			ops.checkpoint = change
			if mode == "early failure" {
				ops.openInput = func(int, string, int, uint32) (int, error) {
					change()
					return -1, unix.EIO
				}
			}
			_, err = inspectImport(request, ops)
			if mode == "distinct replacement" {
				if err == nil || !strings.Contains(err.Error(), "ownership state binding changed") {
					t.Fatalf("snapshot replacement went undetected: %v", err)
				}
			} else if mode == "stat failure" {
				if !errors.Is(err, unix.EACCES) {
					t.Fatalf("final registry inspection error lost: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "aliases reserved ownership state") {
				t.Fatalf("late registry alias accepted or unreported: %v", err)
			}
			if mode == "early failure" && !errors.Is(err, unix.EIO) {
				t.Fatalf("original input failure lost: %v", err)
			}
			if calls != 2 {
				t.Fatalf("registry checked %d times, want initial and final checks", calls)
			}
			after, readErr := os.ReadFile(request.Input)
			if readErr != nil || string(before) != string(after) {
				t.Fatal("input content changed")
			}
			if existing {
				if _, err := os.Stat(request.Ownership.Registry + ".preserved"); err != nil {
					t.Fatal("original registry fixture lost")
				}
			}
		})
	}
}

func TestImportCheckRoute(t *testing.T) {
	for _, mode := range []string{"valid", "identity mismatch", "missing", "read only", "device failure", "cross device"} {
		t.Run(mode, func(t *testing.T) {
			request := importInspectionFixture(t, false)
			name := filepath.Dir(request.Input)
			var input, parent unix.Stat_t
			if err := unix.Stat(request.Input, &input); err != nil {
				t.Fatal(err)
			}
			if err := unix.Stat(name, &parent); err != nil {
				t.Fatal(err)
			}
			expected := identity(parent)
			ops := nativeImportInspectionOps()
			switch mode {
			case "identity mismatch":
				expected.inode++
			case "missing":
				name += "/missing"
			case "read only":
				ops.writable = func(int) error { return unix.EROFS }
			case "device failure":
				ops.device = func(int) (uint64, error) { return 0, unix.EIO }
			case "cross device":
				ops.device = func(int) (uint64, error) { return uint64(input.Dev) + 1, nil }
			}
			before := treeState(t, filepath.Dir(request.Root))
			err := importCheckRoute(name, expected, input, ops)
			if (err != nil) != (mode != "valid") {
				t.Fatalf("route check: %v", err)
			}
			if mode == "read only" && !errors.Is(err, unix.EROFS) {
				t.Fatalf("read-only error lost: %v", err)
			}
			if !reflect.DeepEqual(before, treeState(t, filepath.Dir(request.Root))) {
				t.Fatal("route inspection wrote fixture")
			}
		})
	}
}
