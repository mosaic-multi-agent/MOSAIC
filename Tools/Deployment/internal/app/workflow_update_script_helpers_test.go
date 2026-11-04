package app_test

// workflow_update_script_helpers_test.go holds the builders and assertions of the tests that
// run the workflow-only update over a workspace with a deployed orchestrator-script. The
// workspace comes from a real deploy of the tool-local testdata/workflowpair catalog, then user
// state is planted into both orchestrator-role files: a different model in each, sentinels in
// project, infrastructure and custom regions. The manifest hashes are re-recorded, so each file
// is still the file the manifest describes.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	wpsOrch   = "orchestrator"
	wpsScript = "orchestrator-script"

	wpsOrchModel   = "orchestrator-planted-model"
	wpsScriptModel = "script-planted-model"

	wpsFlowParity = "parity-flow"
	wpsFlowAlpha  = "pair-flow-alpha"
	wpsFlowBeta   = "pair-flow-beta"

	wpsRegionOpen  = "<AvailableWorkflows"
	wpsRegionClose = "</AvailableWorkflows>"
)

var wpsAllFlows = []string{wpsFlowParity, wpsFlowAlpha, wpsFlowBeta}

// wpsPairKeys are the two orchestrator-role agents.
var wpsPairKeys = []string{wpsOrch, wpsScript}

// wpsService is the parity service over the workflowpair catalog.
func wpsService(t *testing.T, opts ...func(*app.Deps)) app.Service {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "workflowpair", "catalog"))
	if err != nil {
		t.Fatalf("resolve workflowpair catalog root: %v", err)
	}
	cat, err := catalog.Load(t.TempDir(), root)
	if err != nil {
		t.Fatalf("load workflowpair catalog: %v", err)
	}
	setCatalog := func(d *app.Deps) { d.Catalog = cat }
	return parityServiceWith(t, append([]func(*app.Deps){setCatalog}, opts...)...)
}

// wpsExt is the agent file extension a harness deploys.
func wpsExt(harness string) string {
	if harness == hCLI || harness == hVSCode {
		return ".agent.md"
	}
	return ".md"
}

// wpsRel is the workspace-relative path of a deployed agent file.
func wpsRel(t *testing.T, harness, key string) string {
	t.Helper()
	dir := rbModule(t, harness).Descriptor().Paths.Agents.Project
	return strings.TrimSuffix(filepath.ToSlash(dir), "/") + "/" + key + wpsExt(harness)
}

// wpsDeploy deploys the workflowpair catalog with the given workflows to a fresh workspace and
// plants the user state into both orchestrator-role files. It returns the workspace.
func wpsDeploy(t *testing.T, harness string, workflows []string) string {
	t.Helper()
	ws := t.TempDir()
	models := map[string]string{}
	for _, a := range parityAgents {
		models[a.key] = parityModel
	}
	_, err := wpsService(t).DeployNew(context.Background(), app.DeployRequest{
		HarnessID: harness, WorkspacePath: ws, Scope: domain.ScopeProject,
		WorkflowIDs: workflows, UtilityAgentIDs: []string{"parity-helper"},
		AgentModels: models, AutoConfirmPlan: true})
	if err != nil {
		t.Fatalf("deploy to %s: %v", harness, err)
	}
	m := gamManifest(t, ws)
	for _, key := range wpsPairKeys {
		rel := wpsRel(t, harness, key)
		content := wpsPlantState(t, key, string(gamRead(t, ws, rel)))
		gamWrite(t, ws, rel, content)
		for i, e := range m.Entries {
			if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key {
				m.Entries[i].ContentHash = manifest.Hash([]byte(content))
			}
		}
	}
	gamSaveManifest(t, ws, m)
	return ws
}

// wpsPlantState puts the user state into a deployed orchestrator-role document.
func wpsPlantState(t *testing.T, key, doc string) string {
	t.Helper()
	for _, tag := range []string{`<CodebaseContext type="project">`, `<ContextLimits type="project">`} {
		doc = strings.Replace(doc, tag+"\n", tag+"\n"+gamProjectSentinel+"\n", 1)
	}
	doc = strings.Replace(doc, `<InfrastructureAgents type="managed">`+"\n",
		`<InfrastructureAgents type="managed">`+"\n"+gamInfraSentinel+"\n", 1)
	model := wpsOrchModel
	if key == wpsScript {
		model = wpsScriptModel
	}
	lines := strings.Split(doc, "\n")
	planted := false
	for i, l := range lines {
		if strings.HasPrefix(l, "model:") {
			lines[i] = "model: " + model
			planted = true
			break
		}
	}
	if !planted {
		t.Fatalf("%s: deployed file has no model line to replace", key)
	}
	doc = strings.Join(lines, "\n")
	return strings.TrimRight(doc, "\n") + "\n\n" + `<UserNotes type="custom">` + "\n" +
		gamCustomSentinel + "\n" + `</UserNotes>` + "\n"
}

