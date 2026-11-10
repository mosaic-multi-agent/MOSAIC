package session_test

// Tests that a graceful stop keeps its behaviour next to a force stop: alone it
// waits for the running agent and records no force-stopped row; armed before a
// force stop, the run still ends stopped without dispatching further agents.

import (
	"context"
	"sync/atomic"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// newGracefulRun wires a linear manual-routed session whose first agent-a
// invocation arms the graceful stop and, when force is set, is then killed.
func newGracefulRun(t *testing.T, force bool) (session.Session, *harness.MockAdapter, *memStore, *choiceUser, string) {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, store := harness.NewMockAdapter(), &memStore{}
	user := &choiceUser{option: domain.ForceStopNextStop}
	var stop atomic.Bool
	var h domain.HarnessAdapter = &stopArmingHarness{delegate: f, agent: "agent-a", arm: func() { stop.Store(true) }}
	win := &forceWindow{}
	if force {
		bh := newBlockingHarness(f, win, "agent-a", 1)
		bh.onBlock = func() { stop.Store(true) }
		h = bh
	}
	ses := session.New(session.Deps{
		Harness: h, Store: store, Clock: fixedClock{t: epoch}, Interact: user,
		Manual: &scriptedRoutingConsultant{}, ForceStop: win, StopRequested: stop.Load,
	})
	return ses, f, store, user, orchPath
}

// stopArmingHarness arms the graceful stop while agent runs, then delegates.
type stopArmingHarness struct {
	delegate domain.HarnessAdapter
	agent    string
	arm      func()
}

func (h *stopArmingHarness) Invoke(ctx context.Context, agent domain.AgentReference, req domain.ProtocolRequest) (domain.ProtocolResponse, error) {
	if agent.Identifier == h.agent {
		h.arm()
	}
	return h.delegate.Invoke(ctx, agent, req)
}

func TestSession_GracefulStopAlone_WaitsForTheRunningAgentAndRecordsNoForceStoppedRow(t *testing.T) {
	ses, f, store, user, orchPath := newGracefulRun(t, false)
	f.Queue("agent-a", successEntry("done"))
	f.Queue("agent-b", successEntry("done"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 1 {
		t.Errorf("want the running agent-a to complete, got %d invocations", n)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-b"); n != 0 {
		t.Errorf("want no dispatch after the graceful stop, got %d", n)
	}
	if rows := forceStoppedSteps(store); len(rows) != 0 {
		t.Errorf("want no force-stopped row, got %+v", rows)
	}
	if n := user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question, got %d", n)
	}
}

func TestSession_GracefulStopArmed_ThenForceStop_RecordsForceStoppedRowAndEndsStopped(t *testing.T) {
	ses, f, store, _, orchPath := newGracefulRun(t, true)
	f.Queue("agent-a", successEntry("done"))
	f.Queue("agent-b", successEntry("done"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, store, "agent-a", false)
	if n := countE501Steps(store); n != 0 {
		t.Errorf("want no BLOCKED/E501 row, got %d", n)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-b"); n != 0 {
		t.Errorf("want agent-b not dispatched, got %d", n)
	}
	if store.state.CurrentState.LastAgent != "" {
		t.Errorf("want current_state unchanged, got %+v", store.state.CurrentState)
	}
}
