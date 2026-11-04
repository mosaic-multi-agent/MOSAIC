package app_test

// workflow_update_script_legacy_test.go verifies the workflow-only update on a GHCP workspace
// whose orchestrator and orchestrator-script are recorded under the legacy name X.md: after the
// run both are X.agent.md with identical workflows, the script keeps its own model, project,
// infrastructure and custom content, and the reviewed plan shows the script as a rename of its
// legacy file, never as a new artifact. Both GHCP targets run over real files.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// wpsLegacyWorkspace deploys the workflowpair catalog to a GHCP target, plants the user state
// and renames every agent back to X.md. It returns the workspace and the legacy bytes by key.
func wpsLegacyWorkspace(t *testing.T, harness string) (string, map[string][]byte) {
	t.Helper()
	ws := wpsDeploy(t, harness, []string{wpsFlowParity})
	return ws, wpsToLegacy(t, ws)
}

// TestWorkflowUpdate_LegacyScript_MigratesWithTheSameWorkflowsAndKeepsItsState verifies the
// recorded legacy script ends as a recorded X.agent.md that lists the orchestrator's workflows
// and holds its own model and region content, with the legacy bytes recoverable from backup.
func TestWorkflowUpdate_LegacyScript_MigratesWithTheSameWorkflowsAndKeepsItsState(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, legacy := wpsLegacyWorkspace(t, h)
			c := gamNewReviewCapture(ws)

			// Act
			sum, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			for _, key := range wpsPairKeys {
				gamAssertMigrated(t, ws, key, legacy[key])
				wpsAssertKeepsState(t, key, string(gamRead(t, ws, gamRel(key, ".agent.md"))))
			}
			wpsAssertWorkflows(t, ws, h, wpsAllFlows)
			wpsAssertNoModelQuestion(t, c)
			gsAssertNoModelTodo(t, sum, wpsPairKeys)
			gamAssertKept(t, ws, "parity-runner", legacy["parity-runner"])
			gamAssertKept(t, ws, "parity-helper", legacy["parity-helper"])
		})
	}
}

// TestWorkflowUpdate_LegacyScript_ReducedWorkflows_DropsThemFromBothMigratedFiles verifies the
// reduced set also reaches a script that is migrated in the same run.
func TestWorkflowUpdate_LegacyScript_ReducedWorkflows_DropsThemFromBothMigratedFiles(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, wpsAllFlows)
			wpsToLegacy(t, ws)
			c := gamNewReviewCapture(ws)

			// Act
			_, err := wpsRun(t, ws, h, []string{wpsFlowBeta}, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			for _, key := range wpsPairKeys {
				if !gamExists(ws, gamRel(key, ".agent.md")) || gamExists(ws, gamRel(key, ".md")) {
					t.Errorf("%s was not migrated to X.agent.md", key)
				}
			}
			wpsAssertWorkflows(t, ws, h, []string{wpsFlowBeta})
		})
	}
}

// TestWorkflowUpdate_LegacyScript_IsShownAsARenameNotANewArtifact verifies the reviewed plan
// holds the script as an update that names the legacy file it replaces, like the orchestrator,
// and that the review shows the legacy file's removal.
func TestWorkflowUpdate_LegacyScript_IsShownAsARenameNotANewArtifact(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, _ := wpsLegacyWorkspace(t, h)
			c := gamNewReviewCapture(ws)

			// Act
			_, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			for _, key := range wpsPairKeys {
				item, planned := wpsPlanItem(t, c, key)
				if !planned {
					t.Errorf("%s is missing from the reviewed plan", key)
					continue
				}
				if item.Action != domain.ActionUpdate {
					t.Errorf("%s planned as %q, want %q", key, item.Action, domain.ActionUpdate)
				}
				if item.RenamedFrom != gamRel(key, ".md") {
					t.Errorf("%s RenamedFrom = %q, want %q", key, item.RenamedFrom, gamRel(key, ".md"))
				}
				if !strings.HasPrefix(item.Reason, "renamed from ") {
					t.Errorf("%s reason = %q, want it to start with \"renamed from \"", key, item.Reason)
				}
			}
			removed := map[string]bool{}
			for _, r := range c.plans[0].Removals {
				removed[r.Path] = true
			}
			for _, key := range wpsPairKeys {
				if !removed[gamRel(key, ".md")] {
					t.Errorf("review does not show the removal of %s", gamRel(key, ".md"))
				}
			}
		})
	}
}

// TestWorkflowUpdate_LegacyScript_NoModelQuestionAndNoCreateAction verifies a recorded legacy
// script counts as present: it is never asked for a model and never recorded as created.
func TestWorkflowUpdate_LegacyScript_NoModelQuestionAndNoCreateAction(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, _ := wpsLegacyWorkspace(t, h)
			c := gamNewReviewCapture(ws)

			// Act
			sum, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			wpsAssertNoModelQuestion(t, c)
			taken, acted := wpsTaken(sum, wpsScript)
			if !acted {
				t.Fatal("the run recorded no action for the legacy script")
			}
			if taken == domain.TakenCreated {
				t.Errorf("legacy script recorded as %q, want it shown as a rename of an existing agent", taken)
			}
		})
	}
}
