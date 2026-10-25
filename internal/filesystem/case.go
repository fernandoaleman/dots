package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type nameRules struct {
	caseSensitive           bool
	normalizationSensitive  bool
	ignoreDefaultIgnorables bool
}

// Use canonical normalization and full folding as a conservative candidate
// filter. Native directory capabilities determine which comparisons apply;
// unrelated prefixes do not require capability queries.
func compareDestinationNames(a, b string, rulesAt func(string) (nameRules, error)) (bool, error) {
	left := strings.Split(strings.TrimPrefix(a, string(filepath.Separator)), string(filepath.Separator))
	right := strings.Split(strings.TrimPrefix(b, string(filepath.Separator)), string(filepath.Separator))
	count := min(len(left), len(right))
	for i := 0; i < count; i++ {
		if left[i] != right[i] && potentialNameKey(left[i]) != potentialNameKey(right[i]) {
			return false, nil
		}
	}
	parent := string(filepath.Separator)
	for i := 0; i < count; i++ {
		if left[i] != right[i] {
			rules, err := rulesAt(parent)
			if err != nil {
				return false, err
			}
			if !equivalentName(left[i], right[i], rules) {
				return false, nil
			}
		}
		parent = filepath.Join(parent, left[i])
	}
	return true, nil
}

func potentialPathPrefixes(name string) []string {
	parts := strings.Split(strings.TrimPrefix(name, string(filepath.Separator)), string(filepath.Separator))
	prefixes := make([]string, len(parts))
	prefix := ""
	for i, part := range parts {
		// Do not filepath.Join folded names: normalization must not turn
		// literal component text into lexical traversal instructions.
		prefix += string(filepath.Separator) + potentialNameKey(part)
		prefixes[i] = prefix
	}
	return prefixes
}

func potentialNameKey(name string) string {
	if !utf8.ValidString(name) {
		// ext4 treats invalid UTF-8 names as opaque byte sequences.
		return name
	}
	return norm.NFD.String(cases.Fold().String(norm.NFD.String(withoutDefaultIgnorables(name))))
}

func equivalentName(a, b string, rules nameRules) bool {
	if a == b {
		return true
	}
	if !utf8.ValidString(a) || !utf8.ValidString(b) {
		return false
	}
	if rules.ignoreDefaultIgnorables {
		a, b = withoutDefaultIgnorables(a), withoutDefaultIgnorables(b)
	}
	if !rules.normalizationSensitive {
		a, b = norm.NFD.String(a), norm.NFD.String(b)
	}
	if !rules.caseSensitive {
		a, b = cases.Fold().String(a), cases.Fold().String(b)
		if !rules.normalizationSensitive {
			a, b = norm.NFD.String(a), norm.NFD.String(b)
		}
	}
	return a == b
}

func withoutDefaultIgnorables(name string) string {
	return strings.Map(func(value rune) rune {
		// Definition of Default_Ignorable_Code_Point in Unicode's UCD.
		// https://www.unicode.org/Public/15.0.0/ucd/DerivedCoreProperties.txt
		exception := unicode.IsSpace(value) || unicode.Is(unicode.Prepended_Concatenation_Mark, value) ||
			value >= 0xfff9 && value <= 0xfffb || value >= 0x13430 && value <= 0x13440
		if !exception && (unicode.Is(unicode.Cf, value) ||
			unicode.Is(unicode.Other_Default_Ignorable_Code_Point, value) || unicode.Is(unicode.Variation_Selector, value)) {
			return -1
		}
		return value
	}, name)
}

func nameRulesAt(name string) (nameRules, error) {
	for {
		info, err := os.Stat(name)
		if err == nil {
			if !info.IsDir() {
				return nameRules{}, fmt.Errorf("%q is not a directory", name)
			}
			return directoryNameRules(name)
		}
		if !os.IsNotExist(err) || filepath.Dir(name) == name {
			return nameRules{}, err
		}
		// New ext4 directories inherit the parent's case-folding flag.
		// https://github.com/torvalds/linux/blob/v6.12/fs/ext4/ext4.h
		name = filepath.Dir(name)
	}
}
