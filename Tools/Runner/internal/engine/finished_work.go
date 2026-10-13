package engine

import "mosaic-run/internal/domain"

// finishedWorkDeviation applies the finished-work check to the decision the
// routing of a SUCCESS produced. Only a Dispatch is checked; any other decision
// is returned unchanged. When the dispatched step already has a SUCCESS in the
// Execution Log, the dispatch is replaced by a DeviationFinishedWork deviation
// whose PlannedContinuation is that step.
//
// currentRow and pos describe the row that just succeeded. resp is its response
// (the zero ProtocolResponse on resume).
func finishedWorkDeviation(
	in NextInput,
	pos position,
	currentRow domain.RoutingRow,
	resp domain.ProtocolResponse,
	decision domain.EngineDecision,
) domain.EngineDecision {
	if decision.Dispatch == nil || len(decision.Dispatch.Steps) != 1 {
		return decision
	}
	step := decision.Dispatch.Steps[0]
	if !stepFinished(in, step) {
		return decision
	}
	return domain.EngineDecision{Deviation: &domain.DeviationDecision{
		Info: domain.DeviationInfo{
			Kind:                domain.DeviationFinishedWork,
			Response:            resp,
			CurrentRow:          pos.rowIdx,
			CurrentPhase:        currentRow.Phase,
			CurrentStage:        pos.stage,
			PlannedContinuation: &step,
			ArtifactState:       in.State,
		},
	}}
}

// stepFinished reports whether the Execution Log holds a SUCCESS entry that
// records step's row and an iteration value equal to step's. Entries without a
// recorded row, and entries whose iteration value cannot be read for the row,
// never count.
func stepFinished(in NextInput, step domain.DispatchStep) bool {
	rows := in.Workflow.Table.Rows
	if step.RowIndex < 0 || step.RowIndex >= len(rows) {
		return false
	}
	row := rows[step.RowIndex]
	want, err := domain.ReadIterationValue(step.Stage, row)
	if err != nil {
		return false
	}
	wantRow := domain.WorkflowRowFromIndex(step.RowIndex)
	for _, e := range in.State.ExecutionLog {
		if e.Status != domain.StatusSUCCESS || e.WorkflowRow == domain.NoWorkflowRow || e.WorkflowRow != wantRow {
			continue
		}
		got, err := domain.ReadIterationValue(e.Stage, row)
		if err != nil {
			continue
		}
		if got.Equal(want) {
			return true
		}
	}
	return false
}
