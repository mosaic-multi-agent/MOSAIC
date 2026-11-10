package session

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

// forceStopError marks the error of an invocation that a force stop killed.
// invokeAndLog returns it instead of the harness error, so every invocation
// site can tell a force stop from a harness error and from a hard cancel.
type forceStopError struct{ err error }

func (e *forceStopError) Error() string { return e.err.Error() }
func (e *forceStopError) Unwrap() error { return e.err }

// isForceStopped reports whether err is the error of a force-stopped invocation.
func isForceStopped(err error) bool {
	var fe *forceStopError
	return errors.As(err, &fe)
}

// Targets of the force-stop debug event.
const (
	forceStopTargetDispatch       = "dispatch"
	forceStopTargetInfrastructure = "infrastructure"
)

// forcedAttempt describes the killed attempt a force-stopped row records.
type forcedAttempt struct {
	req   domain.ProtocolRequest
	phase string
	stage string
	row   domain.WorkflowRow
	infra bool
}

// attemptOfStep describes the killed attempt of a workflow dispatch step whose
// request is req (the step's own, or a bypass or re-dispatch of it).
func attemptOfStep(step domain.DispatchStep, req domain.ProtocolRequest) forcedAttempt {
	return forcedAttempt{
		req:   req,
		phase: step.Phase,
		stage: step.Stage,
		row:   domain.WorkflowRowFromIndex(step.RowIndex),
	}
}

// instanceSeq reads the sequence number an agent instance identifier ends in.
func instanceSeq(instanceID string, fallback int) int {
	i := strings.LastIndex(instanceID, "#")
	if i < 0 {
		return fallback
	}
	n, err := strconv.Atoi(instanceID[i+1:])
	if err != nil {
		return fallback
	}
	return n
}

// recordForceStop records the killed attempt as an unaccepted Execution Log
// row and advances state to the recorded state.
func (s *sessionImpl) recordForceStop(ctx context.Context, state *domain.ArtifactState, a forcedAttempt) error {
	target := forceStopTargetDispatch
	if a.infra {
		target = forceStopTargetInfrastructure
	}
	s.deps.Debug.Log(domain.EventSessionForceStop, "invocation force-stopped by the user",
		domain.F("agent", a.req.AgentInstanceID),
		domain.F("target", target),
	)
	step := domain.CompletedStep{
		Seq:              instanceSeq(a.req.AgentInstanceID, state.GlobalSequence+1),
		AgentInstance:    a.req.AgentInstanceID,
		Phase:            a.phase,
		Stage:            a.stage,
		WorkflowRow:      a.row,
		Status:           domain.StatusPARTIALLY_DONE,
		Summary:          domain.ForceStoppedSummary,
		Timestamp:        s.deps.Clock.Now(),
		Inputs:           formatInputs(a.req.InputArtifacts),
		IsInfrastructure: a.infra,
		ForceStopped:     true,
	}
	newState, err := s.applyAndAnnounce(ctx, *state, step)
	if err != nil {
		s.deps.Debug.Log(domain.EventSessionApplyFailed, err.Error())
		return err
	}
	*state = newState
	return nil
}

// forceStopStopOutcome is the resumable stop after a force stop.
func forceStopStopOutcome(instanceID string) domain.RunOutcome {
	return domain.RunOutcome{
		Status:  domain.RunStopped,
		Message: "run stopped after force stop of " + instanceID,
	}
}

// askForceStopNext asks the user what happens after a force stop and reports
// whether the manual routing dialogue was chosen. Anything but an answered
// manual choice (including no manual resolver wired) means stop.
func (s *sessionImpl) askForceStopNext(ctx context.Context, instanceID string) (manual bool) {
	ans, err :=s.deps.Interact.SelectOne(ctx, interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:              domain.QuestionForceStopNext,
			Title:           "Force stop",
			Prompt:          fmt.Sprintf("%s was force-stopped. What next?", instanceID),
			DefaultOptionID: domain.ForceStopNextStop,
		},
		Options: []interaction.Option{
			{ID: domain.ForceStopNextStop, Label: "Stop the run (resumable)"},
			{ID: domain.ForceStopNextManual, Label: "Open the manual routing dialogue"},
		},
	})
	return err == nil && ans.Status == interaction.Answered && ans.OptionID == domain.ForceStopNextManual &&
		s.deps.Manual != nil
}

