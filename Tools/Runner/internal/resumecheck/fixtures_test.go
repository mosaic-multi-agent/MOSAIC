package resumecheck_test

import (
	"mosaic-run/internal/domain"
	"mosaic-run/internal/resumecheck"
)

const (
	fixtureRunID    = "20260727T170000Z-a3f9"
	fixtureWorkflow = domain.WorkflowID("build-flow")
)

// declaredAgents returns two checkpoint-class agents and one commit-class agent.
func declaredAgents() []domain.DeclaredInfraAgent {
	return []domain.DeclaredInfraAgent{
		{Name: "cp-a", Class: "checkpoint", OnFailure: "halt"},
		{Name: "cp-b", Class: "checkpoint", OnFailure: "halt"},
		{Name: "git-commit", Class: "commit", OnFailure: "halt"},
	}
}

// satisfiableDeployment declares the fixture workflow and every agent.
func satisfiableDeployment() resumecheck.Deployment {
	return resumecheck.Deployment{
		Workflows: []domain.WorkflowRegion{
			{Info: domain.WorkflowInfo{ID: "other-flow", Version: "1.0"}},
			{Info: domain.WorkflowInfo{ID: fixtureWorkflow, Version: "2.1"}},
		},
		InfraAgents: declaredAgents(),
	}
}

// recordedState returns a run that records checkpoints and commits on, a
// selection for each class, a valid override and a workflow version that
// differs from the deployed one.
func recordedState() domain.ArtifactState {
	state := domain.ArtifactState{
		RunID:           fixtureRunID,
		Workflow:        fixtureWorkflow,
		WorkflowVersion: "2.0",
		InfrastructureOverrides: []domain.InfrastructureOverride{
			{AgentName: "git-commit", Triggers: []domain.DeclaredInfraTrigger{{Trigger: domain.InfraTriggerIterationEnd}}},
		},
	}
	state.Mode = domain.ExecutionModeAuto
	state.Checkpoints = true
	state.Commits = true
	state.CommitBranch = "mosaic/run/" + fixtureRunID
	state.InfraClassSelections = map[string]string{"checkpoint": "cp-b", "commit": "git-commit"}
	return state
}
