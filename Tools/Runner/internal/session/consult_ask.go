package session

import (
	"context"
	"errors"

	"mosaic-run/internal/domain"
)

// withAskOrchestratorOffer returns req for a manual routing dialogue the user
// asked for (in-run request, stop-screen Manual dispatch, post-force-stop
// choice). The dialogue offers handing the decision to the orchestrator only
// when a routing consultant is wired.
func (s *sessionImpl) withAskOrchestratorOffer(req domain.ConsultationRequest) domain.ConsultationRequest {
	req.OfferAskOrchestrator = s.deps.Routing != nil
	req.UserMessage = nil
	return req
}

// withUserMessage returns req carrying msg as the user message of this one
// consultation. The message lives only in the returned copy: the caller's
// request and the session state are untouched.
func withUserMessage(req domain.ConsultationRequest, msg *string) domain.ConsultationRequest {
	req.UserMessage = msg
	return req
}

// consultPrimary asks the orchestrator for the routing decision of req. A
// force-stopped consultation offers the stop or the user-requested manual
// dialogue (stop=true: the user chose to stop). A failed consultation falls
// back to the manual resolver when the run allows it; that fallback never
// offers handing the decision back to the orchestrator.
func (s *sessionImpl) consultPrimary(ctx context.Context, req domain.ConsultationRequest, config domain.RunConfig) (instr domain.RoutingInstruction, stop bool, err error) {
	req.OfferAskOrchestrator = false
	instr, err = s.consultOrchestratorAgent(ctx, req)
	// A consultation that left the artifact damaged is repaired before its
	// reply or failure is acted on: no fallback, no force-stop dialogue.
	if dc := s.damageAfterConsult(ctx, err); dc != nil {
		return domain.RoutingInstruction{}, false, dc
	}
	if isForceStopped(err) {
		return s.consultForceStopped(ctx, req)
	}
	if err != nil && config.ManualResolution && s.deps.Manual != nil {
		s.deps.Debug.Log(domain.EventSessionManualResolve, "primary consultation failed; trying manual resolver",
			domain.F("error", err.Error()),
		)
		instr, err = s.deps.Manual.ConsultRouting(ctx, req)
	} else if err != nil && config.ManualResolution {
		// Unreachable after the start-time port check; reported rather
		// than silently skipped.
		err = errors.Join(err, newManualMissing())
	}
	return instr, false, err
}
