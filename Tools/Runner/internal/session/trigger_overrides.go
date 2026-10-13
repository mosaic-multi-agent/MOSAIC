package session

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// allowedTriggersForClass returns the set of trigger names that are permitted
// in an infrastructure_overrides entry for the given infrastructure agent
// class. A nil return means all triggers are allowed (no class-level
// restriction). Currently:
//   - "commit" class: only ITERATION_END is permitted
//   - All other classes: no restriction
func allowedTriggersForClass(class string) map[string]bool {
	switch class {
	case "commit":
		return map[string]bool{domain.InfraTriggerIterationEnd: true}
	default:
		return nil
	}
}

// validateOverrides validates each infrastructure_overrides entry against the
// declared infrastructure agents, in recorded order. The first failure is
// returned.
//
// Returns an error if:
//   - An override names an agent not in declared (unknown agent name).
//   - An override names a trigger outside the trigger vocabulary (including a
//     blank trigger and the former STAGE_END name).
//   - An override specifies a trigger not allowed for the agent's class.
func validateOverrides(overrides []domain.InfrastructureOverride, declared []domain.DeclaredInfraAgent) error {
	if len(overrides) == 0 {
		return nil
	}

	agentByName := make(map[string]domain.DeclaredInfraAgent, len(declared))
	for _, a := range declared {
		agentByName[a.Name] = a
	}

	for _, ov := range overrides {
		agent, ok := agentByName[ov.AgentName]
		if !ok {
			return fmt.Errorf("infrastructure_overrides: agent %q is not declared in the orchestrator file", ov.AgentName)
		}

		for _, tr := range ov.Triggers {
			if !domain.IsValidInfraTrigger(tr.Trigger) {
				return fmt.Errorf("infrastructure_overrides: %s",
					domain.InfraTriggerRefusalReason(ov.AgentName, tr.Trigger, domain.InfraTriggerInOverride))
			}
		}

		allowed := allowedTriggersForClass(agent.Class)
		if allowed == nil {
			continue
		}
		for _, tr := range ov.Triggers {
			if !allowed[tr.Trigger] {
				return fmt.Errorf(
					"infrastructure_overrides: trigger %q is not allowed for %s-class agent %q",
					tr.Trigger, agent.Class, ov.AgentName,
				)
			}
		}
	}
	return nil
}

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
