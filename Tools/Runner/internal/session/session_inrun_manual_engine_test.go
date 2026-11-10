package session_test

// Tests of the in-run manual-dispatch request at engine-routed decisions (auto
// and auto-review): a request pending when agent A's outcome is accepted opens
// the manual dialogue in place of the engine's next dispatch, applies once, and
// is never used up by mechanical re-dispatches or infrastructure dispatches.

import (
	"context"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
)

var linearAgents = []string{"agent-a", "agent-b"}

func linearRig(t *testing.T, user *manualUser) *inRunRig {
	t.Helper()
	return newInRunRig(t, user, inRunOptions{fixture: "linear-orch.md", agents: linearAgents})
}

// runManualRedoOfA runs the linear workflow with a request set while agent-a
// runs and a dialogue that re-dispatches agent-a (row 1) instead of the
// engine's agent-b.
func runManualRedoOfA(t *testing.T, mode domain.ExecutionMode) (*inRunRig, domain.RunOutcome, error) {
	t.Helper()
	rig := linearRig(t, linearDialogue(t, "1", "redo the plan"))
	rig.queueOK("agent-a", 2)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")
	got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))
	return rig, got, err
}

func TestSession_InRunManual_RequestSetWhileAgentRuns_OpensDialogueInPlaceOfTheEnginesNextStep(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig, got, err := runManualRedoOfA(t, mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			if want := []string{"agent-a", "agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Fatalf("want the dialogue's agent-a before the engine's agent-b: %v, got %v", want, invokedAgents(rig.f))
			}
			if n := rig.user.rowQuestions(); n != 1 {
				t.Errorf("want the dialogue opened once, got %d", n)
			}
			if want := []bool{false, true, true}; !slices.Equal(rig.dialogueOpenAtInvoke, want) {
				t.Errorf("want the dialogue open before the second dispatch only: %v, got %v", want, rig.dialogueOpenAtInvoke)
			}
		})
	}
}

func TestSession_InRunManual_ChosenStep_IsDispatchedWithTheDialoguesTaskAndRecordedAtItsRow(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig, got, err := runManualRedoOfA(t, mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			invs := rig.f.Invocations()
			if len(invs) < 2 || invs[1].Request.TaskDescription != "redo the plan" {
				t.Fatalf("want the second dispatch to carry the dialogue's task, got %+v", invs)
			}
			steps := workflowSteps(rig.store)
			if len(steps) != 3 {
				t.Fatalf("want three recorded workflow steps, got %+v", steps)
			}
			requireStepAt(t, steps[1], "PLANNING", 1, "")
		})
	}
}

func TestSession_InRunManual_AppliesOnce_TheDecisionAfterTheChosenStepIsEngineRouted(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig, got, err := runManualRedoOfA(t, mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			if n := rig.user.rowQuestions(); n != 1 {
				t.Errorf("want one dialogue for one request, got %d", n)
			}
			if rig.flag.isPending() {
				t.Error("want the request consumed")
			}
			if n := rig.appliedCount(); n != 1 {
				t.Errorf("want the application logged once, got %d", n)
			}
		})
	}
}

func TestSession_InRunManual_RequestCancelledBeforeAgentFinishes_HasNoEffect(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := linearRig(t, linearDialogue(t, "1", "redo"))
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.afterInvoke = func(id string) {
				if id == "agent-a" {
					rig.flag.request()
					rig.flag.cancel()
				}
			}

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want the engine's own order %v, got %v", want, invokedAgents(rig.f))
			}
			if n := rig.user.rowQuestions(); n != 0 {
				t.Errorf("want no dialogue, got %d", n)
			}
		})
	}
}

