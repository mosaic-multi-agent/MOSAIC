package cost_test

// Tests for per-request long-context pricing.
//
// Rules under test:
//   - A request is in the long-context tier when input + cache read + cache
//     write reach or exceed the model's long_context threshold (>=).
//   - The tier then applies to ALL categories of that request: input, cache
//     read, both cache-write tiers and output are priced from the long-context
//     rate set.
//   - Output tokens never count toward the context.
//   - Below the threshold, or for a model without a long_context block, the
//     standard rate set applies to every category.
//   - Each request is tiered on its own; summing requests first is wrong.
//
// Rates (per 1M tokens). Every long-context rate differs from its standard
// counterpart so a wrongly chosen rate set is always visible.
//
//	            standard  long-context
//	input         3.00        6.00
//	cache read    0.30        0.60
//	cache 5m      3.75        7.50
//	cache 1h      6.00       12.00
//	output       15.00       22.50
//
// Nanodollars for n tokens at rate r per 1M tokens are n * r * 1000.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis/cost"
	"mosaic-log-analyzer/internal/domain"
)

const requestTestModel = domain.ModelID("request-test-model")

// longContextPricing returns a model whose long-context rate set differs from
// the standard set in every category.
func longContextPricing(t *testing.T, threshold int64) domain.ModelPricing {
	t.Helper()
	p := flatPricing(t, requestTestModel)
	p.HasLongContext = true
	p.LongContext = domain.LongContextPricing{
		Threshold: threshold,
		Rates: domain.RateSet{
			Input:        mustParseRate(t, "6.00"),
			CacheRead:    mustParseRate(t, "0.60"),
			CacheWrite5m: mustParseRate(t, "7.50"),
			CacheWrite1h: mustParseRate(t, "12.00"),
			Output:       mustParseRate(t, "22.50"),
		},
	}
	return p
}

// wantMoney is the expected nanodollar amount per category of one priced request.
type wantMoney struct {
	input, cacheRead, cacheWrite, output int64
}

func assertRequestMoney(t *testing.T, ic cost.InvocationCost, want wantMoney) {
	t.Helper()
	assertKnownMoney(t, "Input", ic.Money.Input, want.input)
	assertKnownMoney(t, "CacheRead", ic.Money.CacheRead, want.cacheRead)
	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, want.cacheWrite)
	assertKnownMoney(t, "Output", ic.Money.Output, want.output)
	assertKnownMoney(t, "Total", ic.Money.Total, want.input+want.cacheRead+want.cacheWrite+want.output)
}

func priceOne(t *testing.T, usage domain.TokenUsage) cost.InvocationCost {
	t.Helper()
	table := testTable(longContextPricing(t, 200_000))
	return priceAtDefaultTier(inv(requestTestModel, usage), table)
}

func TestPriceInvocation_ContextExactlyAtThreshold_AllCategoriesUseLongContextRates(t *testing.T) {
	// Context = 100_000 + 50_000 + 50_000 = 200_000 == threshold.
	ic := priceOne(t, domain.TokenUsage{
		Input:         domain.Tokens(100_000),
		CacheRead:     domain.Tokens(50_000),
		CacheCreation: domain.Tokens(50_000),
		Output:        domain.Tokens(1_000),
	})

	if ic.Tier != cost.TierLongContext {
		t.Fatalf("Tier = %v, want TierLongContext (context == threshold)", ic.Tier)
	}
	assertRequestMoney(t, ic, wantMoney{
		input:      600_000_000, // 100_000 x 6.00
		cacheRead:  30_000_000,  // 50_000 x 0.60
		cacheWrite: 375_000_000, // 50_000 x 7.50 (no split: 5m rate)
		output:     22_500_000,  // 1_000 x 22.50
	})
}

func TestPriceInvocation_ContextOneBelowThreshold_AllCategoriesUseStandardRates(t *testing.T) {
	// Context = 100_000 + 50_000 + 49_999 = 199_999 == threshold - 1.
	ic := priceOne(t, domain.TokenUsage{
		Input:         domain.Tokens(100_000),
		CacheRead:     domain.Tokens(50_000),
		CacheCreation: domain.Tokens(49_999),
		Output:        domain.Tokens(1_000),
	})

	if ic.Tier != cost.TierStandard {
		t.Fatalf("Tier = %v, want TierStandard (context one below threshold)", ic.Tier)
	}
	assertRequestMoney(t, ic, wantMoney{
		input:      300_000_000, // 100_000 x 3.00
		cacheRead:  15_000_000,  // 50_000 x 0.30
		cacheWrite: 187_496_250, // 49_999 x 3.75
		output:     15_000_000,  // 1_000 x 15.00
	})
}

