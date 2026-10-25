// Package filesystem inspects destinations and safely installs selected symlinks.
package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// Result classifies a destination for a selected source.
type Result struct {
	Status string
	Reason string
	// State describes read-only inspection independently of legacy action labels.
	State State
}

// State distinguishes observed destination conditions, without claiming ownership.
type State string

// Inspection states retain separate labels for occupied entry types.
const (
	Correct           State = "OK"
	Missing           State = "MISSING"
	WrongLink         State = "WRONG"
	BrokenLink        State = "BROKEN"
	OccupiedFile      State = "FILE"
	OccupiedDirectory State = "DIRECTORY"
	OccupiedOther     State = "SPECIAL"
	BlockedParent     State = "BLOCKED"
	Conflict          State = "CONFLICT"
	InspectionError   State = "ERROR"
)

// Destination describes a selected leaf or a required source directory.
// Source is retained for diagnostics; inspection never changes selection.
type Destination struct {
	Path      string
	Source    string
	Directory bool
}

// Inspect checks ancestors before the destination. Existing ancestor symlinks
// must resolve to directories. Source selection is never changed here.
func Inspect(source, destination string) Result {
	if !filepath.IsAbs(source) || !filepath.IsAbs(destination) {
		return conflict(InspectionError, "source and destination paths must be absolute")
	}
	if _, err := os.Lstat(source); err != nil {
		return conflict(InspectionError, "source unavailable: %v", err)
	}
	var ancestors []string
	for name := parentPath(destination); ; name = parentPath(name) {
		ancestors = append(ancestors, name)
		if parentPath(name) == name {
			break
		}
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		name := ancestors[i]
		info, err := os.Lstat(name)
		if os.IsNotExist(err) {
			for _, part := range strings.Split(destination[len(name):], string(filepath.Separator)) {
				if part == ".." {
					return conflict(BlockedParent, "unresolved parent traversal beneath missing ancestor %q", name)
				}
			}
			return Result{Status: "LINK", State: Missing}
		}
		if err != nil {
			return conflict(InspectionError, "cannot inspect ancestor %q: %v", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			info, err = os.Stat(name)
			if missingReferent(err) {
				return conflict(BlockedParent, "broken ancestor symlink %q", name)
			}
			if err != nil {
				return conflict(InspectionError, "cannot inspect ancestor symlink %q: %v", name, err)
			}
		}
		if !info.IsDir() {
			return conflict(BlockedParent, "ancestor %q is not a directory", name)
		}
	}
	sourceEntry, err := canonicalEntry(source)
	if err != nil {
		return conflict(InspectionError, "cannot resolve source parent: %v", err)
	}
	destinationEntry, err := canonicalEntry(destination)
	if err != nil {
		return conflict(InspectionError, "cannot resolve destination parent: %v", err)
	}
	same, err := compareEntries(sourceEntry, destinationEntry, directoryNameRules)
	if err != nil {
		return conflict(InspectionError, "cannot compare source and destination: %v", err)
	}
	if same {
		return conflict(Conflict, "source and destination are the same filesystem entry")
	}
	info, err := os.Lstat(destination)
	if os.IsNotExist(err) {
		return Result{Status: "LINK", State: Missing}
	}
	if err != nil {
		return conflict(InspectionError, "cannot inspect destination: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if _, err := os.Stat(destination); err != nil {
			if missingReferent(err) {
				return conflict(BrokenLink, "broken destination symlink")
			}
			return conflict(InspectionError, "cannot inspect destination symlink: %v", err)
		}
		target, err := os.Readlink(destination)
		if err != nil {
			return conflict(InspectionError, "cannot read destination symlink: %v", err)
		}
		if !filepath.IsAbs(target) {
			// Preserve symlink/.. traversal order from the stored link text.
			target = parentPath(destination) + string(filepath.Separator) + target
		}
		targetEntry, err := canonicalEntry(target)
		if err != nil {
			return conflict(InspectionError, "cannot resolve destination symlink target: %v", err)
		}
		same, err := compareEntries(targetEntry, sourceEntry, directoryNameRules)
		if err != nil {
			return conflict(InspectionError, "cannot compare destination symlink target: %v", err)
		}
		if same {
			return Result{Status: "OK", State: Correct}
		}
		return conflict(WrongLink, "destination symlink points to another source")
	}
	if info.IsDir() {
		return conflict(OccupiedDirectory, "destination is an existing directory")
	}
	if info.Mode().IsRegular() {
		return conflict(OccupiedFile, "destination is an existing regular file")
	}
	return conflict(OccupiedOther, "destination is an existing special file")
}

// A non-directory component makes a symlink referent unreachable just as a
// missing component does. Permission and resource failures remain uncertain.
func missingReferent(err error) bool {
	return os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR)
}

