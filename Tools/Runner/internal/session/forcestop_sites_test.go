package session_test

// Tests that every invocation site of a dispatched agent is force-stoppable
// with the same recording and budget rules: consultant-routed dispatch, manual
// dispatch, raw-text bypass, HITL re-dispatch, infrastructure agent dispatch
// and commit setup.

import (
	"context"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// siteRun is a session whose nth invocation of one agent is force-stopped.
type siteRun struct {
	ses   session.Session
	f     *harness.MockAdapter
	store *memStore
	user  *choiceUser
}

type siteDeps struct {
	routing   domain.RoutingConsultant
	manual    domain.RoutingConsultant
	approvals domain.ApprovalReader
}

// newSiteRun lays out the fixture and wires a session that force-stops the
// nth invocation of agent. choice answers the post-kill question.
func newSiteRun(t *testing.T, fixture string, agents []string, agent string, nth int, choice string, d siteDeps) (siteRun, string) {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, fixture)
	for _, a := range agents {
		writeAgentFile(t, dir, a)
	}
	r := siteRun{f: harness.NewMockAdapter(), store: &memStore{}, user: &choiceUser{option: choice}}
	win := &forceWindow{}
	r.ses = session.New(session.Deps{
		Harness: newBlockingHarness(r.f, win, agent, nth), Store: r.store, Clock: fixedClock{t: epoch},
		Interact: r.user, Routing: d.routing, Manual: d.manual, Approvals: d.approvals, ForceStop: win,
	})
	return r, orchPath
}

func requireNoRetryOrConsultation(t *testing.T, r siteRun, consultCalls int, wantCalls int) {
	t.Helper()
	requireE501Rows(t, r, 0)
	requireConsultations(t, consultCalls, wantCalls)
}

// requireE501Rows asserts the number of BLOCKED/E501 rows. A bypass scenario
// legitimately records one for the original raw-text failure; the killed
// bypass attempt itself must add none.
func requireE501Rows(t *testing.T, r siteRun, want int) {
	t.Helper()
	if n := countE501Steps(r.store); n != want {
		t.Errorf("want %d BLOCKED/E501 rows (none for the force stop), got %d", want, n)
	}
}

func requireConsultations(t *testing.T, consultCalls int, wantCalls int) {
	t.Helper()
	if consultCalls != wantCalls {
		t.Errorf("want %d consultations, got %d", wantCalls, consultCalls)
	}
}

func TestSession_ForceStop_ConsultantRoutedDispatch_RecordsUnacceptedRow(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "draft the plan", 0)
	r, orchPath := newSiteRun(t, "linear-orch.md", linearAgents, "agent-a", 1, domain.ForceStopNextStop, siteDeps{routing: consultant})

	got, err := r.ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, r.store, "agent-a", false)
	requireNoRetryOrConsultation(t, r, consultant.CallCount, 1)
	if r.store.state.CurrentState.LastAgent != "" {
		t.Errorf("want current_state unchanged, got %+v", r.store.state.CurrentState)
	}
	if n := r.user.postKillAsks(); n != 1 {
		t.Errorf("want the post-kill question asked once, got %d", n)
	}
}

func TestSession_ForceStop_ManualRoutedDispatch_RecordsUnacceptedRowAndAsksAgain(t *testing.T) {
	win := &forceWindow{}
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
	writeAgentFile(t, dir, "agent-a")
	writeAgentFile(t, dir, "agent-b")
	f, store := harness.NewMockAdapter(), &memStore{}
	manual := &scriptedRoutingConsultant{}
	manual.queueDispatch("agent-b", "go on", 1)
	h := newBlockingHarness(newBlockingHarness(f, win, "agent-a", 1), win, "agent-b", 1)
	user := &choiceUser{option: domain.ForceStopNextManual, stopAfterFirst: true}
	ses := session.New(session.Deps{
		Harness: h, Store: store, Clock: fixedClock{t: epoch}, Interact: user, Manual: manual, ForceStop: win,
	})

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if got.Status != domain.RunStopped {
		t.Errorf("want RunStopped after choosing stop at the second kill, got %q (%q)", got.Status, got.Message)
	}
	rows := forceStoppedSteps(store)
	if len(rows) != 2 || rows[0].AgentInstance == rows[1].AgentInstance {
		t.Fatalf("want two force-stopped rows for the two killed attempts, got %+v", store.Applied)
	}
	if n := user.postKillAsks(); n != 2 {
		t.Errorf("want the post-kill question after each kill, got %d", n)
	}
	if n := countE501Steps(store); n != 0 {
		t.Errorf("want no BLOCKED/E501 row, got %d", n)
	}
}

