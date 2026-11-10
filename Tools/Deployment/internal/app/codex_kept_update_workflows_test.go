package app_test

// codex_kept_update_workflows_test.go specifies that Update workflows over a real Codex
// deployment keeps a custom workflow planted into the deployed orchestrator, as it does for a
// Markdown harness: the workflow is offered as deployed-only in the checklist and its block
// survives the rewrite byte for byte. The workspace comes from a real deploy of the tool-local
// workflowpair catalog to the Codex module; the custom block is then planted into the
// developer_instructions of the deployed TOML and the manifest hash is re-recorded.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	_ "mosaic-deploy/internal/agentformat/all"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const codexKeptUWModel = "gpt-5.6-sol"

// codexKeptUWService is the workflowpair service whose only harness is the real Codex module.
func codexKeptUWService(t *testing.T, tweak ...func(*app.Deps)) (app.Service, string) {
	t.Helper()
	mod := codexAppModule(t)
	useCodex := func(d *app.Deps) {
		d.Registry = &stubRegistry{
			list:    []domain.HarnessRef{mod.Ref()},
			modules: map[string]domain.HarnessModule{mod.Ref().ID: mod},
		}
	}
	return wpsService(t, append([]func(*app.Deps){useCodex}, tweak...)...), mod.Ref().ID
}

// codexKeptUWDeploy deploys the parity workflow to Codex in a fresh workspace.
func codexKeptUWDeploy(t *testing.T) (ws, harnessID string) {
	t.Helper()
	svc, id := codexKeptUWService(t)
	ws = t.TempDir()
	models := map[string]string{}
	for _, a := range parityAgents {
		models[a.key] = codexKeptUWModel
	}
	_, err := svc.DeployNew(context.Background(), app.DeployRequest{
		HarnessID: id, WorkspacePath: ws, Scope: domain.ScopeProject,
		WorkflowIDs: []string{wpsFlowParity}, UtilityAgentIDs: []string{"parity-helper"},
		AgentModels: models, AutoConfirmPlan: true})
	if err != nil {
		t.Fatalf("deploy to codex: %v", err)
	}
	return ws, id
}

// codexKeptUWPlant inserts the block at the end of the workflow region held in the
// developer_instructions of the deployed Codex file for key, keeping the leading stamp comments
// and every other key, and re-records the file's manifest hash.
func codexKeptUWPlant(t *testing.T, ws, key, block string) {
	t.Helper()
	rel := ".codex/agents/" + key + ".toml"
	raw := gamRead(t, ws, rel)
	var doc map[string]any
	if err := toml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s: deployed file is not valid TOML: %v", key, err)
	}
	instructions, _ := doc["developer_instructions"].(string)
	at := strings.Index(instructions, wpsRegionClose)
	if at < 1 {
		t.Fatalf("%s: developer_instructions has no workflow region close tag", key)
	}
	doc["developer_instructions"] = instructions[:at] + block + instructions[at:]
	body, err := toml.Marshal(doc)
	if err != nil {
		t.Fatalf("%s: marshal planted file: %v", key, err)
	}
	var stamps strings.Builder
	for _, line := range strings.SplitAfter(string(raw), "\n") {
		if !strings.HasPrefix(line, "#") {
			break
		}
		stamps.WriteString(line)
	}
	planted := stamps.String() + "\n" + string(body)
	gamWrite(t, ws, rel, planted)
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key {
			m.Entries[i].ContentHash = manifest.Hash([]byte(planted))
		}
	}
	gamSaveManifest(t, ws, m)
}

func TestUpdateWorkflows_Codex_CustomWorkflowInDeployedOrchestrator_OfferedAndKept(t *testing.T) {
	// Arrange
	ws, harnessID := codexKeptUWDeploy(t)
	codexKeptUWPlant(t, ws, wpsOrch, wukBlockX)
	checklist := newWukChecklist(wpsFlowParity)
	svc, _ := codexKeptUWService(t, func(d *app.Deps) { d.Interaction = checklist })

	// Act
	_, err := svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: harnessID, WorkspacePath: ws, AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})

	// Assert
	if err != nil {
		t.Fatalf("UpdateWorkflows over a Codex orchestrator holding a custom workflow: %v", err)
	}
	if len(checklist.asked) != 1 {
		t.Fatalf("checklist asked %d times, want once", len(checklist.asked))
	}
	opt, ok := wukOption(checklist.asked[0], wukIDX)
	if !ok || opt.Hint != wukCustomHint {
		t.Errorf("checklist option for %s = %+v (found %v), want a deployed-only option with hint %q",
			wukIDX, opt, ok, wukCustomHint)
	}
	written, readErr := os.ReadFile(filepath.Join(ws, ".codex", "agents", wpsOrch+".toml"))
	if readErr != nil {
		t.Fatalf("read rewritten orchestrator: %v", readErr)
	}
	blocks := codexInstructionBlocks(t, "orchestrator", written)
	requireVerbatim(t, "orchestrator", blocks, wukIDX, wukBlockX)
}
