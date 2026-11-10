package session_test

// A pre-consultation announces its start and end around the orchestrator call,
// on every outcome, and announces nothing when it does not run.

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

// newPreConsultRig is a linear rig whose pre-consultant is pc, traced.
func newPreConsultRig(t *testing.T, pc domain.PreConsultant) *consultRig {
	t.Helper()
	return newConsultRig(t, "linear-orch.md", linearAgents, func(d *session.Deps, tr *consultTrace) {
		d.PreConsult = &tracedPreConsultant{inner: pc, tr: tr}
	})
}

func preConsultConfig(orchPath string, mode domain.ExecutionMode, enabled bool) domain.RunConfig {
	cfg := configForMode(orchPath, mode)
	cfg.PreConsultation = enabled
	return cfg
}

func requirePreConsultationAnnounced(t *testing.T, rig *consultRig) {
	t.Helper()
	if got := rig.tr.count(evPreCall); got != 1 {
		t.Fatalf("scenario did not run as intended: want one pre-consultation call, got %d: %v", got, rig.tr.snapshot())
	}
	requireBracketed(t, rig.tr, evPreCall, 1)
	requireBalanced(t, rig.tr)
	want := domain.ConsultationActivity{Context: domain.ConsultContextPreConsultation}
	if got := rig.tr.started(); len(got) < 1 || got[0] != want {
		t.Errorf("first announced activity = %+v, want %+v", got, want)
	}
}

func TestConsultAnnounce_PreConsultation_RunsInAutoModes_AnnouncedBeforeTheFirstDispatch(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newPreConsultRig(t, &scriptedPreConsultant{})
			queueLinearSuccess(rig.f)

			rig.ses.Start(context.Background(), preConsultConfig(rig.orchPath, mode, true)) //nolint:errcheck

			requirePreConsultationAnnounced(t, rig)
			events := rig.tr.without(evNotice)
			want := []string{evStart, evPreCall, evEnd, evInvokePfx + "agent-a"}
			if len(events) < len(want) || !slices.Equal(events[:len(want)], want) {
				t.Errorf("want the pre-consultation first (start, call, end) and agent-a right after; got %v", events)
			}
		})
	}
}

func TestConsultAnnounce_PreConsultation_Failure_EndsTheConsultation(t *testing.T) {
	rig := newPreConsultRig(t, &scriptedPreConsultant{err: errors.New("pre-consultation failed")})
	queueLinearSuccess(rig.f)

	rig.ses.Start(context.Background(), preConsultConfig(rig.orchPath, domain.ExecutionModeAuto, true)) //nolint:errcheck

	requirePreConsultationAnnounced(t, rig)
	if n := len(rig.f.Invocations()); n != 0 {
		t.Errorf("want no dispatch after a failed pre-consultation, got %d", n)
	}
}

func TestConsultAnnounce_PreConsultation_Stop_EndsTheConsultation(t *testing.T) {
	rig := newPreConsultRig(t, stoppingPreConsultant())
	queueLinearSuccess(rig.f)

	got, err := rig.ses.Start(context.Background(), preConsultConfig(rig.orchPath, domain.ExecutionModeAuto, true))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requirePreConsultationAnnounced(t, rig)
}

func TestConsultAnnounce_PreConsultation_Disabled_AnnouncesNothing(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			pc := &scriptedPreConsultant{}
			rig := newPreConsultRig(t, pc)
			queueLinearSuccess(rig.f)

			rig.ses.Start(context.Background(), preConsultConfig(rig.orchPath, mode, false)) //nolint:errcheck

			if len(rig.f.Invocations()) == 0 {
				t.Fatal("scenario did not run as intended: the run dispatched nothing")
			}
			if pc.Called || rig.tr.count(evStart) != 0 || rig.tr.count(evEnd) != 0 {
				t.Errorf("a disabled pre-consultation was announced or run (called=%v): %v", pc.Called, rig.tr.snapshot())
			}
		})
	}
}

func TestConsultAnnounce_PreConsultation_OrchestratedMode_AnnouncesNoPreConsultation(t *testing.T) {
	pc := &scriptedPreConsultant{}
	rig := newPreConsultRig(t, pc)
	rig.consult.queueStop("stop at once")

	rig.ses.Start(context.Background(), preConsultConfig(rig.orchPath, domain.ExecutionModeOrchestrated, true)) //nolint:errcheck

	if rig.consult.CallCount != 1 {
		t.Fatalf("scenario did not run as intended: want one routing consultation, got %d", rig.consult.CallCount)
	}
	if pc.Called {
		t.Fatal("scenario did not run as intended: pre-consultation ran in orchestrated mode")
	}
	requireKinds(t, rig.tr, "")
	for _, a := range rig.tr.started() {
		if a.Context == domain.ConsultContextPreConsultation {
			t.Errorf("announced a pre-consultation that did not run: %+v", rig.tr.started())
		}
	}
}
