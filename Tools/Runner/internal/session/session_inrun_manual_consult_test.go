package session_test

// Tests of the in-run manual-dispatch request at consultation-routed decisions
// (orchestrated mode and engine deviations): the dialogue replaces the
// orchestrator consultation, a request made during a consultation waits for the
// next decision, the outcome and Execution Log match the stop-screen manual
// dispatch, and the application is logged.

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"mosaic-run/internal/deviation"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// requestingConsultant makes the in-run request while its first consultation is
// running, then behaves as the wrapped scripted consultant.
type requestingConsultant struct {
	inner *scriptedRoutingConsultant
	flag  *requestFlag
}

func (c *requestingConsultant) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	if c.inner.CallCount == 0 {
		c.flag.request()
	}
	return c.inner.ConsultRouting(ctx, req)
}

func orchestratedRig(t *testing.T, user *manualUser, consultant *scriptedRoutingConsultant) *inRunRig {
	t.Helper()
	return newInRunRig(t, user, inRunOptions{fixture: "linear-orch.md", agents: linearAgents, consultant: consultant})
}

func TestSession_InRunManual_Orchestrated_DialogueReplacesTheConsultationAfterTheRunningAgent(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueStop("after the manual step")
	rig := orchestratedRig(t, linearDialogue(t, "2", "do b now"), consultant)
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Fatalf("want the dialogue's agent-b right after agent-a: %v, got %v", want, invokedAgents(rig.f))
	}
	if consultant.CallCount != 2 {
		t.Errorf("want the consultant asked at the first and the third decision only, got %d calls", consultant.CallCount)
	}
	if want := []bool{false, true}; !slices.Equal(rig.dialogueOpenAtInvoke, want) {
		t.Errorf("want the dialogue open before agent-b: %v, got %v", want, rig.dialogueOpenAtInvoke)
	}
	if got := invocationTask(rig, 1); got != "do b now" {
		t.Errorf("want the dialogue's task on the second dispatch, got %q", got)
	}
}

func invocationTask(rig *inRunRig, i int) string {
	invs := rig.f.Invocations()
	if i >= len(invs) {
		return ""
	}
	return invs[i].Request.TaskDescription
}

func TestSession_InRunManual_RequestMadeDuringConsultation_LeavesItsResultAndAppliesAtTheNextDecision(t *testing.T) {
	inner := &scriptedRoutingConsultant{}
	inner.queueDispatch("agent-a", "start", 0)
	inner.queueStop("after the manual step")
	wrapper := &requestingConsultant{inner: inner}
	rig := newInRunRig(t, linearDialogue(t, "2", "do b now"), inRunOptions{
		fixture: "linear-orch.md", agents: linearAgents, routing: wrapper,
	})
	wrapper.flag = rig.flag
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
		t.Fatalf("want the running consultation's agent-a, then the dialogue's agent-b: %v, got %v", want, invokedAgents(rig.f))
	}
	if want := []bool{false, true}; !slices.Equal(rig.dialogueOpenAtInvoke, want) {
		t.Errorf("want the dialogue to open only at the decision after agent-a: %v, got %v", want, rig.dialogueOpenAtInvoke)
	}
	if inner.CallCount != 2 {
		t.Errorf("want two consultations (before agent-a and after agent-b), got %d", inner.CallCount)
	}
}

func TestSession_InRunManual_EngineDeviation_DialogueReplacesTheConsultation(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			rig := orchestratedRig(t, linearDialogue(t, "2", "carry on with b"), consultant)
			rig.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"))
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-a")

			_, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			if err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want the dialogue's agent-b after the deviation: %v, got %v", want, invokedAgents(rig.f))
			}
			if consultant.CallCount != 0 {
				t.Errorf("want the orchestrator not consulted for the deviation, got %d calls", consultant.CallCount)
			}
		})
	}
}

