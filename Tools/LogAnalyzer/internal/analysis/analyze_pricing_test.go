package analysis_test

// Tests for per-request pricing and the assumed-tier cache-write count over
// fixture logs.
//
// Fixtures (testdata/tier_*.jsonl), model "tier-model", threshold 200_000:
//
//	run t401  worker#1   usage_records: r1 (context 150K, no split),
//	                     r2 (210K, split), r3 (exactly 200K, no split)
//	run t401  legacy#1   no usage_record; two invocation_end events:
//	                     130K (no split) and 255K (split with a zero 1h part)
//	run t401  orchestrator  two usage_records, no split, both below threshold
//	run t402  worker#2   split request on tier-model; total-only cache write on
//	                     a model that has no price
//	unattributable       one total-only usage_record
//
// Rates per 1M tokens: standard input 3.00 / cache read 0.30 / 5m 3.75 /
// 1h 6.00 / output 15.00; long context 6.00 / 0.60 / 7.50 / 12.00 / 22.50.
// Nanodollars for n tokens at rate r are n * r * 1000.
//
// Rules under test:
//   - Each usage_record, or each legacy usage-bearing event of an actor with no
//     records, is one request and is tiered alone, with all categories repriced.
//   - Cache writes without a split are priced at the 5m rate and counted as
//     assumed; split tokens are never counted.
//   - The assumed count is carried on actor, run, all-runs (named runs only) and
//     unattributable totals. Unpriced models contribute nothing.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
)

const (
	tierRunA = "20261021T100000Z-t401"
	tierRunB = "20261021T110000Z-t402"
)

// loadFixtureStream decodes a testdata fixture into an analysis stream.
func loadFixtureStream(t *testing.T, ref domain.StreamRef, name string) analysis.Stream {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", name)
	events, findings, err := logread.NewReader().ReadAll(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadAll(%s): %v", name, err)
	}
	if len(findings) != 0 {
		t.Fatalf("fixture %s raised findings: %v", name, findings)
	}
	return analysis.Stream{Ref: ref, Events: events}
}

func tierFixtureInput(t *testing.T) analysis.Input {
	t.Helper()
	runA, runB := domain.NamedRun(tierRunA), domain.NamedRun(tierRunB)
	unattributable := domain.StreamRef{
		Run: domain.UnattributableRun(), Kind: domain.StreamAgentInstance, InstanceHint: "stray#1",
	}
	return analysis.Input{Streams: []analysis.Stream{
		loadFixtureStream(t, orchStream(runA), "tier_orchestrator.jsonl"),
		loadFixtureStream(t, agentStream(runA, "worker#1"), "tier_agent_records.jsonl"),
		loadFixtureStream(t, agentStream(runA, "legacy#1"), "tier_agent_legacy.jsonl"),
		loadFixtureStream(t, agentStream(runB, "worker#2"), "tier_run_b_agent.jsonl"),
		loadFixtureStream(t, unattributable, "tier_unattributable.jsonl"),
	}}
}

// fullTierPricing is a model with distinct standard and long-context rates.
func fullTierPricing(t *testing.T, threshold int64) domain.ModelPricing {
	t.Helper()
	p := flatModelPricing(t, tierModel)
	p.HasLongContext = true
	p.LongContext = domain.LongContextPricing{
		Threshold: threshold,
		Rates: domain.RateSet{
			Input:        mustParseRateAnalyze(t, "6.00"),
			CacheRead:    mustParseRateAnalyze(t, "0.60"),
			CacheWrite5m: mustParseRateAnalyze(t, "7.50"),
			CacheWrite1h: mustParseRateAnalyze(t, "12.00"),
			Output:       mustParseRateAnalyze(t, "22.50"),
		},
	}
	return p
}

func analyzeTierFixtures(t *testing.T, table domain.PricingTable) domain.Report {
	t.Helper()
	return analysis.Analyze(tierFixtureInput(t), table, testSource(), nil)
}

func requireRunReport(t *testing.T, r domain.Report, id string) domain.RunReport {
	t.Helper()
	run, ok := r.FindRun(id)
	if !ok {
		t.Fatalf("report has no run %s", id)
	}
	return run
}

func requireActorReport(t *testing.T, run domain.RunReport, id domain.AgentInstanceID) domain.ActorReport {
	t.Helper()
	for _, a := range run.Agents {
		if a.Actor.Instance == id {
			return a
		}
	}
	t.Fatalf("run %s has no actor %s", run.Run.ID, id)
	return domain.ActorReport{}
}

func assertAssumedTokens(t *testing.T, name string, got domain.Totals, want int64) {
	t.Helper()
	if got.AssumedTierCacheWrite != want {
		t.Errorf("%s: AssumedTierCacheWrite = %d, want %d", name, got.AssumedTierCacheWrite, want)
	}
}

// wantCategories asserts the four category money cells of totals.
func wantCategories(t *testing.T, name string, m domain.CategoryMoney, input, cacheRead, cacheWrite, output int64) {
	t.Helper()
	assertMoneyKnown(t, name+" Input", m.Input, input)
	assertMoneyKnown(t, name+" CacheRead", m.CacheRead, cacheRead)
	assertMoneyKnown(t, name+" CacheCreation", m.CacheCreation, cacheWrite)
	assertMoneyKnown(t, name+" Output", m.Output, output)
}

