package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

type commandDescription struct{ name, description, group string }

var commands = []commandDescription{
	{"help", "Show general or command-specific help", "Documentation"},
	{"version", "Show version information", "Documentation"},
	{"license", "Show the project license", "Documentation"},
	{"add", "Import an existing dotfile without replacement", "Daily"},
	{"up", "Create configured symlinks", "Daily"},
	{"down", "Remove verified owned symlinks", "Daily"},
	{"list", "List resolved dotfile mappings", "Daily"},
	{"status", "Inspect destination state", "Daily"},
	{"recover", "Inspect or explicitly resolve unfinished state", "Recovery"},
}

const configHelp = "  -c, --config FILE    Select a TOML configuration file (once, in either scope)\n"
const dryHelp = "  -n, --dry-run        Preview without making changes\n"
const shortHelp = "  -h, --help           Show help\n"

func helpText(command string) (string, error) {
	var out strings.Builder
	if command == "" {
		out.WriteString("Usage: dots [GLOBAL FLAGS] COMMAND [FLAGS]\n")
		group := ""
		for _, c := range commands {
			if c.group != group {
				fmt.Fprintf(&out, "\n%s commands:\n", c.group)
				group = c.group
			}
			fmt.Fprintf(&out, "  %-10s %s\n", c.name, c.description)
		}
		out.WriteString("\nGlobal flags:\n" + configHelp + shortHelp + "      --version       Show version information\n")
		out.WriteString("\nConfiguration applies to add, up, down, list and status.\nDry-run is command-specific; use dots help COMMAND for applicable flags.\n")
		return out.String(), nil
	}
	known := false
	for _, c := range commands {
		known = known || c.name == command
	}
	if !known {
		return "", fmt.Errorf("unknown help command %q", command)
	}
	switch command {
	case "help":
		out.WriteString("Usage: dots help [COMMAND]\nShow help without loading configuration or ownership state.\n")
	case "version":
		out.WriteString("Usage: dots version\n       dots --version\nShow the release version or a development-build fallback; -v is reserved.\n")
	case "license":
		out.WriteString("Usage: dots license\nPrint the embedded MIT license without network or checkout access.\n")
	case "add":
		out.WriteString("Usage: dots [GLOBAL FLAGS] add [FLAGS] PATH\nFlags precede PATH; use -- before a filename beginning with -.\nImport one existing regular file, without replacement or adoption.\n\nFlags:\n" + configHelp + "      --root DIR      Select a configured source root; required with multiple roots\n" + dryHelp)
	case "up", "down":
		fmt.Fprintf(&out, "Usage: dots [GLOBAL FLAGS] %s [FLAGS]\n\nFlags:\n%s%s", command, configHelp, dryHelp)
	case "list", "status":
		fmt.Fprintf(&out, "Usage: dots [GLOBAL FLAGS] %s [FLAGS]\nRead-only; does not support --dry-run.\n\nFlags:\n%s", command, configHelp)
	case "recover":
		out.WriteString("Usage: dots recover\n       dots recover --retire INTENT_ID --revision N [--dry-run]\n       dots recover --settle --revision N [--dry-run]\nUses stored XDG ownership evidence without TOML; -c and --config are rejected.\nMutations revalidate under the exclusive lock; retirement never settles the barrier.\n\nFlags:\n      --retire ID      Retire the exact stored intent; leave the barrier\n      --settle         Clear an eligible barrier; do not repair files\n      --revision N     Require the exact observed unsigned registry revision\n      --dry-run        Preview the selected action without writes or exclusive locking\n")
	}
	if command != "add" && command != "up" && command != "down" && command != "list" && command != "status" && command != "recover" {
		out.WriteString("\nFlags:\n")
	}
	out.WriteString(shortHelp)
	return out.String(), nil
}

func showHelp(command string, w io.Writer) error {
	text, err := helpText(command)
	if err != nil {
		return err
	}
	return recoveryOutput(w, text)
}

// FlagSet still owns parsing and error messages. All help paths share one layout.
func setUsage(f *flag.FlagSet, command string, w io.Writer) *error {
	var writeErr error
	f.Usage = func() { writeErr = showHelp(command, w) }
	return &writeErr
}

func parsedHelp(err error, stderr io.Writer) int {
	if err != nil {
		return fail(stderr, 1, fmt.Errorf("write help: %w", err))
	}
	return 0
}