// Resolve parent aliases while preserving the final entry's own identity.
// In particular, a source symlink remains distinct from its referent.
func canonicalEntry(name string) (string, error) {
	parent, leaf := ".", name
	if i := strings.LastIndexByte(name, byte(filepath.Separator)); i >= 0 {
		parent, leaf = name[:i+1], name[i+1:]
	}
	if leaf == "" || leaf == "." || leaf == ".." {
		return filepath.EvalSymlinks(name)
	}
	parent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, leaf), nil
}

// CollisionReasons checks leaves and required directories after resolving
// ancestor aliases, including paths beneath missing parent directories.
// Directories may merge and contain leaves, but cannot replace a leaf.
// Call alongside Inspect: paths with uninspectable ancestors are classified
// by Inspect rather than assigned a physical collision key here.
func CollisionReasons(destinations []Destination) []string {
	return collisionReasons(destinations, nameRulesAt)
}

func collisionReasons(destinations []Destination, rulesAt func(string) (nameRules, error)) []string {
	results := collisionResults(destinations, rulesAt)
	reasons := make([]string, len(results))
	for i, result := range results {
		reasons[i] = result.Reason
	}
	return reasons
}

// CollisionResults reports confirmed collisions separately from unavailable name
// capabilities. Empty results mean no collision was found; inspect paths too.
func CollisionResults(destinations []Destination) []Result {
	return collisionResults(destinations, nameRulesAt)
}

func collisionResults(destinations []Destination, rulesAt func(string) (nameRules, error)) []Result {
	keys := make([]string, len(destinations))
	prefixes := make([][]string, len(destinations))
	byLeaf := make(map[string][]int)
	messages := make([][]string, len(destinations))
	states := make([]State, len(destinations))
	for i, destination := range destinations {
		if key, err := physicalDestination(destination.Path); err == nil {
			keys[i] = key
			prefixes[i] = potentialPathPrefixes(key)
			if !destination.Directory {
				full := prefixes[i][len(prefixes[i])-1]
				byLeaf[full] = append(byLeaf[full], i)
			}
		}
	}
	// Only equal names and names of ancestors can collide. Index folded
	// leaf components once so unrelated destinations never require pairwise
	// work, and merging directories never need comparisons with each other.
	// Keep raw paths for native capability checks and deterministic diagnostics.
	for i, parts := range prefixes {
		for depth, prefix := range parts {
			for _, j := range byLeaf[prefix] {
				if depth == len(parts)-1 && !destinations[i].Directory && j >= i {
					continue // Visit equal-name pairs once, excluding self.
				}
				leftIndex, rightIndex := min(i, j), max(i, j)
				a, b := keys[leftIndex], keys[rightIndex]
				collides, err := compareDestinationNames(a, b, rulesAt)
				if !collides && err == nil {
					continue
				}
				participants := describeDestination(destinations[leftIndex], a) + " and " + describeDestination(destinations[rightIndex], b)
				message := "aliased destination collision: " + participants
				if err != nil {
					message = fmt.Sprintf("cannot determine target case sensitivity or normalization for %s: %v",
						participants, err)
				} else if a != b &&
					!strings.HasPrefix(a, b+string(filepath.Separator)) &&
					!strings.HasPrefix(b, a+string(filepath.Separator)) {
					message = "case- or normalization-equivalent destination collision: " + participants
				}
				messages[leftIndex] = append(messages[leftIndex], message)
				messages[rightIndex] = append(messages[rightIndex], message)
				for _, index := range []int{leftIndex, rightIndex} {
					if err != nil {
						states[index] = InspectionError
					} else if states[index] != InspectionError {
						states[index] = Conflict
					}
				}
			}
		}
	}
	results := make([]Result, len(destinations))
	for i := range messages {
		sort.Strings(messages[i])
		if len(messages[i]) != 0 {
			results[i] = Result{Status: "CONFLICT", Reason: strings.Join(messages[i], "; "), State: states[i]}
		}
	}
	return results
}

