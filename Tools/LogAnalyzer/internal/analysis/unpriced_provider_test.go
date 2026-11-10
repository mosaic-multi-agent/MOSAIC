package analysis_test

// Tests that unpriced requests are reported per (provider, model) at actor,
// run, report, group, dispatch and across-runs level, and that the by-model
// rows of the run overview are keyed the same way.
//
// Rules under test:
//   - One model logged under two providers with no pricing yields two gaps.
//   - A model priced under one provider and unpriced under another is
//     reported only for the unpriced provider (strict lookup, no fallback).
//   - A request without provider is a gap of its own, listed by bare model id.
//   - A gap prints as "<model> [<provider>]", or the bare model id without a
//     provider.
//
// The unpriced lists are compared through their printed form so that the
// assertions describe the reported gap rather than the Go element type.

import (
	"fmt"
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const gapModel = domain.ModelID("shared-model")

// gapLabels prints every element of an unpriced list (any slice type).
func gapLabels(list any) []string {
	v := reflect.ValueOf(list)
	out := make([]string, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		out = append(out, fmt.Sprint(v.Index(i).Interface()))
	}
	return out
}

func requireGaps(t *testing.T, name string, list any, want ...string) {
	t.Helper()
	got := gapLabels(list)
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: unpriced = %q, want %q", name, got, want)
	}
}

func gapRecord(id domain.AgentInstanceID, recordID string, provider domain.ProviderID) domain.Event {
	ev := usageRecordEvent(id, recordID, gapModel, domain.TokenUsage{
		Input: domain.Tokens(1000), Output: domain.Tokens(100),
	})
	ev.UsageRecord.Provider = provider
	return ev
}

// gapInput: orchestrator uses the model under p1; coder#1 uses it under p1 and
// p2; coder#2 uses it under p2 only; legacy#3 logs it without provider.
func gapInput() analysis.Input {
	run := testRunRef
	agent := func(id domain.AgentInstanceID, events ...domain.Event) analysis.Stream {
		return analysis.Stream{Ref: agentStream(run, id), Events: events}
	}
	return analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(run), Events: []domain.Event{gapRecord("", "o1", "p1")}},
		agent("coder#1",
			rmInvStart("coder#1", "coder", rmAt(10)),
			gapRecord("coder#1", "c1a", "p1"),
			gapRecord("coder#1", "c1b", "p2"),
			rmInvEnd("coder#1", "SUCCESS", gapModel, rmAt(20))),
		agent("coder#2",
			rmInvStart("coder#2", "coder", rmAt(30)),
			gapRecord("coder#2", "c2a", "p2"),
			rmInvEnd("coder#2", "SUCCESS", gapModel, rmAt(40))),
		agent("legacy#3",
			rmInvStart("legacy#3", "coder", rmAt(50)),
			gapRecord("legacy#3", "l3a", ""),
			rmInvEnd("legacy#3", "SUCCESS", gapModel, rmAt(60))),
	}}
}

func analyzeGaps(t *testing.T, table domain.PricingTable) domain.Report {
	t.Helper()
	return analysis.Analyze(gapInput(), table, domain.DefaultAnalysisSettings(), testSource(), nil)
}

func pricedUnder(t *testing.T, provider domain.ProviderID) domain.ModelPricing {
	t.Helper()
	p := flatModelPricing(t, gapModel)
	p.Provider = provider
	return p
}

func gapActor(t *testing.T, run domain.RunReport, id domain.AgentInstanceID) domain.ActorReport {
	t.Helper()
	return requireActorReport(t, run, id)
}

func TestUnpriced_SameModelUnderTwoProviders_IsTwoGapsAtActorLevel(t *testing.T) {
	report := analyzeGaps(t, domain.EmptyPricingTable())
	run := requireRunReport(t, report, testRunID)

	requireGaps(t, "orchestrator", run.Orchestrator.UnpricedModels, "shared-model [p1]")
	requireGaps(t, "coder#1", gapActor(t, run, "coder#1").UnpricedModels, "shared-model [p1]", "shared-model [p2]")
	requireGaps(t, "coder#2", gapActor(t, run, "coder#2").UnpricedModels, "shared-model [p2]")
	requireGaps(t, "legacy#3", gapActor(t, run, "legacy#3").UnpricedModels, "shared-model")
}

