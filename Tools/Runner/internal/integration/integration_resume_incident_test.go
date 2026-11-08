package integration_test

// Reconstruction of a resume incident: after a plan review with findings, the
// engine routed back to the planner, whose HITL outputs were not approved
// (PARTIALLY_DONE, row not accepted into current_state). A graceful stop was
// observed at the HITL re-dispatch, and the resumed run dispatched the
// implementation agent instead of the planner.
//
// The workflow and the artifact are tool-local fixtures under
// testdata/integration/incident.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

const incidentFixtureDir = "../../testdata/integration/incident"

// incidentPlanTDD is the plan the re-dispatched planner writes:
// its stage 1 runs both groups (the recorded plan runs the Impl group only).
const incidentPlanTDD = `# Plan

## Stages

| Stage | Name | Goal | Depends On | HITL | Approach |
|-------|------|------|------------|:----:|----------|
| 1 | Stage One | Address the review comments | - | FALSE | TDD |
`

type incidentRun struct {
	adapter   *harness.MockAdapter
	cfg       domain.RunConfig
	runFolder string
	artifact  string
}

// newIncidentRun lays the incident fixtures out in a fresh workspace (also the
// working directory) and returns the run ready to resume.
func newIncidentRun(t *testing.T) *incidentRun {
	t.Helper()
	fixtures, absErr := filepath.Abs(incidentFixtureDir)
	if absErr != nil {
		t.Fatalf("resolve fixtures: %v", absErr)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	runFolder := scopedRunFolder(t, dir)
	orchPath := copyFile(t, dir, "orchestrator.md", filepath.Join(fixtures, "orchestrator.md"))
	artifactPath := copyFile(t, runFolder, "Orchestration.md", filepath.Join(fixtures, "Orchestration.md"))
	copyFile(t, runFolder, "Plan.md", filepath.Join(fixtures, "Plan.md"))
	for _, a := range []string{
		"pull-request-comment-interface", "requirements-refinement", "requirements-review",
		"codebase-research", "planner-tdd-soft", "plan-review", "test-writer-tdd",
		"tests-review-tdd", "implementation-tdd", "implementation-review", "test-runner",
	} {
		writeAgentFile(t, dir, a)
	}
	f := harness.NewMockAdapter()
	f.SetWriteRoot(dir)
	return &incidentRun{
		adapter: f, runFolder: runFolder, artifact: artifactPath,
		cfg: domain.RunConfig{
			RunID:                integrationRunID,
			OrchestratorFilePath: orchPath,
			WorkflowID:           "incident-pr-fix",
			Task:                 "Fix the pull request comments",
			IsNewRun:             false,
			RunFolder:            runFolder,
			RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAutoReview, ReviewLoopLimit: 3},
		},
	}
}

// queueRestOfRun scripts the dispatches that follow: the planner writes a plan
// whose stage runs both groups (TDD), then the review, that group, and the
// test runner succeed.
func (r *incidentRun) queueRestOfRun() {
	planner := okResponse("plan revised")
	planner.Writes = []harness.ScriptedWrite{{Path: runPath("Plan.md"), Content: incidentPlanTDD}}
	r.adapter.Queue("planner-tdd-soft", planner)
	for _, a := range []string{"plan-review", "test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review", "test-runner"} {
		r.adapter.Queue(a, okResponse("ok"))
	}
}

func TestIntegration_IncidentResume_RedispatchesPlannerAtRow5NotImplementationAtRow9(t *testing.T) {
	r := newIncidentRun(t)
	r.queueRestOfRun()

	got, err := newSession(r.adapter, r.artifact).Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	invs := r.adapter.Invocations()
	if len(invs) == 0 {
		t.Fatal("want dispatches after the resume, got none")
	}
	first := invs[0]
	if first.Agent.Identifier != "planner-tdd-soft" {
		t.Fatalf("want the interrupted planner dispatched first, got %q", first.Agent.Identifier)
	}
	if first.Request.AgentInstanceID != "planner-tdd-soft#8" {
		t.Errorf("want planner-tdd-soft#8 (next sequence after the recorded attempt), got %q", first.Request.AgentInstanceID)
	}
	if !first.Request.HumanInTheLoop {
		t.Error("want the planner row's HITL on the re-dispatch")
	}
	if !containsArtifact(first.Request.InputArtifacts, "plan-review.md") {
		t.Errorf("want the recorded Inputs (including the review that routed back), got %v", first.Request.InputArtifacts)
	}
}

// Support coverage: the re-dispatched planner writes the Source of the Stage
// block, so the schedule is read again. The new plan adds the
// Test group too; a stale schedule would dispatch implementation-tdd next.
func TestIntegration_IncidentResume_RedispatchedPlannerWritingSource_RereadsSchedule(t *testing.T) {
	r := newIncidentRun(t)
	r.queueRestOfRun()

	got, err := newSession(r.adapter, r.artifact).Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	var order []string
	for _, inv := range r.adapter.Invocations() {
		order = append(order, inv.Agent.Identifier)
	}
	want := []string{"planner-tdd-soft", "plan-review", "test-writer-tdd", "tests-review-tdd", "implementation-tdd", "implementation-review", "test-runner"}
	if len(order) != len(want) {
		t.Fatalf("dispatch order: want %v, got %v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("dispatch order: want %v, got %v", want, order)
		}
	}
}

// The recorded attempt row stays in the log and the re-dispatch is appended
// after it; current_state moves only with the accepted outcome.
func TestIntegration_IncidentResume_KeepsRecordedRowAndAppendsRedispatch(t *testing.T) {
	r := newIncidentRun(t)
	r.queueRestOfRun()

	got, err := newSession(r.adapter, r.artifact).Start(context.Background(), r.cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	data, readErr := os.ReadFile(r.artifact)
	if readErr != nil {
		t.Fatalf("read artifact: %v", readErr)
	}
	text := string(data)
	for _, id := range []string{"planner-tdd-soft#7", "planner-tdd-soft#8", "plan-review#9"} {
		if !strings.Contains(text, id) {
			t.Errorf("want %s in the Execution Log, artifact:\n%s", id, text)
		}
	}
}
