package cost_test

// Tests for pricing cache writes that carry no 5m/1h split at a chosen tier.
//
// Rules under test:
//   - Cache-write tokens without a reported split are priced at the rate of the
//     tier passed in, in the standard and in the long-context rate set.
//   - Tokens with a reported split are priced as reported, whatever tier is
//     passed in.
//   - The unsplit tokens are still counted as assumed, and the result names the
//     tier that was assumed; nothing is named when nothing was assumed.
//
// Rates are those of price_request_test.go (standard 5m 3.75 / 1h 6.00, long
// context 5m 7.50 / 1h 12.00 per 1M tokens).

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
)

func priceAtTier(t *testing.T, usage domain.TokenUsage, tier domain.CacheWriteTier) cost.InvocationCost {
	t.Helper()
	table := testTable(longContextPricing(t, 200_000))
	return cost.PriceInvocation(inv(requestTestModel, usage), table, tier)
}

func assertAssumedTier(t *testing.T, ic cost.InvocationCost, want domain.CacheWriteTier) {
	t.Helper()
	if ic.AssumedTier != want {
		t.Errorf("AssumedTier = %q, want %q", ic.AssumedTier, want)
	}
}

func TestPriceInvocation_UnsplitCacheWriteAtOneHourTier_PricesAtOneHourRate(t *testing.T) {
	ic := priceAtTier(t, domain.TokenUsage{CacheCreation: domain.Tokens(150_000)}, domain.CacheWriteTier1h)

	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 900_000_000) // 150_000 x 6.00
	assertAssumed(t, ic, 150_000)
	assertAssumedTier(t, ic, domain.CacheWriteTier1h)
}

func TestPriceInvocation_UnsplitCacheWriteAtFiveMinuteTier_PricesAtFiveMinuteRateAndNamesIt(t *testing.T) {
	ic := priceAtTier(t, domain.TokenUsage{CacheCreation: domain.Tokens(150_000)}, domain.CacheWriteTier5m)

	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 562_500_000) // 150_000 x 3.75
	assertAssumed(t, ic, 150_000)
	assertAssumedTier(t, ic, domain.CacheWriteTier5m)
}

func TestPriceInvocation_UnsplitCacheWriteAtOneHourTierInLongContext_UsesLongContextOneHourRate(t *testing.T) {
	ic := priceAtTier(t, domain.TokenUsage{
		Input:         domain.Tokens(150_000),
		CacheCreation: domain.Tokens(100_000),
	}, domain.CacheWriteTier1h)

	if ic.Tier != cost.TierLongContext {
		t.Fatalf("Tier = %v, want TierLongContext", ic.Tier)
	}
	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 1_200_000_000) // 100_000 x 12.00
	assertAssumed(t, ic, 100_000)
}

func TestPriceInvocation_SplitCacheWrite_IsPricedAsReportedWhateverTheTier(t *testing.T) {
	usage := domain.TokenUsage{
		CacheCreation:   domain.Tokens(150_000),
		CacheCreation5m: domain.Tokens(100_000),
		CacheCreation1h: domain.Tokens(50_000),
	}

	for _, tier := range []domain.CacheWriteTier{domain.CacheWriteTier5m, domain.CacheWriteTier1h} {
		ic := priceAtTier(t, usage, tier)

		assertKnownMoney(t, string(tier)+" CacheCreation", ic.Money.CacheCreation, 675_000_000) // 100_000 x 3.75 + 50_000 x 6.00
		assertAssumed(t, ic, 0)
		assertAssumedTier(t, ic, "")
	}
}

func TestPriceInvocation_SplitWithOnlyFiveMinuteTokens_StaysAtFiveMinuteRateUnderOneHourTier(t *testing.T) {
	ic := priceAtTier(t, domain.TokenUsage{
		CacheCreation:   domain.Tokens(80_000),
		CacheCreation5m: domain.Tokens(80_000),
		CacheCreation1h: domain.Tokens(0),
	}, domain.CacheWriteTier1h)

	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 300_000_000) // 80_000 x 3.75
	assertAssumed(t, ic, 0)
	assertAssumedTier(t, ic, "")
}

func TestPriceInvocation_PartialSplitUnderOneHourTier_PricesOnlyTheRemainderAtTheTier(t *testing.T) {
	ic := priceAtTier(t, domain.TokenUsage{
		CacheCreation:   domain.Tokens(190_000),
		CacheCreation5m: domain.Tokens(100_000),
		CacheCreation1h: domain.Tokens(50_000),
	}, domain.CacheWriteTier1h)

	// 100_000 x 3.75 + 50_000 x 6.00 + 40_000 x 6.00
	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 915_000_000)
	assertAssumed(t, ic, 40_000)
	assertAssumedTier(t, ic, domain.CacheWriteTier1h)
}

func TestPriceInvocation_TotalMoneyIncludesOneHourPricedUnsplitWrites(t *testing.T) {
	ic := priceAtTier(t, domain.TokenUsage{
		Input:         domain.Tokens(1_000),
		CacheRead:     domain.Tokens(2_000),
		CacheCreation: domain.Tokens(150_000),
		Output:        domain.Tokens(3_000),
	}, domain.CacheWriteTier1h)

	assertRequestMoney(t, ic, wantMoney{
		input:      3_000_000,   // 1_000 x 3.00
		cacheRead:  600_000,     // 2_000 x 0.30
		cacheWrite: 900_000_000, // 150_000 x 6.00
		output:     45_000_000,  // 3_000 x 15.00
	})
}

func TestPriceInvocation_NothingAssumed_NamesNoTier(t *testing.T) {
	cases := map[string]domain.TokenUsage{
		"absent cache write":       {Input: domain.Tokens(1_000)},
		"present zero cache write": {CacheCreation: domain.Tokens(0)},
	}
	for name, usage := range cases {
		ic := priceAtTier(t, usage, domain.CacheWriteTier1h)

		assertAssumed(t, ic, 0)
		if ic.AssumedTier != "" {
			t.Errorf("%s: AssumedTier = %q, want none", name, ic.AssumedTier)
		}
	}
}

func TestPriceInvocation_UnpricedModelUnderOneHourTier_AssumesNothing(t *testing.T) {
	table := testTable(longContextPricing(t, 200_000))

	ic := cost.PriceInvocation(inv("unknown-model", domain.TokenUsage{
		CacheCreation: domain.Tokens(150_000),
	}), table, domain.CacheWriteTier1h)

	assertUnpriced(t, "CacheCreation", ic.Money.CacheCreation)
	assertAssumed(t, ic, 0)
	assertAssumedTier(t, ic, "")
}

func TestPriceInvocation_MixedRequestsUnderOneHourTier_SumUsesEachRequestsOwnPricing(t *testing.T) {
	table := testTable(longContextPricing(t, 200_000))
	split := cost.PriceInvocation(inv(requestTestModel, domain.TokenUsage{
		CacheCreation:   domain.Tokens(150_000),
		CacheCreation5m: domain.Tokens(100_000),
		CacheCreation1h: domain.Tokens(50_000),
	}), table, domain.CacheWriteTier1h)
	totalOnly := cost.PriceInvocation(inv(requestTestModel, domain.TokenUsage{
		CacheCreation: domain.Tokens(40_000),
	}), table, domain.CacheWriteTier1h)

	sum := cost.SumCosts(split, totalOnly)

	assertAssumed(t, split, 0)
	assertAssumed(t, totalOnly, 40_000)
	assertKnownMoney(t, "sum.CacheCreation", sum.CacheCreation, 915_000_000) // 675_000_000 + 40_000 x 6.00
}
