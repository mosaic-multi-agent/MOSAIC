package session

import (
	"context"
	"strconv"

	"mosaic-run/internal/domain"
)

// inRunManualState holds an in-run manual-dispatch request that has been taken
// from the request source but not yet applied to a resolver. It exists because
// a decision may need to know that a manual resolver will answer (to skip the
// "no routing consultant" refusal) before the resolver is selected.
type inRunManualState struct {
	held bool
	// replaced is the engine step the manual dialogue stands in for; nil when
	// the request applies to a routing consultation.
	replaced *domain.DispatchStep
}

// manualResolverReady reports whether a manual resolver will answer the
// current routing decision: through the stop-screen one-shot, or through an
// in-run request, which is taken here and held until the resolver is selected.
func (s *sessionImpl) manualResolverReady() bool {
	if s.deps.Manual == nil {
		return false
	}
	return s.manualDispatchPending || s.takeInRunManual()
}

// takeInRunManual takes the in-run request (or reports one already held).
// It never takes when no manual resolver exists, so the request is not lost.
func (s *sessionImpl) takeInRunManual() bool {
	if s.deps.Manual == nil {
		return false
	}
	if s.inRunManual.held {
		return true
	}
	if !s.deps.TakeManualDispatchRequest() {
		return false
	}
	s.inRunManual.held = true
	return true
}

// useManualResolver decides whether the manual resolver answers req. The
// stop-screen one-shot takes precedence over the in-run request. Applying an
// in-run request is logged before the dialogue is asked.
func (s *sessionImpl) useManualResolver(req domain.ConsultationRequest) bool {
	if s.deps.Manual == nil {
		return false
	}
	if s.manualDispatchPending {
		s.manualDispatchPending = false
		return true
	}
	if !s.takeInRunManual() {
		return false
	}
	applied := s.inRunManual
	s.inRunManual = inRunManualState{}
	s.logInRunManualApplied(req, applied)
	return true
}

func (s *sessionImpl) logInRunManualApplied(req domain.ConsultationRequest, applied inRunManualState) {
	kind := ""
	if req.Deviation != nil {
		kind = string(req.Deviation.Kind)
	}
	fields := []domain.DebugField{
		domain.F("request", string(domain.InRunRequestManualDispatch)),
		domain.F("kind", kind),
	}
	msg := "in-run manual dispatch request applied to a routing consultation"
	decision := "routing_consultation"
	if applied.replaced != nil {
		msg = "in-run manual dispatch request replaced the engine's dispatch"
		decision = "engine_dispatch"
		fields = append(fields,
			domain.F("row", strconv.Itoa(int(domain.WorkflowRowFromIndex(applied.replaced.RowIndex)))),
			domain.F("stage", applied.replaced.Stage),
		)
	}
	fields = append(fields, domain.F("decision", decision))
	s.deps.Debug.Log(domain.EventSessionRequestApplied, msg, fields...)
}

// divertEngineDispatch replaces a new-step engine Dispatch with a manual
// routing-consultation cycle when an in-run request is pending. It reports
// diverted=false, touching nothing, for mechanical re-dispatches, when no
// manual resolver exists, when a graceful stop is confirmed (that checkpoint
// runs as usual) and when the stop-screen one-shot is still pending.
func (s *sessionImpl) divertEngineDispatch(ctx context.Context, rs *runStartCtx, step domain.DispatchStep) (out domain.RunOutcome, done, diverted bool, err error) {
	if step.Retry != domain.RetryNone || s.deps.Manual == nil || s.manualDispatchPending || s.deps.StopRequested() {
		return domain.RunOutcome{}, false, false, nil
	}
	if !s.takeInRunManual() {
		return domain.RunOutcome{}, false, false, nil
	}
	s.inRunManual.replaced = &step
	rs.antiLoop.escalations = 0
	cDone, cOut, cErr := s.consultRoute(ctx, nil, &rs.state, &rs.seq,
		&rs.lastResponse, &rs.prevWorkflowStep, &rs.stages,
		rs.table, rs.agents, rs.config, rs.declaredInfraAgents, rs.admitted, &rs.antiLoop)
	return cOut, cDone, true, cErr
}
