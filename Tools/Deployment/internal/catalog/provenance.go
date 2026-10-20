package catalog

import "mosaic-deploy/internal/domain"

// ProvenanceReporter is implemented by catalogs that know the supplying catalog of each resolved
// item. ValidateRegistry type-asserts to it to fill Issue.Catalog for the issue codes it raises
// with an agent-key Subject (today missing-skill-folder only); other issues keep a zero Catalog.
type ProvenanceReporter interface {
	// Provenance returns the layer that supplied item. item.Key follows the catalog item key
	// convention (domain.CatalogItem doc). The two orchestrator agents
	// {agent, "orchestrator"} and {agent, "orchestrator-script"} and the pair item
	// {orchestrator-pair, ""} all return the layer that supplies the pair.
	// ok is false for an item the catalog does not contain.
	Provenance(item domain.CatalogItem) (layer domain.CatalogLayer, ok bool)
}
