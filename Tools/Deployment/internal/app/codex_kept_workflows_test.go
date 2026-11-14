package app_test

// codex_kept_workflows_test.go specifies that Update workspace keeps the workflows of a
// deployed Codex orchestrator that the catalog stack does not supply, exactly as it does for a
// Markdown harness. The deployed orchestrator-role files are Codex TOML: the workflow blocks
// live inside the developer_instructions string, so reading them needs the decoded canonical
// form, not the raw file bytes.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	_ "mosaic-deploy/internal/agentformat/all"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

const (
	codexKeptOrchPath   = ".codex/agents/orchestrator.toml"
	codexKeptScriptPath = ".codex/agents/orchestrator-script.toml"
)

// codexKeptDeployedTOML returns a deployed Codex orchestrator-role file whose
// developer_instructions hold an AvailableWorkflows region with the given blocks in order.
func codexKeptDeployedTOML(t *testing.T, blocks ...string) []byte {
	t.Helper()
	body := "<AvailableWorkflows type=\"managed\">\n" + strings.Join(blocks, "") + "</AvailableWorkflows>\n"
	doc, err := toml.Marshal(map[string]any{
		"sandbox_mode":           "workspace-write",
		"developer_instructions": body,
	})
	if err != nil {
		t.Fatalf("marshal deployed codex fixture: %v", err)
	}
	return append([]byte("# mosaic_version: 0.9\n\n"), doc...)
}

// writeCodexKeptWorkspace writes the deployed Codex orchestrator and orchestrator-script.
func writeCodexKeptWorkspace(t *testing.T, ws string, orch, script []byte) {
	t.Helper()
	for path, content := range map[string][]byte{codexKeptOrchPath: orch, codexKeptScriptPath: script} {
		full := filepath.Join(ws, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// codexKeptPlan returns an update plan with one item per deployed Codex orchestrator-role file.
func codexKeptPlan(ws string) domain.Plan {
	item := func(key string) domain.PlanItem {
		return domain.PlanItem{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
			TargetPath: ".codex/agents/" + key + ".toml",
			SourcePath: "/fake/" + key + ".md",
			Action:     domain.ActionUpdate,
			Model:      domain.ModelSelection{ModelID: "gpt-5.6-sol", Origin: domain.OriginTierMapping},
		}
	}
	return domain.Plan{
		Mode: domain.ModeUpdateWorkspace, Harness: codexOpHarnessRef, WorkspacePath: ws, Scope: domain.ScopeProject,
		Items: []domain.PlanItem{item("orchestrator"), item("orchestrator-script")},
	}
}

// runCodexKeptUpdate runs Update workspace over a Codex workspace and returns the content the
// executor would write for each agent item, keyed by target path.
func runCodexKeptUpdate(t *testing.T, ws string) (*keptRunResult, error) {
	t.Helper()
	res := &keptRunResult{outputs: map[string][]byte{}, errs: map[string]error{}}
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	deps.Registry = newCodexOpRegistry()
	deps.Catalog = keptCatalog()
	deps.Executor = &keptExecutor{res: res}
	planner := &capturingPlanner{result: codexKeptPlan(ws)}
	deps.Planner = planner

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: codexOpHarnessRef.ID, WorkspacePath: ws, AutoConfirmPlan: true,
	})
	res.input = planner.capturedInput
	return res, err
}

// codexInstructionBlocks returns the direct workflow blocks held in the developer_instructions
// of an encoded Codex file.
func codexInstructionBlocks(t *testing.T, label string, out []byte) []transform.WorkflowBlock {
	t.Helper()
	if len(out) == 0 {
		t.Fatalf("%s: the run produced no content", label)
	}
	var doc map[string]any
	if err := toml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("%s: output is not valid TOML: %v\n%s", label, err, out)
	}
	instructions, _ := doc["developer_instructions"].(string)
	return keptBlocksIn(t, label, []byte(instructions))
}

func TestUpdate_Codex_KeptWorkflows_EachFileKeepsItsOwnBlocksAtTheirPositions(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	writeCodexKeptWorkspace(t, ws,
		codexKeptDeployedTOML(t, keptQuickFixDeployed, keptCustomXOrch, keptOtherDeployed),
		codexKeptDeployedTOML(t, keptCustomYScript, keptQuickFixDeployed, keptOtherDeployed))

	// Act
	res, err := runCodexKeptUpdate(t, ws)

	// Assert
	if err != nil {
		t.Fatalf("Update over a deployed Codex orchestrator with a kept workflow must succeed: %v", err)
	}
	for path, cerr := range res.errs {
		t.Fatalf("content for %s failed: %v", path, cerr)
	}
	orch := codexInstructionBlocks(t, "orchestrator", res.outputs[codexKeptOrchPath])
	requireKeptIDs(t, "orchestrator", keptIDsOf(orch), []string{"quick-fix", "custom-x", "other-flow"})
	requireVerbatim(t, "orchestrator", orch, "custom-x", keptCustomXOrch)
	requireRefreshed(t, "orchestrator", orch, "quick-fix", "CATALOG-QUICKFIX-BODY", "OLD-QUICKFIX-BODY")
	script := codexInstructionBlocks(t, "orchestrator-script", res.outputs[codexKeptScriptPath])
	requireKeptIDs(t, "orchestrator-script", keptIDsOf(script), []string{"custom-y", "quick-fix", "other-flow"})
	requireVerbatim(t, "orchestrator-script", script, "custom-y", keptCustomYScript)
	requireKeptInput(t, res.input, "custom-x", "custom-y")
}
