package app_test

// workflow_update_kept_test.go specifies the Update workflows checklist for custom workflows:
// workflows deployed in an orchestrator-role file that the catalog stack does not supply. They
// are offered as deployed-only options, left unchecked like every other option, and a kept custom
// workflow is not reported as version drift.

import (
	"testing"
)

func TestWorkflowUpdate_Checklist_ListsCustomWorkflowsOfEitherFileAsDeployedOnlyAndUnchecked(t *testing.T) {
	// Arrange: X only in the orchestrator, Z only in the script, W in both.
	ws := wukWorkspace(t, []string{wukBlockX, wukBlockWOrch}, []string{wukBlockWScript, wukBlockZ})
	c := newWukChecklist(wpsFlowParity)

	// Act
	wukRun(t, ws, c)

	// Assert
	if len(c.asked) != 1 {
		t.Fatalf("checklist asked %d times, want once", len(c.asked))
	}
	q := c.asked[0]
	var customIDs []string
	for _, id := range []string{wukIDX, wukIDW, wukIDZ} {
		opt, ok := wukOption(q, id)
		if !ok {
			t.Errorf("checklist has no option for the deployed custom workflow %s", id)
			continue
		}
		customIDs = append(customIDs, id)
		if opt.Hint != wukCustomHint {
			t.Errorf("option %s hint = %q, want %q", id, opt.Hint, wukCustomHint)
		}
		if containsID(q.DefaultOptionIDs, id) {
			t.Errorf("option %s is selected by default; DefaultOptionIDs = %v, want it unchecked", id, q.DefaultOptionIDs)
		}
	}
	if len(customIDs) != 3 {
		return
	}
	seen := map[string]int{}
	for _, o := range q.Options {
		seen[o.ID]++
	}
	for _, id := range customIDs {
		if seen[id] != 1 {
			t.Errorf("custom workflow %s appears %d times in the checklist, want once", id, seen[id])
		}
	}
}

func TestWorkflowUpdate_Checklist_CatalogOptionsStayCatalogOptions(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, nil)
	c := newWukChecklist(wpsFlowParity)

	wukRun(t, ws, c)

	if len(c.asked) != 1 {
		t.Fatalf("checklist asked %d times, want once", len(c.asked))
	}
	q := c.asked[0]
	if _, ok := wukOption(q, wukIDX); !ok {
		t.Fatal("checklist has no option for the deployed custom workflow; the catalog-option check would pass on any list")
	}
	for _, id := range wpsAllFlows {
		opt, ok := wukOption(q, id)
		if !ok {
			t.Errorf("catalog workflow %s is missing from the checklist", id)
			continue
		}
		if opt.Hint == wukCustomHint {
			t.Errorf("catalog workflow %s is marked %q", id, wukCustomHint)
		}
	}
	if opt, _ := wukOption(q, wpsFlowParity); opt.Hint != "currently deployed" {
		t.Errorf("deployed catalog workflow hint = %q, want %q", opt.Hint, "currently deployed")
	}
}

func TestWorkflowUpdate_KeptCustomWorkflow_ReportsNoVersionDrift(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, []string{wukBlockZ})
	c := newWukChecklist(wpsFlowParity, wukIDX, wukIDZ)

	sum := wukRun(t, ws, c)

	for _, id := range []string{wukIDX, wukIDZ} {
		if got := wukMentions(sum, c, id); len(got) != 0 {
			t.Errorf("kept custom workflow %s is reported as drift: %v", id, got)
		}
	}
	if len(c.plans) == 0 {
		t.Fatal("the run never reached Review")
	}
}
