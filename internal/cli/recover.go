package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/fernandoaleman/dots/internal/filesystem"
)

func recoverCommand(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("recover", flag.ContinueOnError)
	f.SetOutput(stderr)
	var retire, revision string
	var settle, dry bool
	f.StringVar(&retire, "retire", "", "retire the exact stored import intent ID; leaves the barrier")
	f.StringVar(&revision, "revision", "", "exact observed registry revision (unsigned decimal)")
	f.BoolVar(&settle, "settle", false, "clear an eligible barrier; does not repair files")
	f.BoolVar(&dry, "dry-run", false, "inspect the selected action without writes or exclusive locking")
	f.Usage = func() {
		fmt.Fprintln(stderr, "Usage: dots recover")
		fmt.Fprintln(stderr, "       dots recover --retire INTENT_ID --revision N [--dry-run]")
		fmt.Fprintln(stderr, "       dots recover --settle --revision N [--dry-run]")
		fmt.Fprintln(stderr, "Uses XDG ownership state without TOML. Mutations revalidate under the exclusive lock.")
		f.PrintDefaults()
	}
	seen := map[string]bool{}
	for _, arg := range args {
		key, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if strings.HasPrefix(arg, "-") && (key == "retire" || key == "revision" || key == "settle" || key == "dry-run") {
			if seen[key] {
				return fail(stderr, 2, fmt.Errorf("duplicate --%s flag", key))
			}
			seen[key] = true
		}
	}
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(f.Args()) != 0 || seen["settle"] && !settle || seen["dry-run"] && !dry {
		return fail(stderr, 2, fmt.Errorf("recover accepts no positional arguments or explicit false action flags"))
	}
	action := seen["retire"] || settle
	if seen["retire"] && settle || action != seen["revision"] || dry && !action {
		return fail(stderr, 2, fmt.Errorf("choose --retire INTENT_ID or --settle with --revision N; --dry-run requires a selected action"))
	}
	if seen["retire"] && (len(retire) != 32 || strings.Trim(retire, "0123456789abcdef") != "") {
		return fail(stderr, 2, fmt.Errorf("--retire requires the exact 32-character lowercase hexadecimal intent ID"))
	}
	var rev uint64
	if seen["revision"] {
		var err error
		rev, err = strconv.ParseUint(revision, 10, 64)
		if err != nil || revision == "" || strings.Trim(revision, "0123456789") != "" {
			return fail(stderr, 2, fmt.Errorf("--revision requires an unsigned decimal integer within uint64 range"))
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fail(stderr, 1, err)
	}
	path := filesystem.StatePath(home, os.Getenv("XDG_STATE_HOME"))
	if action && !dry {
		var output string
		if settle {
			r, e := filesystem.SettleBarrier(path, rev)
			err = e
			if e == nil {
				status := "SETTLED"
				if r.NoOp {
					status = "NOOP"
				}
				output = fmt.Sprintf("%s registry %q revision %d; no user paths repaired; ownership history preserved\n", status, path, r.Revision)
			} else {
				output = fmt.Sprintf("SETTLEMENT STOPPED registry %q revision %d confirmed-settled=%t; preserve evidence; no automatic retry\n", path, r.Revision, r.Settled)
			}
		} else {
			r, e := filesystem.RetireImport(path, filesystem.RecoverySelection{ID: retire, Revision: rev})
			err = e
			if e == nil {
				output = fmt.Sprintf("RETIRED intent %s; surviving file %q unchanged; registry revision %d; unfinished barrier remains\nInspect again, then separately authorize: dots recover --settle --revision %d\n", retire, r.Survivor, r.Revision, r.Revision)
			} else {
				output = fmt.Sprintf("RETIREMENT STOPPED registry %q revision %d confirmed-invalidated=%t confirmed-retired=%t surviving-file=%q; preserve evidence; no automatic retry\n", path, r.Revision, r.Invalidated, r.Retired, r.Survivor)
			}
		}
		if outputErr := recoveryOutput(stdout, output); outputErr != nil {
			err = errors.Join(err, fmt.Errorf("write recovery result: %w; registry action may already have completed; inspect again before any further action", outputErr))
		}
		if err != nil {
			return fail(stderr, 1, err)
		}
		return 0
	}
	var selection *filesystem.RecoverySelection
	if seen["retire"] {
		selection = &filesystem.RecoverySelection{ID: retire, Revision: rev}
	}
	r, inspectErr := filesystem.InspectRecovery(path, selection)
	var out strings.Builder
	fmt.Fprintf(&out, "REGISTRY %q exists=%t revision=%d unfinished=%t\n", path, r.Exists, r.Revision, r.Unfinished)
	if i := r.Intent; i != nil {
		fmt.Fprintf(&out, "INTENT %s phase=%s profile=%q target=%q\nINPUT %q: %s\nSOURCE %q: %s\n", i.ID, i.Phase, i.Profile, i.Target, i.Input.Path, i.Input.Kind, i.Source.Path, i.Source.Kind)
		for _, c := range i.Checks {
			status := "OK"
			if !c.Passed {
				status = "BLOCKED"
			}
			fmt.Fprintf(&out, "%s %q: %q\n", status, c.Name, c.Detail)
		}
		fmt.Fprintf(&out, "RETIREMENT observed-eligible=%t; select --retire %s --revision %d\n", i.RetirementObserved, i.ID, r.Revision)
	}
	for _, reason := range r.SettlementReasons {
		fmt.Fprintf(&out, "SETTLEMENT %q\n", reason)
	}
	if inspectErr == nil && r.Unfinished && r.Intent == nil && len(r.SettlementReasons) == 0 {
		fmt.Fprintf(&out, "SETTLEMENT observed-eligible; preview with: dots recover --settle --revision %d --dry-run\n", r.Revision)
	}
	code := 0
	if dry {
		eligible := false
		if settle {
			eligible = inspectErr == nil && r.Revision == rev && (r.Exists || rev == 0) && r.Intent == nil && (!r.Unfinished || len(r.SettlementReasons) == 0)
			fmt.Fprintln(&out, "PREVIEW settle: clear only an eligible barrier; preserve registry version/history; no user paths repaired")
			if r.Revision != rev || !r.Exists && rev != 0 {
				fmt.Fprintln(&out, "BLOCKED expected revision does not match observed state")
			}
		} else {
			eligible = inspectErr == nil && r.Intent != nil && r.Intent.RetirementObserved
			fmt.Fprintln(&out, "PREVIEW retire: invalidate prior owned/removing authority and retire only the intent; retain history, user files and unfinished barrier")
		}
		fmt.Fprintf(&out, "PREVIEW observed-eligible=%t; 0 changes made; actual action revalidates under the exclusive lock\n", eligible)
		if !eligible {
			code = 1
		}
	}
	fmt.Fprintln(&out, "Observations grant no mutation authority; do not edit ownership.json; preserve evidence if recovery refuses")
	if err := recoveryOutput(stdout, out.String()); err != nil {
		return fail(stderr, 1, errors.Join(inspectErr, fmt.Errorf("write recovery inspection: %w", err)))
	}
	if inspectErr != nil {
		return fail(stderr, 1, inspectErr)
	}
	return code
}

func recoveryOutput(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	return err
}
