package analysis

import (
	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
)

// priceUsage prices one request, charging cache writes without a reported tier
// split at the default tier configured for the request's harness.
func priceUsage(u domain.InvocationUsage, table domain.PricingTable, settings domain.AnalysisSettings) cost.InvocationCost {
	return cost.PriceInvocation(u, table, settings.CacheWriteTierFor(u.Harness))
}

// assumedTiersOf lists the distinct tiers behind the assumed cache-write tokens
// of costs, in domain.CacheWriteTiers() order.
func assumedTiersOf(costs []cost.InvocationCost) []domain.CacheWriteTier {
	var seen []domain.CacheWriteTier
	for _, c := range costs {
		if c.AssumedTierCacheWrite > 0 {
			seen = append(seen, c.AssumedTier)
		}
	}
	return domain.MergeCacheWriteTiers(seen)
}
