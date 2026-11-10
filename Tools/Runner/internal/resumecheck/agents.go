package resumecheck

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// Refusal texts for recorded settings the declared agents cannot satisfy.
const (
	NoCheckpointProviderText = "checkpoints requested but no checkpoint provider is available"
	NoCommitAgentText        = "commits enabled but no commit-class infrastructure agent is declared"
)

// HasAgentOfClass reports whether any declared infrastructure agent has the
// given class.
func HasAgentOfClass(agents []domain.DeclaredInfraAgent, class string) bool {
	for _, a := range agents {
		if a.Class == class {
			return true
		}
	}
	return false
}

// allowedTriggersForClass returns the trigger names permitted in an
// infrastructure_overrides entry for the class. A nil return means all
// triggers are allowed. Commit-class agents accept only ITERATION_END.
func allowedTriggersForClass(class string) map[string]bool {
	if class == "commit" {
		return map[string]bool{domain.InfraTriggerIterationEnd: true}
	}
	return nil
}

// ValidateOverrides validates each infrastructure_overrides entry against the
// declared infrastructure agents, in recorded order, and returns the first
// failure. It fails for an agent that is not declared, a trigger outside the
// trigger vocabulary (including a blank one and the former STAGE_END name), and
// a trigger the agent's class does not allow.
func ValidateOverrides(overrides []domain.InfrastructureOverride, declared []domain.DeclaredInfraAgent) error {
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
				return fmt.Errorf("infrastructure_overrides: trigger %q is not allowed for %s-class agent %q",
					tr.Trigger, agent.Class, ov.AgentName)
			}
		}
	}
	return nil
}