// afterForceStop is the post-kill choice. It returns the resumable stop, or
// runs route with the manual routing dialogue answering the next decision.
// last receives the force-stopped summary as the last response.
func (s *sessionImpl) afterForceStop(
	ctx context.Context, instanceID string, last **domain.ProtocolResponse,
	route func() (bool, domain.RunOutcome, error),
) (done bool, out domain.RunOutcome, err error) {
	if !s.askForceStopNext(ctx, instanceID) {
		return true, forceStopStopOutcome(instanceID), nil
	}
	*last = &domain.ProtocolResponse{StatusMessage: domain.ForceStoppedSummary}
	s.manualDispatchPending = true
	return route()
}

// forceStopAttempt handles a force-stopped workflow invocation: it records the
// attempt, withdraws it from the anti-loop count when counted, and runs the
// post-kill choice.
func (s *sessionImpl) forceStopAttempt(
	ctx context.Context, a forcedAttempt, counted bool, state *domain.ArtifactState, seq *int,
	last **domain.ProtocolResponse, antiLoop *antiLoopState,
	route func() (bool, domain.RunOutcome, error),
) (done bool, out domain.RunOutcome, err error) {
	if counted {
		antiLoop.forgetLastDispatch()
	}
	if recErr := s.recordForceStop(ctx, state, a); recErr != nil {
		return true, domain.RunOutcome{Status: domain.RunFailed, Message: recErr.Error()}, recErr
	}
	*seq = state.GlobalSequence
	return s.afterForceStop(ctx, a.req.AgentInstanceID, last, route)
}

// autoForceStop is forceStopAttempt for the engine-routed paths, which keep
// their loop state on rs.
func (s *sessionImpl) autoForceStop(ctx context.Context, rs *runStartCtx, a forcedAttempt, counted bool) (bool, domain.RunOutcome, error) {
	return s.forceStopAttempt(ctx, a, counted, &rs.state, &rs.seq, &rs.lastResponse, &rs.antiLoop,
		func() (bool, domain.RunOutcome, error) { return s.routeFromRS(ctx, rs) })
}

// routeFromRS runs one routing consultation cycle over the loop state of rs.
func (s *sessionImpl) routeFromRS(ctx context.Context, rs *runStartCtx) (bool, domain.RunOutcome, error) {
	return s.consultRoute(ctx, nil, &rs.state, &rs.seq,
		&rs.lastResponse, &rs.prevWorkflowStep, &rs.stages,
		rs.table, rs.agents, rs.config, rs.declaredInfraAgents, rs.admitted, &rs.antiLoop)
}

// pendingInfraForceStop is the post-kill choice of a force-stopped
// infrastructure agent, held until the trigger evaluation that dispatched it
// has returned to a caller able to act on it.
type pendingInfraForceStop struct {
	instanceID string
	manual     bool
}

// infraForceStop handles a force-stopped infrastructure agent: it records the
// attempt, asks the post-kill choice and parks it for the trigger caller. The
// returned haltRun makes the trigger evaluation return at once.
func (s *sessionImpl) infraForceStop(ctx context.Context, state *domain.ArtifactState, seq *int, a forcedAttempt) error {
	a.infra = true
	if err := s.recordForceStop(ctx, state, a); err != nil {
		return err
	}
	*seq = state.GlobalSequence
	s.pendingInfra = &pendingInfraForceStop{
		instanceID: a.req.AgentInstanceID,
		manual:     s.askForceStopNext(ctx, a.req.AgentInstanceID),
	}
	return nil
}

// followUpInfraForceStop acts on a parked infrastructure force stop after a
// halt from the trigger evaluation. handled is false when none is parked.
func (s *sessionImpl) followUpInfraForceStop(
	last **domain.ProtocolResponse, route func() (bool, domain.RunOutcome, error),
) (done bool, out domain.RunOutcome, handled bool, err error) {
	p := s.pendingInfra
	s.pendingInfra = nil
	if p == nil {
		return false, domain.RunOutcome{}, false, nil
	}
	if !p.manual {
		return true, forceStopStopOutcome(p.instanceID), true, nil
	}
	*last = &domain.ProtocolResponse{StatusMessage: domain.ForceStoppedSummary}
	s.manualDispatchPending = true
	done, out, err = route()
	return done, out, true, err
}

// antiLoopSnapshot is the anti-loop counters before a counted dispatch.
type antiLoopSnapshot struct {
	valid       bool
	rowIndex    int
	count       int
	lastAgent   string
	escalations int
}

// forgetLastDispatch withdraws the last counted dispatch from the counters, as
// if it had not happened. It does nothing when that dispatch was not counted.
func (a *antiLoopState) forgetLastDispatch() {
	b := a.before
	a.before.valid = false
	if !b.valid {
		return
	}
	a.rowIndex, a.count, a.lastAgent, a.escalations = b.rowIndex, b.count, b.lastAgent, b.escalations
}
