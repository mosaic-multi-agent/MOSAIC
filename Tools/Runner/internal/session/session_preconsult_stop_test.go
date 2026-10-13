package session_test

// Tests for a pre-consultation stop: the orchestrator's pre-consultation reply
// ends the run before any workflow dispatch as a consultant stop that identifies
// itself as a pre-consultation stop, and leaves the run state exactly as a
// failed pre-consultation does.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const preConsultStopReason = "Uncommitted changes on the run branch; resolve them first"

// stoppingPreConsultant returns a scripted pre-consultant whose reply is a stop.
func stoppingPreConsultant() *scriptedPreConsultant {
	return &scriptedPreConsultant{advice: domain.PreConsultationAdvice{
		Stop: &domain.StopInstruction{Reason: preConsultStopReason},
	}}
}

// newPreConsultStopSession builds a session like newPreConsultSession and also
// returns the recording debug logger wired into it.
func newPreConsultStopSession(t *testing.T, pc domain.PreConsultant) (session.Session, *harness.MockAdapter, *memStore, string, *sessionRecordingLogger) {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f := harness.NewMockAdapter()
	store := &memStore{}
	debug := &sessionRecordingLogger{}
	ses := session.New(session.Deps{
		Harness:    f,
		Store:      store,
		Clock:      fixedClock{t: epoch},
		Interact:   &noopInteraction{},
		PreConsult: pc,
		Debug:      debug,
	})
	return ses, f, store, orchPath, debug
}

var preConsultStopModes = []struct {
	name string
	mode domain.ExecutionMode
}{
	{"auto", domain.ExecutionModeAuto},
	{"auto-review", domain.ExecutionModeAutoReview},
}

// TestSession_Start_PreConsultationStop_EndsAsConsultantStopBeforeDispatch
// verifies the outcome of a new run whose pre-consultation returns a stop: a
// consultant stop carrying the reason and the pre-consultation marker, with no
// workflow dispatch.
func TestSession_Start_PreConsultationStop_EndsAsConsultantStopBeforeDispatch(t *testing.T) {
	for _, m := range preConsultStopModes {
		t.Run(m.name, func(t *testing.T) {
			pc := stoppingPreConsultant()
			ses, f, _, orchPath, _ := newPreConsultStopSession(t, pc)
			queueLinearSuccess(f)

			cfg := baseLinearConfig(orchPath)
			cfg.Mode = m.mode
			cfg.PreConsultation = true

			got, err := ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if got.StopReason != preConsultStopReason {
				t.Errorf("want StopReason=%q, got %q", preConsultStopReason, got.StopReason)
			}
			if !got.PreConsultationStop {
				t.Error("want PreConsultationStop=true on a pre-consultation stop")
			}
			if got.Cause != nil || got.ArtifactPath != "" || got.LastState != nil {
				t.Errorf("want Cause, ArtifactPath and LastState unset, got %+v", got)
			}
			if got.Status == domain.RunStartFailed {
				t.Error("a pre-consultation stop must not be reported as a start failure")
			}
			if !pc.Called {
				t.Error("want PreConsult called")
			}
			if n := len(f.Invocations()); n != 0 {
				t.Errorf("want no workflow dispatch after a pre-consultation stop, got %d", n)
			}
		})
	}
}

// TestSession_Start_PreConsultationStop_LeavesArtifactUntouched verifies that
// the stop keeps the artifact and writes nothing: no row, no sequence, no
// current_state.
func TestSession_Start_PreConsultationStop_LeavesArtifactUntouched(t *testing.T) {
	for _, m := range preConsultStopModes {
		t.Run(m.name, func(t *testing.T) {
			ses, _, store, orchPath, _ := newPreConsultStopSession(t, stoppingPreConsultant())

			cfg := baseLinearConfig(orchPath)
			cfg.Mode = m.mode
			cfg.PreConsultation = true

			got, err := ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if !store.exists {
				t.Error("want the artifact kept after a pre-consultation stop")
			}
			if len(store.Applied) != 0 || len(store.state.ExecutionLog) != 0 {
				t.Errorf("want no Execution Log row, got %d applied, %d logged",
					len(store.Applied), len(store.state.ExecutionLog))
			}
			if store.state.GlobalSequence != 0 {
				t.Errorf("want global_sequence untouched (0), got %d", store.state.GlobalSequence)
			}
			if !reflect.DeepEqual(store.state.CurrentState, domain.CurrentState{}) {
				t.Errorf("want current_state untouched, got %+v", store.state.CurrentState)
			}
		})
	}
}

