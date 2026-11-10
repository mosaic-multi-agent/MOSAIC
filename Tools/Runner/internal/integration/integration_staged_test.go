package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// ===== Staged workflow completion =====

// TestIntegration_StagedWorkflow_TwoStages_Completes verifies that a staged
// implementation-only workflow with two stages dispatches the correct set of
// agents (2 rows × 2 stages = 4 total invocations) and returns RunCompleted.
func TestIntegration_StagedWorkflow_TwoStages_Completes(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyFile(t, dir, "orchestrator.md",
		filepath.Join(sessionTestdataDir, "staged-orch.md"))
	writeAgentFile(t, dir, "implementation-tdd")
	writeAgentFile(t, dir, "implementation-review")

	// Write Plan.md with two stages so the session can read the stage set.
	planContent := `# Plan

## Stages

| Stage | Name | Goal | Depends On | HITL |
|-------|------|------|------------|:----:|
| 1 | Stage One | First stage | - | FALSE |
| 2 | Stage Two | Second stage | 1 | FALSE |
`
	if err := os.WriteFile(filepath.Join(dir, "Plan.md"), []byte(planContent), 0600); err != nil {
		t.Fatalf("write Plan.md: %v", err)
	}

	artifactPath := filepath.Join(dir, "Orchestration.md")
	f := harness.NewMockAdapter()

	// Stage 1: implementation-tdd → implementation-review.
	f.Queue("implementation-tdd", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "implementation-tdd#1",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "stage 1 implemented",
	}})
	f.Queue("implementation-review", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "implementation-review#2",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "stage 1 reviewed",
	}})

	// Stage 2: implementation-tdd → implementation-review.
	f.Queue("implementation-tdd", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "implementation-tdd#3",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "stage 2 implemented",
	}})
	f.Queue("implementation-review", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "implementation-review#4",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "stage 2 reviewed",
	}})

	sess := newSession(f, artifactPath)
	cfg := domain.RunConfig{
		RunID: integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "staged",
		Task:                 "two-stage task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
		RunFolder:            dir, // Plan.md was written into dir; the session must resolve it here.
	}

	got, err := sess.Start(context.Background(), cfg)
	requireRunStatus(t, got, err, domain.RunCompleted)

	// Verify 4 harness invocations: 2 rows × 2 stages.
	invs := f.Invocations()
	if len(invs) != 4 {
		t.Errorf("want 4 harness invocations (2 per stage × 2 stages), got %d", len(invs))
	}

	// Verify stage-specific artifacts in the invocation requests.
	if len(invs) >= 1 && !containsArtifact(invs[0].Request.InputArtifacts, "Stage-1/Plan.md") {
		t.Errorf("first invocation should have Stage-1/Plan.md as input, got %v",
			invs[0].Request.InputArtifacts)
	}
	if len(invs) >= 3 && !containsArtifact(invs[2].Request.InputArtifacts, "Stage-2/Plan.md") {
		t.Errorf("third invocation should have Stage-2/Plan.md as input, got %v",
			invs[2].Request.InputArtifacts)
	}
}

// ===== No Plan.md for staged workflow =====

// TestIntegration_StagedWorkflow_NoPlanMd_RefusedAtStartNamingSource verifies
// that when the workflow's first row begins a block, so that the block's
// schedule source is a seed the run needs from its first dispatch, and no
// Plan.md exists in the run folder, the session refuses the run before any
// agent is dispatched. The refusal names the missing schedule source and the
// schedule rule it breaks, instead of panicking or dispatching an agent it
// cannot route.
func TestIntegration_StagedWorkflow_NoPlanMd_RefusedAtStartNamingSource(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyFile(t, dir, "orchestrator.md",
		filepath.Join(sessionTestdataDir, "staged-orch.md"))
	writeAgentFile(t, dir, "implementation-tdd")
	writeAgentFile(t, dir, "implementation-review")
	// Deliberately do NOT write Plan.md. SeedInputs is also deliberately
	// unset: this run never had a block schedule seeded into it at all.

	artifactPath := filepath.Join(dir, "Orchestration.md")
	f := harness.NewMockAdapter()
	sess := newSession(f, artifactPath)

	cfg := domain.RunConfig{
		RunID: integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "staged",
		Task:                 "task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
		RunFolder:            dir, // no Plan.md ever appears here
	}

	got, err := sess.Start(context.Background(), cfg)

	msg := requireRefused(t, got, err)

	if !strings.Contains(msg, "Plan.md") {
		t.Errorf("want refusal to name the missing schedule source Plan.md, got %q", msg)
	}
	if !strings.Contains(msg, "rule V1") {
		t.Errorf("want refusal to name rule V1 (source file missing), got %q", msg)
	}
	if n := len(f.Invocations()); n != 0 {
		t.Errorf("want no harness invocation before the refusal, got %d", n)
	}
}

// ===== Stage-* wildcard resolution on non-EXECUTION rows =====

