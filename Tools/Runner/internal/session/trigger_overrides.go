package session

import (
	"mosaic-run/internal/domain"
)

// applyOverrides applies replacement semantics to already validated overrides:
// each override replaces the named agent's trigger list entirely. It returns a
// copy of declared; when overrides is empty, declared is returned unchanged.
func applyOverrides(overrides []domain.InfrastructureOverride, declared []domain.DeclaredInfraAgent) []domain.DeclaredInfraAgent {
	if len(overrides) == 0 {
		return declared
	}

	overrideMap := make(map[string][]domain.DeclaredInfraTrigger, len(overrides))
	for _, ov := range overrides {
		overrideMap[ov.AgentName] = ov.Triggers
	}

	result := make([]domain.DeclaredInfraAgent, len(declared))
	copy(result, declared)
	for i := range result {
		if newTriggers, ok := overrideMap[result[i].Name]; ok {
			result[i].Triggers = newTriggers
		}
	}
	return result
}
