// Package stackrec holds the record types of the catalog stack that the app layer consumes.
// It imports internal/domain only and is never imported by the stack package's own logic
// beyond building these records.
package stackrec

import "mosaic-deploy/internal/domain"

// Shadow records one item present in more than one layer.
type Shadow struct {
	Item     domain.CatalogItem
	Winner   domain.CatalogLayer
	Shadowed []domain.CatalogLayer // every lower layer that has the item, highest first
	Declared bool                  // Winner's identity declares Item (identity.Overrides.Declares)
}
