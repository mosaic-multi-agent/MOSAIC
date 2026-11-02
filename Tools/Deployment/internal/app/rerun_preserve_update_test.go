package app_test

// rerun_preserve_update_test.go is regression coverage, through real runs on all four built-in
// harnesses, that update and workflows keep what they have always kept on an agent they
// rewrite after its catalog version was bumped: project regions, custom regions, tool choices,
// the deployed model and the infrastructure declarations.

import (
	"bytes"
	"context"
	"os"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestUpdateRerun_StaleAgents_KeepRegionsToolsModelAndDeclarations(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{utility: []string{"parity-helper"}, infra: []string{"parity-checker"}})
			rpPlant(t, ws, h, rpRewrittenKeys...)
			declared := rpManagedRegion(t, rpReadAgent(t, ws, h, "orchestrator"), "InfrastructureAgents")
			if declared == "" {
				t.Fatalf("the first deploy declared no infrastructure agent")
			}
			rpBumpAll(t, cat)

			_, err := rpService(t, cat).Update(context.Background(), app.UpdateRequest{
				HarnessID: h, WorkspacePath: ws, SkipAll: rpSkipModels(), AutoConfirmPlan: true,
			})
			if err != nil {
				t.Fatalf("update: %v", err)
			}

			for _, key := range rpRewrittenKeys {
				got := rpReadAgent(t, ws, h, key)
				rpAssertRewritten(t, key, got)
				rpAssertKept(t, h, key, got)
				rpAssertModel(t, key, got, rpDeployedModel)
			}
			after := rpManagedRegion(t, rpReadAgent(t, ws, h, "orchestrator"), "InfrastructureAgents")
			if after != declared {
				t.Errorf("update changed the infrastructure declarations:\nbefore: %q\nafter:  %q", declared, after)
			}
		})
	}
}

func TestWorkflowsRerun_StaleOrchestrator_KeepsRegionsToolsModelAndDeclarations(t *testing.T) {
	for _, h := range parityHarnesses {
		h := h
		t.Run(h, func(t *testing.T) {
			cat := rpCopyCatalog(t)
			ws := rpFirstDeploy(t, h, cat, rpFirst{infra: []string{"parity-checker"}})
			rpPlant(t, ws, h, "orchestrator", "parity-runner")
			declared := rpManagedRegion(t, rpReadAgent(t, ws, h, "orchestrator"), "InfrastructureAgents")
			if declared == "" {
				t.Fatalf("the first deploy declared no infrastructure agent")
			}
			runnerBefore, err := os.ReadFile(rpAgentWritePath(t, ws, h, "parity-runner"))
			if err != nil {
				t.Fatalf("read deployed runner: %v", err)
			}
			rpBumpAll(t, cat)

			_, err = rpService(t, cat).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
				HarnessID: h, WorkspacePath: ws, WorkflowIDs: []string{"parity-flow"},
				SkipAll: map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true}, AutoConfirmPlan: true,
			})
			if err != nil {
				t.Fatalf("workflows: %v", err)
			}

			got := rpReadAgent(t, ws, h, "orchestrator")
			rpAssertRewritten(t, "orchestrator", got)
			rpAssertKept(t, h, "orchestrator", got)
			rpAssertModel(t, "orchestrator", got, rpDeployedModel)
			if after := rpManagedRegion(t, got, "InfrastructureAgents"); after != declared {
				t.Errorf("workflows changed the infrastructure declarations:\nbefore: %q\nafter:  %q", declared, after)
			}
			runnerAfter, err := os.ReadFile(rpAgentWritePath(t, ws, h, "parity-runner"))
			if err != nil {
				t.Fatalf("read runner after the run: %v", err)
			}
			if !bytes.Equal(runnerBefore, runnerAfter) {
				t.Errorf("workflows rewrote an agent that was already deployed")
			}
		})
	}
}
