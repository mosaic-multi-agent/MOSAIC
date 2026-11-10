package session_test

// Tests for the force stop of a consultation the user handed over from the
// manual dialogue, and for the manual dialogue opened after a force stop: the
// post-kill choice is offered, reopening the dialogue offers the option again
// when a routing consultant is wired, and never when none is.

import (
	"context"
	"slices"
	"testing"

	"mosaic-run/internal/deviation"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// askForceRig is a consultForceRig whose second routing consultation is the one
// handed over from an in-run manual dialogue (requested while agent-a ran) and
// is force-stopped. The dialogue is the real manual resolver.
func askForceRig(t *testing.T, postKill string, answers ...string) (*consultForceRig, *manualUser, *rowProbe) {
	t.Helper()
	user := newManualUser(t)
	probe := scriptRow(user, answers...)
	flag := &requestFlag{}
	rig := newConsultForceRig(t, postKill, 2, func(d *session.Deps) {
		d.Manual = &deviation.ManualResolver{Interact: user}
		d.TakeManualDispatchRequest = flag.take
		d.Harness = &callbackHarness{delegate: d.Harness, onInvoke: func(id string) {
			if id == "agent-a" {
				flag.request()
			}
		}}
	})
	rig.queueFirstDispatch("agent-a finished")
	return rig, user, probe
}

func TestSession_AskOrchestrator_ForceStopOfTheConsultation_ChoiceStop_StopsResumablyWithNothingRecorded(t *testing.T) {
	rig, user, probe := askForceRig(t, domain.ForceStopNextStop, domain.ManualAskOrchestratorOptionID)

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	if want := []bool{true}; !slices.Equal(probe.offered, want) {
		t.Fatalf("want the dialogue opened once with the option: %v, got %v", want, probe.offered)
	}
	if n := rig.user.postKillAsks(); n != 1 {
		t.Errorf("want the post-kill question asked once, got %d", n)
	}
	requireNothingRecordedBeyond(t, rig.store, 1, "agent-a#1")
	if user.rowQuestions() != 1 {
		t.Errorf("want the dialogue not reopened after choosing stop, got %d openings", user.rowQuestions())
	}
	if s, e := rig.trace.count(evStart), rig.trace.count(evEnd); s != 2 || e != 2 {
		t.Errorf("want both consultations announced and ended, got %d and %d", s, e)
	}
}

func TestSession_AskOrchestrator_ForceStopOfTheConsultation_ChoiceManual_ReopensTheDialogueWithTheOptionAgain(t *testing.T) {
	rig, user, probe := askForceRig(t, domain.ForceStopNextManual,
		domain.ManualAskOrchestratorOptionID, domain.ManualStopOptionID)

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if want := []bool{true, true}; !slices.Equal(probe.offered, want) {
		t.Errorf("want the option offered at the reopened dialogue as well: %v, got %v", want, probe.offered)
	}
	if user.rowQuestions() != 2 {
		t.Errorf("want the dialogue opened twice, got %d", user.rowQuestions())
	}
	requireNothingRecordedBeyond(t, rig.store, 1, "agent-a#1")
}

// postKillRig is a linear auto-mode run whose first agent-a invocation is
// force-stopped, with the real manual resolver as Deps.Manual and the post-kill
// choice "manual". withRouting selects whether a routing consultant is wired.
func postKillRig(t *testing.T, withRouting bool, user *manualUser) (session.Session, domain.RunConfig) {
	t.Helper()
	var consultant *scriptedRoutingConsultant
	if withRouting {
		consultant = &scriptedRoutingConsultant{}
	}
	return postKillRigWith(t, user, consultant)
}

// postKillRigWith is postKillRig with the given routing consultant (nil: none).
func postKillRigWith(t *testing.T, user *manualUser, consultant *scriptedRoutingConsultant) (session.Session, domain.RunConfig) {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, win := harness.NewMockAdapter(), &forceWindow{}
	deps := session.Deps{
		Harness:   newBlockingHarness(f, win, "agent-a", 1),
		Store:     &memStore{},
		Clock:     fixedClock{t: epoch},
		Interact:  &choiceUser{option: domain.ForceStopNextManual},
		Manual:    &deviation.ManualResolver{Interact: user},
		ForceStop: win,
	}
	if consultant != nil {
		deps.Routing = consultant
	}
	return session.New(deps), baseLinearConfig(orchPath)
}

func TestSession_AskOrchestrator_ManualDialogueAfterForceStopOfAnAgent_OfferedOnlyWithARoutingConsultant(t *testing.T) {
	for _, tc := range []struct {
		name        string
		withRouting bool
	}{
		{"routing consultant wired", true},
		{"manual resolver only", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := newManualUser(t)
			probe := scriptRow(user, domain.ManualStopOptionID)
			ses, cfg := postKillRig(t, tc.withRouting, user)

			got, err := ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if want := []bool{tc.withRouting}; !slices.Equal(probe.offered, want) {
				t.Errorf("want the option offered=%v after a force stop, got %v", tc.withRouting, probe.offered)
			}
		})
	}
}
