package integration_test

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// ===== Auto and auto-review - zero consultations on SUCCESS, consult on deviation =====

// TestIntegration_AutoMode_AllSuccess_ZeroConsultations verifies that in auto
// and auto-review modes an all-SUCCESS run never invokes the routing consultant.
// The absence of a wired consultant means any consultation would end the run
// with RunDeviationUnresolved; RunCompleted proves none occurred.
func TestIntegration_AutoMode_AllSuccess_ZeroConsultations(t *testing.T) {
	for _, mode := range []domain.ExecutionMode{
		domain.ExecutionModeAuto,
		domain.ExecutionModeAutoReview,
	} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			dir := t.TempDir()
			orchPath := copyFile(t, dir, "orchestrator.md",
				filepath.Join(sessionTestdataDir, "linear-orch.md"))
			writeAgentFile(t, dir, "agent-a")
			writeAgentFile(t, dir, "agent-b")

			f := harness.NewMockAdapter()
			f.Queue("agent-a", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
				AgentInstanceID: "agent-a#1",
				StatusCode:      domain.StatusSUCCESS,
				StatusMessage:   "done",
			}})
			f.Queue("agent-b", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
				AgentInstanceID: "agent-b#2",
				StatusCode:      domain.StatusSUCCESS,
				StatusMessage:   "done",
			}})

			// No routing consultant: any deviation would produce
			// RunDeviationUnresolved. RunCompleted proves zero consultations.
			artifactPath := filepath.Join(dir, "Orchestration.md")
			sess := newSession(f, artifactPath)
			cfg := domain.RunConfig{
				RunID:                integrationRunID,
				OrchestratorFilePath: orchPath,
				WorkflowID:           "linear",
				Task:                 "all-success task",
				IsNewRun:             true,
				RunSettings:          domain.RunSettings{Mode: mode},
			}

			got, err := sess.Start(context.Background(), cfg)
			requireRunStatus(t, got, err, domain.RunCompleted)
		})
	}
}

// TestIntegration_AutoMode_Deviation_ConsultCalledOnce verifies that in auto
// and auto-review modes a non-SUCCESS response triggers exactly one routing
// consultation. The scripted consultant returns stop; the run ends with a
// resumable artifact.
func TestIntegration_AutoMode_Deviation_ConsultCalledOnce(t *testing.T) {
	for _, mode := range []domain.ExecutionMode{
		domain.ExecutionModeAuto,
		domain.ExecutionModeAutoReview,
	} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			dir := t.TempDir()
			orchPath := copyFile(t, dir, "orchestrator.md",
				filepath.Join(sessionTestdataDir, "linear-orch.md"))
			writeAgentFile(t, dir, "agent-a")
			writeAgentFile(t, dir, "agent-b")

			f := harness.NewMockAdapter()
			// agent-a returns PARTIALLY_DONE on the original dispatch and all 3 engine re-dispatches; the deviation follows.
			for i := 0; i < 4; i++ {
				f.Queue("agent-a", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
					AgentInstanceID: "agent-a#1", StatusCode: domain.StatusPARTIALLY_DONE, StatusMessage: "only partially done"}})
			}

			consultant := &intScriptedRoutingConsultant{}
			consultant.queueStop("operator requested stop after deviation")

			artifactPath := filepath.Join(dir, "Orchestration.md")
			sess := newSessionWithRouting(f, artifactPath, consultant)
			cfg := domain.RunConfig{
				RunID:                integrationRunID,
				OrchestratorFilePath: orchPath,
				WorkflowID:           "linear",
				Task:                 "deviation task",
				IsNewRun:             true,
				RunSettings:          domain.RunSettings{Mode: mode},
			}

			got, err := sess.Start(context.Background(), cfg)
			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)

			// The deviation must trigger exactly one consultation.
			if consultant.CallCount != 1 {
				t.Errorf("want exactly 1 consultation for the deviation, got %d",
					consultant.CallCount)
			}
		})
	}
}
