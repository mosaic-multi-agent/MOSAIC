package analysis_test

// Rule-level tests for the run views that the shared scenario cannot pin on its
// own: rounding of the per-invocation average, the full status ordering, the
// share numerator rule, cache hit ratio with absent cache reads, orchestrator
// tool errors, unknown-type ordering in the tools view, per-dispatch model
// lists, and the views of the unattributable bucket.

import (
	"fmt"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// oneNanoPricing prices m-a input at exactly one nanodollar per token.
func oneNanoPricing(t *testing.T) domain.PricingTable {
	t.Helper()
	return domain.NewPricingTable([]domain.ModelPricing{{
		Model:    viewsModel,
		Standard: domain.RateSet{Input: mustParseRateAnalyze(t, "0.001")},
	}})
}

func coderInvocation(id domain.AgentInstanceID, status string, at int, events ...domain.Event) analysis.Stream {
	all := []domain.Event{rmInvStart(id, "coder", rmAt(at))}
	all = append(all, events...)
	all = append(all, rmInvEnd(id, status, viewsModel, rmAt(at+1)))
	return analysis.Stream{Ref: agentStream(testRunRef, id), Events: all}
}

func TestByAgentType_Average_RoundsHalfUpToTheNanodollar(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		coderInvocation("coder#1", "SUCCESS", 1, usageRecordEvent("coder#1", "a", viewsModel, viewsUsage(1_000_001, 0, 0, 0))),
		coderInvocation("coder#2", "SUCCESS", 5, usageRecordEvent("coder#2", "b", viewsModel, viewsUsage(1_000_002, 0, 0, 0))),
	}}

	report := analysis.Analyze(in, oneNanoPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	stats := requireGroup(t, requireRunReport(t, report, testRunID), "coder").PerInvocation
	assertMoneyKnown(t, "min", stats.Min, 1_000_001)
	assertMoneyKnown(t, "max", stats.Max, 1_000_002)
	// The exact mean is 1,000,001.5 nanodollars; half-up gives 1,000,002.
	assertMoneyKnown(t, "average", stats.Average, 1_000_002)
}

func TestByAgentType_Average_RoundsFractionalMeanToNearest(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		coderInvocation("coder#1", "SUCCESS", 1, usageRecordEvent("coder#1", "a", viewsModel, viewsUsage(1, 0, 0, 0))),
		coderInvocation("coder#2", "SUCCESS", 5, usageRecordEvent("coder#2", "b", viewsModel, viewsUsage(2, 0, 0, 0))),
		coderInvocation("coder#3", "SUCCESS", 9, usageRecordEvent("coder#3", "c", viewsModel, viewsUsage(2, 0, 0, 0))),
	}}

	report := analysis.Analyze(in, oneNanoPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	// Exact mean 5/3 = 1.67 nanodollars: nearest is 2, truncation would give 1.
	assertMoneyKnown(t, "average", requireGroup(t, requireRunReport(t, report, testRunID), "coder").PerInvocation.Average, 2)
}

func TestByStatus_Run_FullEnumOrderThenOtherCodesAscending(t *testing.T) {
	codes := []string{
		"ZED", "BLOCKED", "CAPABILITY_EXCEEDED", "ALPHA", "NEEDS_CLARIFICATION",
		"PARTIALLY_DONE", "COMPLETED_NEEDS_ACTION", "SUCCESS", "",
	}
	var streams []analysis.Stream
	for i, code := range codes {
		id := domain.AgentInstanceID(fmt.Sprintf("coder#%d", i+1))
		streams = append(streams, coderInvocation(id, code, 1+i*3))
	}

	report := analysis.Analyze(analysis.Input{Streams: streams}, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	run := requireRunReport(t, report, testRunID)
	want := []wantGroup{
		{domain.StatusReported, "SUCCESS", 1},
		{domain.StatusReported, "COMPLETED_NEEDS_ACTION", 1},
		{domain.StatusReported, "PARTIALLY_DONE", 1},
		{domain.StatusReported, "NEEDS_CLARIFICATION", 1},
		{domain.StatusReported, "CAPABILITY_EXCEEDED", 1},
		{domain.StatusReported, "BLOCKED", 1},
		{domain.StatusReported, "ALPHA", 1},
		{domain.StatusReported, "ZED", 1},
		{domain.StatusNone, "", 1},
	}
	assertStatusGroups(t, "run", run.ByStatus.Run, want)
	assertStatusGroups(t, "coder", run.ByStatus.ByAgentType[0].Groups, want)
}

func TestOverview_Shares_NumeratorIsZeroForAnOrchestratorTotalThatIsNotKnown(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{
			usageRecordEvent("", "o", viewsUnpriced, viewsUsage(500, 0, 0, 0)),
		}},
		coderInvocation("coder#1", "SUCCESS", 1, usageRecordEvent("coder#1", "a", viewsModel, viewsUsage(1000, 0, 0, 0))),
	}}

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	ov := requireRunReport(t, report, testRunID).Overview
	assertMoneyState(t, "orchestrator total", ov.Orchestrator.Money.Total, domain.MoneyUnpriced)
	assertRatio(t, "orchestrator share", ov.OrchestratorShare, 0, 1_000_000)
	assertRatio(t, "subagent share", ov.SubagentShare, 1_000_000, 1_000_000)
}

