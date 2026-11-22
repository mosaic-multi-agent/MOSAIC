package app_test

// workflow_update_kept_legacy_ghcp_test.go specifies Update workflows on a GHCP workspace still
// in the pre-migration form (agents recorded as X.md) that holds a workflow the catalog stack
// does not supply: the run completes and migrates the orchestrator-role files, the workflow is
// offered in the checklist as on other harnesses, a selected one is kept byte for byte and an
// unselected one is removed and named once, and an orchestrator that cannot be read as an agent
// document is still reported as a conflict. Both GHCP targets run.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestWorkflowUpdate_LegacyGhcpWorkspace_OffersRemovedWorkflowUncheckedWithHint(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, _ := klgWorkspace(t, h, true)
			c := newWukChecklist(append(append([]string{}, wpsAllFlows...), klgKeptID)...)

			// Act
			klgRunWorkflows(t, ws, h, c)

			// Assert
			if len(c.asked) != 1 {
				t.Fatalf("checklist asked %d times, want once", len(c.asked))
			}
			q := c.asked[0]
			opt, ok := wukOption(q, klgKeptID)
			if !ok {
				t.Fatalf("checklist has no option for the deployed workflow %s", klgKeptID)
			}
			if opt.Hint != wukCustomHint {
				t.Errorf("option %s hint = %q, want %q", klgKeptID, opt.Hint, wukCustomHint)
			}
			if containsID(q.DefaultOptionIDs, klgKeptID) {
				t.Errorf("option %s is checked by default; want it unchecked", klgKeptID)
			}
		})
	}
}

func TestWorkflowUpdate_LegacyGhcpWorkspace_SelectedRemovedWorkflowIsKeptInMigratedFiles(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, legacy := klgWorkspace(t, h, true)
			wantIDs := map[string][]string{}
			for _, key := range wpsPairKeys {
				wantIDs[key] = klgIDsOf(t, key, legacy[key])
			}
			c := newWukChecklist(append(append([]string{}, wpsAllFlows...), klgKeptID)...)

			// Act
			sum := klgRunWorkflows(t, ws, h, c)

			// Assert
			for _, key := range wpsPairKeys {
				gamAssertMigrated(t, ws, key, legacy[key])
				klgAssertKeptInFile(t, ws, h, key, wantIDs[key])
			}
			if got := wukRemovalNotices(sum, klgKeptID); len(got) != 0 {
				t.Errorf("kept workflow %s is reported as removed: %+v", klgKeptID, got)
			}
		})
	}
}

func TestWorkflowUpdate_LegacyGhcpWorkspace_UnselectedRemovedWorkflowIsDroppedAndNamedOnce(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, legacy := klgWorkspace(t, h, true)
			c := newWukChecklist(wpsAllFlows...)

			// Act
			sum := klgRunWorkflows(t, ws, h, c)

			// Assert
			for _, key := range wpsPairKeys {
				gamAssertMigrated(t, ws, key, legacy[key])
				for _, id := range klgIDsOf(t, key, gamRead(t, ws, wpsRel(t, h, key))) {
					if id == klgKeptID {
						t.Errorf("%s still lists the deselected workflow %s", key, id)
					}
				}
			}
			if got := len(wukRemovalNotices(sum, klgKeptID)); got != 1 {
				t.Errorf("run summary names the removed workflow %s %d times, want once; items: %+v", klgKeptID, got, sum.Todos)
			}
		})
	}
}

func TestWorkflowUpdate_LegacyGhcpWorkspace_UnreadableLegacyOrchestratorIsAConflict(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, _ := klgWorkspace(t, h, true)
			const notAnAgent = "recorded copy, no frontmatter\n"
			gamWrite(t, ws, gamRel(wpsOrch, ".md"), notAnAgent)
			klgRecordHash(t, ws, wpsOrch, notAnAgent)
			c := gamNewReviewCapture(ws)

			// Act
			_, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			item, planned := wpsPlanItem(t, c, wpsOrch)
			if !planned {
				t.Fatal("the orchestrator is missing from the reviewed plan")
			}
			if item.Action != domain.ActionConflict {
				t.Errorf("orchestrator planned as %q, want %q", item.Action, domain.ActionConflict)
			}
			gamAssertRecoverable(t, ws, wpsOrch, []byte(notAnAgent))
		})
	}
}
