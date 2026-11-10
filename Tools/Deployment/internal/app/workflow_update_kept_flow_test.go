package app_test

// workflow_update_kept_flow_test.go specifies what Update workflows writes for custom
// workflows, the ones the catalog stack does not supply: a selected one is kept byte for byte
// in the file or files that hold it, together with newly added catalog workflows; an unselected
// one is removed from every file that holds it and named once in the run summary.

import (
	"testing"
)

func TestWorkflowUpdate_KeepsCustomWorkflowAndAddsCatalogWorkflowInOneRun(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, nil)
	c := newWukChecklist(wpsFlowBeta, wukIDX)

	wukRun(t, ws, c)

	orch := keptBlocksIn(t, wpsOrch, gamRead(t, ws, wpsRel(t, hClaude, wpsOrch)))
	requireKeptIDs(t, wpsOrch, keptIDsOf(orch), []string{wpsFlowBeta, wukIDX})
	requireVerbatim(t, wpsOrch, orch, wukIDX, wukBlockX)
	script := keptBlocksIn(t, wpsScript, gamRead(t, ws, wpsRel(t, hClaude, wpsScript)))
	requireKeptIDs(t, wpsScript, keptIDsOf(script), []string{wpsFlowBeta})
}

func TestWorkflowUpdate_UnselectedCustomWorkflowIsRemovedAndNamedInTheSummary(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, nil)
	c := newWukChecklist(wpsFlowParity)

	sum := wukRun(t, ws, c)

	requireKeptIDs(t, wpsOrch, wukIDsIn(t, ws, wpsOrch), []string{wpsFlowParity})
	if got := len(wukRemovalNotices(sum, wukIDX)); got != 1 {
		t.Errorf("run summary names the removed workflow %s %d times, want once; items: %+v", wukIDX, got, sum.Todos)
	}
}

func TestWorkflowUpdate_CustomWorkflowDeployedOnlyInScriptIsKeptOnlyInScript(t *testing.T) {
	ws := wukWorkspace(t, nil, []string{wukBlockZ})
	c := newWukChecklist(wpsFlowParity, wukIDZ)

	wukRun(t, ws, c)

	script := keptBlocksIn(t, wpsScript, gamRead(t, ws, wpsRel(t, hClaude, wpsScript)))
	requireKeptIDs(t, wpsScript, keptIDsOf(script), []string{wpsFlowParity, wukIDZ})
	requireVerbatim(t, wpsScript, script, wukIDZ, wukBlockZ)
	requireKeptIDs(t, wpsOrch, wukIDsIn(t, ws, wpsOrch), []string{wpsFlowParity})
}

func TestWorkflowUpdate_UnselectedCustomWorkflowInBothFilesIsRemovedFromBothAndNamedOnce(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockWOrch}, []string{wukBlockWScript})
	c := newWukChecklist(wpsFlowParity)

	sum := wukRun(t, ws, c)

	for _, key := range wpsPairKeys {
		requireKeptIDs(t, key, wukIDsIn(t, ws, key), []string{wpsFlowParity})
	}
	if got := len(wukRemovalNotices(sum, wukIDW)); got != 1 {
		t.Errorf("run summary names the removed workflow %s %d times, want once; items: %+v", wukIDW, got, sum.Todos)
	}
}

func TestWorkflowUpdate_SelectedCustomWorkflowInBothFilesKeepsEachFilesOwnBytes(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockWOrch}, []string{wukBlockWScript})
	c := newWukChecklist(wpsFlowParity, wukIDW)

	sum := wukRun(t, ws, c)

	orch := keptBlocksIn(t, wpsOrch, gamRead(t, ws, wpsRel(t, hClaude, wpsOrch)))
	requireKeptIDs(t, wpsOrch, keptIDsOf(orch), []string{wpsFlowParity, wukIDW})
	requireVerbatim(t, wpsOrch, orch, wukIDW, wukBlockWOrch)
	script := keptBlocksIn(t, wpsScript, gamRead(t, ws, wpsRel(t, hClaude, wpsScript)))
	requireKeptIDs(t, wpsScript, keptIDsOf(script), []string{wpsFlowParity, wukIDW})
	requireVerbatim(t, wpsScript, script, wukIDW, wukBlockWScript)
	if got := wukRemovalNotices(sum, wukIDW); len(got) != 0 {
		t.Errorf("kept workflow %s is reported as removed: %+v", wukIDW, got)
	}
}

func TestWorkflowUpdate_SelectedCustomWorkflowIsNotNamedAsRemoved(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX, wukBlockWOrch}, nil)
	c := newWukChecklist(wpsFlowParity, wukIDX)

	sum := wukRun(t, ws, c)

	if got := wukRemovalNotices(sum, wukIDX); len(got) != 0 {
		t.Errorf("kept workflow %s is reported as removed: %+v", wukIDX, got)
	}
	if got := len(wukRemovalNotices(sum, wukIDW)); got != 1 {
		t.Errorf("run summary names the removed workflow %s %d times, want once", wukIDW, got)
	}
}