func TestSession_InRunManual_RequestMadeDuringDeviationConsultation_AppliesAtTheNextDecision(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			inner := &scriptedRoutingConsultant{}
			inner.queueDispatch("agent-a", "retry a", 0)
			wrapper := &requestingConsultant{inner: inner}
			rig := newInRunRig(t, linearDialogue(t, "2", "b by hand"), inRunOptions{
				fixture: "linear-orch.md", agents: linearAgents, routing: wrapper,
			})
			wrapper.flag = rig.flag
			rig.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"), okEntry())
			rig.queueOK("agent-b", 1)

			got, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			requireRunStatus(t, got, err, domain.RunCompleted)
			if want := []string{"agent-a", "agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Fatalf("want the consultation's agent-a, then the dialogue's agent-b: %v, got %v", want, invokedAgents(rig.f))
			}
			if want := []bool{false, false, true}; !slices.Equal(rig.dialogueOpenAtInvoke, want) {
				t.Errorf("want the dialogue to open only after the consulted step: %v, got %v", want, rig.dialogueOpenAtInvoke)
			}
			if inner.CallCount != 1 {
				t.Errorf("want the running consultation to finish and be the only one, got %d", inner.CallCount)
			}
		})
	}
}

func TestSession_InRunManual_CancelInDialogue_EndsWithTheResumableStopOfTheStopScreenPath(t *testing.T) {
	cases := []struct {
		name string
		mode domain.ExecutionMode
	}{
		{"orchestrated", domain.ExecutionModeOrchestrated},
		{"auto", domain.ExecutionModeAuto},
		{"auto-review", domain.ExecutionModeAutoReview},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueDispatch("agent-a", "start", 0)
			rig := orchestratedRig(t, newManualUser(t), consultant) // unscripted dialogue: cancelled
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-a")
			cfg := configForMode(rig.orchPath, tc.mode)

			got, err := rig.ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			var ce *domain.ConsultationError
			if !errors.As(got.Cause, &ce) || ce.Failure != domain.ConsultFailUserAbandoned {
				t.Errorf("want cause %q, got %v", domain.ConsultFailUserAbandoned, got.Cause)
			}
			if want := []string{"agent-a"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want nothing dispatched after the cancel: %v, got %v", want, invokedAgents(rig.f))
			}
		})
	}
}

// newParityRig builds the manual-routing rig of a resumed orchestrated run. With
// inRun false the first decision is the stop screen's manual dispatch; with
// inRun true the same decision is reached through a pending in-run request.
func newParityRig(t *testing.T, inRun bool) (session.Session, *harness.MockAdapter, *memStore, domain.RunConfig) {
	t.Helper()
	user := newManualUser(t).completeDialogue("3", "docs/extra.md")
	base := newManualRig(t, user, nil)
	flag := &requestFlag{}
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("done")
	f := harness.NewMockAdapter()
	store := &memStore{state: base.store.state, exists: true}
	deps := session.Deps{
		Harness: f, Store: store, Routing: consultant,
		Manual: &deviation.ManualResolver{Interact: user},
		Clock:  fixedClock{t: epoch}, Interact: &noopInteraction{},
	}
	cfg := base.cfg
	if inRun {
		flag.request()
		deps.TakeManualDispatchRequest = flag.take
		cfg.ManualDispatch = false
	}
	queueSuccess(f, "build-review", "build-review#3")
	return session.New(deps), f, store, cfg
}

func TestSession_InRunManual_ChosenStep_IsRecordedLikeTheStopScreenManualDispatch(t *testing.T) {
	stopSes, stopF, stopStore, stopCfg := newParityRig(t, false)
	runSes, runF, runStore, runCfg := newParityRig(t, true)

	stopGot, stopErr := stopSes.Start(context.Background(), stopCfg)
	runGot, runErr := runSes.Start(context.Background(), runCfg)

	requireRunStatus(t, stopGot, stopErr, domain.RunStoppedByConsultant)
	requireRunStatus(t, runGot, runErr, domain.RunStoppedByConsultant)
	if len(stopStore.Applied) == 0 {
		t.Fatal("want the stop-screen path to record its manual step")
	}
	if !reflect.DeepEqual(runStore.Applied, stopStore.Applied) {
		t.Errorf("want the same recorded steps\n stop screen: %+v\n in run:      %+v", stopStore.Applied, runStore.Applied)
	}
	if !reflect.DeepEqual(runStore.state.ExecutionLog, stopStore.state.ExecutionLog) {
		t.Errorf("want the same Execution Log\n stop screen: %+v\n in run:      %+v", stopStore.state.ExecutionLog, runStore.state.ExecutionLog)
	}
	runInvs, stopInvs := runF.Invocations(), stopF.Invocations()
	if len(runInvs) != len(stopInvs) || len(runInvs) == 0 || !sameDispatch(runInvs[0], stopInvs[0]) {
		t.Errorf("want the same dispatch\n stop screen: %+v\n in run:      %+v", stopF.Invocations(), runF.Invocations())
	}
}

