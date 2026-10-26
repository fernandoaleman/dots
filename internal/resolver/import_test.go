package resolver

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScanRetainsImportCollisionEvidence(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{
		"local/gitconfig",
		"local/.gitconfig",
		"shared/.gitconfig",
	} {
		writeConfig(t, filepath.Join(base, name), name)
	}
	if err := os.Mkdir(filepath.Join(base, "local/empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		DotfilesDirs: []string{
			filepath.Join(base, "local"),
			filepath.Join(base, "shared"),
		},
		TargetDir: filepath.Join(base, "target"),
	}
	candidates, diagnostics := scanSources(cfg)
	if len(diagnostics.Errors) != 0 {
		t.Fatal(diagnostics.Errors)
	}
	leaves, directories := 0, 0
	for _, item := range candidates {
		if item.directory {
			directories++
		} else {
			leaves++
		}
	}
	if leaves != 3 || directories != 1 {
		t.Fatalf("lost collision evidence: %d leaves, %d directories", leaves, directories)
	}
	want := map[string]string{
		filepath.Join(base, "local/gitconfig"):   filepath.Join(base, "target/.gitconfig"),
		filepath.Join(base, "local/.gitconfig"):  filepath.Join(base, "target/.gitconfig"),
		filepath.Join(base, "shared/.gitconfig"): filepath.Join(base, "target/.gitconfig"),
		filepath.Join(base, "local/empty"):       filepath.Join(base, "target/.empty"),
	}
	for _, item := range candidates {
		if destination, exists := want[item.source]; !exists || destination != item.destination {
			t.Fatalf("unexpected scanned candidate: %+v", item)
		}
		delete(want, item.source)
	}
	if len(want) != 0 {
		t.Fatalf("missing scanned candidates: %+v", want)
	}
	plan := resolveCandidates(cfg, candidates, diagnostics)
	if len(plan.Links) != 1 || plan.Links[0].Source != filepath.Join(base, "local/.gitconfig") {
		t.Fatalf("unexpected selected mapping: %+v", plan)
	}
	if len(plan.Directories) != 1 {
		t.Fatalf("lost empty-directory evidence: %+v", plan)
	}
	if !reflect.DeepEqual(plan, Resolve(cfg)) {
		t.Fatal("shared candidate resolution differs from Resolve")
	}
}

func importConfig(t *testing.T, roots ...string) Config {
	t.Helper()
	base := t.TempDir()
	cfg := Config{TargetDir: filepath.Join(base, "target"), WarnDuplicates: true}
	for _, root := range roots {
		name := filepath.Join(base, root)
		if err := os.MkdirAll(name, 0o700); err != nil {
			t.Fatal(err)
		}
		cfg.DotfilesDirs = append(cfg.DotfilesDirs, name)
	}
	return cfg
}

