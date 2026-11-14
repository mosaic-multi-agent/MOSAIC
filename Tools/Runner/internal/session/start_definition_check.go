package session

import (
	commonharness "mosaic-common/harness"
	"mosaic-run/internal/agentresolve"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/orchfile"
)

// checkAgentDefinitions refuses a run whose dispatchable agents are misdeclared
// (run-start step 5): a missing or empty model on any CLI harness, and on
// Claude Code a name that differs from the agent identifier.
//
// Checked: every routing-table agent, the orchestrator, and every declared
// infrastructure agent the infrastructure filter admits, whatever the per-class
// selection and the commits setting. An infrastructure agent whose definition
// file cannot be found is left to its dispatch-time handling. The check reads
// files only, so a refusal precedes every artifact write and every invocation.
func (s *sessionImpl) checkAgentDefinitions(rs *runStartCtx) (domain.RunOutcome, bool) {
	if !commonharness.IsCLIHarness(rs.config.HarnessID) {
		return domain.RunOutcome{}, false
	}
	requireName := rs.config.HarnessID == commonharness.HarnessIDClaudeCode

	refs := make([]domain.AgentReference, 0, len(rs.identifiers)+1)
	refs = append(refs, s.orchRef)
	for _, id := range rs.identifiers {
		refs = append(refs, rs.agents[id])
	}
	infraRefs, err := admittedInfraDefinitions(rs.config, rs.orchDir)
	if err != nil {
		return s.refusal(err.Error()), true
	}
	refs = append(refs, infraRefs...)

	for _, ref := range refs {
		if err := agentresolve.CheckDefinition(ref, requireName); err != nil {
			return s.refusalCaused(err.Error(), err), true
		}
	}
	return domain.RunOutcome{}, false
}

// admittedInfraDefinitions returns the definition references of the declared
// infrastructure agents admitted by the run's infrastructure filter, skipping
// agents whose definition file cannot be found.
func admittedInfraDefinitions(config domain.RunConfig, orchDir string) ([]domain.AgentReference, error) {
	declared, err := orchfile.EnumerateInfrastructureAgents(config.OrchestratorFilePath)
	if err != nil {
		return nil, err
	}
	var admitted map[string]bool
	if config.InfrastructureFilter != nil {
		admitted = make(map[string]bool, len(config.InfrastructureFilter))
		for _, key := range config.InfrastructureFilter {
			admitted[key] = true
		}
	}
	var refs []domain.AgentReference
	for _, agent := range declared {
		if admitted != nil && !admitted[agent.Name] {
			continue
		}
		ref, err := agentresolve.ResolveOne(orchDir, agent.Name)
		if err != nil {
			continue
		}
		refs = append(refs, ref)
	}
	return refs, nil
}
