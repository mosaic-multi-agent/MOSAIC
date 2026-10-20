package app_test

// unknown_workflow_hint_test.go verifies what the deploy, update and workflow-update entry
// points tell the user when a requested or deployed workflow id is not in the catalog stack in
// effect: the error lists the catalogs searched and, only when that workflow is already
// deployed in the workspace, says it may have come from a layer that is no longer in the stack.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/plan"
)

// removedLayerHintPhrase is the part of the removed-layer hint the tests look for.
const removedLayerHintPhrase = "no longer in the stack"

// unknownWorkflowOp is one entry point driven with a workflow id the stack does not supply.
type unknownWorkflowOp struct {
	name string
	run  func(svc app.Service, ws, workflowID string) error
}

// unknownWorkflowOps lists the entry points that probe workflow ids: deploy, workflow update,
// and update (through the id of a workflow already deployed in the workspace).
func unknownWorkflowOps() []unknownWorkflowOp {
	return []unknownWorkflowOp{
		{"deploy", func(svc app.Service, ws, id string) error {
			_, err := svc.DeployNew(context.Background(), app.DeployRequest{HarnessID: "stub-harness",
				WorkspacePath: ws, WorkflowIDs: []string{id}, AutoConfirmPlan: true})
			return err
		}},
		{"workflows", func(svc app.Service, ws, id string) error {
			_, err := svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{HarnessID: "stub-harness",
				WorkspacePath: ws, WorkflowIDs: []string{id}, AutoConfirmPlan: true})
			return err
		}},
	}
}

// requireSearchedCatalogs fails the test unless err names both fixture catalogs.
func requireSearchedCatalogs(t *testing.T, err error) {
	t.Helper()
	for _, id := range []string{crUserID, crDefaultID} {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("error %q does not name the searched catalog %q", err.Error(), id)
		}
	}
}

func TestUnknownDeployedWorkflow_Update_NamesSearchedCatalogsAndRemovedLayerHint(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
	crWriteDeployed(t, ws, "orchestrator.md", "team-review")

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, AutoConfirmPlan: true})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), "team-review") {
		t.Errorf("error %q does not name the workflow", err.Error())
	}
	requireSearchedCatalogs(t, err)
	if strings.Count(err.Error(), removedLayerHintPhrase) != 1 {
		t.Errorf("error %q must carry the removed-layer hint exactly once", err.Error())
	}
}

func TestUnknownWorkflowRequestedByUpdate_NotDeployed_HasNoRemovedLayerHint(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, AddWorkflowIDs: []string{"team-review"}, AutoConfirmPlan: true})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	requireSearchedCatalogs(t, err)
	if strings.Contains(err.Error(), removedLayerHintPhrase) {
		t.Errorf("error %q must not carry the removed-layer hint for a workflow that is not deployed", err.Error())
	}
}

func TestUnknownDeployedWorkflow_DeployAndWorkflowUpdate_AddRemovedLayerHint(t *testing.T) {
	for _, op := range unknownWorkflowOps() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
			crWriteDeployed(t, ws, "orchestrator.md", "team-review")

			err := op.run(app.New(deps), ws, "team-review")

			if !errors.Is(err, plan.ErrUnknownWorkflow) {
				t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
			}
			requireSearchedCatalogs(t, err)
			if strings.Count(err.Error(), removedLayerHintPhrase) != 1 {
				t.Errorf("error %q must carry the removed-layer hint exactly once", err.Error())
			}
		})
	}
}

func TestUnknownWorkflow_NotDeployed_DeployAndWorkflowUpdate_HaveNoRemovedLayerHint(t *testing.T) {
	for _, op := range unknownWorkflowOps() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
			crWriteDeployed(t, ws, "orchestrator.md", "legacy-flow") // a different workflow is deployed

			err := op.run(app.New(deps), ws, "team-review")

			if !errors.Is(err, plan.ErrUnknownWorkflow) {
				t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
			}
			requireSearchedCatalogs(t, err)
			if strings.Contains(err.Error(), removedLayerHintPhrase) {
				t.Errorf("error %q must not carry the removed-layer hint for a workflow that is not deployed", err.Error())
			}
		})
	}
}

func TestUnknownWorkflow_DeployedIdInScriptOrchestrator_AddsRemovedLayerHint(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
	crWriteDeployed(t, ws, "orchestrator-script.md", "team-review")

	_, err := app.New(deps).DeployNew(context.Background(), app.DeployRequest{HarnessID: "stub-harness",
		WorkspacePath: ws, WorkflowIDs: []string{"team-review"}, AutoConfirmPlan: true})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), removedLayerHintPhrase) {
		t.Errorf("error %q must carry the removed-layer hint (workflow embedded in the script orchestrator)", err.Error())
	}
}
