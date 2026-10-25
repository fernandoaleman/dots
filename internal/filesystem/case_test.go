package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaseEquivalenceRules(t *testing.T) {
	tests := []struct {
		name      string
		right     string
		sensitive bool
		unknown   bool
		collision bool
		wantError bool
	}{
		{"sensitive", "/target/.config/NVIM/init.lua", true, false, false, false},
		{"insensitive", "/target/.config/NVIM/init.lua", false, false, true, false},
		{"unknown", "/target/.config/NVIM/init.lua", false, true, false, true},
		{"unrelated on unknown", "/target/.config/tmux/init.lua", false, true, false, false},
		{"ordinary prefix", "/target/.config/nvim/init.lua", false, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := func(string) (nameRules, error) {
				if tt.unknown {
					return nameRules{}, errors.New("capability unavailable")
				}
				return nameRules{caseSensitive: tt.sensitive, normalizationSensitive: tt.sensitive}, nil
			}
			got, err := compareDestinationNames("/target/.config/nvim", tt.right, query)
			if got != tt.collision || (err != nil) != tt.wantError {
				t.Fatalf("collision=%v error=%v", got, err)
			}
		})
	}
}

func TestNativeDirectoryCaseSensitivity(t *testing.T) {
	root := t.TempDir()
	fixtureFile(t, filepath.Join(root, "CaseProbe"))
	sensitive, err := directoryCaseSensitive(root)
	if err != nil {
		t.Skipf("native capability query unavailable: %v", err)
	}
	original, err := os.Lstat(filepath.Join(root, "CaseProbe"))
	if err != nil {
		t.Fatal(err)
	}
	variant, err := os.Lstat(filepath.Join(root, "caseprobe"))
	actualSensitive := true
	if err == nil {
		actualSensitive = !os.SameFile(original, variant)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if sensitive != actualSensitive {
		t.Fatalf("native sensitivity=%v; fixture demonstrates %v", sensitive, actualSensitive)
	}
}

func TestCollisionQueryFailureBlocksAmbiguousEntries(t *testing.T) {
	root := t.TempDir()
	destinations := []Destination{
		{Path: filepath.Join(root, ".config", "nvim")},
		{Path: filepath.Join(root, ".config", "NVIM", "init.lua")},
		{Path: filepath.Join(root, ".zshrc")},
	}
	reasons := collisionReasons(destinations, func(string) (nameRules, error) {
		return nameRules{}, errors.New("injected capability failure")
	})
	for _, reason := range reasons[:2] {
		if !strings.Contains(reason, "cannot determine target case sensitivity") ||
			!strings.Contains(reason, "injected capability failure") ||
			!strings.Contains(reason, destinations[0].Path) ||
			!strings.Contains(reason, destinations[1].Path) {
			t.Fatalf("query failure did not block both participants: %v", reasons)
		}
	}
	if reasons[2] != "" {
		t.Fatalf("unrelated entry was blocked: %v", reasons)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("collision inspection modified the fixture: %v, %v", entries, err)
	}
}

func TestUnicodeDestinationEquivalence(t *testing.T) {
	tests := []struct {
		name          string
		left, right   string
		rules         nameRules
		unknown       bool
		wantCollision bool
		wantError     bool
	}{
		{"fold and normalization", ".É", ".e\u0301", nameRules{}, false, true, false},
		{"sensitive bytes", ".é", ".e\u0301", nameRules{caseSensitive: true, normalizationSensitive: true}, false, false, false},
		{"normalization on case-sensitive volume", ".é", ".e\u0301", nameRules{caseSensitive: true}, false, true, false},
		{"case distinction on normalizing volume", ".É", ".e\u0301", nameRules{caseSensitive: true}, false, false, false},
		{"full fold expansion", ".straße", ".STRASSE", nameRules{}, false, true, false},
		{"full fold on sensitive volume", ".straße", ".STRASSE", nameRules{caseSensitive: true, normalizationSensitive: true}, false, false, false},
		{"unknown normalization", ".é", ".e\u0301", nameRules{}, true, false, true},
		{"unrelated Unicode on unknown volume", ".é", ".ñ", nameRules{}, true, false, false},
		{"invalid UTF-8 is opaque", ".\xffA", ".\xffa", nameRules{}, false, false, false},
		{"ext4 ignorable format character", ".filename", ".file\u200bname", nameRules{ignoreDefaultIgnorables: true}, false, true, false},
		{"ext4 variation selector", ".filename", ".file\ufe0fname", nameRules{ignoreDefaultIgnorables: true}, false, true, false},
		{"APFS preserves format characters", ".filename", ".file\u200bname", nameRules{}, false, false, false},
		{"ext4 preserves exceptional format characters", ".filename", ".file\u0600name", nameRules{ignoreDefaultIgnorables: true}, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := func(string) (nameRules, error) {
				if tt.unknown {
					return nameRules{}, errors.New("capability unavailable")
				}
				return tt.rules, nil
			}
			got, err := compareDestinationNames("/target/"+tt.left, "/target/"+tt.right, query)
			if got != tt.wantCollision || (err != nil) != tt.wantError {
				t.Fatalf("collision=%v error=%v; want collision=%v error=%v", got, err, tt.wantCollision, tt.wantError)
			}
		})
	}
}

func TestRequiredDirectoryCollisions(t *testing.T) {
	tests := []struct {
		name          string
		leaf          string
		directory     string
		rules         nameRules
		unknown       bool
		wantCollision bool
	}{
		{"same path", ".config", ".config", nameRules{caseSensitive: true, normalizationSensitive: true}, false, true},
		{"empty case variant", ".config", ".CONFIG", nameRules{}, false, true},
		{"sensitive case variant", ".config", ".CONFIG", nameRules{caseSensitive: true, normalizationSensitive: true}, false, false},
		{"parent of empty directory", ".config", ".CONFIG/empty", nameRules{}, false, true},
		{"directory above leaf", ".config/nvim/init.lua", ".config", nameRules{}, true, false},
		{"unknown capabilities", ".config", ".CONFIG", nameRules{}, true, true},
		{"unrelated on unknown volume", ".config", ".other", nameRules{}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			destinations := []Destination{
				{Path: filepath.Join(root, tt.leaf), Source: "/source/file"},
				{Path: filepath.Join(root, tt.directory), Source: "/source/directory", Directory: true},
				{Path: filepath.Join(root, ".zshrc"), Source: "/source/unrelated"},
			}
			query := func(string) (nameRules, error) {
				if tt.unknown {
					return nameRules{}, errors.New("capability unavailable")
				}
				return tt.rules, nil
			}
			reasons := collisionReasons(destinations, query)
			for _, reason := range reasons[:2] {
				if (reason != "") != tt.wantCollision {
					t.Fatalf("collision=%v; want %v", reasons, tt.wantCollision)
				}
				if tt.wantCollision && (!strings.Contains(reason, "/source/file") || !strings.Contains(reason, "/source/directory")) {
					t.Fatalf("missing source participant: %v", reasons)
				}
			}
			if reasons[2] != "" {
				t.Fatalf("unrelated entry was blocked: %v", reasons)
			}
			// Directory/ directory merging remains valid, including unknown
			// capabilities. It must not manufacture a planned link or conflict.
			destinations[0].Directory = true
			for _, reason := range collisionReasons(destinations[:2], query) {
				if reason != "" {
					t.Fatalf("directory merge was blocked: %s", reason)
				}
			}
		})
	}
}
