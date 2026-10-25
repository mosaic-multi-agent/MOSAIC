package runner

import (
	"fmt"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/preflight"
)

// catalogueTargets attributes each deployed agent to the tier it declares in
// the test catalog folder the deployer was built with, joined by agent key,
// and expects the model selected for that tier. Subject identity plays no part.
// The deployment tool's output is never consulted for the tier.
//
// The catalog is listed only when agents were deployed. Tier resolution is
// evaluated for the agents in order and the first agent that cannot be
// resolved is returned, before any read-back failure. Resolution failures
// are *DefinitionModelError, except catalog read errors, which are plain
// wrapped errors.
func catalogueTargets(agents []domain.DeployedAgent, catalogFolder string, models domain.ModelSelection) ([]definitionTarget, error) {
	if len(agents) == 0 {
		return nil, nil
	}
	index, err := preflight.CatalogAgentFilesByKey(catalogFolder)
	if err != nil {
		return nil, fmt.Errorf("pre-spawn check: reading test catalog %s: %w", catalogFolder, err)
	}
	tierModels := models.TierModels()
	targets := make([]definitionTarget, 0, len(agents))
	for _, a := range agents {
		t, err := resolveTarget(a, index[a.Key], catalogFolder, tierModels)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, nil
}

// resolveTarget resolves one deployed agent's declared tier and expected model
// from the catalog files its key joined to.
func resolveTarget(a domain.DeployedAgent, files []string, catalogFolder string, tierModels map[string]string) (definitionTarget, error) {
	fail := func(problem DefinitionModelProblem, tier string, matched []string) error {
		return &DefinitionModelError{
			Agent: a.Key, Path: a.DestinationPath, Tier: tier,
			CatalogFolder: catalogFolder, CatalogFiles: matched, Problem: problem,
		}
	}
	switch len(files) {
	case 0:
		return definitionTarget{}, fail(DefinitionModelNoCatalogEntry, "", nil)
	case 1:
	default:
		return definitionTarget{}, fail(DefinitionModelAmbiguousCatalogEntry, "", files)
	}
	tier, declared, err := preflight.DeclaredTier(files[0])
	if err != nil {
		return definitionTarget{}, fmt.Errorf("pre-spawn check: agent %s: reading catalog file %s: %w", a.Key, files[0], err)
	}
	if !declared {
		return definitionTarget{}, fail(DefinitionModelNoDeclaredTier, "", files)
	}
	expected := tierModels[tier]
	if expected == "" {
		return definitionTarget{}, fail(DefinitionModelUnselectedTier, tier, files)
	}
	return definitionTarget{agent: a.Key, path: a.DestinationPath, tier: tier, expected: expected, files: files}, nil
}

// firstNonEmpty returns the first non-empty value, or "" when all are empty.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
