// Package cli orchestrates dotfile planning, installation, and user-facing output.
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

// Run returns zero for successful plans, one for runtime errors/conflicts,
// and two for invalid command usage.
func Run(args []string, stdout, stderr io.Writer) int {
	var config string
	root := flag.NewFlagSet("dots", flag.ContinueOnError)
	root.SetOutput(stderr)
	root.StringVar(&config, "config", "", "select a TOML configuration file")
	root.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dots [--config PATH] up [--dry-run]")
		root.PrintDefaults()
	}
	if err := root.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	rest := root.Args()
	if len(rest) == 1 && rest[0] == "help" {
		root.Usage()
		return 0
	}
	if len(rest) == 0 {
		root.Usage()
		return 2
	}
	if rest[0] != "up" {
		return fail(stderr, 2, fmt.Errorf("unsupported command %q; use up", rest[0]))
	}
	up := flag.NewFlagSet("up", flag.ContinueOnError)
	up.SetOutput(stderr)
	up.StringVar(&config, "config", config, "select a TOML configuration file")
	var dryRun bool
	up.BoolVar(&dryRun, "dry-run", false, "print a plan without filesystem changes")
	up.BoolVar(&dryRun, "n", false, "alias for --dry-run")
	if err := up.Parse(rest[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(up.Args()) != 0 {
		return fail(stderr, 2, fmt.Errorf("up accepts no positional arguments"))
	}
	configSupplied := false
	for _, flags := range []*flag.FlagSet{root, up} {
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "config" {
				configSupplied = true
			}
		})
	}
	if configSupplied && config == "" {
		return fail(stderr, 2, fmt.Errorf("--config path must not be empty"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fail(stderr, 1, err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(stderr, 1, err)
	}
	cfg, err := resolver.Load(config, home, os.Getenv("XDG_CONFIG_HOME"), cwd)
	if err != nil {
		return fail(stderr, 1, err)
	}
	plan := resolver.Resolve(cfg)
	results := make([]filesystem.Result, len(plan.Links))
	destinations := make([]filesystem.Destination, 0, len(plan.Links)+len(plan.Directories))
	for i, link := range plan.Links {
		destinations = append(destinations, filesystem.Destination{
			Path: link.Destination, Source: link.Source,
		})
		results[i] = filesystem.Result{Status: "CONFLICT", Reason: link.Conflict}
		if link.Conflict == "" {
			results[i] = filesystem.Inspect(link.Source, link.Destination)
		}
	}
	for _, directory := range plan.Directories {
		destinations = append(destinations, filesystem.Destination{
			Path: directory.Destination, Source: directory.Source, Directory: true,
		})
	}
	for i, reason := range filesystem.CollisionReasons(destinations)[:len(results)] {
		if reason != "" {
			if results[i].Reason != "" {
				reason = results[i].Reason + "; " + reason
			}
			results[i] = filesystem.Result{Status: "CONFLICT", Reason: reason}
		}
	}
	if !dryRun {
		return install(plan, destinations, results, stdout, stderr)
	}
	var output strings.Builder
	planned, correct, conflicts := 0, 0, 0
	for i, link := range plan.Links {
		result := results[i]
		fmt.Fprintf(&output, "%s %q -> %q", result.Status, link.Destination, link.Source)
		if result.Reason != "" {
			fmt.Fprintf(&output, ": %s", result.Reason)
		}
		output.WriteByte('\n')
		switch result.Status {
		case "LINK":
			planned++
		case "OK":
			correct++
		default:
			conflicts++
		}
	}
	for _, warning := range plan.Warnings {
		fmt.Fprintf(&output, "WARN %s\n", warning)
	}
	for _, message := range plan.Errors {
		fmt.Fprintf(&output, "ERROR %s\n", message)
	}
	fmt.Fprintf(&output, "SUMMARY %d planned, %d correct, %d conflicts, %d warnings, %d errors, 0 changes made\n",
		planned, correct, conflicts, len(plan.Warnings), len(plan.Errors))
	if _, err := io.WriteString(stdout, output.String()); err != nil {
		return fail(stderr, 1, fmt.Errorf("write output: %w", err))
	}
	if conflicts != 0 || len(plan.Errors) != 0 {
		return 1
	}
	return 0
}

func fail(stderr io.Writer, code int, err error) int {
	fmt.Fprintf(stderr, "ERROR %v\n", err)
	return code
}