// wpsToLegacy renames every agent file and manifest entry of a GHCP workspace from X.agent.md
// back to X.md, the form earlier releases recorded. It returns the legacy bytes by agent key.
func wpsToLegacy(t *testing.T, ws string) map[string][]byte {
	t.Helper()
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.Ref.Kind != domain.ArtifactAgent || !strings.HasSuffix(e.TargetPath, ".agent.md") {
			continue
		}
		legacy := strings.TrimSuffix(e.TargetPath, ".agent.md") + ".md"
		if err := os.Rename(gamAbs(ws, e.TargetPath), gamAbs(ws, legacy)); err != nil {
			t.Fatalf("rename %s to legacy name: %v", e.TargetPath, err)
		}
		m.Entries[i].TargetPath = legacy
	}
	gamSaveManifest(t, ws, m)
	out := map[string][]byte{}
	for _, a := range parityAgents {
		out[a.key] = gamRead(t, ws, gamRel(a.key, ".md"))
	}
	return out
}

// wpsRun runs the workflow-only update with a reviewing capture as the interaction.
func wpsRun(t *testing.T, ws, harness string, ids []string, conflict domain.ConflictDecision,
	c *gamReviewCapture) (domain.RunSummary, error) {
	t.Helper()
	return wpsService(t, gamCaptureDeps(c)).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: harness, WorkspacePath: ws, WorkflowIDs: ids,
		AutoConfirmPlan: true, ConflictDefault: conflict})
}

// wpsRegion returns the available-workflows region of a deployed document.
func wpsRegion(t *testing.T, key, doc string) string {
	t.Helper()
	s, e := strings.Index(doc, wpsRegionOpen), strings.Index(doc, wpsRegionClose)
	if s < 0 || e < s {
		t.Fatalf("%s: document has no available-workflows region", key)
	}
	return doc[s:e]
}

// wpsAssertWorkflows checks both orchestrator-role files list exactly the same workflows:
// identical regions, holding every wanted workflow and none of the others.
func wpsAssertWorkflows(t *testing.T, ws, harness string, want []string) {
	t.Helper()
	regions := map[string]string{}
	for _, key := range wpsPairKeys {
		regions[key] = wpsRegion(t, key, string(gamRead(t, ws, wpsRel(t, harness, key))))
	}
	if regions[wpsOrch] != regions[wpsScript] {
		t.Errorf("orchestrator and orchestrator-script list different workflows:\norchestrator:\n%s\nscript:\n%s",
			regions[wpsOrch], regions[wpsScript])
	}
	for _, id := range wpsAllFlows {
		listed := wanted(want, id)
		for _, key := range wpsPairKeys {
			has := strings.Contains(regions[key], `name="`+id+`"`)
			if listed && !has {
				t.Errorf("%s does not list workflow %s", key, id)
			}
			if !listed && has {
				t.Errorf("%s still lists workflow %s", key, id)
			}
		}
	}
}

func wanted(list []string, id string) bool {
	for _, w := range list {
		if w == id {
			return true
		}
	}
	return false
}

// wpsAssertKeepsState checks the file of an orchestrator-role agent still holds its own planted
// model and its project, infrastructure and custom region content.
func wpsAssertKeepsState(t *testing.T, key, doc string) {
	t.Helper()
	model, other := wpsOrchModel, wpsScriptModel
	if key == wpsScript {
		model, other = wpsScriptModel, wpsOrchModel
	}
	for _, marker := range []string{"model: " + model, gamProjectSentinel, gamInfraSentinel, gamCustomSentinel} {
		if !strings.Contains(doc, marker) {
			t.Errorf("%s lost %q", key, marker)
		}
	}
	if strings.Contains(doc, other) {
		t.Errorf("%s holds the model planted for the other orchestrator-role agent", key)
	}
}

// wpsAssertNoModelQuestion checks the run asked neither a tier nor an agent model question.
func wpsAssertNoModelQuestion(t *testing.T, c *gamReviewCapture) {
	t.Helper()
	for _, id := range c.Stub.QuestionOrder() {
		if id == domain.QAgentModel || id == domain.QTierModel {
			t.Errorf("run asked a model question (%s); no agent needs a model here", id)
		}
	}
}

// wpsPlanItem returns the reviewed plan item of an agent key.
func wpsPlanItem(t *testing.T, c *gamReviewCapture, key string) (domain.PlanItem, bool) {
	t.Helper()
	if len(c.plans) == 0 {
		t.Fatal("the run never reached Review")
	}
	for _, item := range c.plans[0].Items {
		if item.Ref.Kind == domain.ArtifactAgent && item.Ref.Key == key {
			return item, true
		}
	}
	return domain.PlanItem{}, false
}

// wpsTaken returns what the run did with an agent key and whether it recorded an action.
func wpsTaken(sum domain.RunSummary, key string) (domain.ActionTaken, bool) {
	for _, ar := range sum.Actions {
		if ar.Ref.Kind == domain.ArtifactAgent && ar.Ref.Key == key {
			return ar.Taken, true
		}
	}
	return "", false
}
