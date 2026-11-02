package app

// rerun_models.go keeps the model of an agent that is already deployed when a run supplies none
// for it, so deploy and agents rewrite a stale agent in place instead of dropping its model.

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/plan"
)

// suppliedTierModels merges the tier mappings the run itself supplied: request tier models and
// tiers answered interactively. Mappings persisted in user config are deliberately excluded.
func suppliedTierModels(maps ...map[domain.Tier]string) map[domain.Tier]string {
	out := make(map[domain.Tier]string)
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// keepDeployedModels sets the deployed model of each agent in agents unless the run supplied one
// for it. A model from the agent map, from a tier mapping supplied by this run (suppliedTiers) or
// from an interactive answer always wins; with persistedTiersCount false (agents) a tier mapping that was only persisted from an earlier
// run does not. models is updated in place; agents with no deployed file or no deployed model are
// left as resolved so that a genuinely missing model is still reported.
func keepDeployedModels(
	models map[string]domain.ModelSelection,
	suppliedTiers map[domain.Tier]string,
	persistedTiersCount bool,
	agents []domain.Agent,
	paths plan.PlannedPaths,
	deployedState map[string]domain.DeployedArtifactState,
) {
	tierOf := make(map[string]domain.Tier, len(agents))
	for _, a := range agents {
		tierOf[a.Key] = a.RecommendedTier
	}
	for key, sel := range deployedModelSelections(agents, paths, deployedState) {
		cur, resolved := models[key]
		if !resolved {
			models[key] = sel
			continue
		}
		if _, tierSupplied := suppliedTiers[tierOf[key]]; !persistedTiersCount && cur.Origin == domain.OriginTierMapping && !tierSupplied {
			models[key] = sel
		}
	}
}

// keepsDeployedInfrastructure reports whether the deployed infrastructure declarations stay. They
// are replaced by an explicit selection (a non-nil request list, even an empty one) or by a
// non-empty selection answered interactively; when the request lists none and the resolved
// selection is empty (skipped, no catalog agents, an empty answer) they stay.
func keepsDeployedInfrastructure(req DeployRequest, resolved []string) bool {
	return req.InfrastructureAgentIDs == nil && len(resolved) == 0
}
