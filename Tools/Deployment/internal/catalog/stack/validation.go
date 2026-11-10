package stack

// validation.go holds the structural rules a stack must satisfy. They run while the stack is
// composed, in this order, and the first failure is the only error New returns:
//
//  1. no two layers share a catalog id (checkDuplicateIDs);
//  2. a layer supplies both orchestrator files or neither (checkOrchestratorPairs);
//  3. numeric agent ids are unique over the resolved agents (checkNumericIDs).
//
// A numeric-id collision is a composition error, like a duplicate catalog id. It is detected over
// the whole resolved set, so a collision inside one layer is caught here too. The layer's own
// duplicate-agent-id issue may stay in that layer's attributed issues, but this check is the one
// place that reports the collision as an error, so it is reported once.

import (
	"errors"
	"sort"

	"mosaic-deploy/internal/catalog/stack/stackerr"
	"mosaic-deploy/internal/domain"
)

const (
	orchestratorFile       = "orchestrator.md"
	orchestratorScriptFile = "orchestrator-script.md"
)

// checkDuplicateIDs rejects layers that carry the same catalog id. It reports the repeated id whose
// first occurrence is highest in the stack, with every layer root holding that id, highest first.
func checkDuplicateIDs(layers []loadedLayer) error {
	byID := make(map[string][]string, len(layers))
	for _, l := range layers {
		id := l.info.Identity.ID
		byID[id] = append(byID[id], l.info.Root)
	}
	for _, l := range layers {
		id := l.info.Identity.ID
		if roots := byID[id]; len(roots) > 1 {
			return &stackerr.DuplicateCatalogIDError{CatalogID: id, Roots: roots}
		}
	}
	return nil
}

// checkOrchestratorPairs rejects a layer that supplies only one of the two orchestrator files.
func checkOrchestratorPairs(layers []loadedLayer) error {
	for _, l := range layers {
		hasOrch := l.cat.Orchestrator().Key != ""
		_, hasScript := l.cat.OrchestratorScript()
		var missing string
		switch {
		case hasOrch && !hasScript:
			missing = orchestratorScriptFile
		case hasScript && !hasOrch:
			missing = orchestratorFile
		default:
			continue
		}
		return &stackerr.LayerError{
			Root:      l.info.Root,
			CatalogID: l.info.Identity.ID,
			Kind:      stackerr.ErrOrchestratorPairIncomplete,
			Err:       errors.New("missing Orchestrator/" + missing),
		}
	}
	return nil
}

// checkNumericIDs rejects two different resolved agents that declare the same numeric id. With
// several collisions it reports the smallest id (compared as a string), naming the two smallest
// keys holding it and the layer that supplies each.
func checkNumericIDs(r *resolved) error {
	byID := make(map[string][]string)
	for key, a := range r.agentIdx {
		if a.NumericID != "" {
			byID[a.NumericID] = append(byID[a.NumericID], key)
		}
	}
	var ids []string
	for id, keys := range byID {
		if len(keys) > 1 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)
	keys := byID[ids[0]]
	sort.Strings(keys)
	supplier := r.winnerCatalogs()
	return &stackerr.NumericIDCollisionError{
		NumericID: ids[0],
		Keys:      [2]string{keys[0], keys[1]},
		Catalogs:  [2]domain.CatalogIdentity{supplier[keys[0]], supplier[keys[1]]},
	}
}

// winnerCatalogs maps each resolved agent key to the catalog of the layer that supplies it.
func (r *resolved) winnerCatalogs() map[string]domain.CatalogIdentity {
	out := make(map[string]domain.CatalogIdentity, len(r.agentIdx))
	for _, l := range r.layers {
		var agents []domain.Agent
		agents = append(agents, l.cat.Agents()...)
		agents = append(agents, l.cat.UtilityAgents()...)
		agents = append(agents, l.cat.StandaloneAgents()...)
		if o := l.cat.Orchestrator(); o.Key != "" {
			agents = append(agents, o)
		}
		if s, ok := l.cat.OrchestratorScript(); ok {
			agents = append(agents, s)
		}
		for _, a := range agents {
			if _, done := out[a.Key]; !done && r.agentIdx[a.Key].SourcePath == a.SourcePath {
				out[a.Key] = l.info.Identity
			}
		}
	}
	return out
}
