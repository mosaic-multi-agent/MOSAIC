package screens

// review_dropped.go renders the manifest entries dropped before planning on the review screen,
// so the user sees them, with their reason, before approving.

import (
	"fmt"

	"mosaic-deploy/internal/domain"
)

// droppedCountParts is the counts-header entry for dropped manifest entries.
func droppedCountParts(p domain.Plan) []string {
	if n := len(p.DroppedEntries); n > 0 {
		return []string{fmt.Sprintf("%d dropped from the manifest", n)}
	}
	return nil
}

// droppedReviewLines lists every dropped manifest entry with its harness (when known), kind,
// key and path, followed by its reason.
func droppedReviewLines(p domain.Plan) []reviewLine {
	var lines []reviewLine
	for _, d := range p.DroppedEntries {
		e := d.Entry
		subject := fmt.Sprintf("%s:%s", e.Ref.Kind, e.Ref.Key)
		if e.Harness != "" {
			subject = e.Harness + " " + subject
		}
		lines = append(lines,
			reviewLine{text: fmt.Sprintf("%-14s %s %s", "[DROPPED]", subject, e.TargetPath), style: "warning"},
			reviewLine{text: reviewDetailIndent + d.Reason, style: "muted"})
	}
	return lines
}
