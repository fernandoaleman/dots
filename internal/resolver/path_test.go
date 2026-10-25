package resolver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPreservesFilesystemTraversal(t *testing.T) {
	for _, mode := range []string{"explicit", "source and target", "tilde", "xdg"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			work := filepath.Join(root, "work")
			actual := filepath.Join(root, "actual")
			for _, dir := range []string{work, filepath.Join(actual, "subdir")} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(filepath.Join(actual, "subdir"), filepath.Join(work, "alias")); err != nil {
				t.Fatal(err)
			}
			writeConfig(t, filepath.Join(actual, "source", "gitconfig"), "chosen")
			writeConfig(t, filepath.Join(work, "config.toml"), "dotfiles_dirs = ['wrong']")
			explicit := work + "/alias/../config.toml"
			config := filepath.Join(actual, "config.toml")
			body := "dotfiles_dirs = ['source']\ntarget_dir = 'target'"
			xdg := ""
			switch mode {
			case "source and target":
				config = filepath.Join(work, "config.toml")
				explicit = config
				body = "dotfiles_dirs = ['alias/../source']\ntarget_dir = 'alias/../target'"
			case "tilde":
				config = filepath.Join(work, "config.toml")
				explicit = config
				body = "dotfiles_dirs = ['~/alias/../source']\ntarget_dir = '~/alias/../target'"
			case "xdg":
				config = filepath.Join(actual, "dots", "config.toml")
				explicit = ""
				xdg = work + "/alias/.."
				body = "dotfiles_dirs = ['../source']\ntarget_dir = '../target'"
			}
			writeConfig(t, config, body)
			cfg, err := Load(explicit, work, xdg, work)
			if err != nil {
				t.Fatal(err)
			}
			plan := Resolve(cfg)
			if len(plan.Errors) != 0 || len(plan.Links) != 1 {
				t.Fatalf("wrong source traversal: %+v", plan)
			}
			contents, err := os.ReadFile(plan.Links[0].Source)
			if err != nil || string(contents) != "chosen" {
				t.Fatalf("wrong selected source: %q, %v", contents, err)
			}
			if !strings.Contains(plan.Links[0].Destination, "alias/..") {
				t.Fatalf("target traversal was collapsed: %s", plan.Links[0].Destination)
			}
		})
	}
}

func TestExplicitInvalidConfigNeverFallsBack(t *testing.T) {
	for _, mode := range []string{"malformed", "unreadable"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, filepath.Join(root, ".config", "dots", "config.toml"), "dotfiles_dirs = ['source']")
			explicit := filepath.Join(root, "selected.toml")
			body := "dotfiles_dirs = ["
			if mode == "unreadable" {
				body = "dotfiles_dirs = ['source']"
			}
			writeConfig(t, explicit, body)
			if mode == "unreadable" {
				if err := os.Chmod(explicit, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(explicit, 0o600); err != nil {
						t.Error(err)
					}
				})
				if _, err := os.ReadFile(explicit); err == nil {
					t.Skip("filesystem permits reading the restricted fixture")
				} else if !os.IsPermission(err) {
					t.Fatal(err)
				}
			}
			if _, err := Load(explicit, root, "", root); err == nil || !strings.Contains(err.Error(), explicit) {
				t.Fatalf("invalid explicit config fell back: %v", err)
			}
		})
	}
}
