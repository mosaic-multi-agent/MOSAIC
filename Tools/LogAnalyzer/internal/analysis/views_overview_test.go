package analysis_test

// Tests for the run overview figures (RunReport.Overview) over the shared
// scenario in views_scenario_test.go.
//
// Rules under test:
//   - Totals and the orchestrator figure equal the run and orchestrator totals;
//     subagent figures cover every dispatch, the orchestrator never.
//   - Shares are exact nanodollar ratios; they are undefined (denominator 0)
//     when the run total is not a known amount.
//   - The model breakdown is sorted, counts requests and marks unpriced models.
//   - The cache hit ratio is cache-read tokens over total context tokens, from
//     logged token counts only.
//   - Incompleteness (unpriced, assumed tier, missing usage, open) is carried on
//     the affected figures.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func TestOverview_TotalsAndOrchestratorMatchRunFigures(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	ov := run.Overview

	assertMoneyKnown(t, "overview total", ov.Totals.Money.Total, 8_710_000)
	if ov.Totals.Money.Complete {
		t.Error("overview total is complete, want partial because m-unpriced has no price")
	}
	assertTokenPresent(t, "overview input", ov.Totals.Tokens.Input, 5_310)
	assertMoneyKnown(t, "orchestrator total", ov.Orchestrator.Money.Total, 2_925_000)
	assertTokenPresent(t, "orchestrator output", ov.Orchestrator.Tokens.Output, 200)
}

func TestOverview_ByTokenCategory_MoneyPerCategoryIsExact(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	wantCategories(t, "overview", run.Overview.Totals.Money, 4_610_000, 400_000, 1_450_000, 2_250_000)
}

func TestOverview_Shares_AreExactNanodollarRatios(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	assertRatio(t, "orchestrator share", run.Overview.OrchestratorShare, 2_925_000, 8_710_000)
	assertRatio(t, "subagent share", run.Overview.SubagentShare, 5_785_000, 8_710_000)
}

func TestOverview_Shares_UndefinedWhenNothingIsPriced(t *testing.T) {
	run := viewsScenarioRun(t, domain.EmptyPricingTable())

	if run.Overview.Subagents.Invocations != 6 {
		t.Fatalf("subagent invocations = %d, want 6 (overview not computed)", run.Overview.Subagents.Invocations)
	}
	if run.Overview.OrchestratorShare.Denominator != 0 || run.Overview.SubagentShare.Denominator != 0 {
		t.Errorf("shares = %+v / %+v, want undefined (denominator 0) without any pricing",
			run.Overview.OrchestratorShare, run.Overview.SubagentShare)
	}
}

func TestOverview_CacheHitRatio_IsCacheReadOverTotalContext(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	// cache read 4,000 over input 5,310 + cache read 4,000 + cache write 1,100.
	assertRatio(t, "cache hit ratio", run.Overview.CacheHitRatio, 4_000, 10_410)
}

func TestOverview_CacheHitRatio_UndefinedWithoutLoggedTokens(t *testing.T) {
	in := oneAgentStream(testRunRef, "coder#1", rmInvStart("coder#1", "coder", rmAt(1)))
	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	run := requireRunReport(t, report, testRunID)

	if run.Overview.Subagents.Invocations != 1 {
		t.Fatalf("subagent invocations = %d, want 1 (overview not computed)", run.Overview.Subagents.Invocations)
	}
	if run.Overview.CacheHitRatio.Denominator != 0 {
		t.Errorf("cache hit ratio = %+v, want undefined for a run with no token counts", run.Overview.CacheHitRatio)
	}
}