func TestOverview_CacheHitRatio_AbsentCacheReadCountsAsZeroOverLoggedContext(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		coderInvocation("coder#1", "SUCCESS", 1, usageRecordEvent("coder#1", "a", viewsModel, viewsUsage(100, 0, 0, 10))),
	}}

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	assertRatio(t, "cache hit ratio", requireRunReport(t, report, testRunID).Overview.CacheHitRatio, 0, 100)
}

func TestByAgentType_Dispatch_ModelsAreDistinctSortedAndUnpricedOnesListed(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		coderInvocation("coder#1", "SUCCESS", 1,
			usageRecordEvent("coder#1", "a", viewsUnpriced, viewsUsage(10, 0, 0, 0)),
			usageRecordEvent("coder#1", "b", viewsModel, viewsUsage(10, 0, 0, 0)),
			usageRecordEvent("coder#1", "c", viewsModel, viewsUsage(20, 0, 0, 0))),
	}}

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	d := requireDispatch(t, requireGroup(t, requireRunReport(t, report, testRunID), "coder"), "coder#1")
	if len(d.Models) != 2 || d.Models[0] != viewsModel || d.Models[1] != viewsUnpriced {
		t.Errorf("models = %v, want [m-a m-unpriced]", d.Models)
	}
	assertUnpricedModels(t, "coder#1", d.UnpricedModels, viewsUnpriced)
}

func TestTools_Orchestrator_ErrorsAreCounted(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{{Ref: orchStream(testRunRef), Events: []domain.Event{
		rmToolStart("o1", "Read", rmAt(1)), rmToolEnd("o1", domain.ToolCallError, rmAt(2)),
		rmToolStart("o2", "Read", rmAt(3)), rmToolEnd("o2", domain.ToolCallSuccess, rmAt(4)),
	}}}}

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	read := requireTool(t, requireRunReport(t, report, testRunID).Tools.Orchestrator, "Read")
	if read.Calls != 2 || read.Errors != 1 {
		t.Errorf("orchestrator Read = %+v, want 2 calls and 1 error", read)
	}
}

func TestTools_PerAgentType_UnknownTypeIsListedLast(t *testing.T) {
	untyped := analysis.Stream{Ref: agentStream(testRunRef, "x#7"), Events: []domain.Event{
		rmToolStart("u1", "Bash", rmAt(1)), rmToolEnd("u1", domain.ToolCallSuccess, rmAt(2)),
		rmInvEnd("x#7", "SUCCESS", viewsModel, rmAt(3)),
	}}
	in := analysis.Input{Streams: []analysis.Stream{
		untyped,
		coderInvocation("coder#1", "SUCCESS", 10,
			rmToolStart("c1", "Read", rmAt(11)), rmToolEnd("c1", domain.ToolCallSuccess, rmAt(12))),
	}}

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	byType := requireRunReport(t, report, testRunID).Tools.ByAgentType
	if len(byType) != 2 || byType[0].AgentType != "coder" || byType[1].AgentType != "" {
		t.Fatalf("by-agent-type tools = %+v, want coder then the unknown type", byType)
	}
	if got := toolNames(byType[1].Tools); !rmEqualStrings(got, []string{"Bash"}) {
		t.Errorf("unknown-type tools = %q, want Bash", got)
	}
}

func TestUnattributableBucket_CarriesAllRunViews(t *testing.T) {
	ref := agentStream(domain.UnattributableRun(), "Unknown#1")
	in := analysis.Input{Streams: []analysis.Stream{{Ref: ref, Events: []domain.Event{
		rmInvStart("Unknown#1", "coder", rmAt(1)),
		rmToolStart("u1", "Read", rmAt(2)), rmToolEnd("u1", domain.ToolCallSuccess, rmAt(3)),
		usageRecordEvent("Unknown#1", "u", viewsModel, viewsUsage(100, 0, 0, 0)),
		rmInvEnd("Unknown#1", "SUCCESS", viewsModel, rmAt(4)),
	}}}}

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	if report.Unattributable == nil {
		t.Fatal("no unattributable report")
	}
	u := *report.Unattributable
	if len(u.ByAgentType) != 1 || u.ByAgentType[0].AgentType != "coder" {
		t.Errorf("by-agent-type = %+v, want one coder group", u.ByAgentType)
	}
	assertStatusGroups(t, "by-status", u.ByStatus.Run, []wantGroup{{domain.StatusReported, "SUCCESS", 1}})
	if got := toolNames(u.Tools.Run); !rmEqualStrings(got, []string{"Read"}) {
		t.Errorf("tools = %q, want Read", got)
	}
}
