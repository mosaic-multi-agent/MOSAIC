package session

import (
	"context"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// isMechanicalRetryMode reports whether the engine re-dispatches retryable
// failures (PARTIALLY_DONE, BLOCKED/E501) itself in this execution mode. In
// orchestrated mode every such failure leads to a consultation instead.
func isMechanicalRetryMode(mode domain.ExecutionMode) bool {
	return mode == domain.ExecutionModeAuto || mode == domain.ExecutionModeAutoReview
}

// e501BudgetLeft reports whether the Execution Log still has E501 attempts
// left at the given workflow row and recorded stage value. The value is read
// with the table row at rowIndex; a row the table lacks has no budget.
func e501BudgetLeft(state domain.ArtifactState, table domain.RoutingTable, rowIndex int, stage string) bool {
	row, ok := rowAtIndex(table, rowIndex)
	if !ok {
		return false
	}
	return engine.E501BudgetRemaining(state.ExecutionLog, row, stage) > 0
}

// bypassPermitted decides whether a raw-text bypass re-dispatch may run after
// a harness error at rowIndex. The bypass is one attempt of the E501 budget in
// the mechanical-retry modes, so the budget owns the bound there; the
// anti-loop guard only counts the dispatch. In orchestrated mode the guard
// owns the bound.
func bypassPermitted(
	state domain.ArtifactState, table domain.RoutingTable, rowIndex int, stage, agentID string, antiLoop *antiLoopState,
) bool {
	allowed := antiLoop.recordDispatch(rowIndex, agentID)
	if isMechanicalRetryMode(state.Mode) {
		return e501BudgetLeft(state, table, rowIndex, stage)
	}
	return allowed
}

// recordFailedBypass persists a failed raw-text bypass attempt as its own
// BLOCKED/E501 Execution Log row under the bypass agent instance, so the
// attempt counts against the E501 budget.
func (s *sessionImpl) recordFailedBypass(ctx context.Context, state *domain.ArtifactState, req domain.ProtocolRequest, phase, stage string, rowIndex int, bypassErr error) error {
	failed := domain.CompletedStep{
		Seq:           state.GlobalSequence + 1,
		AgentInstance: req.AgentInstanceID,
		Phase:         phase,
		Stage:         stage,
		WorkflowRow:   domain.WorkflowRowFromIndex(rowIndex),
		Status:        domain.StatusBLOCKED,
		ErrorCode:     domain.ErrorTOOL_UNAVAILABLE,
		Summary:       bypassErr.Error(),
		Timestamp:     s.deps.Clock.Now(),
		Inputs:        formatInputs(req.InputArtifacts),
	}
	newState, err := s.deps.Store.Apply(ctx, *state, failed)
	if err != nil {
		s.deps.Debug.Log(domain.EventSessionApplyFailed, err.Error())
		return err
	}
	*state = newState
	return nil
}