func TestSession_ForceStop_RawTextBypassRedispatch_RecordsUnacceptedRowWithoutRetry(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	r, orchPath := newSiteRun(t, "linear-orch.md", linearAgents, "agent-a", 2, domain.ForceStopNextStop, siteDeps{routing: consultant})
	r.f.Queue("agent-a", harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrMalformedJSON)}, successEntry("late"), successEntry("late"))

	got, err := r.ses.Start(context.Background(), configForMode(orchPath, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, r.store, "agent-a", false)
	requireE501Rows(t, r, 1)
	requireConsultations(t, consultant.CallCount, 0)
	if n := countInvocationsFor(r.f.Invocations(), "agent-a"); n != 1 {
		t.Errorf("want only the original raw-text invocation to have completed (the bypass was killed, none after), got %d", n)
	}
}

func TestSession_ForceStop_ConsultantRoutedBypassRedispatch_RecordsUnacceptedRow(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "draft the plan", 0)
	r, orchPath := newSiteRun(t, "linear-orch.md", linearAgents, "agent-a", 2, domain.ForceStopNextStop, siteDeps{routing: consultant})
	r.f.Queue("agent-a", harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrEmptyResponse)})

	got, err := r.ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, r.store, "agent-a", false)
	requireE501Rows(t, r, 1)
	requireConsultations(t, consultant.CallCount, 1)
}

func TestSession_ForceStop_HITLRedispatch_AutoMode_RecordsUnacceptedRow(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	r, orchPath := newSiteRun(t, "hitl-linear-orch.md", linearAgents, "agent-a", 2, domain.ForceStopNextStop,
		siteDeps{routing: consultant, approvals: &fixedApprovalReader{domain.ApprovalFalse}})
	r.f.Queue("agent-a", successEntry("plan v1"), successEntry("plan v2"))

	got, err := r.ses.Start(context.Background(), configForMode(orchPath, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, r.store, "agent-a", false)
	requireNoRetryOrConsultation(t, r, consultant.CallCount, 0)
	if n := countInvocationsFor(r.f.Invocations(), "agent-a"); n != 1 {
		t.Errorf("want the killed HITL re-dispatch not retried or escalated, got %d completed agent-a invocations", n)
	}
}

func TestSession_ForceStop_HITLRedispatch_ConsultantRouted_RecordsUnacceptedRow(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "draft the plan", 0)
	r, orchPath := newSiteRun(t, "hitl-linear-orch.md", linearAgents, "agent-a", 2, domain.ForceStopNextStop,
		siteDeps{routing: consultant, approvals: &fixedApprovalReader{domain.ApprovalFalse}})
	r.f.Queue("agent-a", successEntry("plan v1"), successEntry("plan v2"))

	got, err := r.ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, r.store, "agent-a", false)
	requireNoRetryOrConsultation(t, r, consultant.CallCount, 1)
}

func TestSession_ForceStop_HITLRedispatchBypass_AutoMode_RecordsUnacceptedRow(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	r, orchPath := newSiteRun(t, "hitl-linear-orch.md", linearAgents, "agent-a", 3, domain.ForceStopNextStop,
		siteDeps{routing: consultant, approvals: &fixedApprovalReader{domain.ApprovalFalse}})
	r.f.Queue("agent-a", successEntry("plan v1"), harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrMalformedJSON)})

	got, err := r.ses.Start(context.Background(), configForMode(orchPath, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, r.store, "agent-a", false)
	requireE501Rows(t, r, 1)
	requireConsultations(t, consultant.CallCount, 0)
	if n := countInvocationsFor(r.f.Invocations(), "agent-a"); n != 2 {
		t.Errorf("want the killed bypass not retried (2 completed agent-a invocations), got %d", n)
	}
}