func describeDestination(destination Destination, key string) string {
	description := fmt.Sprintf("%q (%q)", destination.Path, key)
	if destination.Source != "" {
		description += fmt.Sprintf(" from %q", destination.Source)
	}
	if destination.Directory {
		description += " (required directory)"
	}
	return description
}

func physicalDestination(name string) (string, error) {
	parent := parentPath(name)
	var missing []string
	for {
		_, err := os.Lstat(parent)
		if err == nil {
			actual, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return "", err
			}
			info, err := os.Stat(actual)
			if err != nil {
				return "", err
			}
			if !info.IsDir() {
				return "", fmt.Errorf("ancestor %q is not a directory", parent)
			}
			for i := len(missing) - 1; i >= 0; i-- {
				if missing[i] == ".." {
					return "", fmt.Errorf("unresolved parent traversal beneath missing ancestor")
				}
				actual = filepath.Join(actual, missing[i])
			}
			return filepath.Join(actual, filepath.Base(name)), nil
		}
		if !os.IsNotExist(err) || parentPath(parent) == parent {
			return "", err
		}
		missing = append(missing, filepath.Base(parent))
		parent = parentPath(parent)
	}
}

func conflict(state State, format string, args ...any) Result {
	return Result{Status: "CONFLICT", Reason: fmt.Sprintf(format, args...), State: state}
}

// Preserve parent components until the filesystem has resolved symlinks.
func parentPath(name string) string {
	trimmed := strings.TrimRight(name, string(filepath.Separator))
	i := strings.LastIndexByte(trimmed, byte(filepath.Separator))
	if i <= 0 {
		return string(filepath.Separator)
	}
	return trimmed[:i]
}

func sameEntry(a, b string) bool {
	same, err := compareEntries(a, b, directoryNameRules)
	return err == nil && same
}

func compareEntries(a, b string, rulesAt func(string) (nameRules, error)) (bool, error) {
	if a == b {
		return true, nil
	}
	// canonicalEntry has already resolved parent traversal. Directory
	// aliases preserve replacement of a named entry; separate hard-link
	// names do not, even when their current inode is identical.
	leftParent := filepath.Dir(a)
	rightParent := filepath.Dir(b)
	left, leftErr := os.Stat(leftParent)
	right, rightErr := os.Stat(rightParent)
	if leftErr != nil {
		return false, leftErr
	}
	if rightErr != nil {
		return false, rightErr
	}
	if !os.SameFile(left, right) {
		return false, nil
	}
	leftName, rightName := filepath.Base(a), filepath.Base(b)
	if leftName == rightName {
		return true, nil
	}
	if potentialNameKey(leftName) != potentialNameKey(rightName) {
		return false, nil
	}
	rules, err := rulesAt(leftParent)
	if err != nil {
		return false, err
	}
	if !equivalentName(leftName, rightName, rules) {
		return false, nil
	}
	// Unicode tables may be newer than the filesystem's tables. Distinct
	// directory entries must remain distinct even when they are hard links
	// and a conservative comparison considers their names equivalent.
	entries, err := os.ReadDir(leftParent)
	if err != nil {
		return false, err
	}
	matches := 0
	for _, entry := range entries {
		if equivalentName(entry.Name(), leftName, rules) {
			matches++
		}
	}
	if matches != 1 {
		return false, nil
	}
	left, leftErr = os.Lstat(a)
	right, rightErr = os.Lstat(b)
	if leftErr != nil {
		return false, leftErr
	}
	if rightErr != nil {
		return false, rightErr
	}
	return os.SameFile(left, right), nil
}