func TestPlanImportNames(t *testing.T) {
	tests := []struct{ target, source string }{
		{".zshrc", "zshrc"},
		{".gitconfig", "gitconfig"},
		{".config/nvim/init.lua", "config/nvim/init.lua"},
		{".config/nvim/.settings", "config/nvim/.settings"},
		{"..name", "..name"},
		{"...name/child", "...name/child"},
		{".notes*.txt", "notes*.txt"},
		{"", ""}, {"/.zshrc", ""}, {"plain", ""},
		{".config/", ""}, {".config//file", ""},
		{"./.zshrc", ""}, {"../.zshrc", ""},
		{".config/./file", ""}, {".config/../file", ""},
		{".", ""}, {"..", ""}, {".bad\x00name", ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.target), func(t *testing.T) {
			cfg := importConfig(t, "local")
			got, err := PlanImport(cfg, "", tt.target)
			if tt.source == "" {
				if err == nil || !reflect.DeepEqual(got, ImportPlan{}) {
					t.Fatalf("invalid argument returned a plan: %+v, %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := Link{Source: appendPath(cfg.DotfilesDirs[0], tt.source),
				Destination: appendPath(cfg.TargetDir, tt.target)}
			if got.Root != cfg.DotfilesDirs[0] || got.Link != want ||
				len(got.Plan.Links) != 1 || got.Plan.Links[0] != want {
				t.Fatalf("mapping = %+v; want %+v", got, want)
			}
			if _, err := os.Lstat(want.Source); !os.IsNotExist(err) {
				t.Fatalf("prospective source was created or inspected incorrectly: %v", err)
			}
		})
	}
}

func TestPlanImportRootChoice(t *testing.T) {
	for _, mode := range []string{
		"sole root", "multiple omitted", "first explicit", "second explicit",
		"unknown", "missing selected", "other missing", "non-directory",
		"configured alias", "unconfigured alias", "duplicate", "aliased duplicate", "overlap",
		"empty configuration", "relative root", "relative target",
	} {
		t.Run(mode, func(t *testing.T) {
			cfg := importConfig(t, "local", "shared")
			local, shared := cfg.DotfilesDirs[0], cfg.DotfilesDirs[1]
			base := filepath.Dir(local)
			selected, want := local, local
			switch mode {
			case "sole root":
				cfg.DotfilesDirs, selected = []string{local}, ""
			case "multiple omitted":
				selected, want = "", ""
			case "first explicit":
			case "second explicit":
				selected, want = shared, shared
			case "unknown":
				selected, want = filepath.Join(base, "unknown"), ""
			case "missing selected":
				cfg.DotfilesDirs[0] = filepath.Join(base, "missing")
				selected, want = cfg.DotfilesDirs[0], ""
			case "other missing":
				cfg.DotfilesDirs[1] = filepath.Join(base, "missing")
			case "non-directory":
				selected, want = filepath.Join(base, "file"), ""
				writeConfig(t, selected, "preserve")
				cfg.DotfilesDirs[0] = selected
			case "configured alias", "unconfigured alias", "aliased duplicate":
				alias := filepath.Join(base, "alias")
				if err := os.Symlink(local, alias); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "configured alias":
					cfg.DotfilesDirs, selected, want = []string{alias}, "", alias
				case "unconfigured alias":
					selected, want = alias, ""
				default:
					cfg.DotfilesDirs, want = []string{local, alias}, ""
				}
			case "duplicate":
				cfg.DotfilesDirs, want = []string{local, local}, ""
			case "overlap":
				child := filepath.Join(local, "child")
				if err := os.Mkdir(child, 0o700); err != nil {
					t.Fatal(err)
				}
				cfg.DotfilesDirs, want = []string{child, local}, ""
			case "empty configuration":
				cfg.DotfilesDirs, selected, want = nil, "", ""
			case "relative root":
				cfg.DotfilesDirs, selected, want = []string{"local"}, "", ""
			case "relative target":
				cfg.TargetDir, want = "target", ""
			}
			got, err := PlanImport(cfg, selected, ".zshrc")
			if want == "" {
				if err == nil || !reflect.DeepEqual(got, ImportPlan{}) {
					t.Fatalf("unsafe root selection: %+v, %v", got, err)
				}
			} else if err != nil || got.Root != want {
				t.Fatalf("root = %q, %v; want %q", got.Root, err, want)
			}
			if mode == "other missing" && (err != nil || len(got.Plan.Warnings) != 1) {
				t.Fatalf("missing-root warning lost: %+v, %v", got, err)
			}
		})
	}
}

