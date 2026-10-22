package cost

import "mosaic-log-analyzer/internal/domain"

// InvocationCost is the priced result for one invocation.
type InvocationCost struct {
	Model  domain.ModelID
	Tier   Tier
	Priced bool // false when the model had no pricing entry
	Tokens domain.TokenUsage
	Money  domain.CategoryMoney
	// AssumedTierCacheWrite is the number of cache-write tokens of this request
	// priced at the 5m rate because the request carried no tier split. 0 when
	// unpriced, when there is no cache write, or when the split was reported.
	AssumedTierCacheWrite int64
}

// PriceInvocation prices one request (one usage record, or one usage-bearing
// event of a legacy actor).
//
// Binding rules:
//   - The tier is chosen per request. When TierLongContext is selected, EVERY
//     token category of the request (input, cache read, both cache-write tiers,
//     output) is priced from the long-context rate set; otherwise from the
//     standard set. The cliff is all-or-nothing: no graduated calculation exists.
//   - Cache-write tokens reported with the tier split are priced at the 5m and 1h
//     rates. Cache-write tokens not covered by a split (the whole count when no
//     split was reported) are priced at the 5m rate and counted in
//     AssumedTierCacheWrite. Both tiers land in the one CacheCreation cell.
//   - A model with no entry (found == false) yields Priced=false and every money
//     cell MoneyUnpriced - never MoneyKnown(0) - and no assumed-tier tokens.
//   - An absent token category yields MoneyNoData for that category and
//     contributes nothing to the total.
func PriceInvocation(inv domain.InvocationUsage, table domain.PricingTable) InvocationCost {
	result := InvocationCost{
		Model:  inv.Model,
		Tokens: inv.Usage,
	}

	entry, found := table.Lookup(inv.Model)
	if !found {
		// No pricing entry: all money cells are MoneyUnpriced - never MoneyKnown(0).
		unpriced := domain.UnpricedMoney()
		total, complete := domain.SumMoney(unpriced, unpriced, unpriced, unpriced)
		result.Priced = false
		result.Money = domain.CategoryMoney{
			Input:         unpriced,
			CacheRead:     unpriced,
			CacheCreation: unpriced,
			Output:        unpriced,
			Total:         total,
			Complete:      complete,
		}
		return result
	}

	result.Priced = true

	tier, _ := SelectTier(inv.Usage, entry)
	result.Tier = tier

	rates := entry.Standard
	if tier == TierLongContext {
		rates = entry.LongContext.Rates
	}

	usage := inv.Usage
	inputMoney := rates.Input.Apply(usage.Input)
	cacheReadMoney := rates.CacheRead.Apply(usage.CacheRead)
	cacheCreationMoney, assumed := priceCacheWrite(usage, rates)
	outputMoney := rates.Output.Apply(usage.Output)
	result.AssumedTierCacheWrite = assumed

	total, complete := domain.SumMoney(inputMoney, cacheReadMoney, cacheCreationMoney, outputMoney)

	result.Money = domain.CategoryMoney{
		Input:         inputMoney,
		CacheRead:     cacheReadMoney,
		CacheCreation: cacheCreationMoney,
		Output:        outputMoney,
		Total:         total,
		Complete:      complete,
	}

	return result
}

// priceCacheWrite prices the cache-write tokens of one request: the 5m and 1h
// split at their own rates, the unsplit remainder at the 5m rate. It returns the
// money and the number of tokens priced at that assumed 5m tier.
func priceCacheWrite(usage domain.TokenUsage, rates domain.RateSet) (domain.MoneyValue, int64) {
	unsplit := usage.UnsplitCacheCreation()
	money, _ := domain.SumMoney(
		rates.CacheWrite5m.Apply(usage.CacheCreation5m),
		rates.CacheWrite1h.Apply(usage.CacheCreation1h),
		rates.CacheWrite5m.Apply(unsplit),
	)
	return money, unsplit.ValueOr(0)
}

// SumCosts folds per-invocation costs into one CategoryMoney, setting Complete
// false when any contributing invocation was unpriced.
func SumCosts(costs ...InvocationCost) domain.CategoryMoney {
	inputs := make([]domain.MoneyValue, len(costs))
	cacheReads := make([]domain.MoneyValue, len(costs))
	cacheCreations := make([]domain.MoneyValue, len(costs))
	outputs := make([]domain.MoneyValue, len(costs))

	for i, c := range costs {
		inputs[i] = c.Money.Input
		cacheReads[i] = c.Money.CacheRead
		cacheCreations[i] = c.Money.CacheCreation
		outputs[i] = c.Money.Output
	}

	inputTotal, inputComplete := domain.SumMoney(inputs...)
	cacheReadTotal, cacheReadComplete := domain.SumMoney(cacheReads...)
	cacheCreationTotal, cacheCreationComplete := domain.SumMoney(cacheCreations...)
	outputTotal, outputComplete := domain.SumMoney(outputs...)

	// The overall total is the money sum across all four categories.
	total, _ := domain.SumMoney(inputTotal, cacheReadTotal, cacheCreationTotal, outputTotal)

	// Complete is false when any per-category sum contained an unpriced value.
	complete := inputComplete && cacheReadComplete && cacheCreationComplete && outputComplete

	return domain.CategoryMoney{
		Input:         inputTotal,
		CacheRead:     cacheReadTotal,
		CacheCreation: cacheCreationTotal,
		Output:        outputTotal,
		Total:         total,
		Complete:      complete,
	}
}
