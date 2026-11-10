package session

import (
	"strconv"

	"mosaic-run/internal/domain"
)

// raisedStep identifies the workflow step the in-run HITL request was applied
// to, so that mechanical re-dispatches of that same step keep its raised HITL
// value. It is replaced at every new workflow-step dispatch.
type raisedStep struct {
	active   bool
	rowIndex int
	agent    string
}

// takeHITLForNewStep is called once per new workflow-step dispatch (engine
// Dispatch with no retry, or a consultant/manual-routed dispatch), after every
// other HITL resolution, with the HITL value the step would otherwise run with.
// It returns the value the step must run with.
//
// The request can only raise HITL: it is consumed by the first new workflow
// step after it was made and that step runs with HITL true. When the step
// already has HITL true the request is still consumed (it is used up by the
// step it was meant for and does not carry over to a later step) and the step
// is left unchanged; the debug log entry records prior_hitl=true in that case.
//
// Infrastructure dispatches, commit setup, consultations, pre-consultation and
// mechanical re-dispatches never call this function, so they neither receive
// nor consume the request.
func (s *sessionImpl) takeHITLForNewStep(rowIndex int, agent, stage string, hitl bool) bool {
	s.raised = raisedStep{}
	if !s.deps.TakeHITLRequest() {
		return hitl
	}
	s.raised = raisedStep{active: true, rowIndex: rowIndex, agent: agent}
	s.deps.Debug.Log(domain.EventSessionRequestApplied, "in-run HITL request applied to the next workflow step",
		domain.F("request", string(domain.InRunRequestHITLNext)),
		domain.F("agent", agent),
		domain.F("stage", stage),
		domain.F("row", strconv.Itoa(int(domain.WorkflowRowFromIndex(rowIndex)))),
		domain.F("prior_hitl", strconv.FormatBool(hitl)),
	)
	return true
}

// keepRaisedHITL is called by mechanical re-dispatches of the step currently
// running. It returns hitl unchanged unless the step is the one the request was
// applied to, in which case it stays raised.
func (s *sessionImpl) keepRaisedHITL(rowIndex int, agent string, hitl bool) bool {
	if s.raised.active && s.raised.rowIndex == rowIndex && s.raised.agent == agent {
		return true
	}
	return hitl
}

// applyHITLToEngineStep applies the request to an engine-built dispatch step,
// raising both the dispatched request and the effective HITL the gate checks.
func (s *sessionImpl) applyHITLToEngineStep(step domain.DispatchStep) domain.DispatchStep {
	var hitl bool
	if step.Retry == domain.RetryNone {
		hitl = s.takeHITLForNewStep(step.RowIndex, step.Agent.Identifier, step.Stage, step.EffectiveHITL)
	} else {
		hitl = s.keepRaisedHITL(step.RowIndex, step.Agent.Identifier, step.EffectiveHITL)
	}
	step.EffectiveHITL = hitl
	step.Request.HumanInTheLoop = hitl
	return step
}
