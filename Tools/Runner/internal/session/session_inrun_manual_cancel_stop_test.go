package session_test

// Tests of a stop requested while the in-run manual-dispatch dialogue is open
// and a cancelled dialogue in orchestrated mode at a deviation: the stop wins
// over continuing, and the cancel makes the orchestrator decide as usual.

import (
	"context"
	"slices"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

// armStopWhenDialogueOpens returns a stop flag that is armed as soon as the
// manual dialogue asks its row question.
func armStopWhenDialogueOpens(user *manualUser) *stopFlag {
	flag := &stopFlag{}
	user.onAsk = func(id interaction.QuestionID) {
		if id == domain.QuestionManualRow {
			flag.arm()
		}
	}
	return flag
}

func TestSession_InRunManual_StopRequestedDuringDialogue_EscStopsTheRunInsteadOfContinuing(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			user := newManualUser(t) // unscripted dialogue: Esc
			flag := armStopWhenDialogueOpens(user)
			rig := newInRunRig(t, user, inRunOptions{fixture: "linear-orch.md", agents: linearAgents, stopRequested: flag.requested})
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			if err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			if got.Status != domain.RunStopped {
				t.Fatalf("want RunStopped, got %q (message: %q)", got.Status, got.Message)
			}
			if want := []string{"agent-a"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want nothing dispatched after the stop: %v, got %v", want, invokedAgents(rig.f))
			}
		})
	}
}

func TestSession_InRunManual_StopRequestedDuringDialogue_Orchestrated_OrchestratorIsNotConsultedAfterTheCancel(t *testing.T) {
	user := newManualUser(t) // unscripted dialogue: Esc
	flag := armStopWhenDialogueOpens(user)
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueDispatch("agent-b", "after the cancel", 1)
	rig := newInRunRig(t, user, inRunOptions{
		fixture: "linear-orch.md", agents: linearAgents, consultant: consultant, stopRequested: flag.requested,
	})
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if got.Status != domain.RunStopped {
		t.Fatalf("want RunStopped, got %q (message: %q)", got.Status, got.Message)
	}
	if consultant.CallCount != 1 {
		t.Errorf("want only the first decision consulted, got %d calls", consultant.CallCount)
	}
	if want := []string{"agent-a"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Errorf("want nothing dispatched after the stop: %v, got %v", want, invokedAgents(rig.f))
	}
}

func TestSession_InRunManual_CancelInDialogue_OrchestratedDeviation_ConsultsTheOrchestratorOnceMore(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueDispatch("agent-b", "carry on with b", 1)
	consultant.queueStop("done")
	rig := orchestratedRig(t, newManualUser(t), consultant) // unscripted dialogue: Esc
	rig.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"))
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	_, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Errorf("want the orchestrator's agent-b after the cancel: %v, got %v", want, invokedAgents(rig.f))
	}
	if n := rig.user.rowQuestions(); n != 1 {
		t.Errorf("want the dialogue opened once, got %d", n)
	}
	requireCancelNotice(t, rig)
	requireCancelLogged(t, rig, "routing_consultation")
}
