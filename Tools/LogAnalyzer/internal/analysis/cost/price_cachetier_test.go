package cost_test

// Tests for cache-write tier pricing and the assumed-tier count.
//
// Rules under test:
//   - A request carrying the tier split prices its 5m tokens at the 5m
//     cache-write rate and its 1h tokens at the 1h rate. Nothing is assumed.
//   - A request carrying only the total cache-write count prices the whole
//     count at the 5m rate and reports that count as AssumedTierCacheWrite.
//   - A request whose split covers only part of the total (a summed usage)
//     prices the remainder at the 5m rate and counts only the remainder.
//   - Both cache-write tiers land in the single CacheCreation money cell.
//   - Unpriced models, absent cache writes and present-zero cache writes
//     contribute no assumed tokens.
//   - The harness never changes the result.
//
// Rates are those of price_request_test.go (standard 5m 3.75 / 1h 6.00, long
// context 5m 7.50 / 1h 12.00 per 1M tokens).

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
)

func assertAssumed(t *testing.T, ic cost.InvocationCost, want int64) {
	t.Helper()
	if ic.AssumedTierCacheWrite != want {
		t.Errorf("AssumedTierCacheWrite = %d, want %d", ic.AssumedTierCacheWrite, want)
	}
}

func TestPriceInvocation_SplitCacheWrite_PricesEachTierAtItsRate(t *testing.T) {
	ic := priceOne(t, domain.TokenUsage{
		CacheCreation:   domain.Tokens(150_000),
		CacheCreation5m: domain.Tokens(100_000),
		CacheCreation1h: domain.Tokens(50_000),
	})

	// 100_000 x 3.75 + 50_000 x 6.00
	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 675_000_000)
	assertAssumed(t, ic, 0)
}

func TestPriceInvocation_SplitWithZeroFiveMinuteTokens_PricesWholeCountAtOneHourRate(t *testing.T) {
	ic := priceOne(t, domain.TokenUsage{
		CacheCreation:   domain.Tokens(50_000),
		CacheCreation5m: domain.Tokens(0),
		CacheCreation1h: domain.Tokens(50_000),
	})

	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 300_000_000) // 50_000 x 6.00
	assertAssumed(t, ic, 0)
}

func TestPriceInvocation_TotalOnlyCacheWrite_PricesAtFiveMinuteRateAndCountsAsAssumed(t *testing.T) {
	ic := priceOne(t, domain.TokenUsage{CacheCreation: domain.Tokens(150_000)})

	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 562_500_000) // 150_000 x 3.75
	assertAssumed(t, ic, 150_000)
}

func TestPriceInvocation_PartialSplit_CountsAndPricesOnlyTheUnsplitRemainderAtFiveMinuteRate(t *testing.T) {
	// A summed usage: 150_000 reported with a split, 40_000 without.
	ic := priceOne(t, domain.TokenUsage{
		CacheCreation:   domain.Tokens(190_000),
		CacheCreation5m: domain.Tokens(100_000),
		CacheCreation1h: domain.Tokens(50_000),
	})

	// 100_000 x 3.75 + 50_000 x 6.00 + 40_000 x 3.75
	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 825_000_000)
	assertAssumed(t, ic, 40_000)
}

func TestPriceInvocation_TotalOnlyCacheWriteInLongContext_PricesAtLongContextFiveMinuteRate(t *testing.T) {
	ic := priceOne(t, domain.TokenUsage{
		Input:         domain.Tokens(150_000),
		CacheCreation: domain.Tokens(100_000),
	})

	if ic.Tier != cost.TierLongContext {
		t.Fatalf("Tier = %v, want TierLongContext", ic.Tier)
	}
	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 750_000_000) // 100_000 x 7.50
	assertAssumed(t, ic, 100_000)
}

func TestPriceInvocation_AbsentCacheWrite_IsNoDataAndNotAssumed(t *testing.T) {
	ic := priceOne(t, domain.TokenUsage{Input: domain.Tokens(1_000)})

	assertNoData(t, "CacheCreation", ic.Money.CacheCreation)
	assertAssumed(t, ic, 0)
}

func TestPriceInvocation_PresentZeroCacheWrite_IsKnownZeroAndNotAssumed(t *testing.T) {
	ic := priceOne(t, domain.TokenUsage{CacheCreation: domain.Tokens(0)})

	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 0)
	assertAssumed(t, ic, 0)
}

func TestPriceInvocation_UnpricedModelWithTotalOnlyCacheWrite_CountsNothingAsAssumed(t *testing.T) {
	ic := priceAtDefaultTier(inv("unknown-model", domain.TokenUsage{
		CacheCreation: domain.Tokens(150_000),
	}), testTable(longContextPricing(t, 200_000)))

	assertUnpriced(t, "CacheCreation", ic.Money.CacheCreation)
	assertAssumed(t, ic, 0)
}

func TestPriceInvocation_TotalMoneySumsAllFourCellsWithSplitCacheWrite(t *testing.T) {
	ic := priceOne(t, domain.TokenUsage{
		Input:           domain.Tokens(1_000),
		CacheRead:       domain.Tokens(2_000),
		CacheCreation:   domain.Tokens(150_000),
		CacheCreation5m: domain.Tokens(100_000),
		CacheCreation1h: domain.Tokens(50_000),
		Output:          domain.Tokens(3_000),
	})

	assertRequestMoney(t, ic, wantMoney{
		input:      3_000_000,   // 1_000 x 3.00
		cacheRead:  600_000,     // 2_000 x 0.30
		cacheWrite: 675_000_000, // 100_000 x 3.75 + 50_000 x 6.00
		output:     45_000_000,  // 3_000 x 15.00
	})
}

func TestPriceInvocation_HarnessDoesNotChangeCacheWritePricing(t *testing.T) {
	table := testTable(longContextPricing(t, 200_000))
	usage := domain.TokenUsage{CacheCreation: domain.Tokens(150_000)}

	for _, harness := range []domain.Harness{
		domain.HarnessClaudeCode, domain.HarnessOpenCode, domain.HarnessVSCodeGHCP, domain.HarnessGHCPCLI,
	} {
		ic := priceAtDefaultTier(domain.InvocationUsage{
			Model: requestTestModel, Harness: harness, Usage: usage,
		}, table)

		assertKnownMoney(t, string(harness)+" CacheCreation", ic.Money.CacheCreation, 562_500_000)
		assertAssumed(t, ic, 150_000)
	}
}

func TestPriceInvocation_MixedRequests_AssumedCountIsPerRequest(t *testing.T) {
	table := testTable(longContextPricing(t, 200_000))
	split := priceAtDefaultTier(inv(requestTestModel, domain.TokenUsage{
		CacheCreation:   domain.Tokens(150_000),
		CacheCreation5m: domain.Tokens(100_000),
		CacheCreation1h: domain.Tokens(50_000),
	}), table)
	totalOnly := priceAtDefaultTier(inv(requestTestModel, domain.TokenUsage{
		CacheCreation: domain.Tokens(40_000),
	}), table)

	sum := cost.SumCosts(split, totalOnly)

	assertAssumed(t, split, 0)
	assertAssumed(t, totalOnly, 40_000)
	// 675_000_000 + 40_000 x 3.75
	assertKnownMoney(t, "sum.CacheCreation", sum.CacheCreation, 825_000_000)
}
