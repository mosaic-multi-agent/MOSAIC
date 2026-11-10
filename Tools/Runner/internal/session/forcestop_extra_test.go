package session_test

// Further force-stop behaviour: the schedule-defect and resume re-dispatch
// sites, the announcement of the recorded row, a session without a force-stop
// window, a failure that is not a force stop, and the anti-loop guard.

import (
	"context"
	"sync"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// stepRecorder is a domain.StepRecordObserver that keeps the announced entries.
type stepRecorder struct {
	mu      sync.Mutex
	entries []domain.ExecutionLogEntry
}

func (r *stepRecorder) StepRecorded(_ context.Context, e domain.ExecutionLogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

// capturingStore records the steps applied through the wrapped store.
type capturingStore struct {
	domain.ArtifactStore
	mu      sync.Mutex
	Applied []domain.CompletedStep
}

func (s *capturingStore) Apply(ctx context.Context, state domain.ArtifactState, step domain.CompletedStep) (domain.ArtifactState, error) {
	s.mu.Lock()
	s.Applied = append(s.Applied, step)
	s.mu.Unlock()
	return s.ArtifactStore.Apply(ctx, state, step)
}

func TestSession_ForceStop_RecordedRow_IsAnnouncedThroughStepRecords(t *testing.T) {
	r := newForceStopRig(t, domain.ForceStopNextStop)
	rec := &stepRecorder{}
	r.ses = session.New(session.Deps{
		Harness: newBlockingHarness(r.f, r.win, "agent-a", 1), Store: r.store, Clock: fixedClock{t: epoch},
		Interact: r.user, Routing: r.routing, Manual: r.manual, ForceStop: r.win, StepRecords: rec,
	})

	got, err := r.ses.Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunStopped)
	if len(rec.entries) != 1 {
		t.Fatalf("want the force-stopped row announced once, got %+v", rec.entries)
	}
	e := rec.entries[0]
	if e.Status != domain.StatusPARTIALLY_DONE || e.Summary != domain.ForceStoppedSummary {
		t.Errorf("want the announced entry to be the force-stopped row, got %s %q", e.Status, e.Summary)
	}
}

func TestSession_ForceStop_ScheduleDefectRedispatch_RecordsUnacceptedRow(t *testing.T) {
	rig := newBlockRig(t, "audit-plan-block-orch.md", nil, "planner", "auditor", "merger")
	rig.queueDefectivePlanner(3)
	win, user := &forceWindow{}, &choiceUser{option: domain.ForceStopNextStop}
	rig.ses = session.New(session.Deps{
		Harness: newBlockingHarness(rig.adapter, win, "planner", 2), Store: rig.store, Routing: rig.consult,
		Clock: fixedClock{t: epoch}, Interact: user, ForceStop: win,
	})

	got, err := rig.ses.Start(context.Background(), rig.config(sdWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, rig.store, "planner", false)
	if n := countE501Steps(rig.store); n != 0 {
		t.Errorf("want no BLOCKED/E501 row for a force stop, got %d", n)
	}
	if rig.consult.CallCount != 0 {
		t.Errorf("want no consultation after the killed re-dispatch, got %d", rig.consult.CallCount)
	}
	if n := rig.invocationsOf("planner"); n != 1 {
		t.Errorf("want only the first planner dispatch completed, got %d", n)
	}
	if n := user.postKillAsks(); n != 1 {
		t.Errorf("want the post-kill question asked once, got %d", n)
	}
}

func TestSession_ForceStop_ResumeRedispatch_RecordsAnotherUnacceptedRowAndStops(t *testing.T) {
	run := seedSteps(t, "linear-orch.md", nil, forceStoppedStep(1, "agent-a", false, "docs/in-a.md"))
	f := harness.NewMockAdapter()
	win, user := &forceWindow{}, &choiceUser{option: domain.ForceStopNextStop}
	store := &capturingStore{ArtifactStore: artifact.NewFileStore(run.artifactPath)}
	ses := session.New(session.Deps{
		Harness: newBlockingHarness(f, win, "agent-a", 1), Store: store, Routing: &scriptedRoutingConsultant{},
		Clock: fixedClock{t: epoch}, Interact: user, ForceStop: win,
	})

	got, err := ses.Start(context.Background(), run.resumeConfig())

	requireRunStatus(t, got, err, domain.RunStopped)
	if len(store.Applied) != 1 || !store.Applied[0].ForceStopped || store.Applied[0].Summary != domain.ForceStoppedSummary {
		t.Fatalf("want the killed resume re-dispatch recorded as a force-stopped row, got %+v", store.Applied)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 0 {
		t.Errorf("want no completed agent-a dispatch after the kill, got %d", n)
	}
	if n := user.postKillAsks(); n != 1 {
		t.Errorf("want the post-kill question asked once, got %d", n)
	}
}

// A session built without a force-stop window (the CLI wiring) dispatches,
// retries a harness error and completes as before.
func TestSession_NoForceStopWindow_HarnessErrorIsStillRetriedAsE501(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, store := harness.NewMockAdapter(), &memStore{}
	f.Queue("agent-a", e501Entry(), successEntry("done"))
	f.Queue("agent-b", successEntry("done"))
	ses := session.New(session.Deps{Harness: f, Store: store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := countE501Steps(store); n != 1 {
		t.Errorf("want the harness error recorded as one E501 row, got %d", n)
	}
	if len(forceStoppedSteps(store)) != 0 {
		t.Errorf("want no force-stopped row without a window, got %+v", forceStoppedSteps(store))
	}
}

// With a window open but no force stop requested, a failing invocation keeps
// the harness-error handling.
func TestSession_ForceStop_HarnessErrorInsideAnUnforcedWindow_StaysAnE501(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, store, win := harness.NewMockAdapter(), &memStore{}, &forceWindow{}
	f.Queue("agent-a", e501Entry(), successEntry("done"))
	f.Queue("agent-b", successEntry("done"))
	user := &choiceUser{option: domain.ForceStopNextStop}
	ses := session.New(session.Deps{Harness: f, Store: store, Clock: fixedClock{t: epoch}, Interact: user, ForceStop: win})

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := countE501Steps(store); n != 1 {
		t.Errorf("want the harness error recorded as one E501 row, got %d", n)
	}
	if len(forceStoppedSteps(store)) != 0 || user.postKillAsks() != 0 {
		t.Errorf("want no force stop handling for an unrequested failure, got %+v", forceStoppedSteps(store))
	}
}

// The anti-loop guard counts the orchestrator's dispatches as if the
// force-stopped one had not happened: after two completed dispatches, the
// killed third, and a manually chosen fourth, the orchestrator's next request
// of the same agent is still carried out.
func TestSession_ForceStop_KilledDispatch_DoesNotCountForTheAntiLoopGuard(t *testing.T) {
	routing, manual := &scriptedRoutingConsultant{}, &scriptedRoutingConsultant{}
	for i := 0; i < 3; i++ {
		routing.queueDispatch("agent-a", "attempt", 0)
	}
	manual.queueDispatch("agent-a", "go on", 0)
	routing.queueDispatch("agent-a", "attempt", 0)
	routing.queueStop("done")
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, store, win := harness.NewMockAdapter(), &memStore{}, &forceWindow{}
	queueRepeated(f, "agent-a", 6, successEntry("done"))
	ses := session.New(session.Deps{
		Harness: newBlockingHarness(f, win, "agent-a", 3), Store: store, Clock: fixedClock{t: epoch},
		Interact: &choiceUser{option: domain.ForceStopNextManual}, Routing: routing, Manual: manual, ForceStop: win,
	})

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 4 {
		t.Errorf("want two orchestrated, one manual and one further orchestrated dispatch carried out, got %d", n)
	}
}
