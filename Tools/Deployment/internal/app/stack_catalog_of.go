package app

// stack_catalog_of.go implements the provenance method of the stack service interface.

import (
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// CatalogOf answers from the provenance lookup of the stack catalog. A catalog that is not a
// stack has no provenance, so every item is reported as not found.
func (s *service) CatalogOf(item domain.CatalogItem) (domain.CatalogLayer, bool) {
	reporter, ok := s.deps.Catalog.(catalog.ProvenanceReporter)
	if !ok {
		return domain.CatalogLayer{}, false
	}
	return reporter.Provenance(item)
}
