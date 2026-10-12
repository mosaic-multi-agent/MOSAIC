package engine

import (
	"strings"

	"mosaic-run/internal/domain"
)

// extractAgentName strips the "#seq" suffix from an agent instance ID.
// For example, "plan-review#3" becomes "plan-review".
func extractAgentName(instanceID string) string {
	if i := strings.LastIndex(instanceID, "#"); i >= 0 {
		return instanceID[:i]
	}
	return instanceID
}

// recordedIterationValue is the OAF 3.0 recorded Iteration value of an
// iteration of a row of the given group ("{ID}.{Group}", or "{ID}" for an
// ungrouped row), "" for an empty ID. It is what a recorded step carries as
// its Stage value.
func recordedIterationValue(group domain.GroupName, id domain.IterationID) string {
	if id == "" {
		return ""
	}
	return domain.FormatIterationValue(domain.IterationValue{ID: id, Group: group})
}

// blockOfRow returns the block of the staged row at rowIdx, or "" for a row
// outside every block or an index outside the table. The admitted workflow's
// block ranges decide; a hand-built workflow without them falls back to the
// row's own block.
func blockOfRow(workflow domain.AdmittedWorkflow, rowIdx int) domain.BlockName {
	rows := workflow.Table.Rows
	if rowIdx < 0 || rowIdx >= len(rows) || !rows[rowIdx].PhaseParsed.IsStaged {
		return ""
	}
	if b, ok := workflow.BlockOf(rowIdx); ok {
		return b.Name
	}
	name, _ := rows[rowIdx].Block()
	return name
}

// iterationIndex returns the position of id in the schedule (IDs compared
// ignoring case), or -1 when absent.
func iterationIndex(s domain.BlockSchedule, id domain.IterationID) int {
	for i, it := range s.Iterations {
		if it.ID.Equal(id) {
			return i
		}
	}
	return -1
}

// iterationText names an iteration in a stop reason; an empty ID reads "(none)".
func iterationText(id domain.IterationID) string {
	if id == "" {
		return "(none)"
	}
	return string(id)
}

// firstRowPhase returns the phase of row 0, or "" if the table is empty.
func firstRowPhase(workflow domain.AdmittedWorkflow) string {
	if len(workflow.Table.Rows) == 0 {
		return ""
	}
	return workflow.Table.Rows[0].Phase
}
