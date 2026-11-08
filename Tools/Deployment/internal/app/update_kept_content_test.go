package app_test

// update_kept_content_test.go specifies the byte-level and failure behaviour of kept workflows
// in Update workspace: awkward block shapes and CRLF files stay byte-identical, kept workflows
// are not validated while deployed agents still refresh, a catalog workflow with a missing
// agent still fails the run, and an unreadable orchestrator-role file fails it with its path.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdate_KeptWorkflows_RaggedTableAndNestedRegionWrittenUnchanged(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptRaggedX, keptOtherDeployed),
		keptDeployedFile(keptQuickFixDeployed))

	res := mustRunKeptUpdate(t, ws, keptCatalog(), nil)

	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireKeptIDs(t, "orchestrator", keptIDsOf(orch), []string{"quick-fix", "custom-x", "other-flow"})
	requireVerbatim(t, "orchestrator", orch, "custom-x", keptRaggedX)
}

func TestUpdate_KeptWorkflows_UnvalidatedKeptWorkflowDoesNotStopDeployedAgentRefresh(t *testing.T) {
	ws := t.TempDir()
	// custom-x routes to ghost-agent, which no catalog supplies.
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptQuickFixDeployed))

	res := mustRunKeptUpdate(t, ws, keptCatalog(), nil)

	worker := string(res.outputs[keptWorkerPath])
	if !strings.Contains(worker, "NEW-WORKER-MARKER") || strings.Contains(worker, "OLD-WORKER-MARKER") {
		t.Errorf("the deployed worker was not refreshed from the catalog in the same run:\n%s", worker)
	}
	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireVerbatim(t, "orchestrator", orch, "custom-x", keptCustomXOrch)
}

func TestUpdate_KeptWorkflows_CatalogWorkflowWithMissingAgentStillFailsNamingTheAgent(t *testing.T) {
	ws := t.TempDir()
	broken := "<Workflow type=\"managed\" name=\"broken\" version=\"1.0\">\n## Broken\nOLD-BROKEN-BODY\n</Workflow>\n"
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch, broken),
		keptDeployedFile(keptQuickFixDeployed))
	cat := keptCatalog()
	cat.workflows = append(cat.workflows, domainWorkflowRoutingTo("broken", "ghost-agent"))

	res, err := runKeptUpdate(t, ws, cat, nil)

	if err == nil {
		t.Fatal("Update must fail: a catalog workflow names an agent the catalog does not supply")
	}
	if !strings.Contains(err.Error(), "ghost-agent") {
		t.Errorf("error %q does not name the missing agent", err.Error())
	}
	if res.called {
		t.Error("the executor ran although the run failed")
	}
}

func TestUpdate_KeptWorkflows_CrlfFileKeepsKeptBytesUnchanged(t *testing.T) {
	ws := t.TempDir()
	toCRLF := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	xCRLF := toCRLF(keptCustomXOrch)
	writeKeptWorkspace(t, ws,
		toCRLF(keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch, keptOtherDeployed)),
		toCRLF(keptDeployedFile(keptQuickFixDeployed)))

	res := mustRunKeptUpdate(t, ws, keptCatalog(), nil)

	orch := keptBlocksIn(t, "orchestrator", res.outputs[keptOrchPath])
	requireKeptIDs(t, "orchestrator", keptIDsOf(orch), []string{"quick-fix", "custom-x", "other-flow"})
	if got := blockBytes(orch, "custom-x"); !bytes.Equal(got, []byte(xCRLF)) {
		t.Errorf("kept block of a CRLF file was altered\ngot:  %q\nwant: %q", got, xCRLF)
	}
}

func TestUpdate_KeptWorkflows_UnreadableFileFailsRunNamingItsPathAndWritesNothing(t *testing.T) {
	ws := t.TempDir()
	// A line that begins with a workflow tag and carries more content cannot be read as blocks.
	unreadable := keptDeployedHead + keptQuickFixDeployed +
		"<Workflow type=\"managed\" name=\"custom-y\" version=\"1.0\">\nbody\n</Workflow><Workflow type=\"managed\" name=\"b\" version=\"1.0\">\nbody\n</Workflow>\n" +
		"</AvailableWorkflows>\n"
	mainFile := keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch)
	writeKeptWorkspace(t, ws, mainFile, unreadable)

	res, err := runKeptUpdate(t, ws, keptCatalog(), nil)

	if err == nil {
		t.Fatal("Update must fail for a deployed orchestrator-role file whose workflows cannot be read")
	}
	if !strings.Contains(err.Error(), keptScriptPath) {
		t.Errorf("error %q does not name the target path %q", err.Error(), keptScriptPath)
	}
	if res.called {
		t.Error("the executor ran although the run failed; nothing may be written")
	}
	for path, want := range map[string]string{keptOrchPath: mainFile, keptScriptPath: unreadable} {
		got, rerr := os.ReadFile(filepath.Join(ws, path))
		if rerr != nil || string(got) != want {
			t.Errorf("%s changed on disk (read error %v)", path, rerr)
		}
	}
}
