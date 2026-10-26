package session

import (
	"strings"

	"mosaic-run/internal/domain"
)

// failureReason returns the text that explains a non-SUCCESS response: the
// error reason, or the status message when no reason was given.
func failureReason(response domain.ProtocolResponse) string {
	if response.ErrorReason != "" {
		return response.ErrorReason
	}
	return response.StatusMessage
}

// reportInfraStepDone writes the step-done event of a recorded infrastructure
// step, as workflow steps do, plus the class of the agent.
func (s *sessionImpl) reportInfraStepDone(step domain.CompletedStep, class string) {
	s.deps.Debug.Log(domain.EventSessionStepDone, "step applied to artifact",
		domain.F("agent", step.AgentInstance),
		domain.F("class", class),
		domain.F("status", string(step.Status)),
		domain.F("error_code", string(step.ErrorCode)),
	)
}

// reportInfraFailure writes the dedicated failure event of an infrastructure
// dispatch that ended with a status other than SUCCESS. triggers lists the
// matched triggers in declaration order ("" for commit setup).
func (s *sessionImpl) reportInfraFailure(step domain.CompletedStep, class string, triggers []string, policy, reason string) {
	s.deps.Debug.Log(domain.EventSessionInfraFailed, reason,
		domain.F("agent", step.AgentInstance),
		domain.F("class", class),
		domain.F("triggers", strings.Join(triggers, ",")),
		domain.F("status", string(step.Status)),
		domain.F("error_code", string(step.ErrorCode)),
		domain.F("policy", policy),
	)
}

// infraPolicy names the on_failure policy applied to a declared agent.
func infraPolicy(agent domain.DeclaredInfraAgent) string {
	if agent.OnFailure == "halt" {
		return "halt"
	}
	return "continue"
}

// attachInfraFailures puts the run's infrastructure failures on outcome, read
// from the Execution Log of the latest artifact state, so failures recorded by
// earlier sessions of a resumed run are included.
func (rs *runStartCtx) attachInfraFailures(outcome domain.RunOutcome) domain.RunOutcome {
	log := rs.state.ExecutionLog
	if rs.state.RunID == "" {
		log = rs.existingState.ExecutionLog
	}
	outcome.InfraFailures = domain.ListInfraFailures(log, domain.NewInfraAgentSet(rs.allInfraAgents))
	return outcome
}
