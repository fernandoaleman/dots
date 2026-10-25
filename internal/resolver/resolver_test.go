package resolver

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveSelection(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		excludes []string
		reverse  bool
		warnings int
		winners  map[string]string
	}{
		{
			name:    "directory order before dotted preference",
			files:   []string{"local/gitconfig", "shared/.gitconfig", "shared/tmux.conf"},
			winners: map[string]string{".gitconfig": "local/gitconfig", ".tmux.conf": "shared/tmux.conf"},
		},
		{
			name:    "reversed order",
			files:   []string{"local/gitconfig", "shared/.gitconfig"},
			reverse: true,
			winners: map[string]string{".gitconfig": "shared/.gitconfig"},
		},
		{
			name:     "same root dotted preference",
			files:    []string{"local/gitconfig", "local/.gitconfig", "shared/gitconfig"},
			warnings: 1,
			winners:  map[string]string{".gitconfig": "local/.gitconfig"},
		},
		{
			name: "merge nested trees across and within roots",
			files: []string{
				"local/config/nvim/init.lua", "local/.config/nvim/init.lua",
				"local/config/nvim/lua/local.lua", "shared/.config/nvim/init.lua",
				"shared/.config/nvim/lua/shared.lua", "shared/config/git/config",
			},
			warnings: 1,
			winners: map[string]string{
				".config/nvim/init.lua":       "local/.config/nvim/init.lua",
				".config/nvim/lua/local.lua":  "local/config/nvim/lua/local.lua",
				".config/nvim/lua/shared.lua": "shared/.config/nvim/lua/shared.lua",
				".config/git/config":          "shared/config/git/config",
			},
		},
		{
			name: "literal exclusions before selection",
			files: []string{
				"local/.gitconfig", "local/gitconfig", "shared/gitconfig",
				"local/config/nvim/init.lua", "local/.config/nvim/init.lua",
				"local/*.tmp", "local/notes.tmp", "local/nested/README.md",
			},
			excludes: []string{".gitconfig", "gitconfig", "config/nvim", "*.tmp", "README.md"},
			winners: map[string]string{
				".config/nvim/init.lua": "local/.config/nvim/init.lua",
				".notes.tmp":            "local/notes.tmp",
				".nested/README.md":     "local/nested/README.md",
			},
		},
		{
			name:     "excluded local variant permits shared winner",
			files:    []string{"local/gitconfig", "shared/.gitconfig"},
			excludes: []string{"gitconfig"},
			winners:  map[string]string{".gitconfig": "shared/.gitconfig"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"local", "shared"} {
				if err := os.Mkdir(filepath.Join(root, dir), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for i := len(tt.files) - 1; i >= 0; i-- {
				writeConfig(t, filepath.Join(root, tt.files[i]), tt.files[i])
			}
			cfg := Config{
				DotfilesDirs: []string{filepath.Join(root, "local"), filepath.Join(root, "shared")},
				TargetDir:    filepath.Join(root, "target"), Excludes: tt.excludes, WarnDuplicates: true,
			}
			if tt.reverse {
				cfg.DotfilesDirs[0], cfg.DotfilesDirs[1] = cfg.DotfilesDirs[1], cfg.DotfilesDirs[0]
			}
			plan := Resolve(cfg)
			if len(plan.Errors) != 0 || len(plan.Warnings) != tt.warnings {
				t.Fatalf("unexpected diagnostics: %+v", plan)
			}
			got := make(map[string]string)
			for _, link := range plan.Links {
				if link.Conflict != "" {
					t.Fatal(link.Conflict)
				}
				destination, _ := filepath.Rel(cfg.TargetDir, link.Destination)
				source, _ := filepath.Rel(root, link.Source)
				got[filepath.ToSlash(destination)] = filepath.ToSlash(source)
			}
			if !reflect.DeepEqual(got, tt.winners) {
				t.Fatalf("winners = %v; want %v", got, tt.winners)
			}
			if again := Resolve(cfg); !reflect.DeepEqual(plan, again) {
				t.Fatal("resolution changed between identical runs")
			}
			cfg.WarnDuplicates = false
			quiet := Resolve(cfg)
			if !reflect.DeepEqual(plan.Links, quiet.Links) || len(quiet.Warnings) != 0 {
				t.Fatal("disabling duplicate warnings changed selection")
			}
		})
	}
}

func TestResolveMissingRoots(t *testing.T) {
	for _, mode := range []string{"empty fallback", "none usable", "non-directory"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			sources := []string{filepath.Join(root, "missing")}
			switch mode {
			case "empty fallback":
				sources = append(sources, root)
			case "non-directory":
				name := filepath.Join(root, "file")
				writeConfig(t, name, "content")
				sources = []string{name, root}
			}
			plan := Resolve(Config{DotfilesDirs: sources, TargetDir: filepath.Join(root, "target")})
			if (len(plan.Errors) == 0) != (mode == "empty fallback") {
				t.Fatalf("unexpected diagnostics: %+v", plan)
			}
			if mode != "non-directory" && len(plan.Warnings) != 1 {
				t.Fatalf("missing source warning: %+v", plan)
			}
		})
	}
}

func TestResolveStructuralCollisions(t *testing.T) {
	for _, sameRoot := range []bool{false, true} {
		t.Run(map[bool]string{false: "across roots", true: "within root"}[sameRoot], func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, filepath.Join(root, "local/config"), "file")
			other := "shared"
			if sameRoot {
				other = "local"
			}
			writeConfig(t, filepath.Join(root, other, ".config/nvim/init.lua"), "nested")
			writeConfig(t, filepath.Join(root, "local/zshrc"), "unrelated")
			cfg := Config{
				DotfilesDirs: []string{filepath.Join(root, "local"), filepath.Join(root, other)},
				TargetDir:    filepath.Join(root, "target"),
			}
			plan := Resolve(cfg)
			conflicts := 0
			for _, link := range plan.Links {
				if link.Conflict != "" {
					conflicts++
					if !strings.Contains(link.Conflict, "structural collision") {
						t.Fatal(link.Conflict)
					}
				}
			}
			if conflicts != 2 || len(plan.Links) != 3 {
				t.Fatalf("collision lost participants or unrelated entries: %+v", plan)
			}
		})
	}
}

func TestResolveOpaqueSymlinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	external := filepath.Join(root, "external")
	writeConfig(t, filepath.Join(source, "gitconfig"), "source")
	writeConfig(t, filepath.Join(external, "child"), "external")
	if err := os.Symlink(external, filepath.Join(source, "config")); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(source, alias); err != nil {
		t.Fatal(err)
	}
	cfg := Config{DotfilesDirs: []string{alias}, TargetDir: filepath.Join(root, "target")}
	plan := Resolve(cfg)
	if len(plan.Errors) != 0 || len(plan.Links) != 2 {
		t.Fatalf("source symlink was traversed or root alias rejected: %+v", plan)
	}
	if err := os.Symlink("missing", filepath.Join(source, "broken")); err != nil {
		t.Fatal(err)
	}
	if plan := Resolve(cfg); len(plan.Errors) == 0 {
		t.Fatal("dangling source symlink accepted")
	}
	cfg.Excludes = []string{"broken"}
	if plan := Resolve(cfg); len(plan.Errors) != 0 {
		t.Fatalf("excluded dangling link inspected: %+v", plan)
	}
}
