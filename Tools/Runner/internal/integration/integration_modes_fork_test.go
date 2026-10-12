package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// ===== Fork/join tables: admitted in orchestrated mode, refused when the engine routes =====

// forkJoinResponse builds a SUCCESS response for the given instance ID.
func forkJoinResponse(id string) *domain.ProtocolResponse {
	return &domain.ProtocolResponse{
		AgentInstanceID: id,
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "done",
	}
}

// TestIntegration_OrchestratedMode_ForkJoinTable_DispatchesBranchesSequentiallyJoinLast
// verifies that an orchestrated run over a fork/join table invokes every fork
// branch one after another and the join target last.
func TestIntegration_OrchestratedMode_ForkJoinTable_DispatchesBranchesSequentiallyJoinLast(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyFile(t, dir, "orchestrator.md",
		filepath.Join(sessionTestdataDir, "fork-join-orch.md"))
	for _, a := range []string{"agent-a", "agent-b", "agent-c", "agent-d"} {
		writeAgentFile(t, dir, a)
	}

	f := harness.NewMockAdapter()
	// Each consultation takes one Seq and the agent it dispatches the next.
	f.Queue("agent-a", harness.ScriptedEntry{Response: forkJoinResponse("agent-a#2")})
	f.Queue("agent-b", harness.ScriptedEntry{Response: forkJoinResponse("agent-b#4")})
	f.Queue("agent-c", harness.ScriptedEntry{Response: forkJoinResponse("agent-c#6")})
	f.Queue("agent-d", harness.ScriptedEntry{Response: forkJoinResponse("agent-d#8")})

	consultant := &intScriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "Proceed.", 0)
	consultant.queueDispatch("agent-b", "Proceed.", 1)
	consultant.queueDispatch("agent-c", "Proceed.", 2)
	consultant.queueDispatch("agent-d", "Proceed.", 3)
	consultant.queueStop("all workflow steps completed")

	artifactPath := filepath.Join(dir, "Orchestration.md")
	sess := newSessionWithRouting(f, artifactPath, consultant)

	got, err := sess.Start(context.Background(), domain.RunConfig{
		RunID:                integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "fork-join",
		Task:                 "fork join task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeOrchestrated},
	})

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)

	data, readErr := os.ReadFile(artifactPath)
	if readErr != nil {
		t.Fatalf("read produced artifact: %v", readErr)
	}
	state, parseErr := artifact.Parse(data)
	if parseErr != nil {
		t.Fatalf("parse produced artifact: %v", parseErr)
	}
	var order []string
	for _, e := range state.ExecutionLog {
		if strings.HasPrefix(e.Agent, "agent-") {
			order = append(order, strings.SplitN(e.Agent, "#", 2)[0])
		}
	}
	want := []string{"agent-a", "agent-b", "agent-c", "agent-d"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("workflow agents must run one at a time with the join last: want %v, got %v", want, order)
	}
}

// TestIntegration_EngineRoutedModes_ForkJoinTable_RefusedAtStart verifies that
// the same fork/join table is refused before any invocation when the engine
// routes (auto and auto-review).
func TestIntegration_EngineRoutedModes_ForkJoinTable_RefusedAtStart(t *testing.T) {
	for _, mode := range []domain.ExecutionMode{domain.ExecutionModeAuto, domain.ExecutionModeAutoReview} {
		t.Run(string(mode), func(t *testing.T) {
			dir := t.TempDir()
			orchPath := copyFile(t, dir, "orchestrator.md",
				filepath.Join(sessionTestdataDir, "fork-join-orch.md"))
			for _, a := range []string{"agent-a", "agent-b", "agent-c", "agent-d"} {
				writeAgentFile(t, dir, a)
			}
			f := harness.NewMockAdapter()
			sess := newSession(f, filepath.Join(dir, "Orchestration.md"))

			got, err := sess.Start(context.Background(), domain.RunConfig{
				RunID:                integrationRunID,
				OrchestratorFilePath: orchPath,
				WorkflowID:           "fork-join",
				Task:                 "fork join task",
				IsNewRun:             true,
				RunSettings:          domain.RunSettings{Mode: mode},
			})

			msg := requireRefused(t, got, err)
			if !strings.Contains(msg, "parallel") {
				t.Errorf("refusal must keep the parallel-dispatch message; got %q", msg)
			}
		})
	}
}

// TestIntegration_OrchestratedMode_ForkBranchEndsPhase_RefusedAtStart verifies
// that a fork branch that ends its phase without a join is refused at start in
// orchestrated mode, naming the agent.
func TestIntegration_OrchestratedMode_ForkBranchEndsPhase_RefusedAtStart(t *testing.T) {
	dir := t.TempDir()
	orchPath := copyFile(t, dir, "orchestrator.md",
		filepath.Join(sessionTestdataDir, "fork-branch-ends-phase-orch.md"))
	for _, a := range []string{"agent-a", "agent-b", "agent-c"} {
		writeAgentFile(t, dir, a)
	}
	f := harness.NewMockAdapter()
	sess := newSessionWithRouting(f, filepath.Join(dir, "Orchestration.md"), &intScriptedRoutingConsultant{})

	got, err := sess.Start(context.Background(), domain.RunConfig{
		RunID:                integrationRunID,
		OrchestratorFilePath: orchPath,
		WorkflowID:           "fork-branch-ends-phase",
		Task:                 "fork task",
		IsNewRun:             true,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeOrchestrated},
	})

	msg := requireRefused(t, got, err)
	if !strings.Contains(msg, "agent-c") || !strings.Contains(msg, "fork branch") {
		t.Errorf("refusal must name the fork branch shape and agent agent-c; got %q", msg)
	}
}
