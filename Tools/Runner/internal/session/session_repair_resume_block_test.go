package session_test

// A start repair in a workflow with a block: the consultation carries the block
// schedule read from the run folder, and the dispatch the repair returns for a
// block row is validated against it and carried out for that iteration.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const blockWorkflow = "seeded-plan-first-block"

// blockRepairRig is a resume of a block-workflow run whose Execution Log is
// damaged: the first row (implementer, iteration 1) is recorded.
type blockRepairRig struct {
	f    *harness.MockAdapter
	cons *editingConsultant
	cfg  domain.RunConfig
	ses  session.Session
	path string
}

func newBlockRepairRig(t *testing.T, steps []repairStep) *blockRepairRig {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, blockWorkflow+"-orch.md")
	for _, a := range []string{"implementer", "reviewer", "test-runner"} {
		writeAgentFile(t, dir, a)
	}
	runFolder := filepath.Join(dir, domain.RunScopedFolder(testRunID))
	path := filepath.Join(runFolder, "Orchestration.md")
	text := rfArtifact(blockWorkflow, "Iteration", "iteration", `"1"`, rfRow(1, "implementer#1", "EXECUTION", "1", 1))
	text = replaceOnce(t, text, "runner_mode: auto", "runner_mode: orchestrated")
	writeFile(t, path, damageLog(t, text))
	writeFile(t, filepath.Join(runFolder, "Plan.md"), scheduleDoc("Stage", "Stages", ids("1", "2")))
	evidDir, ok := artifact.EvidenceDir(path)
	if !ok {
		t.Fatal("test setup: the run's artifact path has no evidence location")
	}
	r := &blockRepairRig{f: harness.NewMockAdapter(), path: path}
	r.cons = &editingConsultant{t: t, path: path, evidenceDir: evidDir, steps: steps}
	r.f.Queue("reviewer", successEntry("reviewed"))
	r.cfg = domain.RunConfig{
		OrchestratorFilePath: orchPath, WorkflowID: blockWorkflow, Task: "test task", IsNewRun: false,
		RunID: testRunID, RunFolder: runFolder, RunSettings: domain.RunSettings{Mode: domain.ExecutionModeOrchestrated},
	}
	r.ses = session.New(session.Deps{
		Harness: r.f, Store: artifact.NewFileStore(path), Routing: r.cons,
		Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
	})
	return r
}

func TestSession_StartRepairInABlockWorkflow_CarriesTheBlockScheduleAndRunsTheReturnedIterationDispatch(t *testing.T) {
	rig := newBlockRepairRig(t, []repairStep{
		{edit: fixLog, reply: domain.RoutingInstruction{Dispatch: &domain.DispatchInstruction{
			Agent: "reviewer", RowIndex: 1, Iteration: "1", TaskDescription: "review iteration 1",
		}}},
		{reply: stopReply("done")},
	})

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	req := rig.cons.request(0)
	requireDamageReport(t, req)
	if sched, ok := req.Schedules["Stage"]; !ok || len(sched.Iterations) != 2 {
		t.Errorf("want the Stage schedule with 2 iterations in the repair request, got %+v", req.Schedules)
	}
	invs := rig.f.Invocations()
	if len(invs) != 1 || invs[0].Agent.Identifier != "reviewer" {
		t.Fatalf("want the returned reviewer dispatch carried out once, got %d invocations", len(invs))
	}
	if !hasSuffixIn(invs[0].Request.OutputArtifacts, "Stage-1/review.md") {
		t.Errorf("want the dispatch to run for iteration 1, got outputs %v", invs[0].Request.OutputArtifacts)
	}
	if invs[0].Request.AgentInstanceID != "reviewer#2" {
		t.Errorf("want the step numbered after the repaired global_sequence, got %q", invs[0].Request.AgentInstanceID)
	}
}

func TestSession_StartRepairInABlockWorkflow_RefusesAReturnedIterationThatIsNotInTheSchedule(t *testing.T) {
	rig := newBlockRepairRig(t, []repairStep{
		{edit: fixLog, reply: domain.RoutingInstruction{Dispatch: &domain.DispatchInstruction{
			Agent: "reviewer", RowIndex: 1, Iteration: "9", TaskDescription: "review iteration 9",
		}}},
	})

	got, err := rig.ses.Start(context.Background(), rig.cfg)

	if err != nil {
		t.Fatalf("want the ending carried in the outcome, got error %v", err)
	}
	requireDamageReport(t, rig.cons.request(0)) // the repair consultation was made and its reply judged
	if got.Status == domain.RunStoppedByConsultant || got.Status == domain.RunCompleted {
		t.Errorf("want the invalid target to end the run, got status %q", got.Status)
	}
	if len(rig.f.Invocations()) != 0 {
		t.Errorf("want no dispatch for an iteration outside the schedule, got %d", len(rig.f.Invocations()))
	}
}
