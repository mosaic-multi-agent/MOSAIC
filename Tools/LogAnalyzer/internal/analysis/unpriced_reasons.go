package analysis

import (
	"fmt"

	"mosaic-log-analyzer/internal/domain"
)

// unpricedReasons explains, for each unpriced key the table excluded because of a
// currency mismatch, why it is unpriced. Keys unpriced for any other reason have
// no entry. The result follows the order of keys.
func unpricedReasons(keys []domain.PricingKey, table domain.PricingTable) []domain.UnpricedReason {
	var out []domain.UnpricedReason
	for _, key := range keys {
		ex, ok := table.Excluded(key)
		if !ok {
			continue
		}
		out = append(out, domain.UnpricedReason{
			Key: key,
			Reason: fmt.Sprintf("priced only in the global file (%s) in %s; the project file's currency is %s",
				ex.Path, ex.Currency, table.Currency()),
		})
	}
	return out
}
