package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fernandoaleman/dots/internal/filesystem"
	"github.com/fernandoaleman/dots/internal/resolver"
)

func addCommand(args []string, config string, configured bool, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("add", flag.ContinueOnError)
	f.SetOutput(stderr)
	var root string
	var dry bool
	f.StringVar(&config, "config", config, "select a TOML configuration file")
	f.StringVar(&root, "root", "", "select a configured source root; required with multiple roots")
	f.BoolVar(&dry, "dry-run", false, "preview relocation and link without writes or locking")
	f.BoolVar(&dry, "n", false, "alias for --dry-run")
	f.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dots [--config FILE] add [--config FILE] [--root DIR] [--dry-run|-n] PATH")
		fmt.Fprintln(stderr, "Flags precede PATH; use -- before a filename beginning with -. Imports one existing regular file, without replacement or adoption.")
		f.PrintDefaults()
	}
	if err := validateAddFlags(args); err != nil {
		return fail(stderr, 2, err)
	}
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	invalid := false
	f.Visit(func(v *flag.Flag) {
		invalid = invalid || v.Name == "root" && root == "" || v.Name == "config" && config == "" || (v.Name == "n" || v.Name == "dry-run") && v.Value.String() != "true"
	})
	if invalid || configured && config == "" || len(f.Args()) != 1 || f.Arg(0) == "" {
		return fail(stderr, 2, fmt.Errorf("add requires one nonempty PATH, nonempty supplied --config/--root, and no explicit false dry-run flags"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fail(stderr, 1, err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(stderr, 1, err)
	}
	return runAdd(config, root, f.Arg(0), home, cwd, dry, stdout, stderr)
}

// Validate both the global prefix and add flags so a missing string value cannot
// consume a preview flag. Explicit = permits intentional flag-like pathnames.
func validateAddFlags(args []string) error {
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") {
			break
		}
		key, value, assigned := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if key == "n" {
			key = "dry-run"
		}
		if key != "config" && key != "root" && key != "dry-run" {
			continue // Let flag parsing report unknown flags and help.
		}
		if seen[key] || key == "dry-run" && assigned && value != "true" {
			return fmt.Errorf("duplicate --%s or explicit false dry-run flag; preview cannot be overridden", key)
		}
		seen[key] = true
		if key != "dry-run" && !assigned {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return fmt.Errorf("--%s requires a path value; use --%s=VALUE for a pathname beginning with -", key, key)
			}
			i++
		}
	}
	return nil
}

func runAdd(config, root, input, home, cwd string, dry bool, stdout, stderr io.Writer) int {
	var selected resolver.ImportPlan
	refresh := func() (filesystem.ImportRequest, error) {
		cfg, err := resolver.Load(config, home, os.Getenv("XDG_CONFIG_HOME"), cwd)
		if err != nil {
			return filesystem.ImportRequest{}, err
		}
		selected, err = resolver.PlanImportPath(cfg, root, input, home, cwd)
		if err != nil {
			return filesystem.ImportRequest{}, err
		}
		request := filesystem.ImportRequest{Input: selected.Link.Destination, Source: selected.Link.Source, Root: selected.Root,
			Ownership: filesystem.Ownership{Registry: filesystem.StatePath(home, os.Getenv("XDG_STATE_HOME")), Profile: cfg.Path, Target: cfg.TargetDir, Sources: cfg.DotfilesDirs}}
		for _, provider := range selected.Providers {
			request.Entries = append(request.Entries, filesystem.Destination{Path: provider.Destination, Source: provider.Source})
		}
		request.Entries = append(request.Entries, filesystem.Destination{Path: request.Input, Source: request.Source})
		for _, dir := range selected.Plan.Directories {
			request.Entries = append(request.Entries, filesystem.Destination{Path: dir.Destination, Source: dir.Source, Directory: true})
		}
		return request, nil
	}
	request, operationErr := refresh()
	if operationErr != nil {
		operationErr = fmt.Errorf("%w; if an earlier import was interrupted, preserve its original and source paths and inspect with dots recover before retrying", operationErr)
	}
	var out strings.Builder
	if operationErr == nil {
		fmt.Fprintf(&out, "ROOT %q\nRELOCATE %q -> %q\nSYMLINK %q -> %q\n", request.Root, request.Input, request.Source, request.Input, request.Source)
		if dry {
			inspection, err := filesystem.InspectImport(request)
			operationErr = err
			for _, parent := range inspection.MissingParents {
				fmt.Fprintf(&out, "PARENT %q would be created\n", parent)
			}
			fmt.Fprintln(&out, "PREVIEW 0 changes made; observations grant no authority; execution repeats resolution and checks under the exclusive lock")
		} else {
			report, err := filesystem.RunImport(request, refresh)
			operationErr = err
			status := "IMPORTED"
			if err != nil {
				status = "STOPPED"
			}
			fmt.Fprintf(&out, "%s confirmed-relocated=%t confirmed-linked=%t confirmed-owned=%t\n", status, report.Relocated, report.Linked, report.Owned)
			if err != nil {
				fmt.Fprintf(&out, "Preserve original %q, source %q and registry evidence; inspect with dots recover; no automatic retry or rollback\n", request.Input, request.Source)
			}
		}
	}
	for _, warning := range selected.Plan.Warnings {
		fmt.Fprintf(&out, "WARN %s\n", warning)
	}
	if err := recoveryOutput(stdout, out.String()); err != nil {
		if !dry {
			err = fmt.Errorf("write import result: %w; operation may have completed; inspect paths and dots recover before retrying", err)
		}
		operationErr = errors.Join(operationErr, err)
	}
	if operationErr != nil {
		return fail(stderr, 1, operationErr)
	}
	return 0
}
