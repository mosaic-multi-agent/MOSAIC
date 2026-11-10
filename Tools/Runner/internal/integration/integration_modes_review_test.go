package integration_test

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// ===== COMPLETED_NEEDS_ACTION - auto-review auto-routes, auto consults =====

// TestIntegration_AutoReview_CNA_AutoRouteBack_NoConsultAndArtifactInjected
// verifies that in auto-review mode a COMPLETED_NEEDS_ACTION response from a
// review agent with an unambiguous OnFindings target causes the engine to route
// back to that agent automatically, without invoking the routing consultant,
// and that the review agent's output artifacts are injected into the target's
// InputArtifacts.
func TestIntegration_AutoReview_CNA_AutoRouteBack_NoConsultAndArtifactInjected(t *testing.T) {
	dir := t.TempDir()

	// Workflow: test-writer-tdd -> build-review (OnFindings=test-writer-tdd) -> impl-tdd.
	// build-review's output is "build.md" - the artifact injected on loop-back.
	const orchContent = `<Workflow type="core" name="cna-review" version="1.0">
## CNA Review Workflow

| Phase | Subagent          | HITL | On Success  | On Findings      | Input    | Output   |
|-------|-------------------|:----:|-------------|------------------|----------|----------|
| PLANNING | test-writer-tdd | FALSE | build-review | -               | -        | tests.md |
| PLANNING | build-review    | FALSE | impl-tdd    | test-writer-tdd  | tests.md | build.md |
| PLANNING | impl-tdd        | FALSE | COMPLETE    | -                | tests.md | impl.md  |
</Workflow>
`
	orchPath := writeOrchFile(t, dir, "orchestrator.md", orchContent)
	writeAgentFile(t, dir, "test-writer-tdd")
	writeAgentFile(t, dir, "build-review")
	writeAgentFile(t, dir, "impl-tdd")

	f := harness.NewMockAdapter()
	// test-writer-tdd initial dispatch -> SUCCESS.
	f.Queue("test-writer-tdd", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "test-writer-tdd#1",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "tests written",
	}})
	// build-review -> COMPLETED_NEEDS_ACTION with unambiguous OnFindings.
	// In auto-review the engine auto-routes back to test-writer-tdd.
	f.Queue("build-review", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "build-review#2",
		StatusCode:      domain.StatusCOMPLETED_NEEDS_ACTION,
		StatusMessage:   "build failed",
	}})
	// test-writer-tdd loop-back -> SUCCESS.
	f.Queue("test-writer-tdd", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "test-writer-tdd#3",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "tests fixed",
	}})
	// build-review second attempt -> SUCCESS.
	f.Queue("build-review", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "build-review#4",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "build ok",
	}})
	// impl-tdd -> SUCCESS -> COMPLETE.
	f.Queue("impl-tdd", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "impl-tdd#5",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "implemented",
	}})

	// A routing consultant is wired but must NOT be called: the OnFindings
	// auto-route-back is engine-owned in auto-review mode.
	consultant := &intScriptedRoutingConsultant{}

	artifactPath := filepath.Join(dir, "Orchestration.md")
	sess := newSessionWithRouting(f, artifactPath, consultant)

	cfg := domain.RunConfig{
		RunID:                integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "cna-review",
		Task:                 "cna auto-review task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAutoReview},
	}

	got, err := sess.Start(context.Background(), cfg)
	requireRunStatus(t, got, err, domain.RunCompleted)

	// The routing consultant must not have been invoked: the CNA auto-route-back
	// is the engine's decision in auto-review mode.
	if consultant.CallCount != 0 {
		t.Errorf("want 0 consultant calls for auto-review OnFindings auto-route, got %d",
			consultant.CallCount)
	}

	invs := f.Invocations()
	wantOrder := []string{
		"test-writer-tdd", // initial dispatch
		"build-review",    // first attempt -> CNA
		"test-writer-tdd", // loop-back via OnFindings
		"build-review",    // second attempt -> SUCCESS
		"impl-tdd",        // continues normally
	}
	if len(invs) != len(wantOrder) {
		t.Fatalf("want %d harness invocations, got %d", len(wantOrder), len(invs))
	}
	for i, want := range wantOrder {
		if invs[i].Agent.Identifier != want {
			t.Errorf("invocation[%d]: want %q, got %q", i, want, invs[i].Agent.Identifier)
		}
	}

	// The test-writer-tdd loop-back (invs[2]) must have build-review's output
	// artifact ("build.md") injected into its InputArtifacts.
	if !containsArtifact(invs[2].Request.InputArtifacts, "build.md") {
		t.Errorf("test-writer-tdd loop-back must have build.md injected (review artifact injection); got InputArtifacts=%v",
			invs[2].Request.InputArtifacts)
	}
}

