package app_test

// role_region_update_test.go specifies the role-forbidden-region warning on Update workspace:
// a subagent whose catalog source carries the orchestrator harness region, and a harness-only
// subagent whose deployed file carries it, are each flagged once in the output with the
// check's code while the update continues. Orchestrators carrying the region, and agents
// without it, produce no such warning.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// updateWithSubagentSource updates both orchestrators and the test-runner subagent, whose
// catalog source is subagentSource. The content callback runs twice per item.
func updateWithSubagentSource(t *testing.T, subagentSource string) (*repeatingContentExecutor, *spyTodo) {
	t.Helper()
	return updateWithSources(t, stampSourceBothMarkers, subagentSource)
}

// updateWithSources is updateWithSubagentSource with an explicit orchestrator source.
func updateWithSources(t *testing.T, orchestratorSource, subagentSource string) (*repeatingContentExecutor, *spyTodo) {
	t.Helper()
	exec, spy, deps, workspace := deployCaptureDeps(t, orchestratorSource, 2)
	if orchestratorSource == roleRegionOrchestratorWithConductRegion {
		deps.BundleLoader = roleRegionConductBundleLoader()
	}
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

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID:       minimalHarness.ID,
		WorkspacePath:   workspace,
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("Update must succeed: %v", err)
	}
	return exec, spy
}

func TestUpdate_SubagentSourceCarryingOrchestratorRegion_WarnsOnceAndContinues(t *testing.T) {
	exec, spy := updateWithSubagentSource(t, roleRegionSubagentWithRegion)

	if err := exec.errors[roleRegionAgentKey]; err != nil {
		t.Fatalf("content callback for %s failed: %v", roleRegionAgentKey, err)
	}
	if len(exec.contents[roleRegionAgentKey]) == 0 {
		t.Errorf("%s: no content produced; the warning must not stop the update", roleRegionAgentKey)
	}
	assertOneRoleForbiddenWarning(t, roleRegionAgentKey, spy.gaps)
	// The fixture also lacks the conduct regions; only the orchestrator-region issue may surface.
	assertOnlyGapOwnedBy(t, spy.gaps, roleRegionAgentKey)
}

func TestUpdate_ValidSubagentAndOrchestrators_NoRoleForbiddenWarning(t *testing.T) {
	exec, spy := updateWithSubagentSource(t, roleRegionValidSubagent)

	for _, key := range []string{"orchestrator", "orchestrator-script", roleRegionAgentKey} {
		if err := exec.errors[key]; err != nil {
			t.Errorf("content callback for %s failed: %v", key, err)
		}
	}
	assertNoRoleForbiddenWarning(t, spy.gaps)
	assertNoGapsOwnedBy(t, spy.gaps, "orchestrator", "orchestrator-script", roleRegionAgentKey)
}

func TestUpdate_OrchestratorsCarryingConductRegion_NoRoleForbiddenWarning(t *testing.T) {
	exec, spy := updateWithSources(t, roleRegionOrchestratorWithConductRegion, roleRegionValidSubagent)

	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if err := exec.errors[key]; err != nil {
			t.Errorf("content callback for %s failed: %v", key, err)
		}
	}
	assertNoRoleForbiddenWarning(t, spy.gaps)
	assertNoGapsOwnedBy(t, spy.gaps, "orchestrator", "orchestrator-script")
}

// updateHarnessOnly runs Update over a workspace holding one harness-only agent file and
// returns the TODO spy and the executor that invoked the content callback for it.
func updateHarnessOnly(t *testing.T, fileContent []byte) (*spyTodo, *capturingExecutor) {
	t.Helper()
	const agentsDir = harnessOnlyTestAgentsDir
	const fileName = "my-harness-agent.md"
	stub := interactiontest.NewBuilder().
		AnswerSelectOne(domain.QHarnessOnlyRefreshScope, filepath.Join(agentsDir, fileName), string(app.RefreshProtocolOnly)).
		AnswerReview(true).
		Build()
	deps, workspace := newBaseDepsWithAgentsDir(t, stub, agentsDir)
	writeAgentFile(t, filepath.Join(workspace, agentsDir), fileName, fileContent)
	spy := &spyTodo{}
	deps.Todo = spy
	exec := &capturingExecutor{}
	deps.Executor = exec

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID:       "stub-harness",
		WorkspacePath:   workspace,
		AutoConfirmPlan: true,
	})
	if err != nil {
		t.Fatalf("Update must succeed: %v", err)
	}
	if exec.capturedReq == nil || !planHasHarnessOnlyItem(exec.capturedReq, filepath.Join(agentsDir, fileName)) {
		t.Fatal("the harness-only agent is not in the update plan; the fixture is not eligible")
	}
	return spy, exec
}

func TestUpdate_HarnessOnlySubagentCarryingOrchestratorRegion_WarnsOnceAndContinues(t *testing.T) {
	spy, exec := updateHarnessOnly(t, roleRegionHarnessOnlyFile("subagent", true))

	for key, err := range harnessOnlyContentErrors(exec) {
		t.Errorf("content callback for %s failed: %v", key, err)
	}
	assertOneRoleForbiddenWarning(t, "my-harness-agent", spy.gaps)
}

func TestUpdate_HarnessOnlySubagentWithoutOrchestratorRegion_NoWarning(t *testing.T) {
	spy, exec := updateHarnessOnly(t, roleRegionHarnessOnlyFile("subagent", false))

	for key, err := range harnessOnlyContentErrors(exec) {
		t.Errorf("content callback for %s failed: %v", key, err)
	}
	assertNoRoleForbiddenWarning(t, spy.gaps)
}

func TestUpdate_HarnessOnlyOrchestratorCarryingOrchestratorRegion_NoWarning(t *testing.T) {
	spy, exec := updateHarnessOnly(t, roleRegionHarnessOnlyFile("orchestrator", true))

	for key, err := range harnessOnlyContentErrors(exec) {
		t.Errorf("content callback for %s failed: %v", key, err)
	}
	assertNoRoleForbiddenWarning(t, spy.gaps)
}

func TestUpdate_HarnessOnlyOrchestratorCarryingConductRegion_NoWarning(t *testing.T) {
	spy, exec := updateHarnessOnly(t, roleRegionHarnessOnlyFileWithConduct("orchestrator"))

	for key, err := range harnessOnlyContentErrors(exec) {
		t.Errorf("content callback for %s failed: %v", key, err)
	}
	assertNoRoleForbiddenWarning(t, spy.gaps)
}

// harnessOnlyContentErrors returns the content errors of the executor except the one for the
// default stub plan's catalog item, which is unrelated to the harness-only file under test.
func harnessOnlyContentErrors(exec *capturingExecutor) map[string]error {
	out := make(map[string]error)
	for key, err := range exec.contentErrors {
		if key != roleRegionAgentKey {
			out[key] = err
		}
	}
	return out
}
