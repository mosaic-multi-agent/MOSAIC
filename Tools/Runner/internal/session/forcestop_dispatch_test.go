package session_test

// Tests for the force stop of an engine-dispatched agent: only the running
// invocation is killed, the attempt is recorded as an unaccepted row, nothing
// is retried or bypassed, and the user then chooses what happens next.

import (
	"context"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// forceStopRig is a linear auto-mode run whose first agent-a invocation is
// force-stopped.
type forceStopRig struct {
	ses     session.Session
	f       *harness.MockAdapter
	store   *memStore
	win     *forceWindow
	user    *choiceUser
	manual  *scriptedRoutingConsultant
	routing *scriptedRoutingConsultant
	cfg     domain.RunConfig
}

func newForceStopRig(t *testing.T, choice string) *forceStopRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	r := &forceStopRig{
		f: harness.NewMockAdapter(), store: &memStore{}, win: &forceWindow{},
		user: &choiceUser{option: choice}, manual: &scriptedRoutingConsultant{}, routing: &scriptedRoutingConsultant{},
		cfg: baseLinearConfig(orchPath),
	}
	r.ses = session.New(session.Deps{
		Harness:   newBlockingHarness(r.f, r.win, "agent-a", 1),
		Store:     r.store,
		Clock:     fixedClock{t: epoch},
		Interact:  r.user,
		Routing:   r.routing,
		Manual:    r.manual,
		ForceStop: r.win,
	})
	return r
}

func TestSession_ForceStop_EngineDispatch_RecordsUnacceptedRowAndStopsResumably(t *testing.T) {
	r := newForceStopRig(t, domain.ForceStopNextStop)

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	if !strings.Contains(got.Message, "agent-a#1") {
		t.Errorf("want the message to name the stopped instance agent-a#1, got %q", got.Message)
	}
	requireForceStoppedRow(t, r.store, "agent-a", false)
	if len(r.store.Applied) != 1 {
		t.Errorf("want the force-stopped row as the only recorded step, got %+v", r.store.Applied)
	}
	if r.store.state.CurrentState.LastAgent != "" || r.store.state.CurrentState.LastStatus != "" {
		t.Errorf("want current_state unchanged by the force-stopped attempt, got %+v", r.store.state.CurrentState)
	}
}

func TestSession_ForceStop_EngineDispatch_IsNotAHarnessError(t *testing.T) {
	r := newForceStopRig(t, domain.ForceStopNextStop)
	r.f.Queue("agent-a", successEntry("done"), successEntry("done"), successEntry("done"))

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	if n := countHarnessErrorSteps(r.store); n != 0 {
		t.Errorf("want no HARNESS_ERROR row for a force stop, got %d", n)
	}
	if n := countE501Steps(r.store); n != 0 {
		t.Errorf("want no BLOCKED/E501 row for a force stop, got %d", n)
	}
	if n := countInvocationsFor(r.f.Invocations(), "agent-a"); n != 0 {
		t.Errorf("want no mechanical retry or bypass invocation after the kill, got %d further agent-a dispatches", n)
	}
	if r.routing.CallCount != 0 {
		t.Errorf("want no orchestrator consultation after a force stop, got %d", r.routing.CallCount)
	}
}

func TestSession_ForceStop_KillsOnlyTheRunningInvocation(t *testing.T) {
	r := newForceStopRig(t, domain.ForceStopNextManual)
	r.manual.queueDispatch("agent-b", "go on", 1)
	r.f.Queue("agent-b", successEntry("done"))

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := countInvocationsFor(r.f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want the next invocation to run unharmed by the earlier force stop, got %d agent-b dispatches", n)
	}
	if len(forceStoppedSteps(r.store)) != 1 {
		t.Errorf("want only the killed attempt recorded as force-stopped, got %+v", r.store.Applied)
	}
}

func TestSession_ForceStop_ChoiceStop_AsksTheQuestionAndEndsRunStopped(t *testing.T) {
	r := newForceStopRig(t, domain.ForceStopNextStop)

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	if n := r.user.postKillAsks(); n != 1 {
		t.Fatalf("want the post-kill question asked once, got %d", n)
	}
	if r.manual.CallCount != 0 {
		t.Errorf("want no manual dialogue after choosing stop, got %d", r.manual.CallCount)
	}
	var ids []string
	for _, o := range r.user.prompted[len(r.user.prompted)-1].Options {
		ids = append(ids, o.ID)
	}
	if len(ids) != 2 || ids[0] != domain.ForceStopNextStop || ids[1] != domain.ForceStopNextManual {
		t.Errorf("want the options stop and manual, got %v", ids)
	}
}