// TestIntegration_Auto_CNA_ConsultsForDeviation verifies that in auto mode a
// COMPLETED_NEEDS_ACTION response triggers a routing consultation, not an
// engine auto-route-back. The OnFindings auto-route is exclusive to auto-review.
func TestIntegration_Auto_CNA_ConsultsForDeviation(t *testing.T) {
	dir := t.TempDir()

	const orchContent = `<Workflow type="core" name="cna-auto" version="1.0">
## CNA Auto Workflow

| Phase | Subagent          | HITL | On Success  | On Findings      | Input    | Output   |
|-------|-------------------|:----:|-------------|------------------|----------|----------|
| PLANNING | test-writer-tdd | FALSE | build-review | -               | -        | tests.md |
| PLANNING | build-review    | FALSE | impl-tdd    | test-writer-tdd  | tests.md | build.md |
| PLANNING | impl-tdd        | FALSE | COMPLETE    | -                | tests.md | impl.md  |
</Workflow>
`
	orchPath := writeOrchFile(t, dir, "orchestrator.md", orchContent)
	writeAgentFile(t, dir, "test-writer-tdd")
	writeAgentFile(t, dir, "build-review")
	writeAgentFile(t, dir, "impl-tdd")

	f := harness.NewMockAdapter()
	f.Queue("test-writer-tdd", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "test-writer-tdd#1",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "tests written",
	}})
	// build-review returns CNA - in auto mode this is a deviation requiring consultation.
	f.Queue("build-review", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "build-review#2",
		StatusCode:      domain.StatusCOMPLETED_NEEDS_ACTION,
		StatusMessage:   "build failed",
	}})

	// Consultant is called once for the CNA deviation and returns stop.
	consultant := &intScriptedRoutingConsultant{}
	consultant.queueStop("build-review returned CNA in auto mode - stopping")

	artifactPath := filepath.Join(dir, "Orchestration.md")
	sess := newSessionWithRouting(f, artifactPath, consultant)

	cfg := domain.RunConfig{
		RunID:                integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "cna-auto",
		Task:                 "cna auto task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
	}

	got, err := sess.Start(context.Background(), cfg)
	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)

	// In auto mode CNA is a deviation and must route through the consultant.
	if consultant.CallCount != 1 {
		t.Errorf("want 1 consultant call when CNA deviates in auto mode, got %d",
			consultant.CallCount)
	}
}

// TestIntegration_AutoReview_ReviewLoopLimit_AutoRoutesUntilLimitThenDeviates
// verifies that in auto-review mode a reviewer that keeps returning
// COMPLETED_NEEDS_ACTION is auto-routed back to its On Findings target until
// review_loop_limit is reached, after which the run produces a deviation
// (routing consultation) instead of another auto-route.
func TestIntegration_AutoReview_ReviewLoopLimit_AutoRoutesUntilLimitThenDeviates(t *testing.T) {
	dir := t.TempDir()

	const orchContent = `<Workflow type="core" name="loop-limit" version="1.0">
## Loop Limit Workflow

| Phase | Subagent          | HITL | On Success  | On Findings      | Input    | Output   |
|-------|-------------------|:----:|-------------|------------------|----------|----------|
| PLANNING | test-writer-tdd | FALSE | build-review | -               | -        | tests.md |
| PLANNING | build-review    | FALSE | impl-tdd    | test-writer-tdd  | tests.md | build.md |
| PLANNING | impl-tdd        | FALSE | COMPLETE    | -                | tests.md | impl.md  |
</Workflow>
`
	orchPath := writeOrchFile(t, dir, "orchestrator.md", orchContent)
	writeAgentFile(t, dir, "test-writer-tdd")
	writeAgentFile(t, dir, "build-review")
	writeAgentFile(t, dir, "impl-tdd")

	f := harness.NewMockAdapter()
	queue := func(agent string, seq int, status domain.StatusCode, msg string) {
		f.Queue(agent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: agent + "#" + strconv.Itoa(seq),
			StatusCode:      status,
			StatusMessage:   msg,
		}})
	}
	queue("test-writer-tdd", 1, domain.StatusSUCCESS, "tests written")
	queue("build-review", 2, domain.StatusCOMPLETED_NEEDS_ACTION, "findings 1") // count 1 < 2: auto-routed
	queue("test-writer-tdd", 3, domain.StatusSUCCESS, "tests fixed")
	queue("build-review", 4, domain.StatusCOMPLETED_NEEDS_ACTION, "findings 2") // count 2 == limit: deviation

	consultant := &intScriptedRoutingConsultant{}
	consultant.queueStop("review loop limit reached - stopping")

	artifactPath := filepath.Join(dir, "Orchestration.md")
	sess := newSessionWithRouting(f, artifactPath, consultant)

	cfg := domain.RunConfig{
		RunID:                integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "loop-limit",
		Task:                 "review loop limit task",
		IsNewRun:             true,
		RunSettings: domain.RunSettings{
			Mode:            domain.ExecutionModeAutoReview,
			ReviewLoopLimit: 2,
		},
	}

	got, err := sess.Start(context.Background(), cfg)
	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)

	// Exactly one consultation: the deviation at the limit. The first CNA was
	// auto-routed without consulting.
	if consultant.CallCount != 1 {
		t.Errorf("want exactly 1 consultation (the limit deviation), got %d", consultant.CallCount)
	}

	invs := f.Invocations()
	wantOrder := []string{"test-writer-tdd", "build-review", "test-writer-tdd", "build-review"}
	if len(invs) != len(wantOrder) {
		t.Fatalf("want %d harness invocations (no auto-route after the limit), got %d", len(wantOrder), len(invs))
	}
	for i, want := range wantOrder {
		if invs[i].Agent.Identifier != want {
			t.Errorf("invocation[%d]: want %q, got %q", i, want, invs[i].Agent.Identifier)
		}
	}
}
