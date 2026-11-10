package domain

import (
	"fmt"
	"strings"
)

// DispatchTarget is a consultant's proposed workflow position, as received.
type DispatchTarget struct {
	Agent     string      // the bare agent id of the named row
	Row       WorkflowRow // 1-based; NoWorkflowRow = missing
	Iteration string      // the run artifact's Iteration value ("{ID}" or "{ID}.{Group}"); "" = missing/absent
}

// ResolvedDispatchTarget is a validated target.
type ResolvedDispatchTarget struct {
	Row           RoutingRow
	Iteration     IterationID // "" for non-staged rows
	RecordedStage string      // FormatIterationValue of Iteration and the row's group (3.0 value), "" for non-staged rows
}

// DispatchTargetReason names why a dispatch target is invalid.
type DispatchTargetReason string

const (
	ReasonMissingRow          DispatchTargetReason = "missing-row"
	ReasonUnknownRow          DispatchTargetReason = "unknown-row"
	ReasonAgentMismatch       DispatchTargetReason = "agent-mismatch"
	ReasonMissingStage        DispatchTargetReason = "missing-stage"
	ReasonUnknownStage        DispatchTargetReason = "unknown-stage"
	ReasonStageOnNonStagedRow DispatchTargetReason = "stage-on-non-staged-row"
	ReasonStageGroupMismatch  DispatchTargetReason = "stage-group-mismatch"
	// ReasonMalformedIteration: the iteration value has too many segments, a
	// group segment on an ungrouped row, or an ID that breaks the iteration ID
	// character set.
	ReasonMalformedIteration DispatchTargetReason = "malformed-iteration"
)

// DispatchTargetError names why a target is invalid. Callers switch on Reason,
// never on text.
type DispatchTargetError struct {
	Reason DispatchTargetReason
	Target DispatchTarget
	Detail string
}

func (e *DispatchTargetError) Error() string {
	return "invalid dispatch target: " + string(e.Reason)
}

// ValidateDispatchIteration checks a consultant's dispatch target against the
// admitted routing table and the current block schedules.
//
// Checks run in this order: row present, row exists, agent equals the row's
// bare agent id, then the iteration rules: a row outside every block must carry
// no iteration; a block row must carry one, split by the row's group (the ID,
// then the group for a grouped row, nothing more for an ungrouped row), whose
// ID exists, ignoring case, in the schedule of the row's block and whose group
// equals the row's group. The resolved target carries the ID as the schedule
// spells it. Nothing is corrected.
func ValidateDispatchIteration(table RoutingTable, schedules BlockSchedules, t DispatchTarget) (ResolvedDispatchTarget, error) {
	fail := func(reason DispatchTargetReason, detail string) (ResolvedDispatchTarget, error) {
		return ResolvedDispatchTarget{}, &DispatchTargetError{Reason: reason, Target: t, Detail: detail}
	}
	if t.Row == NoWorkflowRow {
		return fail(ReasonMissingRow, "no row named")
	}
	if t.Row < 1 || t.Row.Index() >= len(table.Rows) {
		return fail(ReasonUnknownRow, fmt.Sprintf("row %d is not in the routing table (%d rows)", t.Row, len(table.Rows)))
	}
	row := table.Rows[t.Row.Index()]
	if bare := row.BareAgentID(); bare != t.Agent {
		return fail(ReasonAgentMismatch, fmt.Sprintf("row %d belongs to agent %q, not %q", t.Row, bare, t.Agent))
	}

	block, inBlock := row.Block()
	if !row.PhaseParsed.IsStaged || !inBlock {
		if t.Iteration != "" {
			return fail(ReasonStageOnNonStagedRow, fmt.Sprintf("row %d is outside every block and takes no iteration", t.Row))
		}
		return ResolvedDispatchTarget{Row: row}, nil
	}

	if t.Iteration == "" {
		return fail(ReasonMissingStage, fmt.Sprintf("row %d is in block %q and needs an iteration", t.Row, block))
	}
	id, reason, detail := splitTargetIteration(t.Iteration, row.PhaseParsed.Group)
	if reason != "" {
		return fail(reason, fmt.Sprintf("row %d: %s", t.Row, detail))
	}
	sched, ok := schedules[block]
	if !ok {
		return fail(ReasonUnknownStage, fmt.Sprintf("no schedule is available for block %q", block))
	}
	it, found := sched.Find(id)
	if !found {
		return fail(ReasonUnknownStage, fmt.Sprintf("iteration %s is not in the schedule of block %q", id, block))
	}
	return ResolvedDispatchTarget{
		Row:           row,
		Iteration:     it.ID,
		RecordedStage: FormatIterationValue(IterationValue{ID: it.ID, Group: row.PhaseParsed.Group}),
	}, nil
}

// splitTargetIteration splits an iteration value by the group of the row it
// names. The ID is checked before the group.
func splitTargetIteration(value string, group GroupName) (id IterationID, reason DispatchTargetReason, detail string) {
	segments := strings.Split(value, ".")
	if group == "" {
		if len(segments) > 1 || !validIterationID(value) {
			return "", ReasonMalformedIteration, fmt.Sprintf("%q is not an iteration ID (the row has no group)", value)
		}
		return IterationID(value), "", ""
	}
	if len(segments) > 2 || !validIterationID(segments[0]) {
		return "", ReasonMalformedIteration, fmt.Sprintf("%q is not of the form {ID}.%s", value, group)
	}
	if len(segments) != 2 || GroupName(segments[1]) != group {
		return "", ReasonStageGroupMismatch, fmt.Sprintf("%q does not name the row's group %q", value, group)
	}
	return IterationID(segments[0]), "", ""
}

// IterationTargetValue renders the iteration a dispatch names as the value form
// a DispatchTarget carries: "" for an empty ID, else the ID followed by the
// group of the row at rowIndex when that row has one.
func IterationTargetValue(table RoutingTable, rowIndex int, id IterationID) string {
	if id == "" {
		return ""
	}
	if rowIndex < 0 || rowIndex >= len(table.Rows) {
		return string(id)
	}
	return FormatIterationValue(IterationValue{ID: id, Group: table.Rows[rowIndex].PhaseParsed.Group})
}
