package app_test

// codex_workflow_parse_check_test.go specifies the unreadable-orchestrator check of Update
// workflows over a real Codex deployment: a valid deployed orchestrator .toml raises no conflict,
// and a .toml that cannot be decoded raises one (it is skipped under a skip default instead of
// being rewritten as an ordinary update). The workspace comes from a real deploy of the
// tool-local workflowpair catalog to the Codex module.

import (
	"context"
	"testing"

	_ "mosaic-deploy/internal/agentformat/all"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const codexOrchestratorRel = ".codex/agents/" + wpsOrch + ".toml"

// codexUndecodableToml has duplicate keys, so the Codex translator cannot decode it.
const codexUndecodableToml = "# mosaic_version: 2.0\n\nmodel = \"a\"\nmodel = \"b\"\n"

// codexReplaceOrchestrator overwrites the deployed orchestrator and re-records its manifest
// hash, so the file reads as unmodified and only its readability can raise a conflict.
func codexReplaceOrchestrator(t *testing.T, ws, content string) {
	t.Helper()
	gamWrite(t, ws, codexOrchestratorRel, content)
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == wpsOrch {
			m.Entries[i].ContentHash = manifest.Hash([]byte(content))
		}
	}
	gamSaveManifest(t, ws, m)
}

// codexUpdateWorkflowsOrchestratorAction runs Update workflows with the given conflict default over ws and
// returns what was done with the orchestrator file.
func codexUpdateWorkflowsOrchestratorAction(t *testing.T, ws, harnessID string, decision domain.ConflictDecision) domain.ActionTaken {
	t.Helper()
	checklist := newWukChecklist(wpsFlowParity)
	svc, _ := codexKeptUWService(t, func(d *app.Deps) { d.Interaction = checklist })
	summary, err := svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: harnessID, WorkspacePath: ws, AutoConfirmPlan: true, ConflictDefault: decision})
	if err != nil {
		t.Fatalf("UpdateWorkflows: %v", err)
	}
	for _, a := range summary.Actions {
		if a.Ref.Kind == domain.ArtifactAgent && a.Ref.Key == wpsOrch {
			return a.Taken
		}
	}
	t.Fatalf("no action recorded for the orchestrator; actions: %+v", summary.Actions)
	return ""
}

func TestUpdateWorkflows_Codex_ValidDeployedOrchestrator_NoConflict(t *testing.T) {
	// Arrange
	ws, harnessID := codexDeployedWithTrackedOrchestrator(t)

	// Act
	taken := codexUpdateWorkflowsOrchestratorAction(t, ws, harnessID, domain.DecisionSkip)

	// Assert
	if taken == domain.TakenSkipped {
		t.Errorf("orchestrator action = %q; a valid deployed Codex orchestrator must not be a conflict", taken)
	}
}

func TestUpdateWorkflows_Codex_UndecodableDeployedOrchestrator_IsConflict(t *testing.T) {
	// Arrange
	ws, harnessID := codexDeployedWithTrackedOrchestrator(t)
	codexReplaceOrchestrator(t, ws, codexUndecodableToml)

	// Act
	taken := codexUpdateWorkflowsOrchestratorAction(t, ws, harnessID, domain.DecisionSkip)

	// Assert
	if taken != domain.TakenSkipped {
		t.Errorf("orchestrator action = %q, want %q: an undecodable Codex orchestrator is a conflict resolved by the skip default",
			taken, domain.TakenSkipped)
	}
	if got := string(gamRead(t, ws, codexOrchestratorRel)); got != codexUndecodableToml {
		t.Errorf("undecodable orchestrator was rewritten; content now:\n%s", got)
	}
}

func TestUpdateWorkflows_Codex_UndecodableDeployedOrchestrator_OverwriteDecisionRewrites(t *testing.T) {
	// Arrange
	ws, harnessID := codexDeployedWithTrackedOrchestrator(t)
	codexReplaceOrchestrator(t, ws, codexUndecodableToml)

	// Act
	codexUpdateWorkflowsOrchestratorAction(t, ws, harnessID, domain.DecisionOverwrite)

	// Assert
	if got := string(gamRead(t, ws, codexOrchestratorRel)); got == codexUndecodableToml {
		t.Errorf("overwrite decision left the undecodable orchestrator in place; the conflict must be resolved by the decision")
	}
}

// codexDeployedWithTrackedOrchestrator deploys the workflow pair to Codex and runs one Update
// workflows with an overwrite default, so the orchestrator file has a manifest record and the
// next run judges it as an ordinary tracked file.
func codexDeployedWithTrackedOrchestrator(t *testing.T) (ws, harnessID string) {
	t.Helper()
	ws, harnessID = codexKeptUWDeploy(t)
	codexUpdateWorkflowsOrchestratorAction(t, ws, harnessID, domain.DecisionOverwrite)
	return ws, harnessID
}
