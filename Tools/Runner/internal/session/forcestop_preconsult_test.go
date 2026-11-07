package session_test

// Tests for the force stop of the run-start pre-consultation: the invocation
// is killed, nothing is recorded, the run stops resumably without any manual
// choice, and a resume asks the pre-consultation again.

import (
	"context"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// preForceRig is a run with pre-consultation whose pre-consultant is
// force-stopped.
type preForceRig struct {
	ses    session.Session
	f      *harness.MockAdapter
	store  *memStore
	win    *forceWindow
	user   *choiceUser
	manual *scriptedRoutingConsultant
	pc     *blockingPreConsultant
	trace  *consultTrace
	logger *sessionRecordingLogger
	cfg    domain.RunConfig
}

func newPreForceRig(t *testing.T, mode domain.ExecutionMode) *preForceRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	r := &preForceRig{
		f: harness.NewMockAdapter(), store: &memStore{}, win: &forceWindow{},
		user: &choiceUser{option: domain.ForceStopNextManual}, manual: &scriptedRoutingConsultant{},
		trace: &consultTrace{}, logger: &sessionRecordingLogger{},
	}
	r.pc = &blockingPreConsultant{win: r.win}
	r.cfg = baseLinearConfig(orchPath)
	r.cfg.Mode = mode
	r.cfg.PreConsultation = true
	r.ses = session.New(session.Deps{
		Harness: r.f, Store: r.store, Clock: fixedClock{t: epoch}, Interact: r.user, PreConsult: r.pc,
		Manual: r.manual, ForceStop: r.win, Consultations: r.trace, Debug: r.logger,
	})
	return r
}

func TestSession_ForceStopPreConsultation_StopsTheRunResumablyWithoutAChoice(t *testing.T) {
	for _, m := range preConsultStopModes {
		t.Run(m.name, func(t *testing.T) {
			r := newPreForceRig(t, m.mode)

			got, err := r.ses.Start(context.Background(), r.cfg)

			requireRunStatus(t, got, err, domain.RunStopped)
			if n := r.user.postKillAsks(); n != 0 {
				t.Errorf("want no post-kill question for a pre-consultation, got %d", n)
			}
			if r.manual.CallCount != 0 {
				t.Errorf("want no manual dialogue, got %d", r.manual.CallCount)
			}
			if n := len(r.f.Invocations()); n != 0 {
				t.Errorf("want no workflow dispatch, got %d", n)
			}
		})
	}
}

func TestSession_ForceStopPreConsultation_LogsNothingAndKeepsTheArtifactResumable(t *testing.T) {
	r := newPreForceRig(t, domain.ExecutionModeAuto)

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	if !r.store.exists {
		t.Error("want the artifact kept for a resume")
	}
	if len(r.store.Applied) != 0 || len(r.store.state.ExecutionLog) != 0 || r.store.state.GlobalSequence != 0 {
		t.Errorf("want no row and no sequence advance, got %d applied, %d logged, sequence %d",
			len(r.store.Applied), len(r.store.state.ExecutionLog), r.store.state.GlobalSequence)
	}
	if !reflect.DeepEqual(r.store.state.CurrentState, domain.CurrentState{}) {
		t.Errorf("want current_state untouched, got %+v", r.store.state.CurrentState)
	}
}

func TestSession_ForceStopPreConsultation_AnnouncesEndAndLogsTargetPreconsultation(t *testing.T) {
	r := newPreForceRig(t, domain.ExecutionModeAuto)

	_, err := r.ses.Start(context.Background(), r.cfg)

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if s, e := r.trace.count(evStart), r.trace.count(evEnd); s != 1 || e != 1 {
		t.Errorf("want one announced start and one end, got %d and %d", s, e)
	}
	target, ok := r.logger.fieldValue(domain.EventSessionForceStop, "target")
	if !ok || target != "preconsultation" {
		t.Errorf("want a force-stop debug event with target preconsultation, got %q (logged: %v)", target, ok)
	}
}

func TestSession_ForceStopPreConsultation_ClassifiedByTheRequest_NotByTheErrorShape(t *testing.T) {
	r := newPreForceRig(t, domain.ExecutionModeAuto)
	r.pc.plain = true

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
}

func TestSession_ForceStopPreConsultation_Resume_AutoReview_AsksThePreConsultationAgain(t *testing.T) {
	r := newPreForceRig(t, domain.ExecutionModeAutoReview)
	first, err := r.ses.Start(context.Background(), r.cfg)
	requireRunStatus(t, first, err, domain.RunStopped)
	again := &scriptedPreConsultant{advice: domain.PreConsultationAdvice{TaskDescription: "context"}}
	queueLinearSuccess(r.f)
	resumed := session.New(session.Deps{
		Harness: r.f, Store: r.store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}, PreConsult: again,
	})
	markResume(&r.cfg)

	got, _ := resumed.Start(context.Background(), r.cfg)

	if !again.Called {
		t.Error("want the pre-consultation asked again on resume")
	}
	if got.Status == domain.RunFailed {
		t.Errorf("want a resume that does not fail, got %q (%s)", got.Status, got.Message)
	}
}

func TestSession_ForceStopPreConsultation_Resume_AsksThePreConsultationAgain(t *testing.T) {
	r := newPreForceRig(t, domain.ExecutionModeAuto)
	first, err := r.ses.Start(context.Background(), r.cfg)
	requireRunStatus(t, first, err, domain.RunStopped)
	again := &scriptedPreConsultant{advice: domain.PreConsultationAdvice{TaskDescription: "context"}}
	queueLinearSuccess(r.f)
	resumed := session.New(session.Deps{
		Harness: r.f, Store: r.store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}, PreConsult: again,
	})
	markResume(&r.cfg)

	got, err := resumed.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if !again.Called {
		t.Error("want the pre-consultation asked again on resume")
	}
	if len(r.store.Applied) != 2 || r.store.Applied[0].Seq != 1 {
		t.Errorf("want the workflow rows numbered from 1, got %+v", r.store.Applied)
	}
}
