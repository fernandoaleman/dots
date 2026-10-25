package resolver

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Link describes a desired destination independently of its current occupancy.
type Link struct {
	Source      string
	Destination string
	Conflict    string
}

// Plan contains desired links, directory evidence, and source diagnostics.
type Plan struct {
	Links       []Link
	Directories []Link
	Warnings    []string
	Errors      []string
}

type candidate struct {
	source      string
	destination string
	tier        int
	directory   bool
	dotted      bool
	problem     string
}

// Resolve reads source trees without inspecting or modifying destinations.
func Resolve(cfg Config) Plan {
	var plan Plan
	var candidates []candidate
	excluded := make(map[string]bool)
	for _, name := range cfg.Excludes {
		excluded[name] = true
	}
	usable := 0
	for tier, root := range cfg.DotfilesDirs {
		rootEntry := strings.TrimRight(root, string(filepath.Separator))
		if rootEntry == "" {
			rootEntry = string(filepath.Separator)
		}
		_, err := os.Lstat(rootEntry)
		if os.IsNotExist(err) {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("missing source root %q", root))
			continue
		}
		if err != nil {
			plan.Errors = append(plan.Errors, fmt.Sprintf("inspect source root %q: %v", root, err))
			continue
		}
		actual, err := filepath.EvalSymlinks(root)
		if err != nil {
			plan.Errors = append(plan.Errors, fmt.Sprintf("resolve source root %q: %v", root, err))
			continue
		}
		info, err := os.Stat(actual)
		if err != nil || !info.IsDir() {
			plan.Errors = append(plan.Errors, fmt.Sprintf("source root %q must be an accessible directory", root))
			continue
		}
		if _, err := os.ReadDir(actual); err != nil {
			plan.Errors = append(plan.Errors, fmt.Sprintf("read source root %q: %v", root, err))
			continue
		}
		usable++
		err = filepath.WalkDir(actual, func(name string, entry fs.DirEntry, walkErr error) error {
			relative, err := filepath.Rel(actual, name)
			if err != nil {
				return err
			}
			source := appendPath(root, relative)
			if walkErr != nil {
				plan.Errors = append(plan.Errors, fmt.Sprintf("read source %q: %v", source, walkErr))
				return nil
			}
			if relative == "." {
				return nil
			}
			if excluded[filepath.ToSlash(relative)] {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				plan.Errors = append(plan.Errors, fmt.Sprintf("inspect source %q: %v", source, err))
				return nil
			}
			item := candidate{
				source:      source,
				destination: appendPath(cfg.TargetDir, destinationPath(relative)),
				tier:        tier,
				directory:   info.IsDir(),
				dotted:      strings.HasPrefix(relative, "."),
			}
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				if _, err := os.Stat(name); err != nil {
					item.problem = fmt.Sprintf("invalid source symlink %q: %v", source, err)
					plan.Errors = append(plan.Errors, item.problem)
				}
			case !info.IsDir() && !info.Mode().IsRegular():
				item.problem = fmt.Sprintf("unsupported source type %q: %s", source, info.Mode())
				plan.Errors = append(plan.Errors, item.problem)
			}
			candidates = append(candidates, item)
			return nil
		})
		if err != nil {
			plan.Errors = append(plan.Errors, fmt.Sprintf("scan source root %q: %v", root, err))
		}
	}
	if usable == 0 {
		plan.Errors = append(plan.Errors, "no usable source directories")
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.destination != b.destination {
			return a.destination < b.destination
		}
		if a.tier != b.tier {
			return a.tier < b.tier
		}
		if a.dotted != b.dotted {
			return a.dotted
		}
		return a.source < b.source
	})

	// Preserve directory and losing-leaf evidence until structural conflicts
	// have been detected. Precedence only resolves identical leaf paths.
	blocked := structuralCollisions(candidates)
	winners := make(map[string]bool)
	tierWinners := make(map[int]map[string]candidate)
	for _, item := range candidates {
		if item.directory {
			// Empty and overridden directories still constrain the destination
			// namespace. Keep them for filesystem-aware collision inspection.
			plan.Directories = append(plan.Directories, Link{
				Source: item.source, Destination: item.destination,
			})
			continue
		}
		if tierWinners[item.tier] == nil {
			tierWinners[item.tier] = make(map[string]candidate)
		}
		if previous, exists := tierWinners[item.tier][item.destination]; exists {
			if cfg.WarnDuplicates {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf(
					"duplicate destination %q: %q wins over %q",
					item.destination, previous.source, item.source))
			}
		} else {
			tierWinners[item.tier][item.destination] = item
		}
		if winners[item.destination] {
			continue
		}
		winners[item.destination] = true
		reasons := blocked[item.destination]
		if item.problem != "" {
			reasons = append(reasons, item.problem)
		}
		sort.Strings(reasons)
		plan.Links = append(plan.Links, Link{
			Source:      item.source,
			Destination: item.destination,
			Conflict:    strings.Join(reasons, "; "),
		})
	}
	sort.Strings(plan.Warnings)
	sort.Strings(plan.Errors)
	return plan
}

func destinationPath(relative string) string {
	parts := strings.SplitN(relative, string(filepath.Separator), 2)
	if !strings.HasPrefix(parts[0], ".") {
		parts[0] = "." + parts[0]
	}
	return filepath.Join(parts...)
}

func structuralCollisions(candidates []candidate) map[string][]string {
	byLeaf := make(map[string][]int)
	for i, item := range candidates {
		if !item.directory {
			byLeaf[item.destination] = append(byLeaf[item.destination], i)
		}
	}
	blocked := make(map[string][]string)
	block := func(i, j int) {
		a, b := candidates[min(i, j)], candidates[max(i, j)]
		message := fmt.Sprintf("structural collision: %q -> %q and %q -> %q",
			a.source, a.destination, b.source, b.destination)
		if !a.directory {
			blocked[a.destination] = append(blocked[a.destination], message)
		}
		if !b.directory {
			blocked[b.destination] = append(blocked[b.destination], message)
		}
	}
	for i, item := range candidates {
		if item.directory {
			for _, j := range byLeaf[item.destination] {
				block(i, j)
			}
		}
		// Slice literal ancestor paths rather than cleaning configured
		// symlink/.. traversal. A leaf cannot contain any other entry.
		for parent := item.destination; ; {
			separator := strings.LastIndexByte(parent, byte(filepath.Separator))
			if separator < 0 {
				break
			}
			parent = parent[:separator]
			for _, j := range byLeaf[parent] {
				block(i, j)
			}
		}
	}
	return blocked
}
