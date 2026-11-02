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
	return RunWithLicense(args, stdout, stderr, "")
}

// RunWithLicense supplies the canonical license embedded by main. It shares all
// parsing and execution with Run, without filesystem access or mutable globals.
func RunWithLicense(args []string, stdout, stderr io.Writer, license string) int {
	config := &configSelection{}
	root := flag.NewFlagSet("dots", flag.ContinueOnError)
	root.SetOutput(stderr)
	config.register(root)
	var printVersion, printHelp bool
	root.BoolVar(&printVersion, "version", false, "show version information")
	enableHelp := func(value string) error {
		if printHelp || value != "true" {
			return fmt.Errorf("help flags cannot be repeated or set to false")
		}
		printHelp = true
		return nil
	}
	root.BoolFunc("h", "show help", enableHelp)
	root.BoolFunc("help", "show help", enableHelp)
	usageErr := setUsage(root, "", stderr)
	if err := validateConfigArgs(args, false); err != nil {
		return fail(stderr, 2, err)
	}
	if err := root.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return parsedHelp(*usageErr, stderr)
		}
		return 2
	}
	rest := root.Args()
	if printHelp {
		if len(rest) != 0 {
			command := rest[0]
			if _, err := helpText(command); err != nil {
				return fail(stderr, 2, err)
			}
			var err error
			if command == "version" || command == "license" || command == "recover" {
				err = rejectConfigArgs(command, rest[1:], config.supplied)
			} else {
				err = validateConfigArgs(rest[1:], config.supplied)
			}
			if err != nil {
				return fail(stderr, 2, err)
			}
		}
		return parsedHelp(showHelp("", stderr), stderr)
	}
	versionSupplied := false
	root.Visit(func(f *flag.Flag) { versionSupplied = versionSupplied || f.Name == "version" })
	if versionSupplied {
		if !printVersion || len(rest) != 0 || config.supplied {
			return fail(stderr, 2, fmt.Errorf("--version accepts no command or configuration flags"))
		}
		if err := recoveryOutput(stdout, versionText(version, commit)); err != nil {
			return fail(stderr, 1, fmt.Errorf("write version: %w", err))
		}
		return 0
	}
	if len(rest) == 0 {
		root.Usage()
		return 2
	}
	command := rest[0]
	if command == "help" {
		if len(rest) > 2 {
			return fail(stderr, 2, fmt.Errorf("help accepts at most one command"))
		}
		topic := ""
		if len(rest) == 2 {
			topic = rest[1]
		}
		if topic == "-h" || topic == "--help" {
			topic = "help"
		}
		if _, err := helpText(topic); err != nil {
			return fail(stderr, 2, err)
		}
		return parsedHelp(showHelp(topic, stderr), stderr)
	}
	if command == "version" || command == "license" {
		if err := rejectConfigArgs(command, rest[1:], config.supplied); err != nil {
			return fail(stderr, 2, err)
		}
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(stderr)
		helpErr := setUsage(flags, command, stderr)
		if err := flags.Parse(rest[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return parsedHelp(*helpErr, stderr)
			}
			return 2
		}
		if len(flags.Args()) != 0 {
			return fail(stderr, 2, fmt.Errorf("%s accepts no positional arguments", command))
		}
		text := versionText(version, commit)
		if command == "license" {
			if license == "" {
				return fail(stderr, 1, fmt.Errorf("embedded license is unavailable"))
			}
			text = license
		}
		if err := recoveryOutput(stdout, text); err != nil {
			return fail(stderr, 1, fmt.Errorf("write %s: %w", command, err))
		}
		return 0
	}
	if command == "add" {
		return addCommand(rest[1:], config, stdout, stderr)
	}
	if command == "recover" {
		if config.supplied {
			return fail(stderr, 2, fmt.Errorf("recover does not use --config or -c; recovery uses stored registry evidence"))
		}
		return recoverCommand(rest[1:], stdout, stderr)
	}
	if command == "ls" {
		return fail(stderr, 2, fmt.Errorf("unsupported command ls; use dots list"))
	}
	if command != "up" && command != "down" && command != "list" && command != "status" {
		return fail(stderr, 2, fmt.Errorf("unsupported command %q; use dots help", command))
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	config.register(flags)
	helpErr := setUsage(flags, command, stderr)
	var dryRun bool
	if command == "up" || command == "down" {
		flags.BoolVar(&dryRun, "dry-run", false, "print a plan without filesystem changes")
		flags.BoolVar(&dryRun, "n", false, "alias for --dry-run")
	}
	if err := validateConfigArgs(rest[1:], config.supplied); err != nil {
		return fail(stderr, 2, err)
	}
	if err := flags.Parse(rest[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return parsedHelp(*helpErr, stderr)
		}
		return 2
	}
	if len(flags.Args()) != 0 {
		return fail(stderr, 2, fmt.Errorf("%s accepts no positional arguments", command))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fail(stderr, 1, err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(stderr, 1, err)
	}
	cfg, err := resolver.Load(config.path, home, os.Getenv("XDG_CONFIG_HOME"), cwd)
	if err != nil {
		return fail(stderr, 1, err)
	}
	ownership := filesystem.Ownership{Registry: filesystem.StatePath(home, os.Getenv("XDG_STATE_HOME")), Profile: cfg.Path, Target: cfg.TargetDir, Sources: cfg.DotfilesDirs}
	if command == "down" {
		return remove(ownership, dryRun, stdout, stderr)
	}
	plan := resolver.Resolve(cfg)
	if command == "list" {
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
