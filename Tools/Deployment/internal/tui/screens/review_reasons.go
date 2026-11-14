package screens

// review_reasons.go renders the detail lines of one plan item on the review screen: every
// reason the item carries, then its warnings.

import (
	"mosaic-deploy/internal/domain"
)

// reviewDetailIndent is the indent of an item's detail lines, aligned under the item text.
const reviewDetailIndent = "               "

// itemReasonLines returns one muted line per reason of item, in the order the reasons were
// added, followed by one warning line per warning. An item that carries version deltas but no
// reason (a plan built outside the planner) lists its deltas instead, so they are never hidden.
func itemReasonLines(item domain.PlanItem) []reviewLine {
	var lines []reviewLine
	for _, reason := range item.Reasons {
		lines = append(lines, reviewLine{text: reviewDetailIndent + reason, style: "muted"})
	}
	if len(item.Reasons) == 0 {
		for _, delta := range item.Stale {
			lines = append(lines, reviewLine{text: reviewDetailIndent + formatReviewVersionDelta(delta), style: "muted"})
		}
	}
	if item.BackupBeforeWrite {
		lines = append(lines, reviewLine{text: reviewDetailIndent + domain.BackupThenReplaceText, style: "muted"})
	}
	for _, warning := range item.Warnings {
		lines = append(lines, reviewLine{text: reviewDetailIndent + "warning: " + warning, style: "warning"})
	}
	return lines
}
