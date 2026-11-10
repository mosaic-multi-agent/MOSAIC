package app_test

// workflow_update_script_edge_test.go verifies the workflow-only update at the edges of the
// orchestrator pair: a workspace without a deployed script never gets one and is never asked
// about its model, and a script the tool cannot confirm as its own is decided, recorded and
// reviewed exactly like an unconfirmed orchestrator.

import (
	"os"
	"sort"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

const wpsLocalEdit = "\nLOCAL-EDIT-OUTSIDE-ANY-REGION\n"

var wpsConflictHarnesses = []string{hClaude, hCLI}

// wpsMakeConflicts appends a local edit to both orchestrator-role files and drops their
// manifest records, so the tool can no longer confirm either as its own. It returns the edited bytes.
func wpsMakeConflicts(t *testing.T, ws, harness string) map[string][]byte {
	t.Helper()
	edited := map[string][]byte{}
	for _, key := range wpsPairKeys {
		rel := wpsRel(t, harness, key)
		gamWrite(t, ws, rel, string(gamRead(t, ws, rel))+wpsLocalEdit)
		edited[key] = gamRead(t, ws, rel)
		gamDropEntry(t, ws, key, rel)
	}
	return edited
}

// wpsTodoDetails returns the sorted details of the TODO items about an agent key.
func wpsTodoDetails(sum domain.RunSummary, key string) []string {
	var out []string
	for _, item := range sum.Todos {
		if item.Subject == key {
			out = append(out, item.Detail)
		}
	}
	sort.Strings(out)
	return out
}

// TestWorkflowUpdate_AbsentScript_IsNotCreatedAndNotAsked verifies a workspace deployed
// without a script gets none, no model question is asked, and no TODO entry mentions it.
func TestWorkflowUpdate_AbsentScript_IsNotCreatedAndNotAsked(t *testing.T) {
	for _, h := range parityHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, []string{wpsFlowParity})
			rel := wpsRel(t, h, wpsScript)
			if err := os.Remove(gamAbs(ws, rel)); err != nil {
				t.Fatalf("remove script: %v", err)
			}
			gamDropEntry(t, ws, wpsScript, rel)
			c := gamNewReviewCapture(ws)

			// Act
			sum, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			if gamExists(ws, rel) {
				t.Errorf("%s was created although the script was not deployed", rel)
			}
			if paths := gamEntryPaths(gamManifest(t, ws), wpsScript); len(paths) != 0 {
				t.Errorf("script is recorded at %v although it was not deployed", paths)
			}
			if _, planned := wpsPlanItem(t, c, wpsScript); planned {
				t.Error("the reviewed plan holds an item for the absent script")
			}
			if _, acted := wpsTaken(sum, wpsScript); acted {
				t.Error("the run recorded an action for the absent script")
			}
			wpsAssertNoModelQuestion(t, c)
			for _, item := range sum.Todos {
				if strings.Contains(item.Subject+" "+item.Owner+" "+item.Detail, wpsScript) {
					t.Errorf("TODO entry about the absent script: %+v", item)
				}
			}
			wpsAssertKeepsStateOfOrchestrator(t, ws, h)
		})
	}
}

// wpsAssertKeepsStateOfOrchestrator checks the orchestrator listed all workflows and kept its state.
func wpsAssertKeepsStateOfOrchestrator(t *testing.T, ws, h string) {
	t.Helper()
	doc := string(gamRead(t, ws, wpsRel(t, h, wpsOrch)))
	region := wpsRegion(t, wpsOrch, doc)
	for _, id := range wpsAllFlows {
		if !strings.Contains(region, `name="`+id+`"`) {
			t.Errorf("orchestrator does not list workflow %s", id)
		}
	}
	wpsAssertKeepsState(t, wpsOrch, doc)
}

// TestWorkflowUpdate_ScriptConflict_Skip_LeavesBothFilesAndRecordsTheSameTodo verifies a skip
// decision leaves a modified script untouched and records the same skip entry as for the
// orchestrator.
func TestWorkflowUpdate_ScriptConflict_Skip_LeavesBothFilesAndRecordsTheSameTodo(t *testing.T) {
	for _, h := range wpsConflictHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, []string{wpsFlowParity})
			edited := wpsMakeConflicts(t, ws, h)
			c := gamNewReviewCapture(ws)

			// Act
			sum, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionSkip, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			for _, key := range wpsPairKeys {
				if got := gamRead(t, ws, wpsRel(t, h, key)); string(got) != string(edited[key]) {
					t.Errorf("%s was changed although the decision was skip", key)
				}
			}
			orch, script := wpsTodoDetails(sum, wpsOrch), wpsTodoDetails(sum, wpsScript)
			if len(script) == 0 {
				t.Fatal("no TODO entry for the skipped script")
			}
			if strings.Join(orch, "|") != strings.Join(script, "|") {
				t.Errorf("skip TODO of the script %q differs from the orchestrator's %q", script, orch)
			}
		})
	}
}

