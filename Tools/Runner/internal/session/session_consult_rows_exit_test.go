package session_test

// A consultation announces its end on every outcome: a dispatch, a stop
// instruction, a consultation failure, a decision discarded by a graceful stop
// and a cancelled context. A consultation the session never enters (graceful
// stop confirmed before it) announces nothing.

import (
	"context"
	"errors"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

// armOnCall arms flag when it is asked to decide, then delegates: a stop
// confirmed while the consultation runs.
type armOnCall struct {
	inner domain.RoutingConsultant
	flag  *stopFlag
}

func (c *armOnCall) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.flag.arm()
	return c.inner.ConsultRouting(ctx, req)
}

// cancelOnCall cancels the run's context while it is asked to decide and
// returns the context error.
type cancelOnCall struct{ cancel context.CancelFunc }

func (c *cancelOnCall) ConsultRouting(ctx context.Context, _ domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.cancel()
	return domain.RoutingInstruction{}, ctx.Err()
}

func TestConsultAnnounce_StopInstruction_EndsTheConsultation(t *testing.T) {
	rig := newLinearConsultRig(t)
	rig.consult.queueStop("pause here")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireBracketed(t, rig.tr, evCall, 1)
	requireBalanced(t, rig.tr)
}

func TestConsultAnnounce_ConsultationFailure_EndsTheConsultation(t *testing.T) {
	rig := newLinearConsultRig(t)
	rig.consult.queueError(domain.ConsultFailTransport)

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireBracketed(t, rig.tr, evCall, 1)
	requireBalanced(t, rig.tr)
}

func TestConsultAnnounce_DecisionDiscardedByGracefulStop_EndsTheConsultation(t *testing.T) {
	flag := &stopFlag{}
	rig := newConsultRig(t, "linear-orch.md", linearAgents, func(d *session.Deps, _ *consultTrace) {
		d.StopRequested = flag.requested
		d.Routing = &armOnCall{inner: d.Routing, flag: flag}
	})
	rig.consult.queueDispatch("agent-a", "do the work", 0)

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireBracketed(t, rig.tr, evCall, 1)
	requireBalanced(t, rig.tr)
	if n := invocationCount(rig.f, "agent-a"); n != 0 {
		t.Errorf("agent-a invoked %d times, want 0: the decision was discarded", n)
	}
}

func TestConsultAnnounce_ContextCancelledDuringConsultation_EndsTheConsultation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rig := newConsultRig(t, "linear-orch.md", linearAgents, func(d *session.Deps, tr *consultTrace) {
		d.Routing = &tracedRouting{inner: &cancelOnCall{cancel: cancel}, tr: tr, evt: evCall}
	})

	rig.ses.Start(ctx, baseOrchestratedConfig(rig.orchPath)) //nolint:errcheck

	requireBracketed(t, rig.tr, evCall, 1)
	requireBalanced(t, rig.tr)
}

func TestConsultAnnounce_ConsultantErrorThatIsNotAConsultationError_EndsTheConsultation(t *testing.T) {
	rig := newConsultRig(t, "linear-orch.md", linearAgents, func(d *session.Deps, tr *consultTrace) {
		d.Routing = &tracedRouting{inner: failingConsultant{err: errors.New("consultant crashed")}, tr: tr, evt: evCall}
	})

	rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath)) //nolint:errcheck

	requireBracketed(t, rig.tr, evCall, 1)
	requireBalanced(t, rig.tr)
}

// failingConsultant answers every consultation with err.
type failingConsultant struct{ err error }

func (c failingConsultant) ConsultRouting(context.Context, domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	return domain.RoutingInstruction{}, c.err
}

func TestConsultAnnounce_StopConfirmedBeforeTheConsultation_AnnouncesNothing(t *testing.T) {
	flag := &stopFlag{}
	flag.arm()
	rig := newConsultRig(t, "linear-orch.md", linearAgents, func(d *session.Deps, _ *consultTrace) {
		d.StopRequested = flag.requested
	})
	rig.consult.queueDispatch("agent-a", "never asked", 0)

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	if rig.consult.CallCount != 0 {
		t.Fatalf("scenario did not run as intended: the consultant was asked %d times", rig.consult.CallCount)
	}
	if n := rig.tr.count(evStart) + rig.tr.count(evEnd); n != 0 {
		t.Errorf("announced %d consultation events for a consultation that never ran: %v", n, rig.tr.snapshot())
	}
}

func TestConsultAnnounce_StopConfirmedWhileAnAgentRuns_AnnouncesNoSecondConsultation(t *testing.T) {
	flag := &stopFlag{}
	rig := newConsultRig(t, "linear-orch.md", linearAgents, func(d *session.Deps, _ *consultTrace) {
		d.StopRequested = flag.requested
		d.Harness = &armAfterInvokeHarness{delegate: d.Harness, flag: flag, agent: "agent-a"}
	})
	rig.consult.queueDispatch("agent-a", "first", 0)
	rig.consult.queueDispatch("agent-b", "never asked", 1)
	successAgentEntries(rig.f, "agent-a", 1)

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireBracketed(t, rig.tr, evCall, 1)
	requireBalanced(t, rig.tr)
	if rig.consult.CallCount != 1 {
		t.Errorf("want one consultation before the stop, got %d", rig.consult.CallCount)
	}
}
