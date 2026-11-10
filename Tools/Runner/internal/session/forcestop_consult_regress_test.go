package session_test

// Regression tests: a consultation that errors, times out or is hard-cancelled
// is not a force stop. It keeps the existing consultant-stop path (manual
// fallback only with ManualResolution), and a late force-stop request does not
// turn a delivered answer into a stop.

import (
	"context"
	"errors"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

func TestSession_ConsultationFailure_NotForceStopped_KeepsTheConsultantStopPath(t *testing.T) {
	for _, failure := range []domain.ConsultationFailure{domain.ConsultFailTransport, domain.ConsultFailMalformedJSON} {
		t.Run(string(failure), func(t *testing.T) {
			r := newConsultForceRig(t, domain.ForceStopNextManual, 99)
			r.inner.queueError(failure)

			got, err := r.ses.Start(context.Background(), r.cfg)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := r.user.postKillAsks(); n != 0 {
				t.Errorf("want no post-kill question for a consultation failure, got %d", n)
			}
			if r.manual.CallCount != 0 {
				t.Errorf("want no manual fallback without ManualResolution, got %d", r.manual.CallCount)
			}
			if _, forced := r.logger.fieldValue(domain.EventSessionForceStop, "target"); forced {
				t.Error("want no force-stop event for a consultation failure")
			}
		})
	}
}

func TestSession_ConsultationFailure_NotForceStopped_ManualFallbackOnlyWithManualResolution(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextManual, 99)
	r.cfg.ManualResolution = true
	r.inner.queueError(domain.ConsultFailTransport)
	r.manual.queueStop("manual says stop")

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if r.manual.CallCount != 1 {
		t.Errorf("want the manual fallback dialogue opened once, got %d", r.manual.CallCount)
	}
	if got.StopReason != "manual says stop" {
		t.Errorf("want the manual dialogue's stop, got %q", got.StopReason)
	}
	if n := r.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question, got %d", n)
	}
}

// A consultation that blocks and then fails by itself, with a force-stop window
// open but never requested, is a plain failure.
func TestSession_ConsultationFailure_WindowOpenButNeverRequested_IsNotAForceStop(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextManual, 99, func(d *session.Deps) { d.Routing = failingAfterWindowConsultant{} })

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := r.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question, got %d", n)
	}
}

type failingAfterWindowConsultant struct{}

func (failingAfterWindowConsultant) ConsultRouting(context.Context, domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	return domain.RoutingInstruction{}, &domain.ConsultationError{Failure: domain.ConsultFailTransport, Detail: "timed out"}
}

// A consultation that ends with a bare deadline error while the window is open
// and unrequested is a timeout, not a force stop.
func TestSession_ConsultationFailure_DeadlineExceededWithoutRequest_IsNotAForceStop(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextManual, 99, func(d *session.Deps) { d.Routing = deadlineConsultant{} })

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := r.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question for a timeout, got %d", n)
	}
	if _, forced := r.logger.fieldValue(domain.EventSessionForceStop, "target"); forced {
		t.Error("want no force-stop event for a timeout")
	}
}

type deadlineConsultant struct{}

func (deadlineConsultant) ConsultRouting(context.Context, domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	return domain.RoutingInstruction{}, context.DeadlineExceeded
}

func TestSession_ConsultationHardCancel_IsNotAForceStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := newConsultForceRig(t, domain.ForceStopNextManual, 1)
	r.consultant.win = nil
	r.consultant.onBlock = cancel

	got, err := r.ses.Start(ctx, r.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := r.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question after a hard cancel, got %d", n)
	}
	if r.manual.CallCount != 0 {
		t.Errorf("want no manual dialogue after a hard cancel, got %d", r.manual.CallCount)
	}
	requireNothingRecordedBeyond(t, r.store, 0, "")
}

// A request that arrives after the consultant already produced its answer does
// not discard the answer.
func TestSession_ConsultationAnswerDelivered_LateForceStopRequest_IsAcceptedAsNormal(t *testing.T) {
	late := &lateRequestConsultant{}
	rig := newConsultForceRig(t, domain.ForceStopNextStop, 99, func(d *session.Deps) { d.Routing = late })
	late.inner, late.win = rig.inner, rig.win
	rig.inner.queueDispatch("agent-a", "start", 0)
	rig.inner.queueStop("done")
	rig.f.Queue("agent-a", successEntry("agent-a finished"))

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(rig.f.Invocations(), "agent-a"); n != 1 {
		t.Errorf("want the delivered dispatch carried out, got %d agent-a dispatches", n)
	}
	if n := rig.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question for a delivered answer, got %d", n)
	}
}

// lateRequestConsultant raises the force-stop request while answering, and
// returns its scripted answer regardless.
type lateRequestConsultant struct {
	inner *scriptedRoutingConsultant
	win   *forceWindow
}

func (c *lateRequestConsultant) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	instr, err := c.inner.ConsultRouting(ctx, req)
	c.win.Request()
	return instr, err
}

func TestSession_PreConsultation_NotForceStopped_FailureKeepsTheStartFailedPath(t *testing.T) {
	r := newPreForceRig(t, domain.ExecutionModeAuto)
	failing := &scriptedPreConsultant{err: errors.New("pre-consultation harness error")}
	ses := session.New(session.Deps{
		Harness: r.f, Store: r.store, Clock: fixedClock{t: epoch}, Interact: r.user, PreConsult: failing,
		Manual: r.manual, ForceStop: r.win,
	})

	got, err := ses.Start(context.Background(), r.cfg)

	requireStartFailed(t, got, err)
	if n := r.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question, got %d", n)
	}
}

func TestSession_PreConsultationHardCancel_IsNotAForceStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := newPreForceRig(t, domain.ExecutionModeAuto)
	r.pc.win = nil
	r.pc.onBlock = cancel

	got, err := r.ses.Start(ctx, r.cfg)

	requireStartFailed(t, got, err)
	if _, forced := r.logger.fieldValue(domain.EventSessionForceStop, "target"); forced {
		t.Error("want no force-stop event for a hard cancel")
	}
	if n := r.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question, got %d", n)
	}
}
