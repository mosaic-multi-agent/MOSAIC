package session

import (
	"context"

	"mosaic-run/internal/domain"
)

// Targets of the force-stop debug event for consultations.
const (
	forceStopTargetConsultation    = "consultation"
	forceStopTargetPreConsultation = "preconsultation"
)

// consultationSubject names a force-stopped routing consultation in the
// post-kill question and the stop message.
const consultationSubject = "orchestrator consultation"

// preConsultForceStopMessage is the stop message of a force-stopped
// pre-consultation.
const preConsultForceStopMessage = "run stopped: pre-consultation force-stopped"

// watchConsultation runs call under a context the force-stop request cancels.
// A consultation that ends because the request killed it (and not because the
// run itself was cancelled) is reported as a forceStopError, whatever error the
// consultant wrapped the cancellation in.
func (s *sessionImpl) watchConsultation(ctx context.Context, target string, call func(context.Context) error) error {
	wctx, release := s.deps.ForceStop.Watch(ctx)
	err := call(wctx)
	forced := release()
	if forced && ctx.Err() == nil {
		s.deps.Debug.Log(domain.EventSessionForceStop, "consultation force-stopped by the user",
			domain.F("agent", consultationSubject),
			domain.F("target", target),
		)
		if err == nil {
			return nil
		}
		return &forceStopError{err: err}
	}
	return err
}

// consultForceStopped is the post-kill choice of a force-stopped routing
// consultation: the resumable stop (stop=true), or the manual routing dialogue
// for the same routing decision. Nothing was logged and the state did not move.
func (s *sessionImpl) consultForceStopped(ctx context.Context, req domain.ConsultationRequest) (instr domain.RoutingInstruction, stop bool, err error) {
	if !s.askForceStopNext(ctx, consultationSubject) {
		return domain.RoutingInstruction{}, true, nil
	}
	instr, err = s.deps.Manual.ConsultRouting(ctx, s.withAskOrchestratorOffer(req))
	return instr, false, err
}

// preConsultForceStopOutcome is the resumable stop of a force-stopped
// pre-consultation; a resume asks the pre-consultation again.
func preConsultForceStopOutcome() domain.RunOutcome {
	return domain.RunOutcome{Status: domain.RunStopped, Message: preConsultForceStopMessage}
}
