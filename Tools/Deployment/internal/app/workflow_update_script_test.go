package app_test

// workflow_update_script_test.go verifies the workflow-only update over a workspace with a
// deployed orchestrator-script: both orchestrator-role files list the same workflows after the
// run, whether workflows were added or removed, and the script keeps its own model, project,
// infrastructure and custom content. Every test runs a real flow and asserts file content.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// TestWorkflowUpdate_DeployedScript_ListsTheSameWorkflowsAsTheOrchestrator verifies adding
// workflows reaches the script, not only the orchestrator.
func TestWorkflowUpdate_DeployedScript_ListsTheSameWorkflowsAsTheOrchestrator(t *testing.T) {
	for _, h := range parityHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, []string{wpsFlowParity})
			c := gamNewReviewCapture(ws)

			// Act
			sum, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			wpsAssertWorkflows(t, ws, h, wpsAllFlows)
			if taken, ok := wpsTaken(sum, wpsScript); !ok || taken != domain.TakenUpdated {
				t.Errorf("script action = %q (recorded %v), want %q", taken, ok, domain.TakenUpdated)
			}
		})
	}
}

// TestWorkflowUpdate_DeployedScript_DropsRemovedWorkflowsLikeTheOrchestrator verifies
// reducing the set removes the dropped workflows from the script as well.
func TestWorkflowUpdate_DeployedScript_DropsRemovedWorkflowsLikeTheOrchestrator(t *testing.T) {
	for _, h := range parityHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, wpsAllFlows)
			c := gamNewReviewCapture(ws)

			// Act
			_, err := wpsRun(t, ws, h, []string{wpsFlowBeta}, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			wpsAssertWorkflows(t, ws, h, []string{wpsFlowBeta})
		})
	}
}

// TestWorkflowUpdate_DeployedScript_ReducedToNoWorkflows_ListsNone verifies an empty selection
// empties the script's workflows too.
func TestWorkflowUpdate_DeployedScript_ReducedToNoWorkflows_ListsNone(t *testing.T) {
	for _, h := range parityHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, wpsAllFlows)
			c := gamNewReviewCapture(ws)

			// Act
			_, err := wpsRun(t, ws, h, []string{}, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			wpsAssertWorkflows(t, ws, h, nil)
		})
	}
}

// TestWorkflowUpdate_DeployedScript_KeepsItsModelAndRegions verifies the script's own model,
// project-region content, infrastructure declarations and custom region survive a run that
// changes its workflows, that the orchestrator keeps its own, and that no model is asked or
// reported missing for either.
func TestWorkflowUpdate_DeployedScript_KeepsItsModelAndRegions(t *testing.T) {
	for _, h := range parityHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, []string{wpsFlowParity})
			c := gamNewReviewCapture(ws)

			// Act
			sum, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			wpsAssertWorkflows(t, ws, h, wpsAllFlows)
			for _, key := range wpsPairKeys {
				wpsAssertKeepsState(t, key, string(gamRead(t, ws, wpsRel(t, h, key))))
			}
			wpsAssertNoModelQuestion(t, c)
			gsAssertNoModelTodo(t, sum, wpsPairKeys)
		})
	}
}

// TestWorkflowUpdate_DeployedScript_RecordsOneManifestEntryAtItsPath verifies the rewritten
// script stays recorded once, at its own path, after its content was rewritten.
func TestWorkflowUpdate_DeployedScript_RecordsOneManifestEntryAtItsPath(t *testing.T) {
	for _, h := range parityHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, []string{wpsFlowParity})
			c := gamNewReviewCapture(ws)
			rel := wpsRel(t, h, wpsScript)
			before := string(gamRead(t, ws, rel))

			// Act
			_, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			if after := string(gamRead(t, ws, rel)); after == before {
				t.Error("script file is byte-identical to its pre-run state; its workflows were not rewritten")
			}
			if paths := gamEntryPaths(gamManifest(t, ws), wpsScript); len(paths) != 1 || paths[0] != rel {
				t.Errorf("script manifest entries = %v, want exactly [%s]", paths, rel)
			}
		})
	}
}
