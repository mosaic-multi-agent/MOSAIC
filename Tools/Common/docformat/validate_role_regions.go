package docformat

import "fmt"

// checkOrchestratorHarnessRegion reports a "role-forbidden-region" warning when a subagent
// document carries the orchestrator-only harness region. Other roles are not checked.
func checkOrchestratorHarnessRegion(body *Body, role string) []Issue {
	if role != "subagent" {
		return nil
	}
	if _, ok := body.Deployed(OrchestratorHarnessRegionName); !ok {
		return nil
	}
	return []Issue{{
		Severity: SeverityWarning,
		Code:     "role-forbidden-region",
		Message:  fmt.Sprintf("subagent carries managed region %q which is reserved for orchestrators", OrchestratorHarnessRegionName),
		Node:     OrchestratorHarnessRegionName,
	}}
}
