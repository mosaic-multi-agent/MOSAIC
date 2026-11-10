package app_test

// update_kept_test.go specifies that Update workspace completes when a deployed
// orchestrator-role file holds a workflow the catalog stack does not supply, and that each
// such file keeps its own copy of that workflow, byte for byte, at its original position
// among the workflows of that file while catalog workflows are refreshed.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestUpdate_KeptWorkflows_EachFileKeepsItsOwnBlocksAtTheirPositions(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch, keptOtherDeployed),
		keptDeployedFile(keptCustomYScript, keptQuickFixDeployed, keptOtherDeployed))

	res := mustRunKeptUpdate(t, ws, keptCatalog(), nil)

	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireKeptIDs(t, "orchestrator", keptIDsOf(orch), []string{"quick-fix", "custom-x", "other-flow"})
	requireVerbatim(t, "orchestrator", orch, "custom-x", keptCustomXOrch)
	requireRefreshed(t, "orchestrator", orch, "quick-fix", "CATALOG-QUICKFIX-BODY", "OLD-QUICKFIX-BODY")
	requireRefreshed(t, "orchestrator", orch, "other-flow", "CATALOG-OTHER-BODY", "OLD-OTHER-BODY")

	script := keptBlocksIn(t, "orchestrator-script", res.outputs[keptScriptPath])
	requireKeptIDs(t, "orchestrator-script", keptIDsOf(script), []string{"custom-y", "quick-fix", "other-flow"})
	requireVerbatim(t, "orchestrator-script", script, "custom-y", keptCustomYScript)
	requireRefreshed(t, "orchestrator-script", script, "quick-fix", "CATALOG-QUICKFIX-BODY", "OLD-QUICKFIX-BODY")
	requireRefreshed(t, "orchestrator-script", script, "other-flow", "CATALOG-OTHER-BODY", "OLD-OTHER-BODY")

	requireKeptInput(t, res.input, "custom-x", "custom-y")
}

func TestUpdate_KeptWorkflows_SameIdInBothFilesKeepsEachFilesOwnBytes(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptQuickFixDeployed, keptCustomXScript))

	res := mustRunKeptUpdate(t, ws, keptCatalog(), nil)

	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireVerbatim(t, "orchestrator", orch, "custom-x", keptCustomXOrch)
	script := keptBlocksIn(t, "orchestrator-script", res.outputs[keptScriptPath])
	requireVerbatim(t, "orchestrator-script", script, "custom-x", keptCustomXScript)
}

func TestUpdate_KeptWorkflows_SecondRunLeavesKeptBytesUnchanged(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptRaggedX, keptOtherDeployed),
		keptDeployedFile(keptCustomYScript, keptQuickFixDeployed, keptOtherDeployed))
	first := mustRunKeptUpdate(t, ws, keptCatalog(), nil)
	for path, out := range first.outputs {
		if err := os.WriteFile(filepath.Join(ws, path), out, 0o644); err != nil {
			t.Fatalf("write the first run's output to %s: %v", path, err)
		}
	}

	second := mustRunKeptUpdate(t, ws, keptCatalog(), nil)

	orch := keptBlocksIn(t, "orchestrator, second run", second.outputs[keptOrchPath])
	requireKeptIDs(t, "orchestrator, second run", keptIDsOf(orch), []string{"quick-fix", "custom-x", "other-flow"})
	requireVerbatim(t, "orchestrator, second run", orch, "custom-x", keptRaggedX)
	script := keptBlocksIn(t, "orchestrator-script, second run", second.outputs[keptScriptPath])
	requireKeptIDs(t, "orchestrator-script, second run", keptIDsOf(script), []string{"custom-y", "quick-fix", "other-flow"})
	requireVerbatim(t, "orchestrator-script, second run", script, "custom-y", keptCustomYScript)
}

func TestUpdate_KeptWorkflows_IdTheStackNowSuppliesGetsTheCatalogBlockOnly(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch, keptCustomZOrch),
		keptDeployedFile(keptQuickFixDeployed, keptCustomXScript))
	cat := keptCatalog()
	cat.workflows = append(cat.workflows, domain.Workflow{ID: "custom-x", Name: "Custom X", Version: "1.0"})

	res := mustRunKeptUpdate(t, ws, cat, nil)

	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireKeptIDs(t, "orchestrator", keptIDsOf(orch), []string{"quick-fix", "custom-x", "custom-z"})
	requireRefreshed(t, "orchestrator", orch, "custom-x", "CATALOG-X-BODY", "CUSTOM-X-ORCH-BODY")
	requireVerbatim(t, "orchestrator", orch, "custom-z", keptCustomZOrch)
	script := keptBlocksIn(t, "orchestrator-script", res.outputs[keptScriptPath])
	requireKeptIDs(t, "orchestrator-script", keptIDsOf(script), []string{"quick-fix", "custom-x"})
	requireRefreshed(t, "orchestrator-script", script, "custom-x", "CATALOG-X-BODY", "CUSTOM-X-SCRIPT-BODY")
	requireKeptInput(t, res.input, "custom-z")
}

func TestUpdate_KeptWorkflows_DeployedCustomWithAddWorkflowsIdAcceptedInOneRun(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptQuickFixDeployed))

	res := mustRunKeptUpdate(t, ws, keptCatalog(), []string{"other-flow"})

	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireKeptIDs(t, "orchestrator", keptIDsOf(orch), []string{"quick-fix", "custom-x", "other-flow"})
	requireVerbatim(t, "orchestrator", orch, "custom-x", keptCustomXOrch)
	requireRefreshed(t, "orchestrator", orch, "other-flow", "CATALOG-OTHER-BODY", "OLD-OTHER-BODY")
	requireKeptInput(t, res.input, "custom-x")
	if !containsID(res.input.WorkflowIDs, "other-flow") {
		t.Errorf("plan.Input.WorkflowIDs = %v, want it to hold the catalog workflow %q", res.input.WorkflowIDs, "other-flow")
	}
	script := keptBlocksIn(t, "orchestrator-script", res.outputs[keptScriptPath])
	if got := blockBytes(script, "custom-x"); got != nil {
		t.Errorf("custom-x appears in the orchestrator-script, which did not deploy it: %q", got)
	}
}
