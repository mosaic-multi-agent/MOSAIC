package engine

import (
	"mosaic-run/internal/domain"
)

// reviewLoopLimitReached reports whether the reviewer at the current position
// has produced as many COMPLETED_NEEDS_ACTION iterations at the current phase
// and stage as the review loop limit allows. A limit of 0 means no limit.
//
// The count is keyed by reviewer, phase and iteration value (not by workflow
// row; each entry's value is read with its own row, so both recorded spellings
// of one iteration count together) and only covers the rows since the
// reviewer's last SUCCESS at that phase and iteration. An entry whose value
// cannot be read is neither counted nor a reset.
//
// A CNA row directly following a CNA row of the same agent is a re-dispatch of
// the same iteration (for example after a rejected result) and is not counted
// again. Likewise a reviewer SUCCESS directly following the same reviewer's CNA
// is a gate-discharging re-dispatch of that round, not a pass, and does not
// reset the count; only a reviewer SUCCESS that does not directly follow its
// own CNA does.
func reviewLoopLimitReached(workflow domain.AdmittedWorkflow, state domain.ArtifactState, pos position) bool {
	limit := state.ReviewLoopLimit
	if limit <= 0 {
		return false
	}
	reviewer := extractAgentName(state.CurrentState.LastAgent)
	// An entry that records no row is read with the position's row.
	var posRow *domain.RoutingRow
	if row, ok := tableRow(workflow, pos.rowIdx); ok {
		posRow = &row
	}
	count := 0
	for i, e := range state.ExecutionLog {
		if (e.Status != domain.StatusCOMPLETED_NEEDS_ACTION && e.Status != domain.StatusSUCCESS) ||
			e.Phase != pos.entry.Phase ||
			extractAgentName(e.Agent) != reviewer ||
			!entryValueEquals(workflow, e, posRow, pos.value) {
			continue
		}
		followsOwnCNA := i > 0 &&
			state.ExecutionLog[i-1].Status == domain.StatusCOMPLETED_NEEDS_ACTION &&
			extractAgentName(state.ExecutionLog[i-1].Agent) == reviewer
		switch {
		case e.Status == domain.StatusSUCCESS && !followsOwnCNA:
			count = 0
		case e.Status == domain.StatusCOMPLETED_NEEDS_ACTION && !followsOwnCNA:
			count++
		}
	}
	return count >= limit
}
