package app

// orchestrator_region_undeliverable.go decides, per orchestrator-role agent, whether the
// orchestrator harness region can never be written: the source carries no
// OrchestratorHarnessConstraints marker and the harness has no orchestrator content. Such a file
// never gets the region, so the planner must not expect its version there.
//
// A source without the marker on a harness that does have orchestrator content is not
// undeliverable in this sense: the tool keeps reporting it and warns on every run.

import (
	"strings"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// orchestratorRegionUndeliverable returns the keys of orchestrator-role agents among agents
// whose region cannot be delivered. It returns nil when there are none. A source that cannot
// be read or parsed is never marked, so the planner compares it as before.
func orchestratorRegionUndeliverable(cat catalog.Catalog, module domain.HarnessModule, agents []domain.Agent) map[string]bool {
	var result map[string]bool
	for _, agent := range agents {
		if agent.Role != domain.RoleOrchestrator || agent.SourcePath == "" {
			continue
		}
		if harnessHasOrchestratorContent(module, agent.Key) {
			continue
		}
		hasMarker, ok := sourceHasOrchestratorMarker(cat, agent.SourcePath)
		if !ok || hasMarker {
			continue
		}
		if result == nil {
			result = make(map[string]bool)
		}
		result[agent.Key] = true
	}
	return result
}

// harnessHasOrchestratorContent reports whether the harness supplies non-blank orchestrator
// content for the agent, the same test the undelivered-content warning applies.
func harnessHasOrchestratorContent(module domain.HarnessModule, key string) bool {
	content, ok := module.Injection(domain.InjectionRequest{
		Name:     docformat.OrchestratorHarnessRegionName,
		AgentKey: key,
		Role:     domain.RoleOrchestrator,
	})
	return ok && strings.TrimSpace(content) != ""
}

// sourceHasOrchestratorMarker reports whether the source file carries the orchestrator harness
// region. ok is false when the source cannot be read or parsed.
func sourceHasOrchestratorMarker(cat catalog.Catalog, sourcePath string) (hasMarker, ok bool) {
	data, err := cat.ReadSource(sourcePath)
	if err != nil {
		return false, false
	}
	doc, err := ParseGenericSource(data)
	if err != nil {
		return false, false
	}
	for _, node := range doc.Body().Regions() {
		if node.Kind() == docformat.NodeDeployed && node.Name() == docformat.OrchestratorHarnessRegionName {
			return true, true
		}
	}
	return false, true
}
