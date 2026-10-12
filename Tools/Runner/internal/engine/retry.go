package engine

import "mosaic-run/internal/domain"

// PartiallyDoneRedispatchLimit is N, the Runner-only bound on mechanical
// re-dispatches of one assignment that returned PARTIALLY_DONE in the auto and
// auto-review modes. With k the length of the trailing PARTIALLY_DONE run at
// the last step's row and stage, the engine re-dispatches while
// k <= PartiallyDoneRedispatchLimit and deviates when k > it. The native
// policy has no cap; after the bound the orchestrator applies the unchanged
// policy with judgement.
const PartiallyDoneRedispatchLimit = 3

// E501AttemptLimit is the native Tier 1 budget: at most three BLOCKED/E501
// attempts per workflow row and stage since the last SUCCESS there.
const E501AttemptLimit = 3

// ScheduleDefectRedispatchLimit bounds schedule-defect re-dispatches of one
// producing or writer row at one iteration value: 2 re-dispatches, 3 attempts
// in total.
const ScheduleDefectRedispatchLimit = 2

// ScheduleDefectAttempts counts the SUCCESS entries of row at the recorded
// value in the trailing run of log: the longest suffix holding no entry of
// another routing-table row. Entries with NoWorkflowRow are skipped; entries of
// row with another status, another value or an unreadable value neither end the
// run nor count. Values are compared as parsed iterations read with row, so the
// pre-3.0 and 3.0 spellings of one iteration count together. When recorded
// cannot be read with row the result is the exhausted value
// ScheduleDefectRedispatchLimit + 1. Re-dispatches used = attempts - 1.
func ScheduleDefectAttempts(log []domain.ExecutionLogEntry, row domain.RoutingRow, recorded string) int {
	want, err := domain.ReadIterationValue(recorded, row)
	if err != nil {
		return ScheduleDefectRedispatchLimit + 1
	}
	own := domain.WorkflowRowFromIndex(row.Index)
	attempts := 0
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		if e.WorkflowRow == domain.NoWorkflowRow {
			continue
		}
		if e.WorkflowRow != own {
			break
		}
		if e.Status == domain.StatusSUCCESS && entryHasValue(e, row, want) {
			attempts++
		}
	}
	return attempts
}

// PartiallyDoneContextPrefix introduces the previous status message that a
// PARTIALLY_DONE re-dispatch carries in its task description.
const PartiallyDoneContextPrefix = "Your previous invocation of this assignment returned PARTIALLY_DONE. Its status message was:"

// E501BudgetRemaining returns E501AttemptLimit minus the number of BLOCKED log
// rows with ErrorCode E501 at (row, recorded value) after the last SUCCESS row
// at the same (row, value); never negative. Rows at other rows or values, rows
// whose value cannot be read with row, rows with NoWorkflowRow, and BLOCKED
// rows with another or no error code are ignored. Values are compared as parsed
// iterations, so both spellings of one iteration share one budget. recorded is
// the step's recorded value ("" outside every block); when it cannot be read
// with row the budget is exhausted (0).
func E501BudgetRemaining(log []domain.ExecutionLogEntry, row domain.RoutingRow, recorded string) int {
	want, err := domain.ReadIterationValue(recorded, row)
	if err != nil {
		return 0
	}
	own := domain.WorkflowRowFromIndex(row.Index)
	used := 0
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		if e.WorkflowRow != own || !entryHasValue(e, row, want) {
			continue
		}
		if e.Status == domain.StatusSUCCESS {
			break
		}
		if e.Status == domain.StatusBLOCKED && e.ErrorCode == domain.ErrorTOOL_UNAVAILABLE {
			used++
		}
	}
	return max(E501AttemptLimit-used, 0)
}