func TestPlanImportExclusions(t *testing.T) {
	tests := []struct {
		target, exclude string
		blocked         bool
	}{
		{".gitconfig", "gitconfig", true},
		{".gitconfig", ".gitconfig", false},
		{".config/nvim/init.lua", "config", true},
		{".config/nvim/init.lua", "config/nvim", true},
		{".config/nvim/init.lua", "config/nvim/init.lua", true},
		{".config/nvim/init.lua", "config/nvi", false},
		{".notes*.txt", "notes*.txt", true},
		{".notes.txt", "notes*", false},
		{"..name", "..name", true},
	}
	for _, tt := range tests {
		t.Run(tt.target+"/"+tt.exclude, func(t *testing.T) {
			cfg := importConfig(t, "local")
			cfg.Excludes = []string{tt.exclude}
			got, err := PlanImport(cfg, "", tt.target)
			if tt.blocked {
				if err == nil || !strings.Contains(err.Error(), "excluded") ||
					!reflect.DeepEqual(got, ImportPlan{}) {
					t.Fatalf("exclusion bypassed: %+v, %v", got, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlanImportCollisionEvidence(t *testing.T) {
	tests := []struct {
		name, target string
		files, dirs  []string
	}{
		{"same root undotted", ".gitconfig", []string{"local/gitconfig"}, nil},
		{"same root dotted", ".gitconfig", []string{"local/.gitconfig"}, nil},
		{"later provider", ".gitconfig", []string{"shared/gitconfig"}, nil},
		{"higher provider", ".gitconfig", []string{"local/gitconfig"}, nil},
		{"all losing providers", ".gitconfig", []string{"local/gitconfig", "local/.gitconfig", "shared/gitconfig"}, nil},
		{"ancestor leaf", ".config/nvim/init.lua", []string{"shared/config"}, nil},
		{"descendant leaf", ".config", []string{"shared/config/child"}, nil},
		{"empty directory", ".config", nil, []string{"shared/.config"}},
		{"unrelated existing collision", ".zshrc", []string{"local/config", "shared/.config/child"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := importConfig(t, "local", "shared")
			base := filepath.Dir(cfg.TargetDir)
			for _, name := range tt.files {
				writeConfig(t, filepath.Join(base, name), name)
			}
			for _, name := range tt.dirs {
				if err := os.MkdirAll(filepath.Join(base, name), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			selected := cfg.DotfilesDirs[0]
			if tt.name == "higher provider" {
				selected = cfg.DotfilesDirs[1]
			}
			got, err := PlanImport(cfg, selected, tt.target)
			if err == nil {
				t.Fatalf("collision accepted: %+v", got)
			}
			if got.Root != selected {
				t.Fatalf("root changed after collision: %+v", got)
			}
			providers := make(map[string]bool)
			for _, link := range got.Providers {
				providers[link.Source] = true
			}
			if len(providers) != len(tt.files) {
				t.Fatalf("lost providers: %+v", got)
			}
			for _, name := range tt.files {
				if !providers[filepath.Join(base, name)] {
					t.Fatalf("missing provider %q: %+v", name, got)
				}
			}
			if len(tt.dirs) != 0 && len(got.Plan.Directories) == 0 {
				t.Fatal("empty directory evidence lost")
			}
			if tt.name == "ancestor leaf" || tt.name == "descendant leaf" || tt.name == "empty directory" {
				var reason string
				for _, link := range got.Plan.Links {
					if link.Destination == got.Link.Destination {
						reason = link.Conflict
					}
				}
				existing := "shared/config"
				destination := ".config"
				if tt.name == "descendant leaf" {
					existing, destination = "shared/config/child", ".config/child"
				} else if tt.name == "empty directory" {
					existing = "shared/.config"
				}
				for _, participant := range []string{
					"structural collision",
					fmt.Sprintf("%q -> %q", got.Link.Source, got.Link.Destination),
					fmt.Sprintf("%q -> %q", filepath.Join(base, existing), filepath.Join(cfg.TargetDir, destination)),
				} {
					if !strings.Contains(reason, participant) {
						t.Fatalf("missing structural participant %q in %q", participant, reason)
					}
				}
			}
		})
	}
}

func TestPlanImportExcludedProviderEvidence(t *testing.T) {
	cfg := importConfig(t, "local", "shared")
	base := filepath.Dir(cfg.TargetDir)
	for _, name := range []string{"local/.gitconfig", "local/gitconfig", "shared/gitconfig"} {
		writeConfig(t, filepath.Join(base, name), name)
	}
	cfg.Excludes = []string{".gitconfig"}
	got, err := PlanImport(cfg, cfg.DotfilesDirs[0], ".gitconfig")
	if err == nil {
		t.Fatal("existing unexcluded provider accepted")
	}
	want := []Link{
		{Source: filepath.Join(base, "local/gitconfig"), Destination: filepath.Join(cfg.TargetDir, ".gitconfig")},
		{Source: filepath.Join(base, "shared/gitconfig"), Destination: filepath.Join(cfg.TargetDir, ".gitconfig")},
	}
	if !reflect.DeepEqual(got.Providers, want) {
		t.Fatalf("excluded provider or losing evidence mishandled: %+v", got.Providers)
	}
}

func TestPlanImportMergingAndDiagnostics(t *testing.T) {
	var expected string
	for _, reverse := range []bool{false, true} {
		cfg := importConfig(t, "local", "shared")
		base := filepath.Dir(cfg.TargetDir)
		files := []string{"local/gitconfig", "local/.gitconfig",
			"local/config/nvim/local.lua", "shared/.config/nvim/shared.lua"}
		for i := range files {
			index := i
			if reverse {
				index = len(files) - 1 - i
			}
			writeConfig(t, filepath.Join(base, files[index]), files[index])
		}
		cfg.DotfilesDirs = append(cfg.DotfilesDirs, filepath.Join(base, "missing"))
		got, err := PlanImport(cfg, cfg.DotfilesDirs[0], ".config/nvim/init.lua")
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Plan.Links) != 4 || len(got.Providers) != 4 || len(got.Plan.Directories) != 4 {
			t.Fatalf("directory merge or losing evidence changed: %+v", got)
		}
		if len(got.Plan.Warnings) != 2 ||
			!strings.Contains(got.Plan.Warnings[0], "duplicate destination") ||
			!strings.Contains(got.Plan.Warnings[1], "missing source root") {
			t.Fatalf("diagnostics changed between resolution passes: %+v", got.Plan.Warnings)
		}
		stable := strings.ReplaceAll(fmt.Sprintf("%#v", got), base, "<fixture>")
		if !reverse {
			expected = stable
		} else if expected != stable {
			t.Fatalf("creation order changed plan:\n%s\n%s", expected, stable)
		}
		again, err := PlanImport(cfg, cfg.DotfilesDirs[0], ".config/nvim/init.lua")
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatalf("repeated planning changed result: %+v, %v", again, err)
		}
	}
}

func TestPlanImportReadOnlyAndOccupancyIndependent(t *testing.T) {
	for _, relative := range []string{".zshrc", ".config/nvim/init.lua"} {
		for _, mode := range []string{"missing", "regular", "directory", "symlink"} {
			t.Run(relative+"/"+mode, func(t *testing.T) {
				cfg := importConfig(t, "local")
				base := filepath.Dir(cfg.TargetDir)
				t.Setenv("HOME", filepath.Join(base, "home"))
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "xdg-config"))
				t.Setenv("XDG_STATE_HOME", filepath.Join(base, "xdg-state"))
				input := appendPath(cfg.TargetDir, relative)
				switch mode {
				case "regular":
					writeConfig(t, input, "preserve original bytes")
				case "directory":
					if err := os.MkdirAll(input, 0o700); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					if err := os.MkdirAll(filepath.Dir(input), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink("missing-referent", input); err != nil {
						t.Fatal(err)
					}
				}
				original := cfg
				original.DotfilesDirs = append([]string(nil), cfg.DotfilesDirs...)
				before := importTree(t, base)
				got, err := PlanImport(cfg, "", relative)
				if err != nil || got.Link.Source != appendPath(cfg.DotfilesDirs[0], strings.TrimPrefix(relative, ".")) {
					t.Fatalf("occupancy changed source selection: %+v, %v", got, err)
				}
				if relative != ".zshrc" {
					want := []Link{
						{Source: filepath.Join(cfg.DotfilesDirs[0], "config"), Destination: filepath.Join(cfg.TargetDir, ".config")},
						{Source: filepath.Join(cfg.DotfilesDirs[0], "config/nvim"), Destination: filepath.Join(cfg.TargetDir, ".config/nvim")},
					}
					if !reflect.DeepEqual(got.Plan.Directories, want) {
						t.Fatalf("incorrect prospective parent evidence: %+v", got.Plan.Directories)
					}
				}
				after := importTree(t, base)
				if len(before) != len(after) || !reflect.DeepEqual(cfg, original) {
					t.Fatal("planning changed filesystem entries or configuration")
				}
				for name, old := range before {
					now, exists := after[name]
					if !exists || !os.SameFile(old.info, now.info) || old.data != now.data ||
						old.info.Mode() != now.info.Mode() || old.info.Size() != now.info.Size() ||
						!old.info.ModTime().Equal(now.info.ModTime()) {
						t.Fatalf("planning modified %q", name)
					}
				}
			})
		}
	}
}

type importTreeEntry struct {
	info os.FileInfo
	data string
}

func importTree(t *testing.T, root string) map[string]importTreeEntry {
	t.Helper()
	result := make(map[string]importTreeEntry)
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var data []byte
		if info.Mode().IsRegular() {
			data, err = os.ReadFile(name)
		} else if info.Mode()&os.ModeSymlink != 0 {
			var text string
			text, err = os.Readlink(name)
			data = []byte(text)
		}
		if err != nil {
			return err
		}
		result[name] = importTreeEntry{info, string(data)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPlanImportPreservesRawTraversal(t *testing.T) {
	base := t.TempDir()
	work, actual := filepath.Join(base, "work"), filepath.Join(base, "actual")
	for _, name := range []string{work, filepath.Join(actual, "child"), filepath.Join(actual, "source")} {
		if err := os.MkdirAll(name, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(actual, "child"), filepath.Join(work, "alias")); err != nil {
		t.Fatal(err)
	}
	cfg := Config{DotfilesDirs: []string{work + "/alias/../source"}, TargetDir: work + "/alias/../target"}
	got, err := PlanImport(cfg, "", ".config/nvim/init.lua")
	if err != nil || got.Link.Source != work+"/alias/../source/config/nvim/init.lua" ||
		got.Link.Destination != work+"/alias/../target/.config/nvim/init.lua" {
		t.Fatalf("raw traversal changed: %+v, %v", got, err)
	}
}

func TestPlanImportSourceFailures(t *testing.T) {
	for _, mode := range []string{"broken source", "opaque parent", "unreadable selected"} {
		t.Run(mode, func(t *testing.T) {
			cfg := importConfig(t, "local", "shared")
			if mode == "unreadable selected" {
				root := cfg.DotfilesDirs[0]
				if err := os.Chmod(root, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(root, 0o700); err != nil {
						t.Error(err)
					}
				})
				if _, err := os.ReadDir(root); err == nil {
					t.Skip("filesystem permits reading the restricted fixture")
				} else if !os.IsPermission(err) {
					t.Fatal(err)
				}
			} else {
				text := "missing"
				if mode == "opaque parent" {
					text = cfg.DotfilesDirs[1]
				}
				if err := os.Symlink(text, filepath.Join(cfg.DotfilesDirs[0], "config")); err != nil {
					t.Fatal(err)
				}
			}
			got, err := PlanImport(cfg, cfg.DotfilesDirs[0], ".config/nvim/init.lua")
			if err == nil {
				t.Fatalf("source failure concealed: %+v", got)
			}
			if mode == "broken source" && (len(got.Plan.Errors) == 0 ||
				len(got.Providers) != 1 || !got.Providers[0].InvalidSource) {
				t.Fatalf("invalid source evidence lost: %+v", got)
			}
		})
	}
}
