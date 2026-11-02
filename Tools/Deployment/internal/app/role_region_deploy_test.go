package app_test

// role_region_deploy_test.go specifies the role-forbidden-region warning on deploy: a subagent
// whose catalog source carries the orchestrator harness region is flagged in the TODO output
// with the check's code, once per agent, and the deploy still writes the agent. Orchestrators
// carrying the region and subagents without it produce no such warning and no content failure.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// deployWithSubagentSource deploys both orchestrators (both-marker source) and the test-runner
// subagent with the given catalog source. The content callback runs twice per item.
func deployWithSubagentSource(t *testing.T, subagentSource string) (*repeatingContentExecutor, *spyTodo) {
	t.Helper()
	return deployWithSources(t, stampSourceBothMarkers, subagentSource)
}

// deployWithSources is deployWithSubagentSource with an explicit orchestrator source.
func deployWithSources(t *testing.T, orchestratorSource, subagentSource string) (*repeatingContentExecutor, *spyTodo) {
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

	runDeployCapture(t, deps, workspace)
	return exec, spy
}

func TestDeploy_SubagentSourceCarryingOrchestratorRegion_WarnsOnceAndWritesAgent(t *testing.T) {
	exec, spy := deployWithSubagentSource(t, roleRegionSubagentWithRegion)

	if err := exec.errors[roleRegionAgentKey]; err != nil {
		t.Fatalf("content callback for %s failed: %v", roleRegionAgentKey, err)
	}
	if len(exec.contents[roleRegionAgentKey]) == 0 {
		t.Errorf("%s: no content produced; the warning must not stop the deploy", roleRegionAgentKey)
	}
	assertOneRoleForbiddenWarning(t, roleRegionAgentKey, spy.gaps)
	// The fixture also lacks the conduct regions; only the orchestrator-region issue may surface.
	assertOnlyGapOwnedBy(t, spy.gaps, roleRegionAgentKey)
}

func TestDeploy_OrchestratorsCarryingOrchestratorRegion_NoRoleForbiddenWarning(t *testing.T) {
	exec, spy := deployWithSubagentSource(t, roleRegionValidSubagent)

	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if err := exec.errors[key]; err != nil {
			t.Errorf("content callback for %s failed: %v", key, err)
		}
		if len(exec.contents[key]) == 0 {
			t.Errorf("%s: no content produced", key)
		}
	}
	assertNoRoleForbiddenWarning(t, spy.gaps)
	assertNoGapsOwnedBy(t, spy.gaps, "orchestrator", "orchestrator-script")
}

func TestDeploy_OrchestratorsCarryingConductRegion_NoRoleForbiddenWarning(t *testing.T) {
	exec, spy := deployWithSources(t, roleRegionOrchestratorWithConductRegion, roleRegionValidSubagent)

	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if err := exec.errors[key]; err != nil {
			t.Errorf("content callback for %s failed: %v", key, err)
		}
		if len(exec.contents[key]) == 0 {
			t.Errorf("%s: no content produced", key)
		}
	}
	assertNoRoleForbiddenWarning(t, spy.gaps)
	assertNoGapsOwnedBy(t, spy.gaps, "orchestrator", "orchestrator-script")
}

func TestDeploy_SubagentWithoutOrchestratorRegion_NoNewWarning(t *testing.T) {
	exec, spy := deployWithSubagentSource(t, roleRegionValidSubagent)

	if err := exec.errors[roleRegionAgentKey]; err != nil {
		t.Fatalf("content callback for %s failed: %v", roleRegionAgentKey, err)
	}
	if len(exec.contents[roleRegionAgentKey]) == 0 {
		t.Errorf("%s: no content produced", roleRegionAgentKey)
	}
	assertNoRoleForbiddenWarning(t, spy.gaps)
	// Enabling role-aware validation must not surface the other role rules (for example a
	// missing conduct or contract region) as new warnings.
	for _, g := range spy.gaps {
		if g.Owner == roleRegionAgentKey {
			t.Errorf("%s: unexpected warning for a valid subagent: %+v", roleRegionAgentKey, g)
		}
	}
}

func TestDeploy_SubagentSourceWithRegion_DoesNotWarnForOtherAgents(t *testing.T) {
	_, spy := deployWithSubagentSource(t, roleRegionSubagentWithRegion)

	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if got := roleForbiddenGaps(spy.gaps, key); len(got) != 0 {
			t.Errorf("%s: role-forbidden-region warning for an orchestrator: %+v", key, got)
		}
	}
}
