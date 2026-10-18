package stack

import (
	"sort"

	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/domain"
)

// buildShadows creates one record per item that more than one layer supplies, including the
// orchestrator pair as a single item. Records are ordered by kind, then key.
func buildShadows(layers []loadedLayer, suppliers map[domain.CatalogItem][]int) []stackrec.Shadow {
	var out []stackrec.Shadow
	for item, idxs := range suppliers {
		if len(idxs) < 2 {
			continue
		}
		rec := stackrec.Shadow{
			Item:     item,
			Winner:   layers[idxs[0]].info,
			Declared: layers[idxs[0]].overrides.Declares(item),
		}
		for _, i := range idxs[1:] {
			rec.Shadowed = append(rec.Shadowed, layers[i].info)
		}
		out = append(out, rec)
	}
	return out
}

// sortShadows orders records by item kind, then key.
func sortShadows(recs []stackrec.Shadow) {
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Item.Kind != recs[j].Item.Kind {
			return recs[i].Item.Kind < recs[j].Item.Kind
		}
		return recs[i].Item.Key < recs[j].Item.Key
	})
}

// Shadows returns one record per item present in more than one layer; unshadowed items have none.
func (r *resolved) Shadows() []stackrec.Shadow { return r.shadows }
