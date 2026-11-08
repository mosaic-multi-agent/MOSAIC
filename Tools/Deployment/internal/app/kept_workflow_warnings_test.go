package app_test

// kept_workflow_warnings_test.go specifies the warning that Update workspace gives for each
// workflow that is deployed in an orchestrator-role file but not supplied by the catalog stack:
// one warning per workflow id, however many files hold it, with fixed text, in the run summary
// and in the TODO file the run writes. The run uses the real executor and collector.

import (
	"context"
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/todo"
)

// keptWarningText is the warning text the requirements fix for one kept workflow id.
func keptWarningText(id string) string {
	return id + " is not in the catalog stack; kept as deployed, not updated or validated."
}

// runKeptUpdateWithTodoFile runs Update workspace with the real executor and collector and
// returns the run summary and the bytes of the TODO file the run wrote.
func runKeptUpdateWithTodoFile(t *testing.T, ws string, cat *stubCatalog) (domain.RunSummary, string) {
	t.Helper()
	deps, _ := newBaseDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build())
	deps.Catalog = cat
	deps.Planner = &capturingPlanner{result: keptPlan(ws)}
	deps.Manifest = manifest.NewStore()
	deps.Todo = todo.NewCollector()
	deps.Executor = deploy.NewExecutor(deps.Manifest, deps.Logger, deps.Todo)

	summary, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: minimalHarness.ID, WorkspacePath: ws, AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("Update must succeed: %v", err)
	}
	if summary.TodoFilePath == "" {
		t.Fatalf("the run wrote no TODO file; summary todos: %+v", summary.Todos)
	}
	file, err := os.ReadFile(summary.TodoFilePath)
	if err != nil {
		t.Fatalf("read TODO file %q: %v", summary.TodoFilePath, err)
	}
	return summary, string(file)
}

// keptWarningsIn returns the run summary's kept-workflow items.
func keptWarningsIn(summary domain.RunSummary) []domain.TodoItem {
	var out []domain.TodoItem
	for _, item := range summary.Todos {
		if item.Kind == domain.GapWorkflowKept {
			out = append(out, item)
		}
	}
	return out
}

// requireOneWarning fails unless the summary holds exactly one kept-workflow item with the
// exact text for id and the TODO file shows that text exactly once.
func requireOneWarning(t *testing.T, summary domain.RunSummary, file, id string) {
	t.Helper()
	text := keptWarningText(id)
	n := 0
	for _, item := range keptWarningsIn(summary) {
		if item.Detail == text {
			n++
		}
	}
	if n != 1 {
		t.Errorf("run summary holds %d kept-workflow warnings with text %q, want 1; items: %+v", n, text, summary.Todos)
	}
	if c := strings.Count(file, text); c != 1 {
		t.Errorf("TODO file shows %q %d times, want 1; file:\n%s", text, c, file)
	}
}

func TestUpdate_KeptWorkflowInOrchestrator_WarnsOnceInSummaryAndTodoFile(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptQuickFixDeployed))

	summary, file := runKeptUpdateWithTodoFile(t, ws, keptCatalog())

	requireOneWarning(t, summary, file, "custom-x")
	if got := len(keptWarningsIn(summary)); got != 1 {
		t.Errorf("run summary holds %d kept-workflow warnings, want 1 (only custom-x is kept)", got)
	}
}

func TestUpdate_KeptWorkflowInBothFiles_WarnsOnce(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptQuickFixDeployed, keptCustomXScript))

	summary, file := runKeptUpdateWithTodoFile(t, ws, keptCatalog())

	requireOneWarning(t, summary, file, "custom-x")
	if got := len(keptWarningsIn(summary)); got != 1 {
		t.Errorf("run summary holds %d kept-workflow warnings, want 1 (one id held by two files)", got)
	}
}

func TestUpdate_KeptWorkflowsInDifferentFiles_EachIdWarnsOnce(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch),
		keptDeployedFile(keptCustomYScript, keptQuickFixDeployed))

	summary, file := runKeptUpdateWithTodoFile(t, ws, keptCatalog())

	requireOneWarning(t, summary, file, "custom-x")
	requireOneWarning(t, summary, file, "custom-y")
}

func TestUpdate_WorkflowTheStackSupplies_GetsNoKeptWarning(t *testing.T) {
	ws := t.TempDir()
	writeKeptWorkspace(t, ws,
		keptDeployedFile(keptQuickFixDeployed, keptCustomXOrch, keptCustomZOrch),
		keptDeployedFile(keptQuickFixDeployed, keptCustomXScript))
	cat := keptCatalog()
	cat.workflows = append(cat.workflows, domain.Workflow{ID: "custom-x", Name: "Custom X", Version: "1.0"})

	summary, file := runKeptUpdateWithTodoFile(t, ws, cat)

	requireOneWarning(t, summary, file, "custom-z")
	for _, id := range []string{"custom-x", "quick-fix"} {
		if strings.Contains(file, id+" is not in the catalog stack") {
			t.Errorf("TODO file warns that %q is kept, but the stack supplies it:\n%s", id, file)
		}
	}
	if got := len(keptWarningsIn(summary)); got != 1 {
		t.Errorf("run summary holds %d kept-workflow warnings, want 1 (only custom-z is kept)", got)
	}
}
