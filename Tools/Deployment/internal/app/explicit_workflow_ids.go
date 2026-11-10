package app

// explicit_workflow_ids.go classifies explicitly named workflow ids into catalog-supplied ids
// and ids kept as deployed.

import (
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/plan"
)

// classifyExplicitWorkflowIDs splits ids into catalog-supplied ids and kept ids.
//
// An id the catalog supplies goes to catalogIDs. An id the catalog does not supply but that is
// in deployedCustom goes to keptIDs. Any other id returns the unknown-workflow error naming it.
// Order within each result follows ids.
func classifyExplicitWorkflowIDs(c catalog.Catalog, ids []string, deployedCustom map[string]bool) (catalogIDs, keptIDs []string, err error) {
	for _, id := range ids {
		if _, ok := c.Workflow(id); ok {
			catalogIDs = append(catalogIDs, id)
			continue
		}
		if deployedCustom[id] {
			keptIDs = append(keptIDs, id)
			continue
		}
		return nil, nil, plan.NewUnknownWorkflowError(c, id)
	}
	return catalogIDs, keptIDs, nil
}
