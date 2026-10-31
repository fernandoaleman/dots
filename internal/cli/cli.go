// Package cli orchestrates dotfile planning, inspection, installation, and output.
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

// Run returns zero for successful commands, one for errors or an unsatisfied
// status, and two for invalid usage. Dry-run permits missing destinations.
func Run(args []string, stdout, stderr io.Writer) int {
	var config string
	root := flag.NewFlagSet("dots", flag.ContinueOnError)
	root.SetOutput(stderr)
	root.StringVar(&config, "config", "", "select a TOML configuration file")
	root.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dots [--config PATH] up|down [--dry-run]")
		fmt.Fprintln(stderr, "       dots [--config PATH] ls|status")
		fmt.Fprintln(stderr, "       dots recover [--retire ID | --settle] [--revision N] [--dry-run]")
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
	command := rest[0]
	if command == "recover" {
		configured := false
		root.Visit(func(f *flag.Flag) { configured = configured || f.Name == "config" })
		if configured {
			return fail(stderr, 2, fmt.Errorf("recover does not use --config; recovery uses stored registry evidence"))
		}
		return recoverCommand(rest[1:], stdout, stderr)
	}
	if command != "up" && command != "down" && command != "ls" && command != "status" {
		return fail(stderr, 2, fmt.Errorf("unsupported command %q; use up, down, ls, status, or recover", command))
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&config, "config", config, "select a TOML configuration file")
	var dryRun bool
	if command == "up" || command == "down" {
		flags.BoolVar(&dryRun, "dry-run", false, "print a plan without filesystem changes")
		flags.BoolVar(&dryRun, "n", false, "alias for --dry-run")
	}
	if err := flags.Parse(rest[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(flags.Args()) != 0 {
		return fail(stderr, 2, fmt.Errorf("%s accepts no positional arguments", command))
	}
	configSupplied := false
	for _, set := range []*flag.FlagSet{root, flags} {
		set.Visit(func(f *flag.Flag) {
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
	ownership := filesystem.Ownership{Registry: filesystem.StatePath(home, os.Getenv("XDG_STATE_HOME")), Profile: cfg.Path, Target: cfg.TargetDir, Sources: cfg.DotfilesDirs}
	if command == "down" {
		return remove(ownership, dryRun, stdout, stderr)
	}
	plan := resolver.Resolve(cfg)
	if command == "ls" {
		return list(plan, stdout, stderr)
	}
	destinations, results := inspectPlan(plan)
	if command == "status" {
		return status(plan, results, stdout, stderr)
	}
	if !dryRun {
		return install(plan, destinations, results, ownership, stdout, stderr)
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
