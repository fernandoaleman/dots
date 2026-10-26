// Package resolver loads TOML configuration and deterministically plans
// source links without modifying the filesystem.
package resolver

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config defines ordered sources and their destination root.
type Config struct {
	// Path is the expanded configuration locator, preserving symlink/.. traversal.
	Path           string   `toml:"-"`
	DotfilesDirs   []string `toml:"dotfiles_dirs"`
	TargetDir      string   `toml:"target_dir"`
	Excludes       []string `toml:"excludes"`
	WarnDuplicates bool     `toml:"warn_duplicates"`
}

// Load discovers and validates configuration without changing the filesystem.
// Explicit configuration paths are relative to cwd; configured paths are
// relative to the configuration file's directory.
func Load(explicit, home, xdg, cwd string) (Config, error) {
	cfg := Config{TargetDir: "~", WarnDuplicates: true}
	if !filepath.IsAbs(home) || !filepath.IsAbs(cwd) {
		return cfg, fmt.Errorf("home and working directory must be absolute")
	}
	name := explicit
	if name == "" {
		if !filepath.IsAbs(xdg) {
			xdg = appendPath(home, ".config")
		}
		name = appendPath(xdg, "dots/config.toml")
	}
	name, err := expandPath(name, home, cwd)
	if err != nil {
		return cfg, err
	}
	meta, err := toml.DecodeFile(name, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("read configuration %q: %w; create a TOML file containing dotfiles_dirs = [\"~/dotfiles\"]", name, err)
	}
	if keys := meta.Undecoded(); len(keys) != 0 {
		names := make([]string, len(keys))
		for i, key := range keys {
			names[i] = key.String()
		}
		sort.Strings(names)
		return cfg, fmt.Errorf("configuration %q: unknown keys: %s", name, strings.Join(names, ", "))
	}
	cfg.Path = name
	if len(cfg.DotfilesDirs) == 0 {
		return cfg, fmt.Errorf("configuration %q: dotfiles_dirs must not be empty", name)
	}
	base := directoryPath(name)
	for i, source := range cfg.DotfilesDirs {
		cfg.DotfilesDirs[i], err = expandPath(source, home, base)
		if err != nil {
			return cfg, fmt.Errorf("dotfiles_dirs[%d]: %w", i, err)
		}
	}
	cfg.TargetDir, err = expandPath(cfg.TargetDir, home, base)
	if err != nil {
		return cfg, fmt.Errorf("target_dir: %w", err)
	}
	for _, exclude := range cfg.Excludes {
		if exclude == "." || path.IsAbs(exclude) ||
			path.Clean(exclude) != exclude ||
			exclude == ".." || strings.HasPrefix(exclude, "../") {
			return cfg, fmt.Errorf("exclude %q must be a canonical source-root-relative path", exclude)
		}
	}
	return cfg, nil
}

func expandPath(value, home, base string) (string, error) {
	switch {
	case value == "":
		return "", fmt.Errorf("path must not be empty")
	case value == "~":
		value = home
	case strings.HasPrefix(value, "~/"):
		value = appendPath(home, value[2:])
	case strings.HasPrefix(value, "~"):
		return "", fmt.Errorf("unsupported home expansion in %q; use ~ or ~/", value)
	}
	if !filepath.IsAbs(value) {
		value = appendPath(base, value)
	}
	return value, nil
}

// Joining or cleaning configured paths can change symlink/.. semantics.
func appendPath(base, relative string) string {
	return strings.TrimRight(base, string(filepath.Separator)) +
		string(filepath.Separator) + relative
}

func directoryPath(name string) string {
	i := strings.LastIndexByte(name, byte(filepath.Separator))
	if i <= 0 {
		return string(filepath.Separator)
	}
	return name[:i]
}
