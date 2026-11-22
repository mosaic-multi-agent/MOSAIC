package app

import "mosaic-deploy/internal/domain"

// rebuildRole decides the role a deployed agent is rebuilt with, from the first rule that
// applies: the source's role stamp, the orchestrator file names, the catalog entry's role,
// and finally subagent. The role only drives role-dependent output; no stamp is written
// from it.
func rebuildRole(stamp, fileName string, srcDesc *domain.HarnessDescriptor, catalogEntry *domain.Agent) domain.AgentRole {
	if role, ok := domain.ParseAgentRole(stamp); ok {
		return role
	}
	if isOrchestratorFileName(fileName, srcDesc) {
		return domain.RoleOrchestrator
	}
	if catalogEntry != nil && catalogEntry.Role != "" {
		return catalogEntry.Role
	}
	return domain.RoleSubagent
}

// lookupCatalogEntry finds the catalog entry of a deployed agent by its numeric id, otherwise
// by its agent key only when the source carries no id. It returns nil when there is no lookup or no entry.
func lookupCatalogEntry(cat CatalogAgentLookup, numericID, agentKey string) *domain.Agent {
	if cat == nil {
		return nil
	}
	if numericID != "" {
		if a, ok := cat.AgentByNumericID(numericID); ok {
			return &a
		}
		return nil
	}
	if a, ok := cat.Agent(agentKey); ok {
		return &a
	}
	return nil
}
