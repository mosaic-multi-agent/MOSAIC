package session

import (
	"context"
	"errors"
	"fmt"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// failedAttempt is one dispatch attempt that ended in a harness-level failure
// (no usable subagent response), on either dispatch path.
type failedAttempt struct {
	agent    domain.AgentReference
	req      domain.ProtocolRequest
	phase    string
	stage    string
	rowIndex int
}

// classifyHarnessFailure names the root cause of an invocation error. The
// first matching kind wins. An unparseable-response detail carries only the
// sentinel, never the wrapped text, which may quote the agent's own output.
func classifyHarnessFailure(err error) domain.HarnessFailure {
	text := err.Error()
	switch {
	case errors.Is(err, commonharness.ErrTimeout):
		return domain.HarnessFailure{Kind: domain.HarnessFailureTimeout, Detail: text}
	case errors.Is(err, commonharness.ErrGHCPCLIAgentDeselected):
		return domain.HarnessFailure{Kind: domain.HarnessFailureGHCPAgentDeselected, Detail: text}
	case errors.Is(err, commonharness.ErrGHCPCLISessionError):
		return domain.HarnessFailure{Kind: domain.HarnessFailureGHCPSessionError, Detail: text}
	case errors.Is(err, commonharness.ErrGHCPCLIStreamError),
		errors.Is(err, commonharness.ErrGHCPCLIStreamIncomplete),
		errors.Is(err, commonharness.ErrOpenCodeStreamError),
		errors.Is(err, commonharness.ErrOpenCodeStreamIncomplete):
		return domain.HarnessFailure{Kind: domain.HarnessFailureStreamError, Detail: text}
	case errors.Is(err, commonharness.ErrEmptyResponse):
		return domain.HarnessFailure{Kind: domain.HarnessFailureEmpty, Detail: text}
	case errors.Is(err, commonharness.ErrMalformedJSON):
		return unparseableFailure(commonharness.ErrMalformedJSON)
	case errors.Is(err, commonharness.ErrProtocolNotExtractable):
		return unparseableFailure(commonharness.ErrProtocolNotExtractable)
	}
	var launchErr *domain.HarnessLaunchError
	if errors.Is(err, commonharness.ErrNonZeroExit) ||
		errors.Is(err, commonharness.ErrExecutableNotFound) ||
		errors.As(err, &launchErr) {
		return domain.HarnessFailure{Kind: domain.HarnessFailureProcess, Detail: text}
	}
	return domain.HarnessFailure{Kind: domain.HarnessFailureOther, Detail: text}
}

func unparseableFailure(sentinel error) domain.HarnessFailure {
	return domain.HarnessFailure{
		Kind:   domain.HarnessFailureUnparseable,
		Detail: "response received but not valid protocol JSON (" + sentinel.Error() + ")",
	}
}

// harnessErrorBudgetLeft reports whether the Execution Log still has
// HARNESS_ERROR attempts left at the given workflow row and recorded stage
// value. A row the table lacks has no budget.
func harnessErrorBudgetLeft(state domain.ArtifactState, table domain.RoutingTable, rowIndex int, stage string) bool {
	row, ok := rowAtIndex(table, rowIndex)
	if !ok {
		return false
	}
	return engine.HarnessErrorBudgetRemaining(state.ExecutionLog, row, stage) > 0
}

// recordHarnessFailure persists the failed attempt as a HARNESS_ERROR row,
// which never moves current_state and is never HITL-accepted, and returns the
// response routing and any consultation see for the attempt.
func (s *sessionImpl) recordHarnessFailure(ctx context.Context, state *domain.ArtifactState, a failedAttempt, invokeErr error) (domain.ProtocolResponse, error) {
	failure := classifyHarnessFailure(invokeErr)
	s.deps.Debug.Log(domain.EventSessionHarnessError, failure.Summary(),
		domain.F("agent", a.req.AgentInstanceID),
		domain.F("status", string(domain.StatusHARNESS_ERROR)),
		domain.F("kind", string(failure.Kind)),
	)
	if err := s.recordHarnessFailureRow(ctx, state, a.req, a.phase, a.stage, a.rowIndex, failure); err != nil {
		return domain.ProtocolResponse{}, err
	}
	return domain.HarnessFailureResponse(a.req.AgentInstanceID, state.RunID, failure), nil
}

// recordHarnessFailureRow applies one HARNESS_ERROR row for req.
func (s *sessionImpl) recordHarnessFailureRow(ctx context.Context, state *domain.ArtifactState, req domain.ProtocolRequest, phase, stage string, rowIndex int, failure domain.HarnessFailure) error {
	row := domain.CompletedStep{
		Seq:           state.GlobalSequence + 1,
		AgentInstance: req.AgentInstanceID,
		Phase:         phase,
		Stage:         stage,
		WorkflowRow:   domain.WorkflowRowFromIndex(rowIndex),
		Status:        domain.StatusHARNESS_ERROR,
		ErrorCode:     domain.ErrorNone,
		Summary:       failure.Summary(),
		Timestamp:     s.deps.Clock.Now(),
		Inputs:        formatInputs(req.InputArtifacts),
	}
	newState, err := s.applyAndAnnounce(ctx, *state, row)
	if err != nil {
		s.deps.Debug.Log(domain.EventSessionApplyFailed, err.Error())
		return err
	}
	*state = newState
	return nil
}

// harnessErrorDeviation is the deviation handed to a consultation once the
// failed attempt's mechanical routing is exhausted.
func harnessErrorDeviation(a failedAttempt, resp domain.ProtocolResponse, state domain.ArtifactState) domain.DeviationInfo {
	return domain.DeviationInfo{
		Kind:          domain.DeviationNonSuccess,
		Response:      resp,
		CurrentRow:    a.rowIndex,
		CurrentPhase:  a.phase,
		CurrentStage:  a.stage,
		ArtifactState: state,
	}
}

// bypassKind is how a raw-text bypass attempt ended.
type bypassKind int

const (
	bypassSkipped      bypassKind = iota // not eligible or not permitted; nothing ran
	bypassSucceeded                      // the retry returned a response
	bypassForceStopped                   // the user force-stopped the retry
	bypassFailed                         // the retry failed and its row is recorded
	bypassRecordFailed                   // the retry failed and its row could not be recorded
)

// bypassOutcome is the result of attemptRawTextBypass.
type bypassOutcome struct {
	kind bypassKind
	req  domain.ProtocolRequest // the retry's request (bypass agent instance)
	resp domain.ProtocolResponse
	seq  int   // the retry's sequence number
	err  error // set for bypassRecordFailed
}

// attemptRawTextBypass runs the single direct retry that follows an
// unparseable or empty response, when the error is eligible and the budget
// (mechanical modes) or the anti-loop guard (orchestrated mode) permits it. A
// failed retry is recorded as its own HARNESS_ERROR row under the bypass
// instance, counting against the same budget.
func (s *sessionImpl) attemptRawTextBypass(ctx context.Context, state *domain.ArtifactState, table domain.RoutingTable, a failedAttempt, invokeErr error, antiLoop *antiLoopState) bypassOutcome {
	if !isRawTextHarnessError(invokeErr) || !bypassPermitted(*state, table, a.rowIndex, a.stage, a.agent.Identifier, antiLoop) {
		return bypassOutcome{kind: bypassSkipped}
	}
	return s.runRawTextBypass(ctx, state, a)
}

// runRawTextBypass dispatches the bypass retry of a (under a new agent
// instance, with the same request) and records its failure as its own row.
func (s *sessionImpl) runRawTextBypass(ctx context.Context, state *domain.ArtifactState, a failedAttempt) bypassOutcome {
	seq := state.GlobalSequence + 1
	req := a.req
	req.AgentInstanceID = fmt.Sprintf("%s#%d", a.agent.Identifier, seq)
	resp, err := s.invokeAndLog(ctx, a.agent, req)
	switch {
	case err == nil:
		return bypassOutcome{kind: bypassSucceeded, req: req, resp: resp, seq: seq}
	case isForceStopped(err):
		return bypassOutcome{kind: bypassForceStopped, req: req, seq: seq}
	}
	if recErr := s.recordFailedBypass(ctx, state, req, a.phase, a.stage, a.rowIndex, err); recErr != nil {
		return bypassOutcome{kind: bypassRecordFailed, req: req, seq: seq, err: recErr}
	}
	return bypassOutcome{kind: bypassFailed, req: req, seq: seq}
}