func TestSession_ForceStop_InfrastructureDispatch_RecordsUnacceptedInfrastructureRow(t *testing.T) {
	r, orchPath := newSiteRun(t, "interval-agent-orch.md", append([]string{"checkpoint-manager-git"}, linearAgents...),
		"checkpoint-manager-git", 1, domain.ForceStopNextStop, siteDeps{})
	r.f.Queue("agent-a", successEntry("done"))
	r.f.Queue("agent-b", successEntry("done"))

	got, err := r.ses.Start(context.Background(), checkpointConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, r.store, "checkpoint-manager-git", true)
	if n := countE501Steps(r.store); n != 0 {
		t.Errorf("want no BLOCKED/E501 row, got %d", n)
	}
	if n := countInvocationsFor(r.f.Invocations(), "agent-b"); n != 0 {
		t.Errorf("want agent-b not dispatched after the kill, got %d", n)
	}
	if st := r.store.state.CurrentState; st.LastAgent != "agent-a#1" {
		t.Errorf("want current_state to stay at the last accepted workflow step agent-a#1, got %+v", st)
	}
	if n := r.user.postKillAsks(); n != 1 {
		t.Errorf("want the post-kill question for an infrastructure agent too, got %d", n)
	}
}

func TestSession_ForceStop_InfrastructureDispatch_ManualChoiceOpensTheManualDialogue(t *testing.T) {
	manual := &scriptedRoutingConsultant{}
	manual.queueDispatch("agent-b", "go on", 1)
	r, orchPath := newSiteRun(t, "interval-agent-orch.md", append([]string{"checkpoint-manager-git"}, linearAgents...),
		"checkpoint-manager-git", 1, domain.ForceStopNextManual, siteDeps{manual: manual})
	r.f.Queue("agent-a", successEntry("done"))
	r.f.Queue("agent-b", successEntry("done"))
	r.f.Queue("checkpoint-manager-git", successEntry("checkpoint"))

	got, err := r.ses.Start(context.Background(), checkpointConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireForceStoppedRow(t, r.store, "checkpoint-manager-git", true)
	if manual.CallCount != 1 {
		t.Fatalf("want the manual dialogue opened once, got %d", manual.CallCount)
	}
	req := manual.Requests[0]
	if req.Deviation != nil || req.LastStatusMessage == nil || *req.LastStatusMessage != domain.ForceStoppedSummary {
		t.Errorf("want a dialogue request without deviation that carries the force-stop summary, got %+v", req)
	}
	if n := countInvocationsFor(r.f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want the manually chosen agent-b dispatched, got %d", n)
	}
}

func TestSession_ForceStop_CommitSetup_RecordsRowAndStopsWithoutQuestion(t *testing.T) {
	r, orchPath := newSiteRun(t, "commit-agent-orch.md", append([]string{"commit-manager-git"}, linearAgents...),
		"commit-manager-git", 1, domain.ForceStopNextManual, siteDeps{manual: &scriptedRoutingConsultant{}})

	got, err := r.ses.Start(context.Background(), baseCommitConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStopped)
	requireForceStoppedRow(t, r.store, "commit-manager-git", true)
	if n := r.user.postKillAsks(); n != 0 {
		t.Errorf("want no post-kill question for commit setup, got %d", n)
	}
	if n := len(r.f.Invocations()); n != 0 {
		t.Errorf("want no workflow dispatch after a force-stopped commit setup, got %d", n)
	}
	if r.store.state.CommitBranch != "" {
		t.Errorf("want no commit branch recorded, got %q", r.store.state.CommitBranch)
	}
}

// checkpointConfig enables the checkpoint class so its triggers are evaluated.
func checkpointConfig(orchPath string) domain.RunConfig {
	cfg := baseLinearConfig(orchPath)
	cfg.Checkpoints = true
	return cfg
}