// TestSession_Start_PreConsultationStop_ResumedRun_StateUnchanged verifies that
// a stop on a resumed run leaves the stored artifact exactly as it was.
func TestSession_Start_PreConsultationStop_ResumedRun_StateUnchanged(t *testing.T) {
	ses, f, store, orchPath, _ := newPreConsultStopSession(t, stoppingPreConsultant())
	store.state = resumedAutoState()
	store.exists = true

	cfg := baseLinearConfig(orchPath)
	markResume(&cfg)
	cfg.Mode = domain.ExecutionModeAuto
	cfg.PreConsultation = true

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if !got.PreConsultationStop {
		t.Error("want PreConsultationStop=true on a resumed run's pre-consultation stop")
	}
	if !reflect.DeepEqual(store.state, resumedAutoState()) {
		t.Errorf("want the stored artifact state unchanged, got %+v", store.state)
	}
	if len(store.Applied) != 0 {
		t.Errorf("want no Apply call, got %d", len(store.Applied))
	}
	if n := len(f.Invocations()); n != 0 {
		t.Errorf("want no harness dispatch, got %d", n)
	}
}

// TestSession_Start_PreConsultationStop_ThenResume_RunsPreConsultationAgain
// verifies that resuming a stopped run calls pre-consultation again and, once
// the orchestrator no longer stops, runs the workflow from sequence 1.
func TestSession_Start_PreConsultationStop_ThenResume_RunsPreConsultationAgain(t *testing.T) {
	pc := stoppingPreConsultant()
	ses, f, store, orchPath, _ := newPreConsultStopSession(t, pc)

	cfg := baseLinearConfig(orchPath)
	cfg.Mode = domain.ExecutionModeAuto
	cfg.PreConsultation = true

	first, firstErr := ses.Start(context.Background(), cfg)
	requireRunStatus(t, first, firstErr, domain.RunStoppedByConsultant)

	pc.advice = domain.PreConsultationAdvice{}
	pc.Called = false
	queueLinearSuccess(f)
	markResume(&cfg)

	second, secondErr := ses.Start(context.Background(), cfg)

	requireRunStatus(t, second, secondErr, domain.RunCompleted)
	if second.PreConsultationStop {
		t.Error("want PreConsultationStop=false on a completed run")
	}
	if !pc.Called {
		t.Error("want PreConsult called again on resume")
	}
	if len(store.Applied) != 2 || store.Applied[0].Seq != 1 || store.Applied[1].Seq != 2 {
		t.Errorf("want workflow rows Seq 1 and 2 after the resume, got %+v", store.Applied)
	}
}

// TestSession_Start_PreConsultationStop_LogsConsultantStopWithContext verifies
// that the stop is visible in the diagnostic log with its reason and the
// pre-consultation context.
func TestSession_Start_PreConsultationStop_LogsConsultantStopWithContext(t *testing.T) {
	ses, _, _, orchPath, debug := newPreConsultStopSession(t, stoppingPreConsultant())

	cfg := baseLinearConfig(orchPath)
	cfg.Mode = domain.ExecutionModeAuto
	cfg.PreConsultation = true

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if !debug.eventLogged(domain.EventSessionConsultStop) {
		t.Fatal("want the consultant stop event in the diagnostic log")
	}
	if v, ok := debug.fieldValue(domain.EventSessionConsultStop, "reason"); !ok || v != preConsultStopReason {
		t.Errorf("want reason field %q, got %q (present=%v)", preConsultStopReason, v, ok)
	}
	if v, ok := debug.fieldValue(domain.EventSessionConsultStop, "context"); !ok || v != "pre_consultation" {
		t.Errorf("want context field %q, got %q (present=%v)", "pre_consultation", v, ok)
	}
}

// TestSession_Start_PreConsultationMalformedReply_StaysStartFailed is
// regression coverage: a malformed reply (an error, not a stop) still ends as a
// start failure and does not carry the pre-consultation stop marker.
func TestSession_Start_PreConsultationMalformedReply_StaysStartFailed(t *testing.T) {
	for _, m := range preConsultStopModes {
		t.Run(m.name, func(t *testing.T) {
			pc := &scriptedPreConsultant{err: &domain.ConsultationError{
				Failure: domain.ConsultFailMissingField,
				Detail:  "missing required field: reason (pre-consultation stop)",
			}}
			ses, f, _, orchPath, _ := newPreConsultStopSession(t, pc)

			cfg := baseLinearConfig(orchPath)
			cfg.Mode = m.mode
			cfg.PreConsultation = true

			got, err := ses.Start(context.Background(), cfg)

			requireStartFailed(t, got, err)
			if got.PreConsultationStop {
				t.Error("want PreConsultationStop=false for a start failure")
			}
			var ce *domain.ConsultationError
			if !errors.As(got.Cause, &ce) {
				t.Errorf("want Cause to carry the *domain.ConsultationError, got %v", got.Cause)
			}
			if n := len(f.Invocations()); n != 0 {
				t.Errorf("want no workflow dispatch, got %d", n)
			}
		})
	}
}
