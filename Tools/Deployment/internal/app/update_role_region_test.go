package app_test

// update_role_region_test.go specifies that an update which rewrites the orchestrators and an
// agent whose catalog source carries the orchestrator harness region records the
// role-forbidden-region finding in the run summary the console reads (and the TODO checklist
// built from the same collector), identifiable by its gap kind, exactly once.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/todo"
)

// updateSummaryWithSubagentSource runs an update over both orchestrators and the test-runner
// subagent (source subagentSource) with a real TODO collector and returns the run summary.
func updateSummaryWithSubagentSource(t *testing.T, subagentSource string) domain.RunSummary {
	t.Helper()
	exec, _, deps, workspace := deployCaptureDeps(t, stampSourceBothMarkers, 2)
	deps.Todo = todo.NewCollector()
	cat := deps.Catalog.(*stubCatalog)
	const subagentPath = "/fake/test-runner.md"
	agent := minimalAgent
	agent.Role = domain.RoleSubagent
	agent.SourcePath = subagentPath
	cat.agents = []domain.Agent{agent}
	cat.sources[subagentPath] = []byte(subagentSource)
	crWriteDeployed(t, workspace, "orchestrator.md", minimalWorkflow.ID)
	crWriteDeployed(t, workspace, "orchestrator-script.md", minimalWorkflow.ID)
	crWriteDeployed(t, workspace, "test-runner.md", minimalWorkflow.ID)
	plan := scriptPlan(workspace, "orchestrator.md", "orchestrator-script.md")
	plan.Mode = domain.ModeUpdateWorkspace
	deps.Planner = &stubPlanner{plan: plan}

	sum, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID:       minimalHarness.ID,
		WorkspacePath:   workspace,
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("Update must succeed: %v", err)
	}
	if err := exec.errors[roleRegionAgentKey]; err != nil {
		t.Fatalf("content callback for %s failed: %v", roleRegionAgentKey, err)
	}
	return sum
}

func todosOfKind(items []domain.TodoItem, kind domain.GapKind) []domain.TodoItem {
	var out []domain.TodoItem
	for _, it := range items {
		if it.Kind == kind {
			out = append(out, it)
		}
	}
	return out
}

func TestUpdateSummary_RoleForbiddenRegion_ReachesConsoleTodosOnceWithKind(t *testing.T) {
	sum := updateSummaryWithSubagentSource(t, roleRegionSubagentWithRegion)

	got := todosOfKind(sum.Todos, domain.GapRoleForbiddenRegion)
	if len(got) != 1 {
		t.Fatalf("role-forbidden-region items in the run summary = %d, want exactly 1 (todos: %+v)", len(got), sum.Todos)
	}
	if got[0].Owner != roleRegionAgentKey {
		t.Errorf("item owner = %q, want %q", got[0].Owner, roleRegionAgentKey)
	}
}

func TestUpdateSummary_ValidSources_HaveNoRoleForbiddenItem(t *testing.T) {
	sum := updateSummaryWithSubagentSource(t, roleRegionValidSubagent)

	if got := todosOfKind(sum.Todos, domain.GapRoleForbiddenRegion); len(got) != 0 {
		t.Errorf("unexpected role-forbidden-region items: %+v", got)
	}
}
