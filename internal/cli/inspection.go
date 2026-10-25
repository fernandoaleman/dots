package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/fernandoaleman/dots/internal/filesystem"
	"github.com/fernandoaleman/dots/internal/resolver"
)

// Only up and status inspect destination occupancy. Resolution always precedes it.
func inspectPlan(plan resolver.Plan) ([]filesystem.Destination, []filesystem.Result) {
	results := make([]filesystem.Result, len(plan.Links))
	destinations := make([]filesystem.Destination, 0, len(plan.Links)+len(plan.Directories))
	for i, link := range plan.Links {
		destinations = append(destinations, filesystem.Destination{Path: link.Destination, Source: link.Source})
		results[i] = filesystem.Result{Status: "CONFLICT", Reason: link.Conflict, State: filesystem.Conflict}
		if link.InvalidSource {
			results[i].State = filesystem.InspectionError
		} else if link.Conflict == "" {
			results[i] = filesystem.Inspect(link.Source, link.Destination)
		}
	}
	for _, directory := range plan.Directories {
		destinations = append(destinations, filesystem.Destination{Path: directory.Destination, Source: directory.Source, Directory: true})
	}
	for i, collision := range filesystem.CollisionResults(destinations)[:len(results)] {
		if collision.Reason != "" {
			if results[i].Reason != "" {
				collision.Reason = results[i].Reason + "; " + collision.Reason
			}
			if results[i].State == filesystem.InspectionError {
				collision.State = filesystem.InspectionError
			}
			results[i] = collision
		}
	}
	return destinations, results
}

func list(plan resolver.Plan, stdout, stderr io.Writer) int {
	var output strings.Builder
	conflicts := 0
	for _, link := range plan.Links {
		label := "SELECTED"
		if link.Conflict != "" {
			label = "CONFLICT"
			conflicts++
		}
		printMapping(&output, label, link, link.Conflict)
	}
	printDiagnostics(&output, plan)
	fmt.Fprintf(&output, "SUMMARY %d selected, %d conflicts, %d warnings, %d errors, 0 changes made\n",
		len(plan.Links), conflicts, len(plan.Warnings), len(plan.Errors))
	return finishInspection(&output, stdout, stderr, conflicts != 0 || len(plan.Errors) != 0)
}

func status(plan resolver.Plan, results []filesystem.Result, stdout, stderr io.Writer) int {
	var output strings.Builder
	counts := make(map[filesystem.State]int)
	for i, result := range results {
		printMapping(&output, string(result.State), plan.Links[i], result.Reason)
		counts[result.State]++
	}
	printDiagnostics(&output, plan)
	fmt.Fprintf(&output, "SUMMARY %d correct, %d missing, %d wrong links, %d broken links, %d occupied, %d blocked, %d conflicts, %d warnings, %d errors, 0 changes made\n",
		counts[filesystem.Correct], counts[filesystem.Missing], counts[filesystem.WrongLink], counts[filesystem.BrokenLink],
		counts[filesystem.OccupiedFile]+counts[filesystem.OccupiedDirectory]+counts[filesystem.OccupiedOther],
		counts[filesystem.BlockedParent], counts[filesystem.Conflict], len(plan.Warnings), counts[filesystem.InspectionError]+len(plan.Errors))
	return finishInspection(&output, stdout, stderr, counts[filesystem.Correct] != len(results) || len(plan.Errors) != 0)
}

func printMapping(output *strings.Builder, label string, link resolver.Link, reason string) {
	fmt.Fprintf(output, "%s %q -> %q", label, link.Destination, link.Source)
	if reason != "" {
		fmt.Fprintf(output, ": %s", reason)
	}
	output.WriteByte('\n')
}

func printDiagnostics(output *strings.Builder, plan resolver.Plan) {
	for _, warning := range plan.Warnings {
		fmt.Fprintf(output, "WARN %s\n", warning)
	}
	for _, message := range plan.Errors {
		fmt.Fprintf(output, "ERROR %s\n", message)
	}
}

func finishInspection(output *strings.Builder, stdout, stderr io.Writer, failed bool) int {
	n, err := io.WriteString(stdout, output.String())
	if err == nil && n != output.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fail(stderr, 1, fmt.Errorf("write output: %w", err))
	}
	if failed {
		return 1
	}
	return 0
}
