package plan

import "mosaic-deploy/internal/domain"

// agentVersionStamps builds the source version stamps one agent is compared against.
// Every agent carries the shared injections version (HarnessInjections.md); orchestrator-role
// agents additionally carry the orchestrator injections version
// (HarnessInjectionsOrchestrator.md). The two are never substituted for one another.
func agentVersionStamps(agent domain.Agent, desc *domain.HarnessDescriptor, toolMappingsVersion string) domain.VersionStamps {
	stamps := domain.VersionStamps{
		Version:             agent.Version,
		HarnessVersion:      desc.TransformVersion,
		InjectionsVersion:   desc.InjectionsVersion,
		ToolMappingsVersion: toolMappingsVersion,
	}
	if agent.Role == domain.RoleOrchestrator {
		stamps.OrchestratorInjectionsVersion = desc.OrchestratorInjectionsVersion
	}
	return stamps
}
