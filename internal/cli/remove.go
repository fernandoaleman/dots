package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/fernandoaleman/dots/internal/filesystem"
)

func remove(options filesystem.Ownership, dryRun bool, stdout, stderr io.Writer) int {
	report, operationErr := filesystem.Down(options, dryRun)
	var output strings.Builder
	planned, missing, preserved, skipped := 0, 0, 0, 0
	for _, entry := range report.Entries {
		fmt.Fprintf(&output, "%s %q -> %q", entry.Status, entry.Destination, entry.Source)
		if entry.Reason != "" {
			fmt.Fprintf(&output, ": %s", entry.Reason)
		}
		output.WriteByte('\n')
		switch entry.Status {
		case "REMOVE":
			planned++
		case "MISSING":
			missing++
		case "PRESERVE":
			preserved++
		case "SKIP":
			skipped++
		}
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(&output, "WARN %s\n", warning)
	}
	if operationErr == nil && len(report.Entries) == 0 {
		fmt.Fprintln(&output, "INFO no eligible ownership records; untracked and legacy links are never adopted or removed")
	}
	errors := 0
	if operationErr != nil {
		errors = 1
		fmt.Fprintf(&output, "ERROR %v\n", operationErr)
	}
	fmt.Fprintf(&output, "SUMMARY %d planned, %d removed, %d missing, %d preserved, %d skipped, %d warnings, %d errors, %d changes made\n",
		planned, report.Removed, missing, preserved, skipped, len(report.Warnings), errors, report.Removed)
	n, err := io.WriteString(stdout, output.String())
	if err == nil && n != output.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fail(stderr, 1, fmt.Errorf("write output: %w; %d links already removed", err, report.Removed))
	}
	if operationErr != nil {
		return 1
	}
	return 0
}
