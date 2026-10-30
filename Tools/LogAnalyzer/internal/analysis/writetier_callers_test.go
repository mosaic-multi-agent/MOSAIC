package analysis_test

// Cache-write tier in the figures derived from per-call records: unread cache
// writes, cache net benefit, cold-rewrite cost, per-call and compaction-call
// cost, and the per-agent-type totals.
//
// Fixtures are the flat prices of the context helpers: input 3000, cache read
// 300, cache write 3750 (5m) or 6000 (1h) nanodollars per token. Records built
// by ctxCall carry no split, so their cache writes are priced at the tier of
// the record's harness (claude-code here).

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func priceWithTier(t *testing.T, spec ctxRunSpec, tier domain.CacheWriteTier) domain.RunReport {
	t.Helper()
	return ctxPrice(t, ctxAggregate(spec), settingsWithTier(domain.HarnessClaudeCode, tier))
}

func TestWriteTier_UnreadCacheWriteCostUsesTheHarnessTier(t *testing.T) {
	spec := ctxRunSpec{orchUsage: []domain.InvocationUsage{
		ctxCall("o1", 1, 10, 0, 500, 0),
		ctxCall("o2", 2, 10, 0, 70, 0),
	}}

	oneHour := priceWithTier(t, spec, domain.CacheWriteTier1h)
	fiveMin := ctxPriceDefault(t, spec)

	ccAssertMoneyNanos(t, "1h unread write cost", oneHour.Cache.UnreadWriteCost, 70*6000)
	ccAssertMoneyNanos(t, "1h orchestrator unread write cost", oneHour.OrchestratorCache.UnreadWriteCost, 70*6000)
	ccAssertMoneyNanos(t, "5m unread write cost", fiveMin.Cache.UnreadWriteCost, 70*3750)
}

func TestWriteTier_CacheNetBenefitUsesTheHarnessTier(t *testing.T) {
	spec := ctxRunSpec{orchUsage: []domain.InvocationUsage{ctxCall("o1", 1, 10, 0, 1000, 0)}}

	oneHour := priceWithTier(t, spec, domain.CacheWriteTier1h)
	fiveMin := ctxPriceDefault(t, spec)

	// The write costs its tier price over the input price: -(6000-3000) x 1000.
	ccAssertMoneyNanos(t, "1h net benefit", oneHour.Cache.NetBenefit, -3_000_000)
	ccAssertMoneyNanos(t, "5m net benefit", fiveMin.Cache.NetBenefit, -750_000)
}

func TestWriteTier_ColdRewriteCostUsesTheHarnessTier(t *testing.T) {
	spec := ctxRunSpec{orchUsage: []domain.InvocationUsage{
		crCall("r1", 0, "", 10, 0, 100_000),
		crCall("r2", 600, "", 10, 0, 100_000),
	}}

	oneHour := priceWithTier(t, spec, domain.CacheWriteTier1h).Context.Orchestrator
	fiveMin := ctxPriceDefault(t, spec).Context.Orchestrator

	crRequireRewrites(t, oneHour, 1)
	crRequireRewrites(t, fiveMin, 1)
	ccAssertMoneyNanos(t, "1h rewrite cost", oneHour.Rewrites[0].Cost, 100_000*6000)
	ccAssertMoneyNanos(t, "1h rewrite extra cost", oneHour.Rewrites[0].ExtraCost, 100_000*(6000-300))
	ccAssertMoneyNanos(t, "5m rewrite cost", fiveMin.Rewrites[0].Cost, 100_000*3750)
	ccAssertMoneyNanos(t, "5m rewrite extra cost", fiveMin.Rewrites[0].ExtraCost, 100_000*(3750-300))
}

func TestWriteTier_CallCostUsesTheHarnessTier(t *testing.T) {
	spec := ctxRunSpec{orchUsage: []domain.InvocationUsage{ctxCall("o1", 1, 0, 0, 1000, 0)}}

	oneHour := priceWithTier(t, spec, domain.CacheWriteTier1h).Context.Orchestrator
	fiveMin := ctxPriceDefault(t, spec).Context.Orchestrator

	if len(oneHour.Calls) != 1 || len(fiveMin.Calls) != 1 {
		t.Fatalf("calls = %d (1h) and %d (5m), want 1 each", len(oneHour.Calls), len(fiveMin.Calls))
	}
	ccAssertMoneyNanos(t, "1h call cost", oneHour.Calls[0].Cost, 1000*6000)
	ccAssertMoneyNanos(t, "5m call cost", fiveMin.Calls[0].Cost, 1000*3750)
}

func TestWriteTier_CompactionCallCostUsesTheHarnessTier(t *testing.T) {
	call := crWithKind(ctxCall("k1", 1, 0, 0, 1000, 0), domain.CallCompaction)
	spec := ctxRunSpec{orchUsage: []domain.InvocationUsage{call}}

	oneHour := priceWithTier(t, spec, domain.CacheWriteTier1h).Context.Orchestrator
	fiveMin := ctxPriceDefault(t, spec).Context.Orchestrator

	ccAssertMoneyNanos(t, "1h compaction-call cost", oneHour.CompactionCalls.Cost, 1000*6000)
	ccAssertMoneyNanos(t, "1h unapplied compaction-call cost", oneHour.CompactionCalls.UnappliedCost, 1000*6000)
	ccAssertMoneyNanos(t, "5m compaction-call cost", fiveMin.CompactionCalls.Cost, 1000*3750)
}

func TestWriteTier_AgentTypeTotalsUseTheHarnessTier(t *testing.T) {
	spec := ctxRunSpec{agents: []ctxAgent{{
		id: "coder#1", typ: "coder",
		usage: []domain.InvocationUsage{ctxCall("c1", 1, 10, 0, 250, 0)},
	}}}

	oneHour := priceWithTier(t, spec, domain.CacheWriteTier1h)
	fiveMin := ctxPriceDefault(t, spec)

	ccAssertMoneyNanos(t, "1h agent-type CacheCreation", rwTypeGroup(t, oneHour, "coder").Figures.Totals.Money.CacheCreation, 250*6000)
	ccAssertMoneyNanos(t, "5m agent-type CacheCreation", rwTypeGroup(t, fiveMin, "coder").Figures.Totals.Money.CacheCreation, 250*3750)
}

func TestWriteTier_SplitRecordsAreUnchangedBySetting(t *testing.T) {
	split := ctxCall("o1", 1, 10, 0, 1000, 0)
	split.Usage.CacheCreation5m = domain.Tokens(400)
	split.Usage.CacheCreation1h = domain.Tokens(600)
	spec := ctxRunSpec{orchUsage: []domain.InvocationUsage{split}}

	oneHour := priceWithTier(t, spec, domain.CacheWriteTier1h)
	fiveMin := ctxPriceDefault(t, spec)

	want := int64(400*3750 + 600*6000)
	ccAssertMoneyNanos(t, "1h setting unread write cost", oneHour.Cache.UnreadWriteCost, want)
	ccAssertMoneyNanos(t, "default unread write cost", fiveMin.Cache.UnreadWriteCost, want)
}