func TestSession_InRunManual_Application_IsLoggedWithTheEngineStepItReplaced(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig, got, err := runManualRedoOfA(t, mode)

			requireRunStatus(t, got, err, domain.RunCompleted)
			fields := rig.appliedFields(t)
			if fields["request"] != string(domain.InRunRequestManualDispatch) {
				t.Errorf("want request=%s, got %q", domain.InRunRequestManualDispatch, fields["request"])
			}
			if fields["decision"] != "engine_dispatch" {
				t.Errorf("want decision=engine_dispatch, got %q", fields["decision"])
			}
			if fields["row"] != "2" {
				t.Errorf("want row=2 (the replaced agent-b step), got %q", fields["row"])
			}
		})
	}
}

func TestSession_InRunManual_Application_IsLoggedForAConsultationDecision(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "start", 0)
	consultant.queueStop("done")
	rig := orchestratedRig(t, linearDialogue(t, "2", "do b"), consultant)
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.requestOnceAt("agent-a")

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	fields := rig.appliedFields(t)
	if fields["request"] != string(domain.InRunRequestManualDispatch) || fields["decision"] != "routing_consultation" {
		t.Errorf("want request=manual_dispatch decision=routing_consultation, got %v", fields)
	}
	if n := rig.appliedCount(); n != 1 {
		t.Errorf("want one application entry, got %d", n)
	}
}

func TestSession_InRunManual_Orchestrated_PendingRequestOpensTheDialogueWithoutARoutingConsultant(t *testing.T) {
	rig := newInRunRig(t, linearDialogue(t, "1", "start by hand"), inRunOptions{
		fixture: "linear-orch.md", agents: linearAgents,
	})
	rig.queueOK("agent-a", 1)
	rig.queueOK("agent-b", 1)
	rig.flag.request()

	_, _ = rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	invoked := invokedAgents(rig.f)
	if len(invoked) == 0 || invoked[0] != "agent-a" {
		t.Fatalf("want the dialogue's agent-a dispatched at the first decision, got %v", invoked)
	}
	if !rig.dialogueOpenAtInvoke[0] {
		t.Error("want the dialogue open before the first dispatch")
	}
	if got := invocationTask(rig, 0); got != "start by hand" {
		t.Errorf("want the dialogue's task on the dispatch, got %q", got)
	}
}

func TestSession_InRunManual_EngineDeviation_OpensTheDialogueWithoutARoutingConsultant(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newInRunRig(t, linearDialogue(t, "2", "carry on with b"), inRunOptions{
				fixture: "linear-orch.md", agents: linearAgents,
			})
			rig.f.Queue("agent-a", statusEntry(domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING, "missing input"))
			rig.queueOK("agent-b", 1)
			rig.requestOnceAt("agent-a")

			_, err := rig.ses.Start(context.Background(), configForMode(rig.orchPath, mode))

			if err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			if want := []string{"agent-a", "agent-b"}; !slices.Equal(invokedAgents(rig.f), want) {
				t.Errorf("want the dialogue's agent-b after the deviation: %v, got %v", want, invokedAgents(rig.f))
			}
			if n := rig.user.rowQuestions(); n != 1 {
				t.Errorf("want one dialogue, got %d", n)
			}
		})
	}
}
