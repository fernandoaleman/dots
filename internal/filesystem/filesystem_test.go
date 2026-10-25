package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureFile(t *testing.T, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureLink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
}

func TestInspectDestinations(t *testing.T) {
	tests := []struct {
		mode   string
		status string
		reason string
	}{
		{"missing", "LINK", ""},
		{"correct", "OK", ""},
		{"relative correct", "OK", ""},
		{"wrong", "CONFLICT", "another source"},
		{"broken", "CONFLICT", "broken destination symlink"},
		{"file", "CONFLICT", "regular file"},
		{"directory", "CONFLICT", "existing directory"},
		{"self", "CONFLICT", "same filesystem entry"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source", "gitconfig")
			destination := filepath.Join(root, "target", ".gitconfig")
			fixtureFile(t, source)
			if err := os.Mkdir(filepath.Dir(destination), 0o700); err != nil {
				t.Fatal(err)
			}
			switch tt.mode {
			case "correct":
				fixtureLink(t, source, destination)
			case "relative correct":
				fixtureLink(t, "../source/gitconfig", destination)
			case "wrong":
				other := filepath.Join(root, "other")
				fixtureFile(t, other)
				fixtureLink(t, other, destination)
			case "broken":
				fixtureLink(t, "missing", destination)
			case "file":
				fixtureFile(t, destination)
			case "directory":
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatal(err)
				}
			case "self":
				destination = source
			}
			got := Inspect(source, destination)
			if got.Status != tt.status || !strings.Contains(got.Reason, tt.reason) {
				t.Fatalf("result = %+v; want %s containing %q", got, tt.status, tt.reason)
			}
		})
	}
}

func TestInspectAncestors(t *testing.T) {
	tests := []struct {
		mode   string
		status string
		reason string
	}{
		{"missing", "LINK", ""},
		{"directory", "LINK", ""},
		{"directory symlink", "LINK", ""},
		{"file", "CONFLICT", "not a directory"},
		{"broken", "CONFLICT", "broken ancestor symlink"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			parent := filepath.Join(root, "target")
			destination := filepath.Join(parent, ".config", "nvim", "init.lua")
			fixtureFile(t, source)
			switch tt.mode {
			case "directory":
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatal(err)
				}
			case "directory symlink":
				actual := filepath.Join(root, "actual")
				if err := os.Mkdir(actual, 0o700); err != nil {
					t.Fatal(err)
				}
				fixtureLink(t, actual, parent)
			case "file":
				fixtureFile(t, parent)
			case "broken":
				fixtureLink(t, "missing", parent)
			}
			got := Inspect(source, destination)
			if got.Status != tt.status || !strings.Contains(got.Reason, tt.reason) {
				t.Fatalf("result = %+v; want %s containing %q", got, tt.status, tt.reason)
			}
			if _, err := os.Lstat(filepath.Join(parent, ".config")); err == nil {
				t.Fatal("inspection created a destination parent")
			}
		})
	}
}

func TestInspectPreservesSymlinkParentTraversal(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "work", "source", "gitconfig")
	other := filepath.Join(root, "elsewhere", "source", "gitconfig")
	destination := filepath.Join(root, "work", "home", ".gitconfig")
	fixtureFile(t, source)
	fixtureFile(t, other)
	if err := os.WriteFile(other, []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(root, "elsewhere", "subdir")
	for _, dir := range []string{actual, filepath.Dir(destination)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	fixtureLink(t, actual, filepath.Join(root, "work", "alias"))
	fixtureLink(t, "../alias/../source/gitconfig", destination)
	contents, err := os.ReadFile(destination)
	if err != nil || string(contents) != "different" {
		t.Fatalf("fixture did not reach the alternate file: %q, %v", contents, err)
	}
	if got := Inspect(source, destination); got.Status != "CONFLICT" {
		t.Fatalf("wrong symlink reported as correct: %+v", got)
	}
}

func TestDestinationAliasCollisions(t *testing.T) {
	for _, sameLeaf := range []bool{false, true} {
		t.Run(map[bool]string{false: "ancestor collision", true: "same leaf"}[sameLeaf], func(t *testing.T) {
			root := t.TempDir()
			actual := filepath.Join(root, ".other")
			if err := os.Mkdir(actual, 0o700); err != nil {
				t.Fatal(err)
			}
			fixtureLink(t, ".other", filepath.Join(root, ".config"))
			second := filepath.Join(actual, "nvim", "init.lua")
			if sameLeaf {
				second = filepath.Join(actual, "nvim")
			}
			destinations := []Destination{
				{Path: filepath.Join(root, ".config", "nvim")},
				{Path: second},
				{Path: filepath.Join(root, ".zshrc")},
			}
			reasons := CollisionReasons(destinations)
			if reasons[0] == "" || reasons[1] == "" || reasons[2] != "" {
				t.Fatalf("alias collision classifications: %v", reasons)
			}
		})
	}
}

func BenchmarkDestinationCollisionIndex(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("entries=%d", size), func(b *testing.B) {
			root := b.TempDir()
			destinations := make([]Destination, size)
			for i := range destinations {
				destinations[i].Path = filepath.Join(root, fmt.Sprintf(".entry-%06d", i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, reason := range CollisionReasons(destinations) {
					if reason != "" {
						b.Fatal(reason)
					}
				}
			}
		})
	}
}
