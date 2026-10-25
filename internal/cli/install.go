package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/fernandoaleman/dots/internal/filesystem"
	"github.com/fernandoaleman/dots/internal/resolver"
)

func install(plan resolver.Plan, destinations []filesystem.Destination, results []filesystem.Result, ownership filesystem.Ownership, stdout, stderr io.Writer) int {
	blocked := len(plan.Errors) != 0
	for _, result := range results {
		blocked = blocked || result.Status == "CONFLICT"
	}
	report := filesystem.Report{Results: results}
	diagnostics := append([]string(nil), plan.Errors...)
	if blocked {
		// Destination inspection can observe stale recorded identity even
		// when installation is already blocked. Persist revocation under the
		// ownership barrier rather than leaving old authority replayable.
		warnings, err := filesystem.RevalidateOwnership(ownership)
		report.Warnings = append(report.Warnings, warnings...)
		if err != nil {
			diagnostics = append(diagnostics, "validate ownership after failed preflight: "+err.Error())
		}
		for i, result := range report.Results {
			if result.Status == "LINK" {
				report.Results[i] = filesystem.Result{Status: "SKIP", Reason: "preflight failed; no changes made"}
			}
		}
	} else {
		var err error
		report, err = filesystem.InstallOwned(destinations, ownership)
		if err != nil {
			reported := false
			for _, result := range report.Results {
				reported = reported || result.Status == "ERROR" && result.Reason == err.Error()
			}
			if !reported {
				diagnostics = append(diagnostics, err.Error())
			}
		}
	}
	plan.Warnings = append(plan.Warnings, report.Warnings...)
	return writeInstallation(plan, report, diagnostics, stdout, stderr)
}

func writeInstallation(plan resolver.Plan, report filesystem.Report, diagnostics []string, stdout, stderr io.Writer) int {
	correct, conflicts, skipped, failures := 0, 0, 0, len(diagnostics)
	var output strings.Builder
	for i, result := range report.Results {
		link := plan.Links[i]
		fmt.Fprintf(&output, "%s %q -> %q", result.Status, link.Destination, link.Source)
		if result.Reason != "" {
			fmt.Fprintf(&output, ": %s", result.Reason)
		}
		output.WriteByte('\n')
		switch result.Status {
		case "LINKED":
		case "OK":
			correct++
		case "CONFLICT":
			conflicts++
		case "SKIP":
			skipped++
		default:
			failures++
		}
	}
	for _, warning := range plan.Warnings {
		fmt.Fprintf(&output, "WARN %s\n", warning)
	}
	for _, diagnostic := range diagnostics {
		fmt.Fprintf(&output, "ERROR %s\n", diagnostic)
	}
	// Successful syscall accounting survives an ERROR row from a later sync
	// or ownership publication failure for the very same destination.
	linked := report.LinksCreated
	fmt.Fprintf(&output, "SUMMARY %d linked, %d correct, %d conflicts, %d skipped, %d warnings, %d errors, %d directories created, %d changes made\n",
		linked, correct, conflicts, skipped, len(plan.Warnings), failures, report.DirectoriesCreated, report.LinksCreated+report.DirectoriesCreated)
	n, err := io.WriteString(stdout, output.String())
	if err == nil && n != output.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		return installOutputFailure(stderr, report, err)
	}
	if conflicts != 0 || failures != 0 {
		return 1
	}
	return 0
}

func installOutputFailure(stderr io.Writer, report filesystem.Report, err error) int {
	return fail(stderr, 1, fmt.Errorf("write output: %w; %d links and %d directories already created", err, report.LinksCreated, report.DirectoriesCreated))
}