func TestUnpriced_SameModelUnderTwoProviders_IsTwoGapsAtRunAndReportLevel(t *testing.T) {
	report := analyzeGaps(t, domain.EmptyPricingTable())
	run := requireRunReport(t, report, testRunID)
	want := []string{"shared-model", "shared-model [p1]", "shared-model [p2]"}

	requireGaps(t, "run", run.UnpricedModels, want...)
	requireGaps(t, "report", report.UnpricedModels, want...)
}

func TestUnpriced_PricedUnderOneProvider_OnlyOtherProviderIsReported(t *testing.T) {
	table := domain.NewPricingTable([]domain.ModelPricing{pricedUnder(t, "p1")})

	report := analyzeGaps(t, table)
	run := requireRunReport(t, report, testRunID)

	requireGaps(t, "orchestrator", run.Orchestrator.UnpricedModels)
	requireGaps(t, "coder#1", gapActor(t, run, "coder#1").UnpricedModels, "shared-model [p2]")
	requireGaps(t, "coder#2", gapActor(t, run, "coder#2").UnpricedModels, "shared-model [p2]")
	requireGaps(t, "run", run.UnpricedModels, "shared-model", "shared-model [p2]")
	requireGaps(t, "report", report.UnpricedModels, "shared-model", "shared-model [p2]")
}

func TestUnpriced_TopLevelEntry_DoesNotPriceProviderRecords(t *testing.T) {
	table := domain.NewPricingTable([]domain.ModelPricing{pricedUnder(t, "")})

	report := analyzeGaps(t, table)

	requireGaps(t, "report", report.UnpricedModels, "shared-model [p1]", "shared-model [p2]")
}

func TestUnpriced_GroupAndDispatch_ListGapsPerProvider(t *testing.T) {
	table := domain.NewPricingTable([]domain.ModelPricing{pricedUnder(t, "p1")})

	run := requireRunReport(t, analyzeGaps(t, table), testRunID)
	group := requireGroup(t, run, "coder")

	requireGaps(t, "group", group.Figures.UnpricedModels, "shared-model", "shared-model [p2]")
	requireGaps(t, "dispatch coder#1", requireDispatch(t, group, "coder#1").UnpricedModels, "shared-model [p2]")
	requireGaps(t, "dispatch coder#2", requireDispatch(t, group, "coder#2").UnpricedModels, "shared-model [p2]")
	requireGaps(t, "dispatch legacy#3", requireDispatch(t, group, "legacy#3").UnpricedModels, "shared-model")
}

func TestUnpriced_DispatchAndActorModels_StayDistinctModelIDs(t *testing.T) {
	run := requireRunReport(t, analyzeGaps(t, domain.EmptyPricingTable()), testRunID)

	coder1 := gapActor(t, run, "coder#1")
	dispatch := requireDispatch(t, requireGroup(t, run, "coder"), "coder#1")

	if !reflect.DeepEqual(coder1.Models, []domain.ModelID{gapModel}) {
		t.Errorf("actor Models = %v, want one distinct model id", coder1.Models)
	}
	if !reflect.DeepEqual(dispatch.Models, []domain.ModelID{gapModel}) {
		t.Errorf("dispatch Models = %v, want one distinct model id", dispatch.Models)
	}
}

func TestUnpriced_AcrossRunsSummary_ListsGapsPerProvider(t *testing.T) {
	report := analyzeGaps(t, domain.NewPricingTable([]domain.ModelPricing{pricedUnder(t, "p1")}))

	if len(report.AcrossRuns.Runs) != 1 {
		t.Fatalf("across-runs summaries = %d, want 1", len(report.AcrossRuns.Runs))
	}
	requireGaps(t, "across runs", report.AcrossRuns.Runs[0].UnpricedModels, "shared-model", "shared-model [p2]")
}

func TestUnpriced_OverviewByModel_OneRowPerProviderWithStrictPricedFlag(t *testing.T) {
	table := domain.NewPricingTable([]domain.ModelPricing{pricedUnder(t, "p1")})

	run := requireRunReport(t, analyzeGaps(t, table), testRunID)
	rows := run.Overview.ByModel

	type row struct {
		provider domain.ProviderID
		priced   bool
		requests int
	}
	var got []row
	for _, r := range rows {
		got = append(got, row{r.Provider, r.Priced, r.Requests})
	}
	// Ordered by model id, then provider ascending with "" first.
	want := []row{{"", false, 1}, {"p1", true, 2}, {"p2", false, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("by-model rows (provider, priced, requests) = %v, want %v", got, want)
	}
}
