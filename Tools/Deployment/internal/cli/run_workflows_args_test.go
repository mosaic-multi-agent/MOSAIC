package cli_test

// run_workflows_args_test.go specifies how the workflows command turns its --workflows
// argument into the id list of the request: a comma-separated list, spaces trimmed, empty
// entries dropped, and nothing set when the flag is absent. The deploy command's --workflows
// handling is unchanged: it forwards its ids as given, and deploy refuses deployed custom ids
// in the service.

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

// runWorkflowsCmd runs "workflows" with the extra args and returns the request the service got.
func runWorkflowsCmd(t *testing.T, extra ...string) (*spyService, int) {
	t.Helper()
	ws := t.TempDir()
	svc := &spyService{workflowsResp: successSummary(ws)}
	args := append([]string{"workflows", "--harness", "stub-harness", "--workspace", ws, "--auto-confirm"}, extra...)
	code := cli.Run(context.Background(), args, svc, &bytes.Buffer{}, &bytes.Buffer{})
	return svc, code
}

func TestWorkflowsCmd_WorkflowsFlag_TrimsSpacesAndDropsEmptyEntries(t *testing.T) {
	svc, code := runWorkflowsCmd(t, "--workflows= wf-a, ,wf-b ,, custom-z ,")

	if code != cli.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cli.ExitSuccess)
	}
	if svc.workflowsReq == nil {
		t.Fatal("UpdateWorkflows was not called")
	}
	want := []string{"wf-a", "wf-b", "custom-z"}
	if !reflect.DeepEqual(svc.workflowsReq.WorkflowIDs, want) {
		t.Errorf("WorkflowIDs = %q, want %q", svc.workflowsReq.WorkflowIDs, want)
	}
}

func TestWorkflowsCmd_WorkflowsFlagAbsent_WorkflowIDsIsNil(t *testing.T) {
	svc, _ := runWorkflowsCmd(t)

	if svc.workflowsReq == nil {
		t.Fatal("UpdateWorkflows was not called")
	}
	if svc.workflowsReq.WorkflowIDs != nil {
		t.Errorf("WorkflowIDs = %q, want nil so the checklist is asked", svc.workflowsReq.WorkflowIDs)
	}
}

func TestDeployCmd_WorkflowsFlag_ForwardsCustomIdUnchangedToTheService(t *testing.T) {
	ws := t.TempDir()
	svc := &spyService{deployResp: successSummary(ws)}

	code := cli.Run(context.Background(),
		append(minDeployArgs("stub-harness", ws), "--workflows=wf-a, custom-z"),
		svc, &bytes.Buffer{}, &bytes.Buffer{})

	if code != cli.ExitSuccess || svc.deployReq == nil {
		t.Fatalf("exit code = %d, deployReq = %v", code, svc.deployReq)
	}
	if want := []string{"wf-a", "custom-z"}; !reflect.DeepEqual(svc.deployReq.WorkflowIDs, want) {
		t.Errorf("WorkflowIDs = %q, want %q; deploy must leave kept-id handling to the service", svc.deployReq.WorkflowIDs, want)
	}
}

func TestWorkflowsCmd_HumanOutput_PrintsOneWarningLinePerKeptWorkflow(t *testing.T) {
	ws := t.TempDir()
	summary := successSummary(ws)
	summary.Todos = []domain.TodoItem{
		{Category: domain.TodoManual, Subject: "custom-x", Detail: keptWarningX, Kind: domain.GapWorkflowKept},
		{Category: domain.TodoManual, Subject: "custom-y", Detail: keptWarningY, Kind: domain.GapWorkflowKept},
	}
	svc := &spyService{workflowsResp: summary}
	var out bytes.Buffer

	cli.Run(context.Background(),
		[]string{"workflows", "--harness", "stub-harness", "--workspace", ws, "--auto-confirm"},
		svc, &out, &bytes.Buffer{})

	lines := warningLines(out.String())
	if len(lines) != 2 {
		t.Fatalf("warning lines = %d, want 2; output:\n%s", len(lines), out.String())
	}
	if !strings.HasSuffix(lines[0], keptWarningX) || !strings.HasSuffix(lines[1], keptWarningY) {
		t.Errorf("warning lines = %q", lines)
	}
}