// TestWorkflowUpdate_ScriptConflict_Overwrite_RewritesBothAndKeepsState verifies an overwrite
// decision replaces a modified script's workflows like the orchestrator's and keeps its state.
func TestWorkflowUpdate_ScriptConflict_Overwrite_RewritesBothAndKeepsState(t *testing.T) {
	for _, h := range wpsConflictHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, []string{wpsFlowParity})
			wpsMakeConflicts(t, ws, h)
			c := gamNewReviewCapture(ws)

			// Act
			sum, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionOverwrite, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			wpsAssertWorkflows(t, ws, h, wpsAllFlows)
			for _, key := range wpsPairKeys {
				wpsAssertKeepsState(t, key, string(gamRead(t, ws, wpsRel(t, h, key))))
			}
			orchTaken, _ := wpsTaken(sum, wpsOrch)
			if scriptTaken, _ := wpsTaken(sum, wpsScript); scriptTaken != orchTaken {
				t.Errorf("script action %q differs from the orchestrator's %q", scriptTaken, orchTaken)
			}
		})
	}
}

// TestWorkflowUpdate_ScriptConflict_Backup_BacksUpBothBeforeRewriting verifies a
// backup-then-overwrite decision keeps the modified bytes of the script as a backup.
func TestWorkflowUpdate_ScriptConflict_Backup_BacksUpBothBeforeRewriting(t *testing.T) {
	for _, h := range wpsConflictHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, []string{wpsFlowParity})
			edited := wpsMakeConflicts(t, ws, h)
			c := gamNewReviewCapture(ws)

			// Act
			_, err := wpsRun(t, ws, h, wpsAllFlows, domain.DecisionBackupThenOverwrite, c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			wpsAssertWorkflows(t, ws, h, wpsAllFlows)
			for _, key := range wpsPairKeys {
				if !gamHasBackup(t, ws, wpsRel(t, h, key), edited[key]) {
					t.Errorf("no backup of %s holds the modified bytes", key)
				}
			}
		})
	}
}

// TestWorkflowUpdate_ScriptConflict_Interactive_AsksPerFileAndHonoursEachAnswer verifies a
// modified script is put to the user like a modified orchestrator, shown in the review, and its
// answer is applied to it alone.
func TestWorkflowUpdate_ScriptConflict_Interactive_AsksPerFileAndHonoursEachAnswer(t *testing.T) {
	for _, h := range wpsConflictHarnesses {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := wpsDeploy(t, h, []string{wpsFlowParity})
			edited := wpsMakeConflicts(t, ws, h)
			orchRel, scriptRel := wpsRel(t, h, wpsOrch), wpsRel(t, h, wpsScript)
			stub := interactiontest.NewBuilder().
				AnswerSelectOne(domain.QLocalModification, orchRel, string(domain.DecisionSkip)).
				AnswerSelectOne(domain.QLocalModification, scriptRel, string(domain.DecisionOverwrite)).
				AnswerReview(true).
				Build()
			c := &gamReviewCapture{Stub: stub, ws: ws}

			// Act
			_, err := wpsRun(t, ws, h, wpsAllFlows, "", c)

			// Assert
			if err != nil {
				t.Fatalf("UpdateWorkflows: %v", err)
			}
			for _, rel := range []string{orchRel, scriptRel} {
				if !stub.WasAsked(domain.QLocalModification, rel) {
					t.Errorf("no local-modification question for %s", rel)
				}
			}
			item, planned := wpsPlanItem(t, c, wpsScript)
			if !planned || item.Action != domain.ActionConflict {
				t.Errorf("reviewed script item = %+v (planned %v), want a %q item", item, planned, domain.ActionConflict)
			}
			if got := gamRead(t, ws, orchRel); string(got) != string(edited[wpsOrch]) {
				t.Error("orchestrator was changed although its answer was skip")
			}
			region := wpsRegion(t, wpsScript, string(gamRead(t, ws, scriptRel)))
			if !strings.Contains(region, `name="`+wpsFlowAlpha+`"`) {
				t.Error("script was not rewritten although its answer was overwrite")
			}
		})
	}
}
