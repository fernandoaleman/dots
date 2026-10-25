package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUnreadableSources(t *testing.T) {
	for _, mode := range []string{"root", "nested", "excluded nested"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			local := filepath.Join(root, "local")
			shared := filepath.Join(root, "shared")
			locked := local
			if mode != "root" {
				locked = filepath.Join(local, "config", "locked")
			}
			for _, dir := range []string{locked, shared} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(locked, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(locked, 0o700); err != nil {
					t.Error(err)
				}
			})
			if _, err := os.ReadDir(locked); err == nil {
				t.Skip("filesystem permits reading the restricted fixture")
			} else if !os.IsPermission(err) {
				t.Fatal(err)
			}
			cfg := Config{DotfilesDirs: []string{local, shared}, TargetDir: filepath.Join(root, "target")}
			if mode == "excluded nested" {
				cfg.Excludes = []string{"config/locked"}
			}
			plan := Resolve(cfg)
			if (len(plan.Errors) == 0) != (mode == "excluded nested") {
				t.Fatalf("unexpected permission handling: %+v", plan)
			}
		})
	}
}

func TestResolveRetainsLosingCollisionEvidence(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"local/gitconfig", "shared/gitconfig", "shared/.gitconfig/child",
	} {
		writeConfig(t, filepath.Join(root, name), name)
	}
	plan := Resolve(Config{
		DotfilesDirs: []string{filepath.Join(root, "local"), filepath.Join(root, "shared")},
		TargetDir:    filepath.Join(root, "target"),
	})
	if len(plan.Links) != 2 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	for _, link := range plan.Links {
		if !strings.Contains(link.Conflict, filepath.Join(root, "shared", "gitconfig")) {
			t.Fatalf("losing structural participant was discarded: %+v", link)
		}
	}
}

func TestResolveIndependentOfCreationOrder(t *testing.T) {
	files := []string{
		"local/gitconfig", "local/.gitconfig", "local/config",
		"shared/.config/nvim/init.lua", "shared/zshrc",
	}
	var expected string
	for _, reverse := range []bool{false, true} {
		root := t.TempDir()
		for i := range files {
			index := i
			if reverse {
				index = len(files) - 1 - i
			}
			writeConfig(t, filepath.Join(root, files[index]), files[index])
		}
		plan := Resolve(Config{
			DotfilesDirs: []string{filepath.Join(root, "local"), filepath.Join(root, "missing"), filepath.Join(root, "shared")},
			TargetDir:    filepath.Join(root, "target"), WarnDuplicates: true,
		})
		got := strings.ReplaceAll(fmt.Sprintf("%#v", plan), root, "<root>")
		if !reverse {
			expected = got
		} else if got != expected {
			t.Fatalf("creation order changed plan:\n%s\n%s", expected, got)
		}
	}
}

func TestResolveRetainsEmptyDirectoryEvidence(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, filepath.Join(root, "local", "config"), "file")
	for _, path := range []string{"shared/.CONFIG/empty", "shared/.excluded"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	plan := Resolve(Config{
		DotfilesDirs: []string{filepath.Join(root, "local"), filepath.Join(root, "shared")},
		TargetDir:    filepath.Join(root, "target"), Excludes: []string{".excluded"},
	})
	if len(plan.Links) != 1 || len(plan.Directories) != 2 || len(plan.Errors) != 0 {
		t.Fatalf("directory evidence lost or emitted as links: %+v", plan)
	}
	for i, path := range []string{".CONFIG", ".CONFIG/empty"} {
		directory := plan.Directories[i]
		if directory.Source != filepath.Join(root, "shared", path) || directory.Destination != filepath.Join(root, "target", path) {
			t.Fatalf("incorrect directory evidence: %+v", plan.Directories)
		}
	}
}

func BenchmarkStructuralCollisionIndex(b *testing.B) {
	for _, size := range []int{100, 1000, 10000, 100000} {
		b.Run(fmt.Sprintf("entries=%d", size), func(b *testing.B) {
			candidates := make([]candidate, size+1)
			candidates[0] = candidate{source: "/source/config", destination: "/target/.config", directory: true}
			for i := 1; i <= size; i++ {
				candidates[i] = candidate{
					source:      fmt.Sprintf("/source/config/file-%06d", i),
					destination: fmt.Sprintf("/target/.config/file-%06d", i),
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if blocked := structuralCollisions(candidates); len(blocked) != 0 {
					b.Fatal("unrelated destinations were blocked")
				}
			}
		})
	}
}
