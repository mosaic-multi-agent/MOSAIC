package injectionfile

import (
	"mosaic-common/docformat"

	"mosaic-deploy/internal/domain"
)

// Injection returns the content this harness serves for one region and role.
//
// The orchestrator region is served only to orchestrator-role agents and only from
// HarnessInjectionsOrchestrator.md. Every other region is served from HarnessInjections.md
// for every role. The two files are never merged: no returned string holds text from both.
func (c HarnessContent) Injection(req domain.InjectionRequest) (string, bool) {
	if req.Name == docformat.OrchestratorHarnessRegionName {
		if req.Role != domain.RoleOrchestrator {
			return "", false
		}
		return c.orchestratorRegion()
	}
	content, ok := c.Shared[req.Name]
	return content, ok
}

// orchestratorRegion returns the content of the orchestrator file's harness region. An
// unmigrated file still names that region HarnessConstraints; it is read as if it carried the
// orchestrator region name, so its content is served under that name only and never under the
// shared region name. When a file declares both names, the orchestrator region name wins.
func (c HarnessContent) orchestratorRegion() (string, bool) {
	if content, ok := c.Orchestrator[docformat.OrchestratorHarnessRegionName]; ok {
		return content, true
	}
	content, ok := c.Orchestrator[docformat.HarnessRegionName]
	return content, ok
}
