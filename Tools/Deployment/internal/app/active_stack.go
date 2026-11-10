package app

import (
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// effectiveStackOrigin is the one origin rule shared by the persistence decision and
// ActiveStack: a set StackOrigin wins; otherwise StackFromInput true means stack input and
// false means default.
func (s *service) effectiveStackOrigin() stackconfig.Origin {
	switch {
	case s.deps.StackOrigin != "":
		return s.deps.StackOrigin
	case s.deps.StackFromInput:
		return stackconfig.OriginStackInput
	default:
		return stackconfig.OriginDefault
	}
}

// ActiveStack returns the layers of the catalog in effect, highest first, and the effective
// origin of the stack. A catalog that is not composed from layers reports one layer holding
// its catalog root.
func (s *service) ActiveStack() stackconfig.ActiveStack {
	active := stackconfig.ActiveStack{Origin: s.effectiveStackOrigin()}
	if lister, ok := s.deps.Catalog.(catalog.LayerLister); ok {
		active.Layers = lister.Layers()
	} else if s.deps.Catalog != nil {
		active.Layers = []domain.CatalogLayer{{Root: s.deps.Catalog.CatalogRoot()}}
	}
	return active
}
