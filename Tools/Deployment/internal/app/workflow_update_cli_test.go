package app_test

// workflow_update_cli_test.go specifies how Update workflows treats the ids given to
// --workflows: a custom id deployed in either orchestrator-role file is accepted and kept in
// the file or files that hold it, with exactly one warning per kept id in the run summary and
// the TODO file; an id the stack does not supply and no file deploys fails before any write,
// naming the id and without the removed-layer hint. Deploy keeps rejecting deployed custom ids.
// The runs use the real executor and collector over a real deployed workspace; the console
// check drives the real workflows command over the real service.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// wucRun runs Update workflows with an explicit --workflows id list.
func wucRun(t *testing.T, ws string, ids ...string) (domain.RunSummary, error) {
	t.Helper()
	return wpsService(t).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: hClaude, WorkspacePath: ws, WorkflowIDs: ids, AutoConfirmPlan: true,
		ConflictDefault: domain.DecisionSkip})
}

// wucTodoFile returns the bytes of the TODO file the run wrote.
func wucTodoFile(t *testing.T, sum domain.RunSummary) string {
	t.Helper()
	if sum.TodoFilePath == "" {
		t.Fatalf("the run wrote no TODO file; summary todos: %+v", sum.Todos)
	}
	file, err := os.ReadFile(sum.TodoFilePath)
	if err != nil {
		t.Fatalf("read TODO file %q: %v", sum.TodoFilePath, err)
	}
	return string(file)
}

// wucDeployWith runs Deploy over the workspace with one workflow id.
func wucDeployWith(ws, id string, t *testing.T) error {
	t.Helper()
	_, err := wpsService(t).DeployNew(context.Background(), app.DeployRequest{
		HarnessID: hClaude, WorkspacePath: ws, Scope: domain.ScopeProject,
		WorkflowIDs: []string{id}, AutoConfirmPlan: true})
	return err
}

func TestWorkflowUpdate_ExplicitIds_CustomIdDeployedOnlyInScriptIsAcceptedAndKeptOnlyThere(t *testing.T) {
	ws := wukWorkspace(t, nil, []string{wukBlockZ})

	_, err := wucRun(t, ws, wpsFlowParity, wukIDZ)

	if err != nil {
		t.Fatalf("a custom id deployed in the orchestrator-script must be accepted, got: %v", err)
	}
	script := keptBlocksIn(t, wpsScript, gamRead(t, ws, wpsRel(t, hClaude, wpsScript)))
	requireKeptIDs(t, wpsScript, keptIDsOf(script), []string{wpsFlowParity, wukIDZ})
	requireVerbatim(t, wpsScript, script, wukIDZ, wukBlockZ)
	requireKeptIDs(t, wpsOrch, wukIDsIn(t, ws, wpsOrch), []string{wpsFlowParity})
}

func TestWorkflowUpdate_ExplicitIds_CustomIdDeployedInBothFilesKeepsEachFilesOwnBytes(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockWOrch}, []string{wukBlockWScript})

	_, err := wucRun(t, ws, wpsFlowParity, wukIDW)

	if err != nil {
		t.Fatalf("a custom id deployed in both files must be accepted, got: %v", err)
	}
	orch := keptBlocksIn(t, wpsOrch, gamRead(t, ws, wpsRel(t, hClaude, wpsOrch)))
	requireVerbatim(t, wpsOrch, orch, wukIDW, wukBlockWOrch)
	script := keptBlocksIn(t, wpsScript, gamRead(t, ws, wpsRel(t, hClaude, wpsScript)))
	requireVerbatim(t, wpsScript, script, wukIDW, wukBlockWScript)
}

func TestWorkflowUpdate_ExplicitIds_UnknownIdFailsBeforeAnyWriteAndNamesTheId(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, []string{wukBlockZ})
	before := snapshotDir(t, ws)

	_, err := wucRun(t, ws, wpsFlowParity, wukIDX, "ghost-flow")

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), "ghost-flow") {
		t.Errorf("error %q does not name the unknown id", err.Error())
	}
	after := snapshotDir(t, ws)
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s changed although the run failed", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("%s was created although the run failed", path)
		}
	}
}

