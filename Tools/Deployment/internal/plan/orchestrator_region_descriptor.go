package plan

import "mosaic-deploy/internal/domain"

// descriptorForAgent returns the harness descriptor an agent is compared against. For an
// orchestrator-role agent the caller marked as having an undeliverable orchestrator region
// (no source marker and no harness orchestrator content), the expected orchestrator injections
// version is empty, so a deployed file without that region compares clean. Every other agent
// gets desc unchanged.
func descriptorForAgent(desc *domain.HarnessDescriptor, agent domain.Agent, undeliverable map[string]bool) *domain.HarnessDescriptor {
	if agent.Role != domain.RoleOrchestrator || !undeliverable[agent.Key] {
		return desc
	}
	adjusted := *desc
	adjusted.OrchestratorInjectionsVersion = ""
	return &adjusted
}
