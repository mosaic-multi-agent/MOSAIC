package app_test

// update_add_workflows_test.go specifies how Update workspace treats the ids given to
// --add-workflows: a custom id deployed in either orchestrator-role file is accepted and its
// block stays in the file that holds it; any other id the stack does not supply fails the run
// before any write, and the error names the id without the removed-layer hint.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/plan"
)

func TestUpdate_AddWorkflows_CustomIdDeployedOnlyInScriptIsAcceptedAndKeptThere(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed),
		keptDeployedFile(keptCustomYScript, keptQuickFixDeployed))

	res := mustRunKeptUpdate(t, ws, keptCatalog(), []string{"custom-y"})

	script := keptBlocksIn(t, "orchestrator-script", res.outputs[keptScriptPath])
	requireKeptIDs(t, "orchestrator-script", keptIDsOf(script), []string{"custom-y", "quick-fix"})
	requireVerbatim(t, "orchestrator-script", script, "custom-y", keptCustomYScript)
	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireKeptIDs(t, "orchestrator", keptIDsOf(orch), []string{"quick-fix"})
	requireKeptInput(t, res.input, "custom-y")
}

func TestUpdate_AddWorkflows_CustomIdDeployedInBothFilesIsKeptInBoth(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptQuickFixDeployed, keptCustomXScript))

	res := mustRunKeptUpdate(t, ws, keptCatalog(), []string{"custom-x"})

	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireVerbatim(t, "orchestrator", orch, "custom-x", keptCustomXOrch)
	script := keptBlocksIn(t, "orchestrator-script", res.outputs[keptScriptPath])
	requireVerbatim(t, "orchestrator-script", script, "custom-x", keptCustomXScript)
	requireKeptInput(t, res.input, "custom-x")
}

func TestUpdate_AddWorkflows_UnknownIdFailsBeforeAnyWriteAndNamesTheId(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptCustomYScript, keptQuickFixDeployed))

	res, err := runKeptUpdate(t, ws, keptCatalog(), []string{"ghost-flow"})

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), "ghost-flow") {
		t.Errorf("error %q does not name the unknown id", err.Error())
	}
	if res.called {
		t.Error("the executor ran; an unknown id must fail the run before any write")
	}
}

func TestUpdate_AddWorkflows_UnknownIdErrorCarriesNoRemovedLayerHint(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptCustomYScript, keptQuickFixDeployed))

	_, err := runKeptUpdate(t, ws, keptCatalog(), []string{"ghost-flow", "custom-x"})

	if err == nil {
		t.Fatal("Update with an unknown id must fail")
	}
	if strings.Contains(err.Error(), removedLayerHintPhrase) {
		t.Errorf("error %q must not carry the removed-layer hint in the update flow", err.Error())
	}
}