// TestIntegration_StageWildcardResolution_NonExecutionRow verifies that when a
// workflow uses Stage-* wildcard paths in a non-EXECUTION row's Input column
// (as in brownfield-tdd where plan-review receives Stage-*/Plan.md), the
// session resolves the wildcard to concrete Stage-N paths for the dispatched
// agent — confirming the stage-set re-derivation path end-to-end.
//
// This guards against a regression where the literal Stage-*/Plan.md template
// string is forwarded to the agent unchanged, which would render the agent
// unable to locate the actual plan files. The brownfield-tdd FiveWorkflows
// test exercises this code path but asserts only RunCompleted; this test
// inspects the concrete InputArtifacts slice to catch silent wildcard pass-through.
func TestIntegration_StageWildcardResolution_NonExecutionRow(t *testing.T) {
	dir := t.TempDir()

	// Minimal workflow mirroring the brownfield-tdd PLANNING phase pattern:
	// planner declares Stage-*/Plan.md as output; plan-review takes
	// Stage-*/Plan.md (and Plan.md) as input. The stage-set re-derivation
	// must expand Stage-* to the concrete stage numbers found in Plan.md.
	const orchContent = `<Workflow type="core" name="wildcard-resolve" version="1.0">
## Stage Wildcard Resolution Workflow

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | planner | FALSE | plan-review | - | - | Plan.md, Stage-*/Plan.md |
| PLANNING | plan-review | FALSE | COMPLETE | planner | Plan.md, Stage-*/Plan.md | plan-review.md |
</Workflow>
`
	orchPath := writeOrchFile(t, dir, "orchestrator.md", orchContent)
	writeAgentFile(t, dir, "planner")
	writeAgentFile(t, dir, "plan-review")

	// Plan.md with two stages — the stage-set re-derivation reads this table to
	// expand Stage-*/Plan.md into Stage-1/Plan.md and Stage-2/Plan.md.
	const planContent = `# Plan

## Stages

| Stage | Name | Goal | Depends On | HITL |
|-------|------|------|------------|:----:|
| 1 | Stage One | First stage | - | FALSE |
| 2 | Stage Two | Second stage | 1 | FALSE |
`
	// The run folder is Orchestration-{run id} beside the orchestrator file.
	runFolder := filepath.Join(dir, domain.RunScopedFolder(integrationRunID))
	if err := os.MkdirAll(runFolder, 0700); err != nil {
		t.Fatalf("mkdir run folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runFolder, "Plan.md"), []byte(planContent), 0600); err != nil {
		t.Fatalf("write Plan.md: %v", err)
	}

	// Create Stage-N/Plan.md files to represent what the planner would have
	// written on disk: the session lists the files that exist in the run
	// folder for the Stage-*/Plan.md Input.
	for _, sub := range []string{"Stage-1", "Stage-2"} {
		if err := os.MkdirAll(filepath.Join(runFolder, sub), 0700); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
		if err := os.WriteFile(filepath.Join(runFolder, sub, "Plan.md"), []byte("# Stage Plan\n"), 0600); err != nil {
			t.Fatalf("write %s/Plan.md: %v", sub, err)
		}
	}

	artifactPath := filepath.Join(dir, "Orchestration.md")
	f := harness.NewMockAdapter()

	f.Queue("planner", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "planner#1",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "plan created with two stages",
	}})
	f.Queue("plan-review", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "plan-review#2",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "plan reviewed",
	}})

	sess := newSession(f, artifactPath)
	cfg := domain.RunConfig{
		RunID: integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "wildcard-resolve",
		Task:                 "task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
		RunFolder:            runFolder, // Plan.md was written into it; the session must resolve it here.
	}

	got, err := sess.Start(context.Background(), cfg)
	requireRunStatus(t, got, err, domain.RunCompleted)

	invs := f.Invocations()
	if len(invs) != 2 {
		t.Fatalf("want 2 harness invocations (planner + plan-review), got %d", len(invs))
	}

	// invs[1] is plan-review. Its InputArtifacts must contain the resolved
	// Stage-1/Plan.md and Stage-2/Plan.md, and must NOT contain the literal
	// Stage-*/Plan.md wildcard template.
	planReviewArts := invs[1].Request.InputArtifacts

	if !containsArtifact(planReviewArts, "Stage-1/Plan.md") {
		t.Errorf("plan-review InputArtifacts must contain Stage-1/Plan.md (resolved wildcard); got %v",
			planReviewArts)
	}
	if !containsArtifact(planReviewArts, "Stage-2/Plan.md") {
		t.Errorf("plan-review InputArtifacts must contain Stage-2/Plan.md (resolved wildcard); got %v",
			planReviewArts)
	}
	for _, art := range planReviewArts {
		if strings.Contains(art, "*") {
			t.Errorf("plan-review InputArtifacts must not contain literal wildcard %q; got %v",
				art, planReviewArts)
		}
	}
}
