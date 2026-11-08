package session_test

// Tests for resume and for the budgets after a force stop, on a run recorded
// through the real artifact file store: a force-stopped workflow agent is
// re-dispatched on resume with its recorded inputs, a force-stopped
// infrastructure agent is skipped by routing, and the force-stopped row feeds
// none of the retry budgets.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// forceStoppedStep is the unaccepted row of a killed attempt.
func forceStoppedStep(seq int, agent string, infra bool, inputs string) domain.CompletedStep {
	row := domain.WorkflowRowFromIndex(0)
	if infra {
		row = domain.NoWorkflowRow
	}
	return domain.CompletedStep{
		Seq: seq, AgentInstance: fmt.Sprintf("%s#%d", agent, seq), Phase: "PLANNING", WorkflowRow: row,
		Status: domain.StatusPARTIALLY_DONE, Summary: domain.ForceStoppedSummary, Timestamp: epoch,
		Inputs: inputs, IsInfrastructure: infra, ForceStopped: true,
	}
}

// acceptedStep is an accepted workflow attempt of agent-a on the first row.
func acceptedStep(seq int, status domain.StatusCode, code domain.ErrorCode) domain.CompletedStep {
	return domain.CompletedStep{
		Seq: seq, AgentInstance: fmt.Sprintf("agent-a#%d", seq), Phase: "PLANNING",
		WorkflowRow: domain.WorkflowRowFromIndex(0), Status: status, ErrorCode: code,
		Summary: "seeded attempt", Timestamp: epoch,
	}
}

// seedSteps lays out fixture on disk and records steps through the file store.
func seedSteps(t *testing.T, fixture string, extraAgents []string, steps ...domain.CompletedStep) seededRun {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, fixture)
	for _, a := range append([]string{"agent-a", "agent-b"}, extraAgents...) {
		writeAgentFile(t, dir, a)
	}
	runFolder := filepath.Join(dir, domain.RunScopedFolder(testRunID))
	if err := os.MkdirAll(runFolder, 0o700); err != nil {
		t.Fatalf("seedSteps: %v", err)
	}
	run := seededRun{orchPath: orchPath, artifactPath: filepath.Join(runFolder, "Orchestration.md"), runFolder: runFolder}
	store := artifact.NewFileStore(run.artifactPath)
	state, err := store.Create(context.Background(), domain.WorkflowInfo{ID: "linear", Version: "1.0"}, "test task",
		domain.RunSettings{Mode: domain.ExecutionModeAuto}, epoch, testRunID)
	if err != nil {
		t.Fatalf("seedSteps: create: %v", err)
	}
	for _, s := range steps {
		if state, err = store.Apply(context.Background(), state, s); err != nil {
			t.Fatalf("seedSteps: apply %s: %v", s.AgentInstance, err)
		}
	}
	return run
}

func TestSession_ForceStopResume_WorkflowAgent_RedispatchesTheKilledAssignmentWithItsRecordedInputs(t *testing.T) {
	run := seedSteps(t, "linear-orch.md", nil, forceStoppedStep(1, "agent-a", false, "docs/in-a.md, docs/in-b.md"))
	f := harness.NewMockAdapter()
	f.Queue("agent-a", successEntry("done"))
	f.Queue("agent-b", successEntry("done"))

	got, err := run.newSession(f, &scriptedRoutingConsultant{}).Start(context.Background(), run.resumeConfig())

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := f.Invocations()
	if len(invs) == 0 || invs[0].Agent.Identifier != "agent-a" {
		t.Fatalf("want the killed agent-a assignment dispatched first, got %v", invokedAgents(f))
	}
	req := invs[0].Request
	if req.AgentInstanceID != "agent-a#2" {
		t.Errorf("want the re-dispatch to take the next sequence (agent-a#2), got %q", req.AgentInstanceID)
	}
	if len(req.InputArtifacts) != 2 || !hasSuffixIn(req.InputArtifacts, "docs/in-a.md") || !hasSuffixIn(req.InputArtifacts, "docs/in-b.md") {
		t.Errorf("want the recorded inputs of the killed attempt, got %v", req.InputArtifacts)
	}
}