func TestSession_InRunManual_E501AndPartiallyDoneRedispatches_RunBeforeTheDialogueOpens(t *testing.T) {
	for _, name := range []string{"E501", "PARTIALLY_DONE"} {
		for _, mode := range autoModes {
			t.Run(name+"/"+string(mode), func(t *testing.T) {
				rig := linearRig(t, linearDialogue(t, "1", "redo"))
				if name == "E501" {
					rig.f.Queue("agent-a", e501Entry(), okEntry(), okEntry())
				} else {
					rig.f.Queue("agent-a", statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "half"), okEntry(), okEntry())
				}
				rig.queueOK("agent-b", 1)
				rig.requestOnceAt("agent-a")

				got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

				requireRunStatus(t, got, err, domain.RunCompleted)
				if want := []string{"agent-a", "agent-a", "agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
					t.Fatalf("want the retry, then the dialogue's agent-a, then agent-b: %v, got %v", want, invokedAgents(rig.f))
				}
				if want := []bool{false, false, true, true}; !slices.Equal(rig.dialogueOpenAtInvoke, want) {
					t.Errorf("want the dialogue to open only after the retry: %v, got %v", want, rig.dialogueOpenAtInvoke)
				}
			})
		}
	}
}

func TestSession_InRunManual_InfrastructureTriggerAfterAgent_DoesNotConsumeTheRequest(t *testing.T) {
	user := linearDialogue(t, "1", "redo")
	rig := newInRunRig(t, user, inRunOptions{
		fixture: "interval-agent-orch.md", agents: append([]string{"checkpoint-manager-git"}, linearAgents...),
	})
	rig.queueOK("agent-a", 2)
	rig.queueOK("agent-b", 1)
	rig.queueOK("checkpoint-manager-git", 4)
	rig.requestOnceAt("agent-a")
	cfg := configForMode(rig.orchPath, domain.ExecutionModeAuto)
	cfg.Checkpoints = true

	got, err := rig.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	agents := invokedAgents(rig.f)
	if len(agents) < 3 || !slices.Equal(agents[:3], []string{"agent-a", "checkpoint-manager-git", "agent-a"}) {
		t.Fatalf("want the checkpoint between agent-a and the dialogue's agent-a, got %v", agents)
	}
	if want := []bool{false, false, true}; !slices.Equal(rig.dialogueOpenAtInvoke[:3], want) {
		t.Errorf("want the dialogue closed at the checkpoint and open at the next workflow dispatch: %v, got %v",
			want, rig.dialogueOpenAtInvoke[:3])
	}
}

func TestSession_InRunManual_ReviewConsultationDueAfterAgent_IsReplacedByTheDialogue(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("second decision")
			rig := newInRunRig(t, linearDialogue(t, "2", "do b now"), inRunOptions{
				fixture:    "review-class-orch.md",
				agents:     []string{"agent-a", "agent-b", "review-agent-a", "review-agent-b"},
				consultant: consultant,
			})
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.queueOK("review-agent-a", 2)
			rig.queueOK("review-agent-b", 2)
			rig.requestOnceAt("agent-a")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			want := []string{"agent-a", "review-agent-a", "review-agent-b", "agent-b", "review-agent-a", "review-agent-b"}
			if !slices.Equal(invokedAgents(rig.f), want) {
				t.Fatalf("want the dialogue's agent-b after the first review pass: %v, got %v", want, invokedAgents(rig.f))
			}
			if consultant.CallCount != 1 {
				t.Errorf("want only the decision after the chosen step consulted, got %d consultations", consultant.CallCount)
			}
			if want := []bool{false, false, false, true}; !slices.Equal(rig.dialogueOpenAtInvoke[:4], want) {
				t.Errorf("want the dialogue to open after the review pass: %v, got %v", want, rig.dialogueOpenAtInvoke[:4])
			}
		})
	}
}

func TestSession_InRunManual_RequestPendingWhenEngineCompletes_CompletesWithoutDialogue(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := linearRig(t, linearDialogue(t, "1", "redo"))
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-b")

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if n := rig.user.rowQuestions(); n != 0 {
				t.Errorf("want no dialogue at completion, got %d", n)
			}
			if !rig.flag.isPending() {
				t.Error("want the request left pending for the caller to discard")
			}
		})
	}
}

func TestSession_InRunManual_WithoutManualResolver_RequestIsNotConsumedAndRunIsUnchanged(t *testing.T) {
	rig := newInRunRig(t, newManualUser(t), inRunOptions{
		fixture: "linear-orch.md", agents: linearAgents, noManual: true,
	})
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Errorf("want the engine's own order %v, got %v", want, invokedAgents(rig.f))
	}
	if !rig.flag.isPending() {
		t.Error("want the request kept pending when no manual resolver exists")
	}
}