func TestAnalyze_UsageRecordActor_OnlyRequestsCrossingThresholdUseLongContextRates(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{fullTierPricing(t, 200_000)}))

	worker := requireActorReport(t, requireRunReport(t, report, tierRunA), "worker#1")

	// r1 standard (150K), r2 long context (210K), r3 long context (exactly 200K).
	wantCategories(t, "worker#1", worker.Totals.Money,
		1_800_000_000, // 100_000 x 3.00 + 150_000 x 6.00 + 100_000 x 6.00
		60_000_000,    // 20_000 x 0.30 + 40_000 x 0.60 + 50_000 x 0.60
		660_000_000,   // 30_000 x 3.75 + (15_000 x 7.50 + 5_000 x 12.00) + 50_000 x 7.50
		127_500_000,   // 1_000 x 15.00 + 2_000 x 22.50 + 3_000 x 22.50
	)
}

func TestAnalyze_UsageRecordActor_CountsOnlyRequestsWithoutSplitAsAssumed(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{fullTierPricing(t, 200_000)}))

	worker := requireActorReport(t, requireRunReport(t, report, tierRunA), "worker#1")

	// r1 (30_000) and r3 (50_000) carry no split; r2 carries one.
	assertAssumedTokens(t, "worker#1", worker.Totals, 80_000)
}

func TestAnalyze_LegacyActor_EachUsageBearingEventIsOneRequest(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{fullTierPricing(t, 200_000)}))

	legacy := requireActorReport(t, requireRunReport(t, report, tierRunA), "legacy#1")

	// 130K context -> standard, 255K context -> long context. Summed (385K) tiering
	// would price both at long-context rates.
	assertMoneyKnown(t, "legacy#1 Input", legacy.Totals.Money.Input, 1_860_000_000) // 120_000 x 3.00 + 250_000 x 6.00
	assertMoneyKnown(t, "legacy#1 CacheCreation", legacy.Totals.Money.CacheCreation, 75_000_000)
	assertMoneyKnown(t, "legacy#1 Output", legacy.Totals.Money.Output, 23_250_000) // 500 x 15.00 + 700 x 22.50
	assertMoneyState(t, "legacy#1 CacheRead", legacy.Totals.Money.CacheRead, domain.MoneyNoData)
	assertAssumedTokens(t, "legacy#1", legacy.Totals, 10_000)
}

func TestAnalyze_OrchestratorWithoutSplit_CountsAllCacheWritesAsAssumed(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{fullTierPricing(t, 200_000)}))

	orch := requireRunReport(t, report, tierRunA).Orchestrator

	assertMoneyKnown(t, "orchestrator CacheCreation", orch.Totals.Money.CacheCreation, 37_500_000) // 10_000 x 3.75
	assertAssumedTokens(t, "orchestrator", orch.Totals, 10_000)
}

func TestAnalyze_RunTotals_SumAssumedCountOfAllActors(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{fullTierPricing(t, 200_000)}))

	runA := requireRunReport(t, report, tierRunA)

	assertAssumedTokens(t, "run t401", runA.Totals, 100_000) // 80_000 + 10_000 + 10_000
	assertMoneyKnown(t, "run t401 Total", runA.Totals.Money.Total, 4_656_750_000)
}

func TestAnalyze_RunWithOnlySplitAndUnpricedRequests_HasNoAssumedTokens(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{fullTierPricing(t, 200_000)}))

	runB := requireRunReport(t, report, tierRunB)
	worker := requireActorReport(t, runB, "worker#2")

	// The split request is priced per tier; the total-only cache write belongs to a
	// model without a price and is therefore not counted.
	assertAssumedTokens(t, "worker#2", worker.Totals, 0)
	assertAssumedTokens(t, "run t402", runB.Totals, 0)
	if len(worker.UnpricedModels) != 1 || worker.UnpricedModels[0].Model != "unpriced-model" {
		t.Errorf("worker#2 UnpricedModels = %v, want [unpriced-model]", worker.UnpricedModels)
	}
}

func TestAnalyze_AllRuns_SumsAssumedCountOfNamedRunsOnly(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{fullTierPricing(t, 200_000)}))

	assertAssumedTokens(t, "AllRuns", report.AllRuns, 100_000) // t401 100_000 + t402 0; unattributable excluded
}

func TestAnalyze_UnattributableBucket_CarriesItsOwnAssumedCount(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{fullTierPricing(t, 200_000)}))

	if report.Unattributable == nil {
		t.Fatal("report has no unattributable bucket")
	}

	assertAssumedTokens(t, "unattributable", report.Unattributable.Totals, 2_000)
	assertMoneyKnown(t, "unattributable CacheCreation", report.Unattributable.Totals.Money.CacheCreation, 7_500_000)
}

func TestAnalyze_ModelWithoutLongContext_PricesEveryRequestAtStandardRatesAndKeepsAssumedCount(t *testing.T) {
	report := analyzeTierFixtures(t, domain.NewPricingTable([]domain.ModelPricing{flatModelPricing(t, tierModel)}))

	worker := requireActorReport(t, requireRunReport(t, report, tierRunA), "worker#1")

	wantCategories(t, "worker#1", worker.Totals.Money,
		1_050_000_000, // 350_000 x 3.00
		33_000_000,    // 110_000 x 0.30
		386_250_000,   // (30_000 + 15_000 + 50_000) x 3.75 + 5_000 x 6.00
		90_000_000,    // 6_000 x 15.00
	)
	assertAssumedTokens(t, "worker#1", worker.Totals, 80_000)
}

func TestAnalyze_NoPricingTable_NoAssumedCountAnywhere(t *testing.T) {
	report := analyzeTierFixtures(t, domain.EmptyPricingTable())

	assertAssumedTokens(t, "AllRuns", report.AllRuns, 0)
	assertAssumedTokens(t, "run t401", requireRunReport(t, report, tierRunA).Totals, 0)
	assertAssumedTokens(t, "orchestrator", requireRunReport(t, report, tierRunA).Orchestrator.Totals, 0)
	assertAssumedTokens(t, "unattributable", report.Unattributable.Totals, 0)
}
