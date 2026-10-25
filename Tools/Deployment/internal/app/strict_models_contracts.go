package app

// strict_models_contracts.go declares the strict-models failure contract: the sentinel, the
// per-agent record and the typed error DeployNew returns under DeployRequest.StrictModels.

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"mosaic-deploy/internal/domain"
)

// ErrUnresolvedModels is matched (errors.Is) by every *UnresolvedModelsError.
var ErrUnresolvedModels = errors.New("one or more agents have no resolved model")

// UnresolvedAgentModel names one agent left without a model and its declared tier.
type UnresolvedAgentModel struct {
	AgentKey string
	Tier     domain.Tier // the agent's recommended_tier verbatim; "" when it declares none
}

// UnresolvedModelsError is returned by DeployNew under StrictModels.
type UnresolvedModelsError struct {
	// Agents lists every unresolved agent, sorted by AgentKey, never empty.
	Agents []UnresolvedAgentModel
}

// Error describes every unresolved agent and its tier.
func (e *UnresolvedModelsError) Error() string {
	return "strict models: " + strconv.Itoa(len(e.Agents)) + " agent(s) have no resolved model: " + e.Detail()
}

// Detail lists the agents as "<key> (tier <tier>)" entries joined by ", ". An agent that
// declares no tier is shown with tier <none>.
func (e *UnresolvedModelsError) Detail() string {
	parts := make([]string, 0, len(e.Agents))
	for _, a := range e.Agents {
		tier := string(a.Tier)
		if tier == "" {
			tier = "<none>"
		}
		parts = append(parts, a.AgentKey+" (tier "+tier+")")
	}
	return strings.Join(parts, ", ")
}

// Is reports target == ErrUnresolvedModels.
func (e *UnresolvedModelsError) Is(target error) bool { return target == ErrUnresolvedModels }

// strictModelsError returns the strict failure for the no-model gaps in gaps, or nil when
// there are none. Each agent is named once, sorted by key, with its catalog tier.
func (s *service) strictModelsError(gaps []domain.Gap) error {
	seen := map[string]bool{}
	var agents []UnresolvedAgentModel
	for _, g := range gaps {
		if g.Kind != domain.GapNoModel || seen[g.Subject] {
			continue
		}
		seen[g.Subject] = true
		entry := UnresolvedAgentModel{AgentKey: g.Subject}
		if a, ok := s.deps.Catalog.Agent(g.Subject); ok {
			entry.Tier = a.RecommendedTier
		}
		agents = append(agents, entry)
	}
	if len(agents) == 0 {
		return nil
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].AgentKey < agents[j].AgentKey })
	return &UnresolvedModelsError{Agents: agents}
}
