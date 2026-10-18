package stack

import (
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/catalog/stack/systemfile"
	"mosaic-deploy/internal/domain"
)

const (
	orchestratorKey       = "orchestrator"
	orchestratorScriptKey = "orchestrator-script"
)

// layerItems lists the items one layer supplies, with each orchestrator agent folded into the
// single orchestrator-pair item (the orchestrator agent keys never appear as agent items).
func layerItems(l loadedLayer) []domain.CatalogItem {
	var items []domain.CatalogItem
	addAgents := func(agents []domain.Agent) {
		for _, a := range agents {
			if a.Key == orchestratorKey || a.Key == orchestratorScriptKey {
				continue
			}
			items = append(items, domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: a.Key})
		}
	}
	addAgents(l.cat.Agents())
	addAgents(l.cat.UtilityAgents())
	addAgents(l.cat.StandaloneAgents())
	for _, w := range l.cat.Workflows() {
		items = append(items, domain.CatalogItem{Kind: domain.CatalogItemWorkflow, Key: w.ID})
	}
	for _, s := range l.cat.Skills() {
		items = append(items, domain.CatalogItem{Kind: domain.CatalogItemSkill, Key: s.Key})
	}
	for _, h := range l.cat.Hooks() {
		items = append(items, domain.CatalogItem{Kind: domain.CatalogItemHook, Key: h.Key})
	}
	_, scriptOK := l.cat.OrchestratorScript()
	if l.cat.Orchestrator().Key != "" || scriptOK {
		items = append(items, domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair})
	}
	return items
}

// supplierIndexes maps every item to the indexes of the layers that supply it, highest first.
func supplierIndexes(layers []loadedLayer) map[domain.CatalogItem][]int {
	out := make(map[domain.CatalogItem][]int)
	for i, l := range layers {
		seen := make(map[domain.CatalogItem]bool)
		for _, it := range layerItems(l) {
			if seen[it] {
				continue
			}
			seen[it] = true
			out[it] = append(out[it], i)
		}
	}
	return out
}

// Provenance returns the layer that supplied item. Both orchestrator agent keys answer with the
// layer that supplies the orchestrator pair. ok is false for an item the catalog does not contain.
func (r *resolved) Provenance(item domain.CatalogItem) (domain.CatalogLayer, bool) {
	if item.Kind == domain.CatalogItemAgent && (item.Key == orchestratorKey || item.Key == orchestratorScriptKey) {
		if _, ok := r.agentIdx[item.Key]; !ok {
			return domain.CatalogLayer{}, false
		}
		item = domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair}
	}
	if item.Kind == domain.CatalogItemSystemFile {
		for _, f := range []systemfile.Resolution{r.sysFile.Protocol, r.sysFile.Bundle} {
			if f.Supplied && f.Name == item.Key {
				return f.Supplier, true
			}
		}
		return domain.CatalogLayer{}, false
	}
	if item.Kind == domain.CatalogItemHarnessInjections {
		h := r.harnessInj.Lookup(item.Key)
		return h.Supplier, h.Supplied
	}
	idxs, ok := r.suppliers[item]
	if !ok {
		return domain.CatalogLayer{}, false
	}
	return r.layers[idxs[0]].info, true
}

// layerInputs converts the loaded layers into the plain input of the system-file resolver.
func layerInputs(layers []loadedLayer) []stackrec.LayerInput {
	out := make([]stackrec.LayerInput, len(layers))
	for i, l := range layers {
		out[i] = stackrec.LayerInput{Layer: l.info, Overrides: l.overrides}
	}
	return out
}

// SystemFiles returns the whole-file resolution of the protocol and the deployed-sections bundle.
func (r *resolved) SystemFiles() systemfile.Result { return r.sysFile }

// HarnessInjections returns the per-harness whole-directory resolution of the injection content.
func (r *resolved) HarnessInjections() injections.Result { return r.harnessInj }
