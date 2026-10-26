package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImportPlan describes a prospective regular-file mapping, not permission to
// mutate paths. The source need not exist yet. Filesystem preflight must still
// validate the input, physical names, protected paths, and ownership state.
type ImportPlan struct {
	Root string
	Link Link
	Plan Plan
	// Providers contains every existing nonexcluded leaf, including losers.
	// Excluded entries can still occupy physical paths and require inspection.
	Providers []Link
}

// PlanImport selects a configured root and resolves a prospective regular file.
// relativeTarget must be a literal target-root-relative name, never a cleaned
// raw input locator. Invalid arguments return a zero result. Source diagnostics
// and conflict evidence remain in the result on resolution failure.
// This function only reads source directories; it never inspects the input or
// creates source parents, temporary candidates, locks, or ownership state.
func PlanImport(cfg Config, root, relativeTarget string) (ImportPlan, error) {
	if !filepath.IsAbs(cfg.TargetDir) || strings.ContainsRune(cfg.TargetDir, 0) {
		return ImportPlan{}, fmt.Errorf("import target root must be absolute and contain no NUL")
	}
	relative, err := importSourceName(relativeTarget)
	if err != nil {
		return ImportPlan{}, err
	}
	tier, err := selectImportRoot(cfg.DotfilesDirs, root)
	if err != nil {
		return ImportPlan{}, err
	}
	for _, exclude := range cfg.Excludes {
		if relative == exclude || strings.HasPrefix(relative, exclude+"/") {
			return ImportPlan{}, fmt.Errorf("import source %q is excluded by %q", relative, exclude)
		}
	}

	root = cfg.DotfilesDirs[tier]
	result := ImportPlan{
		Root: root,
		Link: Link{
			Source:      appendPath(root, relative),
			Destination: appendPath(cfg.TargetDir, relativeTarget),
		},
	}
	candidates, diagnostics := scanSources(cfg)
	// Selection sorts diagnostics, so keep the scanner's slices independent
	// for the second pass with the prospective candidate.
	existing := Plan{
		Warnings: append([]string(nil), diagnostics.Warnings...),
		Errors:   append([]string(nil), diagnostics.Errors...),
	}
	result.Plan = resolveCandidates(cfg, candidates, existing)
	for _, item := range candidates {
		if !item.directory {
			result.Providers = append(result.Providers, Link{
				Source: item.source, Destination: item.destination,
				Conflict: item.problem, InvalidSource: item.problem != "",
			})
		}
	}
	if err := importResolutionError(result.Plan); err != nil {
		return result, err
	}
	for _, provider := range result.Providers {
		if provider.Destination == result.Link.Destination {
			return result, fmt.Errorf("import destination %q already has source provider %q",
				result.Link.Destination, provider.Source)
		}
	}

	// Include prospective parent evidence without manufacturing filesystem
	// entries. Existing directories from every root remain available for merging
	// and structural/native collision checks.
	directories := make(map[string]bool)
	for _, item := range candidates {
		if item.directory {
			directories[item.source] = true
		}
	}
	for parent := filepath.Dir(relative); parent != "."; parent = filepath.Dir(parent) {
		source := appendPath(root, parent)
		if !directories[source] {
			candidates = append(candidates, candidate{
				source: source, destination: appendPath(cfg.TargetDir, destinationPath(parent)),
				tier: tier, directory: true, dotted: strings.HasPrefix(parent, "."),
			})
			directories[source] = true
		}
	}
	candidates = append(candidates, candidate{
		source: result.Link.Source, destination: result.Link.Destination,
		tier: tier, dotted: strings.HasPrefix(relative, "."),
	})
	result.Plan = resolveCandidates(cfg, candidates, diagnostics)
	if err := importResolutionError(result.Plan); err != nil {
		return result, err
	}
	for _, link := range result.Plan.Links {
		if link.Destination == result.Link.Destination && link.Source == result.Link.Source {
			return result, nil
		}
	}
	return result, fmt.Errorf("prospective import source was not selected")
}

func importSourceName(relative string) (string, error) {
	valid := func(name string) bool {
		if name == "" || filepath.IsAbs(name) || strings.ContainsRune(name, 0) {
			return false
		}
		for _, part := range strings.Split(name, string(filepath.Separator)) {
			if part == "" || part == "." || part == ".." {
				return false
			}
		}
		return true
	}
	if !valid(relative) {
		return "", fmt.Errorf("import destination %q must be a literal target-root-relative file path", relative)
	}
	undotted := strings.TrimPrefix(relative, ".")
	if valid(undotted) && destinationPath(undotted) == relative {
		return undotted, nil
	}
	if destinationPath(relative) == relative {
		return relative, nil
	}
	return "", fmt.Errorf("import destination %q cannot round-trip through source mapping", relative)
}

func importResolutionError(plan Plan) error {
	if len(plan.Errors) != 0 {
		return fmt.Errorf("resolve import sources: %s", strings.Join(plan.Errors, "; "))
	}
	for _, link := range plan.Links {
		if link.Conflict != "" {
			return fmt.Errorf("resolve import sources: %s", link.Conflict)
		}
	}
	return nil
}

func selectImportRoot(roots []string, selected string) (int, error) {
	if len(roots) == 0 {
		return 0, fmt.Errorf("no configured import source roots")
	}
	if selected == "" {
		if len(roots) != 1 {
			return 0, fmt.Errorf("multiple configured source roots; select one with --root: %q", roots)
		}
		selected = roots[0]
	}
	tier := -1
	for i, name := range roots {
		if !filepath.IsAbs(name) || strings.ContainsRune(name, 0) {
			return 0, fmt.Errorf("import source root %q must be absolute and contain no NUL", name)
		}
		if name == selected {
			tier = i
		}
	}
	if tier < 0 {
		return 0, fmt.Errorf("import root %q is not a configured source root", selected)
	}
	type resolvedRoot struct {
		path string
		info os.FileInfo
	}
	var resolved []resolvedRoot
	for i, name := range roots {
		info, err := os.Stat(name)
		if err != nil || !info.IsDir() {
			if i == tier {
				if err != nil {
					return 0, fmt.Errorf("inspect selected import root %q: %w", name, err)
				}
				return 0, fmt.Errorf("selected import root %q is not a directory", name)
			}
			continue // The shared scanner preserves other root diagnostics.
		}
		actual, err := filepath.EvalSymlinks(name)
		if err != nil {
			return 0, fmt.Errorf("resolve import root %q: %w", name, err)
		}
		for _, previous := range resolved {
			inside, err := importRootContains(previous.info, actual)
			if err != nil {
				return 0, err
			}
			contains, err := importRootContains(info, previous.path)
			if err != nil {
				return 0, err
			}
			if inside || contains {
				return 0, fmt.Errorf("configured import roots overlap or alias: %q and %q", previous.path, name)
			}
		}
		resolved = append(resolved, resolvedRoot{actual, info})
	}
	return tier, nil
}

// Walk only an already resolved existing directory path. Ancestor identities
// catch native spelling aliases and bind-mounted roots that string prefixes miss.
// Mutation preparation must repeat these point-in-time source-root checks.
func importRootContains(root os.FileInfo, resolved string) (bool, error) {
	for name := resolved; ; name = filepath.Dir(name) {
		info, err := os.Stat(name)
		if err != nil {
			return false, fmt.Errorf("inspect import root ancestry %q: %w", name, err)
		}
		if os.SameFile(root, info) {
			return true, nil
		}
		if filepath.Dir(name) == name {
			return false, nil
		}
	}
}
