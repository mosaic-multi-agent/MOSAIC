package session

import (
	"context"
	"errors"
	"strconv"

	"mosaic-common/interaction"
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
//
// The returned origin is non-nil only for a dialogue opened by the in-run
// request. Its cancel offer is set when Esc at the first step can cancel the
// request: for a replaced engine dispatch always (the engine runs the step), for
// a routing consultation only when a routing consultant can make the decision.
func (s *sessionImpl) useManualResolver(req domain.ConsultationRequest) (use bool, origin *inRunManualState) {
	if s.deps.Manual == nil {
		return false, nil
	}
	if s.manualDispatchPending {
		s.manualDispatchPending = false
		return true, nil
	}
	if !s.takeInRunManual() {
		return false, nil
	}
	applied := s.inRunManual
	s.inRunManual = inRunManualState{}
	s.logInRunManualApplied(req, applied)
	return true, &applied
}

// offersCancel reports whether Esc at the first step of the dialogue opened by
// this in-run request cancels it.
func (o *inRunManualState) offersCancel(routingWired bool) bool {
	return o != nil && (o.replaced != nil || routingWired)
}

func (s *sessionImpl) logInRunManualApplied(req domain.ConsultationRequest, applied inRunManualState) {
	msg := "in-run manual dispatch request applied to a routing consultation"
	if applied.replaced != nil {
		msg = "in-run manual dispatch request replaced the engine's dispatch"
	}
	s.deps.Debug.Log(domain.EventSessionRequestApplied, msg, inRunRequestFields(req, applied)...)
}

// inRunRequestFields are the debug fields shared by the events of an in-run
// request applied to a decision.
func inRunRequestFields(req domain.ConsultationRequest, applied inRunManualState) []domain.DebugField {
	kind := ""
	if req.Deviation != nil {
		kind = string(req.Deviation.Kind)
	}
	fields := []domain.DebugField{
		domain.F("request", string(domain.InRunRequestManualDispatch)),
		domain.F("kind", kind),
	}
	decision := "routing_consultation"
	if applied.replaced != nil {
		decision = "engine_dispatch"
		fields = append(fields,
			domain.F("row", strconv.Itoa(int(domain.WorkflowRowFromIndex(applied.replaced.RowIndex)))),
			domain.F("stage", applied.replaced.Stage),
		)
	}
	return append(fields, domain.F("decision", decision))
}

// consultManual runs the manual dialogue for req. For a dialogue opened by an
// in-run request that can be cancelled, Esc at its first step cancels the
// request: a replaced engine dispatch ends with errManualRequestCancelled, a
// routing consultation returns the instruction with CancelRequest set, which
// the caller answers by consulting the orchestrator as if never requested.
func (s *sessionImpl) consultManual(ctx context.Context, req domain.ConsultationRequest, origin *inRunManualState) (instr domain.RoutingInstruction, stop bool, err error) {
	asked := s.withAskOrchestratorOffer(req)
	asked.OfferCancelRequest = origin.offersCancel(s.deps.Routing != nil)
	instr, err = s.deps.Manual.ConsultRouting(ctx, asked)
	if err != nil || instr.CancelRequest == nil {
		return instr, false, err
	}
	s.reportManualCancel(ctx, req, *origin)
	if origin.replaced != nil {
		return domain.RoutingInstruction{}, false, errManualRequestCancelled
	}
	return instr, false, nil
}

// errManualRequestCancelled reports, from the consultation cycle of a replaced
// engine dispatch, that the user cancelled the request: the engine's dispatch
// runs as if the request had never been made.
var errManualRequestCancelled = errors.New("in-run manual dispatch request cancelled")

// reportManualCancel records that the user cancelled the in-run request in the
// dialogue and tells the progress screen.
func (s *sessionImpl) reportManualCancel(ctx context.Context, req domain.ConsultationRequest, origin inRunManualState) {
	s.deps.Debug.Log(domain.EventSessionRequestCancelled,
		"in-run manual dispatch request cancelled in the dialogue", inRunRequestFields(req, origin)...)
	s.deps.Interact.Notify(ctx, interaction.Notice{
		Level:   interaction.NoticeInfo,
		Title:   "Manual dispatch",
		Message: "Manual dispatch request cancelled; the run continues as if it had not been requested.",
	})
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
	if errors.Is(cErr, errManualRequestCancelled) {
		return domain.RunOutcome{}, false, false, nil
	}
	return cOut, cDone, true, cErr
}
