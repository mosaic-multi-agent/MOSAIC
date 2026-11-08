package session_test

// Tests for the force stop of a consultation reached after the manual routing
// dialogue answered a decision on demand, and of a deviation consultation: the
// post-kill choice works there, and the manual dialogue reuses the killed
// consultation's request, deviation included.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_ForceStopConsultation_AfterOnDemandManualDialogue_ChoiceStop_StopsResumably(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextStop, 1)
	r.cfg.ManualDispatch = true
	r.manual.queueDispatch("agent-a", "start", 0)
	r.f.Queue("agent-a", successEntry("agent-a finished"))

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	requireNothingRecordedBeyond(t, r.store, 1, "agent-a#1")
	if r.manual.CallCount != 1 {
		t.Errorf("want only the on-demand dialogue before the kill, got %d dialogues", r.manual.CallCount)
	}
	if n := r.user.postKillAsks(); n != 1 {
		t.Errorf("want the post-kill question asked once, got %d", n)
	}
}

func TestSession_ForceStopConsultation_AfterOnDemandManualDialogue_ChoiceManual_ContinuesTheDialogue(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextManual, 1)
	r.cfg.ManualDispatch = true
	r.manual.queueDispatch("agent-a", "start", 0)
	r.manual.queueDispatch("agent-b", "go on", 1)
	r.f.Queue("agent-a", successEntry("agent-a finished"))
	r.f.Queue("agent-b", successEntry("agent-b finished"))
	r.inner.queueStop("done")

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if r.manual.CallCount != 2 {
		t.Errorf("want the on-demand dialogue and the post-kill dialogue, got %d", r.manual.CallCount)
	}
	if n := countInvocationsFor(r.f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want the post-kill dialogue's answer dispatched once, got %d agent-b dispatches", n)
	}
	if n := len(forceStoppedSteps(r.store)); n != 0 {
		t.Errorf("want no force-stopped row for a consultation, got %d", n)
	}
}

func TestSession_ForceStopConsultation_DeviationConsultation_ChoiceStop_StopsWithNothingRecordedForIt(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextStop, 1)
	r.cfg.RunSettings.Mode = domain.ExecutionModeAuto
	r.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"))

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	if n := r.user.postKillAsks(); n != 1 {
		t.Errorf("want the post-kill question asked once, got %d", n)
	}
	if n := len(forceStoppedSteps(r.store)); n != 0 {
		t.Errorf("want no force-stopped row for a consultation, got %d", n)
	}
}

func TestSession_ForceStopConsultation_DeviationConsultation_ChoiceManual_ReusesTheDeviation(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextManual, 1)
	r.cfg.RunSettings.Mode = domain.ExecutionModeAuto
	r.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"))
	r.manual.queueStop("enough")

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if r.manual.CallCount != 1 {
		t.Fatalf("want one manual routing dialogue, got %d", r.manual.CallCount)
	}
	killed, asked := r.consultant.blockedRequest(t), r.manual.Requests[0]
	if killed.Deviation == nil {
		t.Fatal("want the killed consultation to carry a deviation")
	}
	if !reflect.DeepEqual(asked.Deviation, killed.Deviation) {
		t.Errorf("want the manual dialogue to receive the same deviation, got %+v and %+v", asked.Deviation, killed.Deviation)
	}
}