func TestSession_ForceStopResume_InfrastructureAgent_RoutesNormallyFromTheLastAcceptedState(t *testing.T) {
	run := seedSteps(t, "interval-agent-orch.md", []string{"checkpoint-manager-git"},
		acceptedStep(1, domain.StatusSUCCESS, domain.ErrorNone), forceStoppedStep(2, "checkpoint-manager-git", true, ""))
	f := harness.NewMockAdapter()
	f.Queue("agent-b", successEntry("done"))
	f.Queue("checkpoint-manager-git", successEntry("checkpoint taken"))
	cfg := run.resumeConfig()
	cfg.Checkpoints = true

	got, err := run.newSession(f, &scriptedRoutingConsultant{}).Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 0 {
		t.Errorf("want the accepted agent-a not re-dispatched, got %d", n)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-b"); n != 1 {
		t.Errorf("want routing to continue with agent-b, got %d agent-b dispatches", n)
	}
	// The force-stopped infrastructure attempt is not replayed: the trigger already
	// fired for the accepted agent-a, and agent-b ends the workflow, so normal
	// routing leaves nothing that would start the checkpoint agent again.
	if n := countInvocationsFor(f.Invocations(), "checkpoint-manager-git"); n != 0 {
		t.Errorf("want the force-stopped checkpoint agent not re-dispatched, got %d runs", n)
	}
}

// A force-stopped row between two PARTIALLY_DONE attempts does not extend the
// run the bound counts: three real attempts still allow the mechanical retry.
func TestSession_ForceStopBudgets_PartiallyDoneBound_IgnoresTheForceStoppedRow(t *testing.T) {
	run := seedSteps(t, "linear-orch.md", nil,
		acceptedStep(1, domain.StatusPARTIALLY_DONE, domain.ErrorNone),
		acceptedStep(2, domain.StatusPARTIALLY_DONE, domain.ErrorNone),
		forceStoppedStep(3, "agent-a", false, ""))
	consultant := &scriptedRoutingConsultant{}
	f := harness.NewMockAdapter()
	f.Queue("agent-a", statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "partial"), successEntry("done"))
	f.Queue("agent-b", successEntry("done"))

	got, err := run.newSession(f, consultant).Start(context.Background(), run.resumeConfig())

	requireRunStatus(t, got, err, domain.RunCompleted)
	if consultant.CallCount != 0 {
		t.Errorf("want the third real PARTIALLY_DONE attempt still retried mechanically, got %d consultations", consultant.CallCount)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 2 {
		t.Errorf("want the re-dispatch of the killed attempt and one mechanical retry, got %d agent-a dispatches", n)
	}
}

// A force-stopped row between two BLOCKED/E501 attempts is not an E501
// attempt: two real attempts leave exactly one dispatch of the budget, which
// would be gone were the force-stopped row counted. The E501 counter reads the
// error code, so this is regression coverage of the row's representation.
func TestSession_ForceStopBudgets_E501Budget_IgnoresTheForceStoppedRow(t *testing.T) {
	run := seedSteps(t, "linear-orch.md", nil,
		acceptedStep(1, domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE),
		forceStoppedStep(2, "agent-a", false, ""),
		acceptedStep(3, domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE))
	consultant := &scriptedRoutingConsultant{}
	f := harness.NewMockAdapter()
	f.Queue("agent-a", successEntry("done"))
	f.Queue("agent-b", successEntry("done"))

	got, err := run.newSession(f, consultant).Start(context.Background(), run.resumeConfig())

	requireRunStatus(t, got, err, domain.RunCompleted)
	if consultant.CallCount != 0 {
		t.Errorf("want the third real E501 attempt dispatched within the budget, got %d consultations", consultant.CallCount)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 1 {
		t.Errorf("want the last E501 budget dispatch carried out, got %d agent-a dispatches", n)
	}
}
