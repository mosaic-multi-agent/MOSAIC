package engine

import "mosaic-run/internal/domain"

// HarnessErrorAttemptLimit is the budget of HARNESS_ERROR attempts per
// workflow row and iteration value since the last SUCCESS there.
const HarnessErrorAttemptLimit = 3

// HarnessErrorBudgetRemaining returns HarnessErrorAttemptLimit minus the
// number of HARNESS_ERROR log rows at (row, recorded value) after the last
// SUCCESS row at the same (row, value); never negative. Force-stopped rows,
// rows of other rows or values, rows whose value cannot be read with row and
// rows with NoWorkflowRow are ignored. Values are compared as parsed
// iterations, so both spellings of one iteration share one budget. When
// recorded cannot be read with row the result is 0.
func HarnessErrorBudgetRemaining(log []domain.ExecutionLogEntry, row domain.RoutingRow, recorded string) int {
	want, err := domain.ReadIterationValue(recorded, row)
	if err != nil {
		return 0
	}
	own := domain.WorkflowRowFromIndex(row.Index)
	used := 0
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		if e.IsForceStopped() {
			continue
		}
		if e.WorkflowRow != own || !entryHasValue(e, row, want) {
			continue
		}
		if e.Status == domain.StatusSUCCESS {
			break
		}
		if e.Status == domain.StatusHARNESS_ERROR {
			used++
		}
	}
	return max(HarnessErrorAttemptLimit-used, 0)
}

// harnessErrorDecision routes after a HARNESS_ERROR attempt. The attempt never
// moved current_state, so its position comes from its own Execution Log entry:
// the newest entry of the responding instance.
func harnessErrorDecision(in NextInput, sc ScheduleContext) domain.EngineDecision {
	resp := *in.LastResponse
	pos, err := harnessErrorPosition(in.Workflow, in.State, resp.AgentInstanceID)
	if in.Mode == domain.ExecutionModeOrchestrated {
		return orchestratedDecisionAt(in, pos)
	}
	if err != nil {
		return domain.EngineDecision{Stop: &domain.StopDecision{Reason: err.Error(), Err: err}}
	}
	return nonSuccessDecision(in, sc, pos, in.Workflow.Table.Rows[pos.rowIdx], resp.StatusCode, resp)
}

// harnessErrorPosition resolves the row and iteration of the newest log entry
// of agentInstance. An attempt that is absent from the log or cannot be placed
// in the table is an error; no row is guessed.
func harnessErrorPosition(
	workflow domain.AdmittedWorkflow,
	state domain.ArtifactState,
	agentInstance string,
) (position, error) {
	for i := len(state.ExecutionLog) - 1; i >= 0; i-- {
		e := state.ExecutionLog[i]
		if e.Agent != agentInstance {
			continue
		}
		pos, err := resolvePosition(workflow, e)
		if err == nil && pos.rowIdx < 0 {
			err = recordedRowError(e, domain.CauseNoMatchingRow)
		}
		if err != nil {
			return position{rowIdx: -1, entry: e, stage: e.Stage}, err
		}
		return pos, nil
	}
	return position{rowIdx: -1}, &domain.PositionUnresolvedError{
		AgentInstance: agentInstance,
		Cause:         domain.CauseNoRecordedRow,
	}
}
