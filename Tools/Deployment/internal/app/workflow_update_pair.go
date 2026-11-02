package app

// workflow_update_pair.go holds the orchestrator-script handling of the workflow-only update
// flow. The script orchestrator is the second orchestrator-role file: when it is deployed it
// receives the same workflow set as the orchestrator in the same run, and keeps its own model.
// An absent script is never created and never produces a question or TODO entry.

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

// workflowScript is the probed state of the script orchestrator for a workflow-only update.
// The zero value means the catalog has no script or its path could not be resolved.
type workflowScript struct {
	agent      domain.Agent
	targetPath string
	state      domain.DeployedArtifactState
}

// probeWorkflowScript probes the deployed script orchestrator, including a manifest-recorded
// legacy file, the same way the orchestrator is probed.
func (s *service) probeWorkflowScript(
	module domain.HarnessModule, workspace string, scope domain.Scope, snap manifest.Snapshot,
) workflowScript {
	script, ok := s.deps.Catalog.OrchestratorScript()
	if !ok {
		return workflowScript{}
	}
	path, err := module.TargetPath(domain.TargetPathRequest{
		Kind:     domain.ArtifactAgent,
		Key:      script.Key,
		FileName: plan.AgentFileName(module, script),
		Scope:    scope,
		GOOS:     s.deps.GOOS,
	})
	if err != nil {
		return workflowScript{}
	}
	return workflowScript{
		agent:      script,
		targetPath: path,
		state:      probeAgentWithRecordedLegacy(workspace, module, snap, script.Key, path),
	}
}

// deployed reports whether the script is present in the workspace.
func (w workflowScript) deployed() bool {
	return w.targetPath != "" && w.state.Present
}

// admits reports whether a plan item is the deployed script's agent item.
func (w workflowScript) admits(item domain.PlanItem) bool {
	return w.deployed() && item.Ref.Kind == domain.ArtifactAgent && item.Ref.Key == w.agent.Key
}

// dropAbsentGaps removes plan gaps about an absent script, which this flow never deploys.
func (w workflowScript) dropAbsentGaps(gaps []domain.Gap) []domain.Gap {
	if w.agent.Key == "" || w.deployed() {
		return gaps
	}
	kept := make([]domain.Gap, 0, len(gaps))
	for _, g := range gaps {
		if g.Subject != w.agent.Key && g.Owner != w.agent.Key {
			kept = append(kept, g)
		}
	}
	return kept
}