// partiallyDoneRunLength is k: the length of the trailing run of
// PARTIALLY_DONE log rows at (row, value). Rows without a recorded workflow
// row (infrastructure and ad-hoc steps) are skipped; any other row, or a row of
// another or unreadable value, ends the run.
func partiallyDoneRunLength(
	log []domain.ExecutionLogEntry, row domain.RoutingRow, want domain.IterationValue,
) int {
	own := domain.WorkflowRowFromIndex(row.Index)
	k := 0
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		if e.WorkflowRow == domain.NoWorkflowRow {
			continue
		}
		if e.WorkflowRow != own || e.Status != domain.StatusPARTIALLY_DONE || !entryHasValue(e, row, want) {
			break
		}
		k++
	}
	return k
}

// mechanicalRetry decides the auto-mode re-dispatch of the same assignment
// after a PARTIALLY_DONE or BLOCKED/E501 step. ok is false when the step is not
// retryable or the bound or budget is used up, in which case the caller
// deviates. All counting comes from the Execution Log.
func mechanicalRetry(
	in NextInput,
	sc ScheduleContext,
	pos position,
	status domain.StatusCode,
) (dec domain.EngineDecision, ok bool) {
	row, inTable := tableRow(in.Workflow, pos.rowIdx)
	if !inTable {
		row = domain.RoutingRow{Index: pos.rowIdx}
	}
	log := in.State.ExecutionLog
	var kind domain.RetryKind
	switch status {
	case domain.StatusPARTIALLY_DONE:
		if partiallyDoneRunLength(log, row, pos.value) > PartiallyDoneRedispatchLimit {
			return domain.EngineDecision{}, false
		}
		kind = domain.RetryPartiallyDone
	case domain.StatusBLOCKED:
		if lastErrorCode(in, pos) != domain.ErrorTOOL_UNAVAILABLE ||
			E501BudgetRemaining(log, row, pos.stage) <= 0 {
			return domain.EngineDecision{}, false
		}
		kind = domain.RetryToolUnavailable
	default:
		return domain.EngineDecision{}, false
	}
	// A re-dispatched row resolves its defaults over the schedules of the
	// original attempt, including a block schedule withheld from routing.
	if len(in.RetrySchedules) > 0 {
		merged := make(domain.BlockSchedules, len(sc.Schedules)+len(in.RetrySchedules))
		for name, sched := range in.RetrySchedules {
			merged[name] = sched
		}
		for name, sched := range sc.Schedules {
			merged[name] = sched
		}
		sc.Schedules = merged
	}
	step, err := buildDispatchStep(in.Workflow, pos.rowIdx, pos.iteration,
		in.Agents, in.Seq, in.Now, sc)
	if err != nil {
		return domain.EngineDecision{Stop: &domain.StopDecision{Reason: err.Error()}}, true
	}
	step.Retry = kind
	if kind == domain.RetryPartiallyDone {
		addPartiallyDoneContext(&step, in, pos)
	}
	return domain.EngineDecision{Dispatch: &domain.DispatchDecision{Steps: []domain.DispatchStep{step}}}, true
}

// lastErrorCode is the error code of the last step: from the live response, or
// on resume from the recorded log entry (falling back to the current state).
func lastErrorCode(in NextInput, pos position) domain.ErrorCode {
	if in.LastResponse != nil {
		return in.LastResponse.ErrorCode
	}
	if pos.entry.ErrorCode != domain.ErrorNone {
		return pos.entry.ErrorCode
	}
	return in.State.CurrentState.ErrorCode
}

// addPartiallyDoneContext hands the previous output to the re-dispatched agent:
// the previous output artifacts join the inputs and the previous status message
// is appended to the task description.
func addPartiallyDoneContext(step *domain.DispatchStep, in NextInput, pos position) {
	msg := pos.entry.Summary
	if in.LastResponse != nil {
		msg = in.LastResponse.StatusMessage
	}
	step.Request.InputArtifacts = injectReviewArtifacts(in.State.RunID, step.Request.InputArtifacts, in.LastOutputArtifacts)
	step.Request.TaskDescription = partiallyDoneTaskDescription(in.Workflow.Table.Rows[step.RowIndex], msg)
}
