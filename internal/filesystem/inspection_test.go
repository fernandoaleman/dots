package filesystem

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCollisionResultsDistinguishUncertainty(t *testing.T) {
	for _, mode := range []string{"sensitive", "insensitive", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			destinations := []Destination{
				{Path: filepath.Join(root, "Case")},
				{Path: filepath.Join(root, "case")},
				{Path: filepath.Join(root, "unrelated")},
			}
			unavailable := errors.New("capability unavailable")
			rulesAt := func(string) (nameRules, error) {
				if mode == "unavailable" {
					return nameRules{}, unavailable
				}
				return nameRules{caseSensitive: mode == "sensitive", normalizationSensitive: true}, nil
			}
			before := treeState(t, root)
			results := collisionResults(destinations, rulesAt)
			want := Conflict
			if mode == "sensitive" {
				want = ""
			} else if mode == "unavailable" {
				want = InspectionError
			}
			legacy := collisionReasons(destinations, rulesAt)
			for i, result := range results {
				expected := want
				if i == 2 {
					expected = ""
				}
				if result.State != expected || result.Reason != legacy[i] {
					t.Fatalf("wrong state or changed legacy diagnostic: %+v", results)
				}
			}
			if !reflect.DeepEqual(before, treeState(t, root)) {
				t.Fatal("collision inspection wrote files")
			}
		})
	}
}

func TestEntryComparisonPropagatesMetadataFailures(t *testing.T) {
	root := t.TempDir()
	unavailable := errors.New("name capability unavailable")
	query := func(string) (nameRules, error) { return nameRules{}, unavailable }
	if same, err := compareEntries(filepath.Join(root, "Case"), filepath.Join(root, "case"), query); same || !errors.Is(err, unavailable) {
		t.Fatalf("uncertainty became a wrong-entry conclusion: same=%v err=%v", same, err)
	}
	if same, err := compareEntries(filepath.Join(root, "missing", "a"), filepath.Join(root, "other", "a"), query); same || err == nil {
		t.Fatalf("missing metadata became a wrong-entry conclusion: same=%v err=%v", same, err)
	}
}

func TestCollisionErrorPriorityIsOrderIndependent(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		root := t.TempDir()
		destinations := []Destination{
			{Path: filepath.Join(root, "Case")},
			{Path: filepath.Join(root, "Case", "child")},
			{Path: filepath.Join(root, "case")},
			{Path: filepath.Join(root, "unrelated")},
		}
		if reverse {
			for i, j := 0, len(destinations)-1; i < j; i, j = i+1, j-1 {
				destinations[i], destinations[j] = destinations[j], destinations[i]
			}
		}
		before := treeState(t, root)
		results := collisionResults(destinations, func(string) (nameRules, error) {
			return nameRules{}, errors.New("capability unavailable")
		})
		for i, result := range results {
			path := destinations[i].Path
			if path == filepath.Join(root, "unrelated") {
				if result.State != "" || result.Reason != "" {
					t.Fatal("unrelated entry affected:", result)
				}
				continue
			}
			if result.State != InspectionError || !strings.Contains(result.Reason, "cannot determine target case sensitivity") {
				t.Fatal("confirmed collision hid uncertainty:", result)
			}
			if path != filepath.Join(root, "case") && !strings.Contains(result.Reason, "aliased destination collision") {
				t.Fatal("confirmed collision explanation lost:", result)
			}
		}
		if !reflect.DeepEqual(before, treeState(t, root)) {
			t.Fatal("combined collision inspection wrote files")
		}
	}
}
