package session

import (
	"context"

	"mosaic-run/internal/domain"
)

// activityFor describes a routing consultation for req: its deviation kind
// when the request carries a deviation, none otherwise.
func activityFor(req domain.ConsultationRequest) domain.ConsultationActivity {
	a := domain.ConsultationActivity{Context: req.Context}
	if req.Deviation != nil {
		a.Kind = req.Deviation.Kind
	}
	return a
}

// consultOrchestratorAgent calls the orchestrator-agent routing consultant,
// announcing the consultation's start before the call and its end as soon as
// the call returns, whatever the outcome.
func (s *sessionImpl) consultOrchestratorAgent(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	s.deps.Consultations.ConsultationStarted(ctx, activityFor(req))
	defer s.deps.Consultations.ConsultationEnded(ctx)
	return s.deps.Routing.ConsultRouting(ctx, req)
}

// preConsultOrchestratorAgent calls the pre-consultant, announcing start and
// end around the call.
func (s *sessionImpl) preConsultOrchestratorAgent(ctx context.Context, req domain.ConsultationRequest) (domain.PreConsultationAdvice, error) {
	s.deps.Consultations.ConsultationStarted(ctx, domain.ConsultationActivity{Context: req.Context})
	defer s.deps.Consultations.ConsultationEnded(ctx)
	return s.deps.PreConsult.PreConsult(ctx, req)
}
