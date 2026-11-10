package engine

import "mosaic-run/internal/domain"

// QualifierFocusPrefix introduces the mode-qualifier paragraph that the engine
// appends to the task description of an engine-routed dispatch of a qualified
// row (row.Qualifier != ""):
//
//	GenericTaskDescription + "\n\n" + QualifierFocusPrefix + " " + qualifier
//
// A PARTIALLY_DONE re-dispatch of a qualified row carries both paragraphs, the
// focus paragraph first:
//
//	GenericTaskDescription + "\n\n" + QualifierFocusPrefix + " " + qualifier +
//	"\n\n" + PartiallyDoneContextPrefix + "\n" + msg
//
// An unqualified row keeps today's texts unchanged (no focus paragraph).
const QualifierFocusPrefix = "This workflow row runs you with the mode qualifier:"

// qualifierTaskDescription is the task description of an engine-routed
// dispatch of row: the generic text followed by the qualifier focus paragraph
// for a qualified row, and "" for an unqualified row, which keeps its
// unchanged text.
func qualifierTaskDescription(row domain.RoutingRow) string {
	if row.Qualifier == "" {
		return ""
	}
	return domain.GenericTaskDescription + "\n\n" + QualifierFocusPrefix + " " + row.Qualifier
}

// partiallyDoneTaskDescription is the task description of a PARTIALLY_DONE
// re-dispatch of row: the row's own text (focus paragraph first for a qualified
// row), then the previous status message.
func partiallyDoneTaskDescription(row domain.RoutingRow, msg string) string {
	base := qualifierTaskDescription(row)
	if base == "" {
		base = domain.GenericTaskDescription
	}
	return base + "\n\n" + PartiallyDoneContextPrefix + "\n" + msg
}
