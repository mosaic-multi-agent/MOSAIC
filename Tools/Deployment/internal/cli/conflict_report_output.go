package cli

// conflict_report_output.go renders the catalog-stack conflict report in human output. JSON
// output carries the report inside the result document itself, so nothing is written here for
// it: a machine-readable run keeps one JSON document on stdout.

import (
	"fmt"
	"io"
	"strings"

	"mosaic-deploy/internal/domain"
)

// writeConflictReport writes one line per report entry, with its severity, ahead of the result.
// It writes nothing for JSON output or for an empty report. The report never affects the exit
// code.
func writeConflictReport(out io.Writer, format string, report domain.ConflictReport) {
	if strings.EqualFold(format, "json") {
		return
	}
	for _, line := range report.Lines {
		fmt.Fprintln(out, conflictLineText(line))
	}
}

// conflictLineText formats one entry as
// "<severity>: <kind>[:<key>] from <id> <version> shadows <id> <version>[, ...][; affects workflows: a, b]".
func conflictLineText(line domain.ConflictLine) string {
	item := string(line.Item.Kind)
	if line.Item.Key != "" {
		item += ":" + line.Item.Key
	}
	shadowed := make([]string, len(line.Shadowed))
	for i, id := range line.Shadowed {
		shadowed[i] = id.ID + " " + id.Version
	}
	text := fmt.Sprintf("%s: %s from %s %s shadows %s",
		line.Severity, item, line.Winner.ID, line.Winner.Version, strings.Join(shadowed, ", "))
	if len(line.AffectedWorkflows) > 0 {
		text += "; affects workflows: " + strings.Join(line.AffectedWorkflows, ", ")
	}
	return text
}
