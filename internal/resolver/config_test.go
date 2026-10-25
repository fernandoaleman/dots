package resolver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDiscovery(t *testing.T) {
	for _, mode := range []string{"unset", "empty", "relative", "absolute", "explicit"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			cwd := t.TempDir()
			xdg, explicit := "", ""
			name := filepath.Join(home, ".config", "dots", "config.toml")
			switch mode {
			case "relative":
				xdg = "relative"
			case "absolute":
				xdg = t.TempDir()
				name = filepath.Join(xdg, "dots", "config.toml")
			case "explicit":
				explicit = "custom/settings"
				name = filepath.Join(cwd, explicit)
				writeConfig(t, filepath.Join(home, ".config", "dots", "config.toml"), "invalid TOML")
			}
			writeConfig(t, name, "dotfiles_dirs = ['~/local', 'shared']\n")
			cfg, err := Load(explicit, home, xdg, cwd)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.TargetDir != home || !cfg.WarnDuplicates {
				t.Fatalf("unexpected defaults: %+v", cfg)
			}
			if cfg.DotfilesDirs[0] != filepath.Join(home, "local") ||
				cfg.DotfilesDirs[1] != filepath.Join(filepath.Dir(name), "shared") {
				t.Fatalf("unexpected sources: %v", cfg.DotfilesDirs)
			}
		})
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"malformed", "dotfiles_dirs = [", "read configuration"},
		{"missing sources", "", "must not be empty"},
		{"empty sources", "dotfiles_dirs = []", "must not be empty"},
		{"wrong sources type", "dotfiles_dirs = 'source'", "read configuration"},
		{"wrong member type", "dotfiles_dirs = [1]", "read configuration"},
		{"unknown key", "dotfiles_dirs = ['source']\nunknown = true", "unknown keys"},
		{"empty source", "dotfiles_dirs = ['']", "path must not be empty"},
		{"empty target", "dotfiles_dirs = ['source']\ntarget_dir = ''", "path must not be empty"},
		{"wrong warning type", "dotfiles_dirs = ['source']\nwarn_duplicates = 'yes'", "read configuration"},
		{"absolute exclude", "dotfiles_dirs = ['source']\nexcludes = ['/config']", "source-root-relative"},
		{"escaping exclude", "dotfiles_dirs = ['source']\nexcludes = ['../config']", "source-root-relative"},
		{"empty exclude", "dotfiles_dirs = ['source']\nexcludes = ['']", "source-root-relative"},
		{"noncanonical exclude", "dotfiles_dirs = ['source']\nexcludes = ['config/']", "source-root-relative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			name := filepath.Join(root, "settings")
			writeConfig(t, name, tt.body)
			_, err := Load(name, root, "", root)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v; want %q", err, tt.want)
			}
		})
	}
}

func TestLoadExplicitFailureNeverFallsBack(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, filepath.Join(root, ".config", "dots", "config.toml"), "dotfiles_dirs = ['source']")
	if _, err := Load("missing", root, "", root); err == nil {
		t.Fatal("missing explicit configuration fell back")
	}
}

func TestLoadNoLegacyFallback(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".rcrc", "rcrc", ".dots.toml", ".config/dots/config"} {
		writeConfig(t, filepath.Join(root, name), "dotfiles_dirs = ['source']")
	}
	if _, err := Load("", root, "", root); err == nil {
		t.Fatal("legacy configuration was discovered")
	}
}

func TestLoadOptionalFields(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "config.toml")
	writeConfig(t, name, "dotfiles_dirs = ['source']\ntarget_dir = 'target'\nwarn_duplicates = false\nexcludes = ['.git', '*.tmp', 'config/nvim']")
	cfg, err := Load(name, root, "", root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WarnDuplicates || cfg.TargetDir != filepath.Join(root, "target") || len(cfg.Excludes) != 3 {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
}
