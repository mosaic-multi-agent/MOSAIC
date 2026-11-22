package app_test

// kept_legacy_ghcp_helpers_test.go holds the builders and assertions of the tests that run Update
// and Update workflows over a workspace whose orchestrator-role files hold a workflow the catalog
// stack does not supply, in particular on a GHCP workspace still in the pre-migration form (agent
// files X.md recorded in the manifest). The workspace comes from a real deploy of the workflowpair
// catalog: a custom block is planted at the start of the workflow region of both orchestrator-role
// files and the manifest hashes are re-recorded. Legacy workspaces are then renamed back to X.md.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	// klgKeptID is the id of the planted workflow that no catalog supplies.
	klgKeptID = wukIDW
)

// klgKeptBlock is the planted block of each orchestrator-role file. The two differ, so a block
// read from the wrong file cannot pass.
var klgKeptBlock = map[string]string{wpsOrch: wukBlockWOrch, wpsScript: wukBlockWScript}

// klgWorkspace deploys the workflowpair catalog with all workflows and plants the custom block and
// the kept block. With legacy true the GHCP agent files and manifest entries are then renamed
// to X.md. It returns the workspace and the legacy bytes by agent key (nil when not legacy).
func klgWorkspace(t *testing.T, harness string, legacy bool) (string, map[string][]byte) {
	t.Helper()
	ws := wpsDeploy(t, harness, wpsAllFlows)
	for _, key := range wpsPairKeys {
		klgPlant(t, ws, harness, key)
	}
	if !legacy {
		return ws, nil
	}
	return ws, wpsToLegacy(t, ws)
}

// klgPlant puts the custom block in front of the first workflow of one file, the stale marker
// and re-records the file's manifest hash.
func klgPlant(t *testing.T, ws, harness, key string) {
	t.Helper()
	rel := wpsRel(t, harness, key)
	doc := string(gamRead(t, ws, rel))
	first := strings.Index(doc, "\n<Workflow ")
	if first < 0 {
		t.Fatalf("%s: deployed file holds no workflow block at the start of a line", key)
	}
	doc = doc[:first+1] + klgKeptBlock[key] + doc[first+1:]
	gamWrite(t, ws, rel, doc)
	klgRecordHash(t, ws, key, doc)
}

// klgRecordHash records content as the manifest hash of every agent entry of key.
func klgRecordHash(t *testing.T, ws, key, content string) {
	t.Helper()
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key {
			m.Entries[i].ContentHash = manifest.Hash([]byte(content))
		}
	}
	gamSaveManifest(t, ws, m)
}

// klgIDsOf returns the workflow ids, in file order, of a document.
func klgIDsOf(t *testing.T, label string, doc []byte) []string {
	t.Helper()
	return keptIDsOf(keptBlocksIn(t, label, doc))
}

// klgUpdate runs Update over ws, deciding every conflict as skip.
func klgUpdate(t *testing.T, ws, harness string) domain.RunSummary {
	t.Helper()
	sum, err := wpsService(t).Update(context.Background(), app.UpdateRequest{
		HarnessID: harness, WorkspacePath: ws, AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
	if err != nil {
		t.Fatalf("Update for %s: %v", harness, err)
	}
	return sum
}

// klgRunWorkflows runs Update workflows with no preset selection, answering the checklist with
// the capture's answer.
func klgRunWorkflows(t *testing.T, ws, harness string, c *wukChecklist) domain.RunSummary {
	t.Helper()
	svc := wpsService(t, func(d *app.Deps) { d.Interaction = c })
	sum, err := svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: harness, WorkspacePath: ws, AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
	if err != nil {
		t.Fatalf("UpdateWorkflows for %s: %v", harness, err)
	}
	return sum
}

// klgAssertKeptInFile checks the file of one orchestrator-role key holds the planted block byte
// for byte and lists the workflows in wantIDs order.
func klgAssertKeptInFile(t *testing.T, ws, harness, key string, wantIDs []string) {
	t.Helper()
	doc := gamRead(t, ws, wpsRel(t, harness, key))
	blocks := keptBlocksIn(t, key, doc)
	requireVerbatim(t, key, blocks, klgKeptID, klgKeptBlock[key])
	requireKeptIDs(t, key, keptIDsOf(blocks), wantIDs)
}

// klgAssertKeptWarning checks the summary holds exactly one kept-workflow item for klgKeptID.
func klgAssertKeptWarning(t *testing.T, sum domain.RunSummary) {
	t.Helper()
	n := 0
	for _, item := range keptWarningsIn(sum) {
		if item.Detail == keptWarningText(klgKeptID) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("run summary holds %d kept-workflow warnings for %s, want 1; items: %+v", n, klgKeptID, sum.Todos)
	}
}

// klgAssertAllUnchanged checks every action of the run left its file unchanged.
func klgAssertAllUnchanged(t *testing.T, sum domain.RunSummary) {
	t.Helper()
	if len(sum.Actions) == 0 {
		t.Fatal("the run recorded no actions")
	}
	for _, a := range sum.Actions {
		if a.Taken != domain.TakenUnchanged {
			t.Errorf("%s taken %q, want unchanged", a.TargetPath, a.Taken)
		}
	}
}
