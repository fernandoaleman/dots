package cli

import (
	"flag"
	"fmt"
	"strings"
)

// One selection is shared by the global and command FlagSets. Aliases cannot
// silently select a different repository, even when their values are identical.
type configSelection struct {
	path     string
	supplied bool
}

func (c *configSelection) register(f *flag.FlagSet) {
	selectPath := func(value string) error {
		if c.supplied {
			return fmt.Errorf("duplicate --config/-c selection; supply it only once")
		}
		if value == "" {
			return fmt.Errorf("--config path must not be empty")
		}
		c.path, c.supplied = value, true
		return nil
	}
	f.Func("config", "select a TOML configuration file", selectPath)
	f.Func("c", "alias for --config", selectPath)
}

// A missing pathname must not consume a preview/help flag. Explicit = retains
// support for intentional flag-like pathnames. Do not interpret tokens after --
// or the first positional argument, matching flag.FlagSet's parsing boundary.
func validateConfigArgs(args []string, configured bool) error {
	seen := configured
	for i := 0; i < len(args); i++ {
		if args[i] == "--" || !strings.HasPrefix(args[i], "-") || args[i] == "-" {
			break
		}
		key, value, assigned := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		if key == "config" || key == "c" {
			if seen {
				return fmt.Errorf("duplicate --config/-c selection; supply it only once")
			}
			seen = true
			if assigned && value == "" || !assigned && i+1 < len(args) && args[i+1] == "" {
				return fmt.Errorf("--config path must not be empty")
			}
		}
		if (key == "config" || key == "c" || key == "root") && !assigned {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return fmt.Errorf("--%s requires a path value; use --%s=VALUE for a pathname beginning with -", key, key)
			}
			i++
		}
	}
	return nil
}

// Information and recovery commands do not select live configuration, even
// when help would otherwise stop FlagSet parsing before an invalid selection.
func rejectConfigArgs(command string, args []string, configured bool) error {
	if configured {
		return fmt.Errorf("%s does not use --config or -c", command)
	}
	for _, arg := range args {
		if arg == "--" {
			break
		}
		key, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if strings.HasPrefix(arg, "-") && (key == "config" || key == "c") {
			return fmt.Errorf("%s does not use --config or -c", command)
		}
	}
	return nil
}