func TestWorkflowUpdate_ExplicitIds_EachKeptIdWarnsOnceInSummaryAndTodoFile(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX, wukBlockWOrch}, []string{wukBlockWScript, wukBlockZ})

	sum, err := wucRun(t, ws, wpsFlowParity, wukIDX, wukIDW, wukIDZ)

	if err != nil {
		t.Fatalf("UpdateWorkflows: %v", err)
	}
	file := wucTodoFile(t, sum)
	for _, id := range []string{wukIDX, wukIDW, wukIDZ} {
		requireOneWarning(t, sum, file, id)
	}
	if got := len(keptWarningsIn(sum)); got != 3 {
		t.Errorf("run summary holds %d kept-workflow warnings, want 3 (one per kept id, custom-w is in two files); items: %+v",
			got, sum.Todos)
	}
}

func TestWorkflowUpdate_ExplicitIds_CatalogIdGetsNoKeptWarning(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, nil)

	sum, err := wucRun(t, ws, wpsFlowParity, wpsFlowBeta, wukIDX)

	if err != nil {
		t.Fatalf("UpdateWorkflows: %v", err)
	}
	file := wucTodoFile(t, sum)
	requireOneWarning(t, sum, file, wukIDX)
	for _, id := range []string{wpsFlowParity, wpsFlowBeta} {
		if strings.Contains(file, id+" is not in the catalog stack") {
			t.Errorf("TODO file warns that %q is kept, but the stack supplies it:\n%s", id, file)
		}
	}
}

// The removed-layer hint applies only to an id that is deployed in the workspace. In this flow
// every deployed id is accepted and kept, so no id can be both deployed and rejected: the hint
// cannot show here. This test is regression coverage of that, not a discriminating check of the
// hint's removal.
func TestWorkflowUpdate_ExplicitIds_UnknownIdErrorCarriesNoRemovedLayerHint(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, nil)

	_, err := wucRun(t, ws, wukIDX, "ghost-flow")

	if err == nil {
		t.Fatal("an unknown id must fail the run")
	}
	if strings.Contains(err.Error(), removedLayerHintPhrase) {
		t.Errorf("error %q must not carry the removed-layer hint in the workflows flow", err.Error())
	}
}

func TestWorkflowsCommand_KeptIdPrintsExactlyOneConsoleWarningLine(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, []string{wukBlockX, wukBlockZ})
	var out bytes.Buffer

	code := cli.Run(context.Background(),
		[]string{"workflows", "--harness", hClaude, "--workspace", ws, "--auto-confirm",
			"--workflows=" + wukIDX + "," + wpsFlowParity + "," + wukIDZ},
		wpsService(t), &out, &bytes.Buffer{})

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; output:\n%s", code, cli.ExitSuccess, out.String())
	}
	for _, id := range []string{wukIDX, wukIDZ} {
		warning := id + " is not in the catalog stack; kept as deployed, not updated or validated."
		if got := strings.Count(out.String(), warning); got != 1 {
			t.Errorf("console holds %d lines with %q, want exactly 1; output:\n%s", got, warning, out.String())
		}
	}
	if strings.Contains(out.String(), wpsFlowParity+" is not in the catalog stack") {
		t.Errorf("console warns that the catalog id %q is kept:\n%s", wpsFlowParity, out.String())
	}
}

func TestDeploy_WorkflowsNamingDeployedCustomIdStillFailsWithRemovedLayerHint(t *testing.T) {
	ws := wukWorkspace(t, []string{wukBlockX}, nil)
	before := snapshotDir(t, ws)

	err := wucDeployWith(ws, wukIDX, t)

	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), removedLayerHintPhrase) {
		t.Errorf("error %q must keep the removed-layer hint in the deploy flow", err.Error())
	}
	after := snapshotDir(t, ws)
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s changed although deploy failed", path)
		}
	}
}