func TestPriceInvocation_ContextAboveThreshold_SplitCacheWritesUseLongContextTierRates(t *testing.T) {
	// Context = 150_000 + 100_000 + 60_000 = 310_000 > threshold.
	ic := priceOne(t, domain.TokenUsage{
		Input:           domain.Tokens(150_000),
		CacheRead:       domain.Tokens(100_000),
		CacheCreation:   domain.Tokens(60_000),
		CacheCreation5m: domain.Tokens(40_000),
		CacheCreation1h: domain.Tokens(20_000),
		Output:          domain.Tokens(2_000),
	})

	if ic.Tier != cost.TierLongContext {
		t.Fatalf("Tier = %v, want TierLongContext (context above threshold)", ic.Tier)
	}
	assertRequestMoney(t, ic, wantMoney{
		input:      900_000_000, // 150_000 x 6.00
		cacheRead:  60_000_000,  // 100_000 x 0.60
		cacheWrite: 540_000_000, // 40_000 x 7.50 + 20_000 x 12.00
		output:     45_000_000,  // 2_000 x 22.50
	})
}

func TestPriceInvocation_CacheWriteAloneReachesThreshold_SelectsLongContext(t *testing.T) {
	// Cache writes count toward the context: 10 + 199_990 = 200_000.
	ic := priceOne(t, domain.TokenUsage{
		Input:         domain.Tokens(10),
		CacheCreation: domain.Tokens(199_990),
	})

	if ic.Tier != cost.TierLongContext {
		t.Fatalf("Tier = %v, want TierLongContext (input + cache write == threshold)", ic.Tier)
	}
	assertKnownMoney(t, "CacheCreation", ic.Money.CacheCreation, 1_499_925_000) // 199_990 x 7.50
}

func TestPriceInvocation_LargeOutputDoesNotCountTowardContext(t *testing.T) {
	// Context = 100_000 only; the 5_000_000 output tokens must not push the
	// request over the threshold.
	ic := priceOne(t, domain.TokenUsage{
		Input:  domain.Tokens(100_000),
		Output: domain.Tokens(5_000_000),
	})

	if ic.Tier != cost.TierStandard {
		t.Fatalf("Tier = %v, want TierStandard (output is not context)", ic.Tier)
	}
	assertKnownMoney(t, "Input", ic.Money.Input, 300_000_000)
	assertKnownMoney(t, "Output", ic.Money.Output, 75_000_000_000) // 5_000_000 x 15.00
}

func TestPriceInvocation_ModelWithoutLongContext_UsesStandardRatesForAllCategories(t *testing.T) {
	table := testTable(flatPricing(t, requestTestModel))

	ic := priceAtDefaultTier(inv(requestTestModel, domain.TokenUsage{
		Input:           domain.Tokens(5_000_000),
		CacheRead:       domain.Tokens(1_000_000),
		CacheCreation:   domain.Tokens(1_000_000),
		CacheCreation5m: domain.Tokens(400_000),
		CacheCreation1h: domain.Tokens(600_000),
		Output:          domain.Tokens(1_000_000),
	}), table)

	if ic.Tier != cost.TierStandard {
		t.Fatalf("Tier = %v, want TierStandard (no long_context block)", ic.Tier)
	}
	assertRequestMoney(t, ic, wantMoney{
		input:      15_000_000_000, // 5M x 3.00
		cacheRead:  300_000_000,    // 1M x 0.30
		cacheWrite: 5_100_000_000,  // 400_000 x 3.75 + 600_000 x 6.00
		output:     15_000_000_000, // 1M x 15.00
	})
}

func TestSumCosts_RequestsTieredIndependently_AllCategories(t *testing.T) {
	// Request A is below the threshold (100_000), request B above (300_000).
	// Tiering the summed context (400_000) would price both at long-context rates.
	table := testTable(longContextPricing(t, 200_000))
	costA := priceAtDefaultTier(inv(requestTestModel, domain.TokenUsage{
		Input: domain.Tokens(100_000), Output: domain.Tokens(1_000),
	}), table)
	costB := priceAtDefaultTier(inv(requestTestModel, domain.TokenUsage{
		Input: domain.Tokens(300_000), Output: domain.Tokens(1_000),
	}), table)

	sum := cost.SumCosts(costA, costB)

	const (
		wantInput  = int64(300_000_000 + 1_800_000_000) // 100_000 x 3.00 + 300_000 x 6.00
		wantOutput = int64(15_000_000 + 22_500_000)     // 1_000 x 15.00 + 1_000 x 22.50
	)
	assertKnownMoney(t, "sum.Input", sum.Input, wantInput)
	assertKnownMoney(t, "sum.Output", sum.Output, wantOutput)
}
