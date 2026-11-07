package session_test

// Tests for the force stop of a routing consultation: only the orchestrator
// invocation is killed, nothing is recorded and the state does not move, and
// the user then chooses between stopping the run and the manual routing
// dialogue for the same decision.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

func TestSession_ForceStopConsultation_ChoiceStop_StopsResumablyWithNothingRecorded(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextStop, 1)

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	requireNothingRecordedBeyond(t, r.store, 0, "")
	if !reflect.DeepEqual(r.store.state.CurrentState, domain.CurrentState{}) {
		t.Errorf("want current_state untouched, got %+v", r.store.state.CurrentState)
	}
	if n := len(r.f.Invocations()); n != 0 {
		t.Errorf("want no agent dispatched, got %d", n)
	}
	if r.manual.CallCount != 0 {
		t.Errorf("want no manual dialogue after choosing stop, got %d", r.manual.CallCount)
	}
}

func TestSession_ForceStopConsultation_ClassifiedByTheRequest_NotByTheErrorShape(t *testing.T) {
	for _, plain := range []bool{false, true} {
		name := "wrapped as a consultation error"
		if plain {
			name = "bare context error"
		}
		t.Run(name, func(t *testing.T) {
			r := newConsultForceRig(t, domain.ForceStopNextStop, 1)
			r.consultant.plain = plain

			got, err := r.ses.Start(context.Background(), r.cfg)

			requireRunStatus(t, got, err, domain.RunStopped)
			if n := r.user.postKillAsks(); n != 1 {
				t.Errorf("want the post-kill question asked once, got %d", n)
			}
		})
	}
}

func TestSession_ForceStopConsultation_AsksThePostKillChoiceWithStopAndManual(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextStop, 1)

	_, err := r.ses.Start(context.Background(), r.cfg)

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if n := r.user.postKillAsks(); n != 1 {
		t.Fatalf("want the post-kill question asked once, got %d", n)
	}
	var ids []string
	for _, o := range r.user.prompted[len(r.user.prompted)-1].Options {
		ids = append(ids, o.ID)
	}
	if !reflect.DeepEqual(ids, []string{domain.ForceStopNextStop, domain.ForceStopNextManual}) {
		t.Errorf("want the options stop and manual, got %v", ids)
	}
}

func TestSession_ForceStopConsultation_MidRun_LeavesTheAcceptedStateAsItWas(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextStop, 2)
	r.queueFirstDispatch("agent-a finished")

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	requireNothingRecordedBeyond(t, r.store, 1, "agent-a#1")
}

func TestSession_ForceStopConsultation_AnnouncesTheEndOfTheConsultation(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextStop, 1)

	_, err := r.ses.Start(context.Background(), r.cfg)

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if s, e := r.trace.count(evStart), r.trace.count(evEnd); s != 1 || e != 1 {
		t.Errorf("want one announced start and one end of the consultation, got %d and %d", s, e)
	}
}

func TestSession_ForceStopConsultation_LogsTheForceStopWithTargetConsultation(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextStop, 1)

	_, err := r.ses.Start(context.Background(), r.cfg)

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	target, ok := r.logger.fieldValue(domain.EventSessionForceStop, "target")
	if !ok || target != "consultation" {
		t.Errorf("want a force-stop debug event with target consultation, got %q (logged: %v)", target, ok)
	}
}

func TestSession_ForceStopConsultation_ChoiceManual_OpensTheDialogueForTheSameDecisionAndAppliesItsAnswer(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextManual, 2)
	r.queueFirstDispatch("agent-a finished")
	r.manual.queueDispatch("agent-b", "go on", 1)
	r.f.Queue("agent-b", successEntry("agent-b finished"))
	r.inner.queueStop("done")

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if r.manual.CallCount != 1 {
		t.Fatalf("want one manual routing dialogue, got %d", r.manual.CallCount)
	}
	if n := countInvocationsFor(r.f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want the dialogue's answer dispatched once, got %d agent-b dispatches", n)
	}
	steps := agentRows(r.store, "agent-b")
	if len(steps) != 1 || steps[0].Status != domain.StatusSUCCESS {
		t.Errorf("want the manually routed agent-b recorded as SUCCESS, got %+v", steps)
	}
	if n := len(forceStoppedSteps(r.store)); n != 0 {
		t.Errorf("want no force-stopped row for a consultation, got %d", n)
	}
	if r.inner.CallCount != 2 {
		t.Errorf("want the orchestrator consulted again at the decision after the manual step, got %d answered consultations", r.inner.CallCount)
	}
}

func TestSession_ForceStopConsultation_ChoiceManual_ReusesTheKilledConsultationsRequest(t *testing.T) {
	r := newConsultForceRig(t, domain.ForceStopNextManual, 2)
	r.queueFirstDispatch("agent-a finished")
	r.manual.queueStop("enough")

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if r.manual.CallCount != 1 {
		t.Fatalf("want one manual routing dialogue, got %d", r.manual.CallCount)
	}
	killed, asked := r.consultant.blockedRequest(t), r.manual.Requests[0]
	if asked.Context != killed.Context || asked.Deviation != nil || killed.Deviation != nil {
		t.Errorf("want the same context and no deviation, got %q/%v and %q/%v",
			asked.Context, asked.Deviation, killed.Context, killed.Deviation)
	}
	if !reflect.DeepEqual(asked.Schedules, killed.Schedules) {
		t.Errorf("want the schedules of the killed consultation, got %+v and %+v", asked.Schedules, killed.Schedules)
	}
	if !reflect.DeepEqual(asked.LastStatusMessage, killed.LastStatusMessage) || !reflect.DeepEqual(asked.LastErrorReason, killed.LastErrorReason) {
		t.Errorf("want the last status and error reason of the killed consultation, got %v/%v and %v/%v",
			asked.LastStatusMessage, asked.LastErrorReason, killed.LastStatusMessage, killed.LastErrorReason)
	}
	if asked.LastStatusMessage != nil && *asked.LastStatusMessage == domain.ForceStoppedSummary {
		t.Errorf("want no force-stopped summary as last status: no dispatched agent was stopped")
	}
}

func TestSession_ForceStopConsultation_ChoiceUnavailable_StopsResumably(t *testing.T) {
	cases := []struct {
		name     string
		status   interaction.AnswerStatus
		noManual bool
	}{
		{"question cancelled", interaction.Cancelled, false},
		{"question unsupported", interaction.Unsupported, false},
		{"no manual resolver wired", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newConsultForceRig(t, domain.ForceStopNextManual, 1, func(d *session.Deps) {
				if tc.noManual {
					d.Manual = nil
				}
			})
			rig.user.status = tc.status

			got, err := rig.ses.Start(context.Background(), rig.cfg)

			requireRunStatus(t, got, err, domain.RunStopped)
			requireNothingRecordedBeyond(t, rig.store, 0, "")
			if rig.manual.CallCount != 0 {
				t.Errorf("want no manual dialogue when the choice is unavailable, got %d", rig.manual.CallCount)
			}
		})
	}
}