func TestOverview_ByModel_SortedWithRequestCountsAndPricedFlag(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	models := run.Overview.ByModel
	if len(models) != 2 {
		t.Fatalf("got %d model entries, want 2: %+v", len(models), models)
	}
	priced, unpriced := models[0], models[1]
	if priced.Model != viewsModel || !priced.Priced || priced.Requests != 6 {
		t.Errorf("first entry = %+v, want priced m-a with 6 requests", priced)
	}
	assertMoneyKnown(t, "m-a total", priced.Totals.Money.Total, 8_710_000)
	assertTokenPresent(t, "m-a cache write", priced.Totals.Tokens.CacheCreation, 1_100)
	if unpriced.Model != viewsUnpriced || unpriced.Priced || unpriced.Requests != 1 {
		t.Errorf("second entry = %+v, want unpriced m-unpriced with 1 request", unpriced)
	}
	assertMoneyState(t, "m-unpriced total", unpriced.Totals.Money.Total, domain.MoneyUnpriced)
	assertTokenPresent(t, "m-unpriced input", unpriced.Totals.Tokens.Input, 700)
}

func TestOverview_ByModel_RequestWithoutModelIsListedLast(t *testing.T) {
	in := oneAgentStream(testRunRef, "coder#1",
		usageRecordEvent("coder#1", "r1", "", viewsUsage(10, 0, 0, 0)),
		usageRecordEvent("coder#1", "r2", viewsModel, viewsUsage(20, 0, 0, 0)))
	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	models := requireRunReport(t, report, testRunID).Overview.ByModel

	if len(models) != 2 || models[0].Model != viewsModel || models[1].Model != "" {
		t.Fatalf("by-model order = %+v, want m-a then the no-model entry", models)
	}
	if models[1].Priced || models[1].Requests != 1 {
		t.Errorf("no-model entry = %+v, want unpriced with 1 request", models[1])
	}
}

func TestOverview_Subagents_CoverAllDispatchesAndCarryIncompleteness(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	sub := run.Overview.Subagents

	if sub.Invocations != 6 {
		t.Errorf("subagent invocations = %d, want 6", sub.Invocations)
	}
	assertMoneyKnown(t, "subagent total", sub.Totals.Money.Total, 5_785_000)
	if sub.Totals.Money.Complete {
		t.Error("subagent total is complete, want partial")
	}
	assertUnpricedModels(t, "subagents", sub.UnpricedModels, viewsUnpriced)
	if sub.MissingUsage != 1 || sub.Open != 1 {
		t.Errorf("missing usage = %d, open = %d, want 1 and 1", sub.MissingUsage, sub.Open)
	}
	if sub.Totals.AssumedTierCacheWrite != 200 {
		t.Errorf("subagent assumed-tier tokens = %d, want 200 (tester#4 only)", sub.Totals.AssumedTierCacheWrite)
	}
}

func TestOverview_AssumedTier_RunCountIncludesOrchestratorAndSubagents(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	assertAssumedTokens(t, "overview totals", run.Overview.Totals, 700)
	assertAssumedTokens(t, "orchestrator", run.Overview.Orchestrator, 500)
	for _, m := range run.Overview.ByModel {
		if m.Model == viewsModel && m.Totals.AssumedTierCacheWrite != 700 {
			t.Errorf("m-a assumed-tier tokens = %d, want 700", m.Totals.AssumedTierCacheWrite)
		}
	}
}

func TestOverview_Repricing_RecomputesFromTheSameAggregate(t *testing.T) {
	agg, _ := analysis.Aggregate(viewsScenarioInput())

	unpriced, _ := analysis.Price(agg, domain.EmptyPricingTable(), testSource())
	priced, _ := analysis.Price(agg, viewsPricing(t), testSource())

	before := requireRunReport(t, unpriced, testRunID).Overview
	after := requireRunReport(t, priced, testRunID).Overview
	assertMoneyState(t, "unpriced run total", before.Totals.Money.Total, domain.MoneyUnpriced)
	assertMoneyKnown(t, "priced run total", after.Totals.Money.Total, 8_710_000)
}

func TestOverview_UnattributableBucket_CarriesItsOwnOverview(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{{
		Ref:    agentStream(domain.UnattributableRun(), "Unknown#1"),
		Events: []domain.Event{usageRecordEvent("Unknown#1", "r1", viewsModel, viewsUsage(100, 0, 0, 0))},
	}}}

	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	if report.Unattributable == nil {
		t.Fatal("no unattributable report")
	}
	assertMoneyKnown(t, "unattributable overview total", report.Unattributable.Overview.Totals.Money.Total, 100_000)
}
