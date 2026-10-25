package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectSourceEntryIdentity(t *testing.T) {
	for _, mode := range []string{"different directories", "different names", "directory alias"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "local", "gitconfig")
			other := filepath.Join(root, "shared", "gitconfig")
			fixtureFile(t, source)
			expected := "CONFLICT"
			switch mode {
			case "different names":
				other = filepath.Join(root, "local", "alternative")
			case "directory alias":
				fixtureLink(t, filepath.Dir(source), filepath.Join(root, "alias"))
				other = filepath.Join(root, "alias", "gitconfig")
				expected = "OK"
			}
			if mode != "directory alias" {
				if err := os.MkdirAll(filepath.Dir(other), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(source, other); err != nil {
					t.Fatal(err)
				}
			}
			left, leftErr := os.Stat(source)
			right, rightErr := os.Stat(other)
			if leftErr != nil || rightErr != nil || !os.SameFile(left, right) {
				t.Fatalf("fixture does not share an inode: %v, %v", leftErr, rightErr)
			}
			destination := filepath.Join(root, ".gitconfig")
			fixtureLink(t, other, destination)
			if got := Inspect(source, destination); got.Status != expected {
				t.Fatalf("entry identity classified incorrectly: %+v; want %s", got, expected)
			}
		})
	}
}

func TestInspectNormalizedSourceIdentity(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "é")
	alternate := filepath.Join(root, "source", "e\u0301")
	fixtureFile(t, source)
	if _, err := os.Lstat(alternate); os.IsNotExist(err) {
		t.Skip("filesystem keeps normalization variants distinct")
	} else if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, ".example")
	fixtureLink(t, alternate, destination)
	if got := Inspect(source, destination); got.Status != "OK" {
		t.Fatalf("normalization alias was not recognized: %+v", got)
	}
}