func TestSession_ForceStop_ChoiceManual_OpensTheManualDialogueAndAppliesItsAnswer(t *testing.T) {
	r := newForceStopRig(t, domain.ForceStopNextManual)
	r.manual.queueDispatch("agent-b", "go on", 1)
	r.f.Queue("agent-b", successEntry("done"))

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if r.manual.CallCount != 1 {
		t.Fatalf("want one manual routing dialogue, got %d", r.manual.CallCount)
	}
	req := r.manual.Requests[0]
	if req.Deviation != nil {
		t.Errorf("want no deviation in the post-kill dialogue request, got %+v", req.Deviation)
	}
	if req.LastStatusMessage == nil || *req.LastStatusMessage != domain.ForceStoppedSummary {
		t.Errorf("want LastStatusMessage to be the force-stop summary, got %v", req.LastStatusMessage)
	}
	if req.LastErrorReason != nil {
		t.Errorf("want no error reason, got %q", *req.LastErrorReason)
	}
	steps := agentRows(r.store, "agent-b")
	if len(steps) != 1 || steps[0].Status != domain.StatusSUCCESS {
		t.Errorf("want the manually routed agent-b dispatch recorded as SUCCESS, got %+v", steps)
	}
}

func TestSession_ForceStop_ChoiceUnavailable_StopsResumably(t *testing.T) {
	cases := []struct {
		name  string
		setup func(r *forceStopRig, deps *session.Deps)
	}{
		{"question cancelled", func(r *forceStopRig, _ *session.Deps) { r.user.status = interaction.Cancelled }},
		{"question unsupported", func(r *forceStopRig, _ *session.Deps) { r.user.status = interaction.Unsupported }},
		{"no manual resolver wired", func(_ *forceStopRig, d *session.Deps) { d.Manual = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newForceStopRig(t, domain.ForceStopNextManual)
			deps := session.Deps{
				Harness: newBlockingHarness(r.f, r.win, "agent-a", 1), Store: r.store, Clock: fixedClock{t: epoch},
				Interact: r.user, Routing: r.routing, Manual: r.manual, ForceStop: r.win,
			}
			tc.setup(r, &deps)
			ses := session.New(deps)

			got, err := ses.Start(context.Background(), r.cfg)

			requireRunStatus(t, got, err, domain.RunStopped)
			requireForceStoppedRow(t, r.store, "agent-a", false)
			if r.manual.CallCount != 0 {
				t.Errorf("want no manual dialogue when the choice is unavailable, got %d", r.manual.CallCount)
			}
		})
	}
}

func TestSession_ForceStop_HardCancel_WritesNoRowAndStopsWithoutQuestion(t *testing.T) {
	r := newForceStopRig(t, domain.ForceStopNextManual)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := newBlockingHarness(r.f, r.win, "agent-a", 1)
	h.onBlock = cancel
	r.ses = session.New(session.Deps{
		Harness: h, Store: r.store, Clock: fixedClock{t: epoch}, Interact: r.user,
		Routing: r.routing, Manual: r.manual, ForceStop: r.win,
	})

	got, err := r.ses.Start(ctx, r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	if len(r.store.Applied) != 0 {
		t.Errorf("want no row for a hard cancel, got %+v", r.store.Applied)
	}
	if n := r.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question after a hard cancel, got %d", n)
	}
}

func TestSession_ForceStop_RequestWhileNothingRuns_DoesNotHitALaterInvocation(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, store, win := harness.NewMockAdapter(), &memStore{}, &forceWindow{}
	f.Queue("agent-a", successEntry("done"))
	f.Queue("agent-b", successEntry("done"))
	ses := session.New(session.Deps{
		Harness: f, Store: store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}, ForceStop: win,
	})
	if win.Request() {
		t.Fatal("test setup: no window should be open before the run")
	}

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(forceStoppedSteps(store)) != 0 || len(store.Applied) != 2 {
		t.Errorf("want both agents accepted and nothing force-stopped, got %+v", store.Applied)
	}
}

func TestSession_ForceStop_ResponseCompletedBeforeTheKill_IsAcceptedAsNormal(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, store, win := harness.NewMockAdapter(), &memStore{}, &forceWindow{}
	f.Queue("agent-a", successEntry("done"))
	f.Queue("agent-b", successEntry("done"))
	late := &callbackHarness{delegate: f, onInvoke: func(agent string) {
		if agent == "agent-a" {
			win.Request() // arrives after the response was produced
		}
	}}
	ses := session.New(session.Deps{
		Harness: late, Store: store, Clock: fixedClock{t: epoch}, Interact: &choiceUser{option: domain.ForceStopNextStop}, ForceStop: win,
	})

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(forceStoppedSteps(store)) != 0 {
		t.Errorf("want a completed response treated as completed, got force-stopped rows %+v", forceStoppedSteps(store))
	}
}
