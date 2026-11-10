package stackrec

import (
	"mosaic-deploy/internal/catalog/identity"
	"mosaic-deploy/internal/domain"
)

// LayerInput is the plain per-layer value given to the systemfile and injections resolvers.
type LayerInput struct {
	Layer     domain.CatalogLayer
	Overrides identity.Overrides
}
