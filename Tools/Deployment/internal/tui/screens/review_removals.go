package screens

// review_removals.go renders the files a plan removes or replaces on the review screen, so
// the user sees them before approving. Each entry states the reason and then the backup action
// that will happen.

import (
	"fmt"

	"mosaic-deploy/internal/domain"
)

// removalCountParts are the counts-header entries for removals and replacements.
func removalCountParts(p domain.Plan) []string {
	var parts []string
	if n := len(p.Removals); n > 0 {
		parts = append(parts, fmt.Sprintf("%d to remove", n))
	}
	if n := len(p.Replacements); n > 0 {
		parts = append(parts, fmt.Sprintf("%d to replace (current file backed up first)", n))
	}
	return parts
}

// removalReviewLines lists every removal, then every replacement, each with its reason and the
// backup action.
func removalReviewLines(p domain.Plan) []reviewLine {
	var lines []reviewLine
	for _, r := range p.Removals {
		lines = append(lines, removalLine("[REMOVE]", r.Ref, r.Path, r.Reason, domain.BackupThenRemoveText)...)
	}
	for _, r := range p.Replacements {
		lines = append(lines, removalLine("[REPLACE]", r.Ref, r.Path, r.Reason, domain.BackupThenReplaceText)...)
	}
	return lines
}

func removalLine(badge string, ref domain.ArtifactRef, path, reason, backupAction string) []reviewLine {
	lines := []reviewLine{{
		text:  fmt.Sprintf("%-14s %s:%s %s", badge, ref.Kind, ref.Key, path),
		style: "warning",
	}}
	if reason != "" {
		lines = append(lines, reviewLine{text: reviewDetailIndent + reason, style: "muted"})
	}
	return append(lines, reviewLine{text: reviewDetailIndent + backupAction, style: "muted"})
}
