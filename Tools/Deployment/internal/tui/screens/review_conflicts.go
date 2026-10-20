package screens

// review_conflicts.go renders the catalog-stack conflict report on the plan review, so the user
// sees every shadowing before anything is written. The line text follows the human conflict line
// of the command line.

import (
	"fmt"
	"strings"

	"mosaic-deploy/internal/domain"
)

// conflictReportLines returns the review lines of report: a heading and one line per entry. An
// empty report has no section. A warning (undeclared override) is styled as a warning.
func conflictReportLines(report domain.ConflictReport) []reviewLine {
	if len(report.Lines) == 0 {
		return nil
	}
	lines := []reviewLine{{text: "Catalog conflicts:", style: "warning"}}
	for _, l := range report.Lines {
		style := "muted"
		if l.Severity == domain.ConflictWarning {
			style = "warning"
		}
		lines = append(lines, reviewLine{text: conflictLineText(l), style: style})
	}
	return append(lines, reviewLine{text: "", style: "body"})
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
