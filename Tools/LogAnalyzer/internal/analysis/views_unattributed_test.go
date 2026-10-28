package analysis_test

// Tests for unattributed usage in pricing and the run views, over the shared
// scenario in views_scenario_test.go plus one unattributed record:
// model m-a, input 1000, output 100 = 1,000,000 + 500,000 = 1,500,000
// nanodollars. The scenario alone totals 8,710,000, so with the record the run
// total is 10,210,000 (orchestrator 2,925,000, subagents 5,785,000,
// unattributed 1,500,000).
//
// Rules under test:
//   - The run report and the all-runs total include the unattributed figures.
//   - The unattributed actor is reported on its own and never folded into the
//     orchestrator or the agent list.
//   - V1 shows the unattributed totals and share; the three shares add up to the
//     run total.
//   - V2 (agent types) is unchanged; unattributed usage is no dispatch.
//   - Runs without unattributed usage report none (nil, zero share).

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func viewsUnattributedInput(extra ...domain.Event) analysis.Input {
	in := viewsScenarioInput()
	events := append([]domain.Event{usageRecordEvent("", "u-1", viewsModel, viewsUsage(1000, 0, 0, 100))}, extra...)
	in.Streams = append(in.Streams, analysis.Stream{Ref: unattrStream(testRunRef), Events: events})
	return in
}

func viewsUnattributedRun(t *testing.T, extra ...domain.Event) domain.RunReport {
	t.Helper()
	report := analysis.Analyze(viewsUnattributedInput(extra...), viewsPricing(t), testSource(), nil)
	return requireRunReport(t, report, testRunID)
}

func TestPrice_UnattributedUsage_IsIncludedInRunAndAllRunsTotals(t *testing.T) {
	report := analysis.Analyze(viewsUnattributedInput(), viewsPricing(t), testSource(), nil)

	run := requireRunReport(t, report, testRunID)

	assertMoneyKnown(t, "run total", run.Totals.Money.Total, 10_210_000)
	assertTokenPresent(t, "run input", run.Totals.Tokens.Input, 6_310)
	assertTokenPresent(t, "run output", run.Totals.Tokens.Output, 620)
	assertMoneyKnown(t, "all-runs total", report.AllRuns.Money.Total, 10_210_000)
}

func TestPrice_UnattributedUsage_IsReportedAsItsOwnActor(t *testing.T) {
	baseline := viewsScenarioRun(t, viewsPricing(t))

	run := viewsUnattributedRun(t)

	if run.Unattributed == nil {
		t.Fatal("run.Unattributed is nil, want the priced unattributed actor")
	}
	if got := run.Unattributed.Actor.Label(); got != "unattributed" {
		t.Errorf("unattributed actor label = %q, want %q", got, "unattributed")
	}
	assertMoneyKnown(t, "unattributed total", run.Unattributed.Totals.Money.Total, 1_500_000)
	assertTokenPresent(t, "unattributed input", run.Unattributed.Totals.Tokens.Input, 1_000)
	if len(run.Unattributed.Models) != 1 || run.Unattributed.Models[0] != viewsModel {
		t.Errorf("unattributed models = %v, want [%s]", run.Unattributed.Models, viewsModel)
	}
	assertMoneyKnown(t, "orchestrator total", run.Orchestrator.Totals.Money.Total, 2_925_000)
	if len(run.Agents) != len(baseline.Agents) {
		t.Errorf("agents = %d, want %d: unattributed usage must not be listed as an agent", len(run.Agents), len(baseline.Agents))
	}
}

func TestPrice_UnattributedUsage_PartsSumToTheRunTotal(t *testing.T) {
	run := viewsUnattributedRun(t)
	if run.Unattributed == nil {
		t.Fatal("run.Unattributed is nil, want the priced unattributed actor")
	}

	var agents int64
	for _, a := range run.Agents {
		agents += a.Totals.Money.Total.Amount.Nanos()
	}
	parts := run.Orchestrator.Totals.Money.Total.Amount.Nanos() + agents + run.Unattributed.Totals.Money.Total.Amount.Nanos()

	if want := run.Totals.Money.Total.Amount.Nanos(); parts != want {
		t.Errorf("orchestrator + agents + unattributed = %d nanos, want the run total %d", parts, want)
	}
}

func TestPrice_UnattributedUsageWithUnpricedModel_MakesTheRunPartialAndNamesTheModel(t *testing.T) {
	in := viewsUnattributedInput(usageRecordEvent("", "u-x", "m-unpriced-unattr", viewsUsage(50, 0, 0, 5)))

	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	run := requireRunReport(t, report, testRunID)
	if run.Unattributed == nil {
		t.Fatal("run.Unattributed is nil")
	}
	assertUnpricedModels(t, "unattributed actor", run.Unattributed.UnpricedModels, "m-unpriced-unattr")
	found := false
	for _, k := range run.UnpricedModels {
		found = found || k.Model == "m-unpriced-unattr"
	}
	if !found {
		t.Errorf("run unpriced models = %v, want m-unpriced-unattr among them", run.UnpricedModels)
	}
	if run.Totals.Money.Complete {
		t.Error("run total is complete, want partial because the unattributed model has no price")
	}
}

func TestOverview_UnattributedUsage_ShowsItsTotalsAndShare(t *testing.T) {
	run := viewsUnattributedRun(t)

	ov := run.Overview

	if ov.Unattributed == nil {
		t.Fatal("overview.Unattributed is nil, want the unattributed totals")
	}
	assertMoneyKnown(t, "overview unattributed", ov.Unattributed.Money.Total, 1_500_000)
	assertTokenPresent(t, "overview unattributed output", ov.Unattributed.Tokens.Output, 100)
	assertRatio(t, "unattributed share", ov.UnattributedShare, 1_500_000, 10_210_000)
}

func TestOverview_UnattributedUsage_SharesCoverTheRunTotal(t *testing.T) {
	ov := viewsUnattributedRun(t).Overview

	assertRatio(t, "orchestrator share", ov.OrchestratorShare, 2_925_000, 10_210_000)
	assertRatio(t, "subagent share", ov.SubagentShare, 5_785_000, 10_210_000)
	sum := ov.OrchestratorShare.Numerator + ov.SubagentShare.Numerator + ov.UnattributedShare.Numerator
	if sum != ov.OrchestratorShare.Denominator {
		t.Errorf("share numerators sum to %d, want the run total %d", sum, ov.OrchestratorShare.Denominator)
	}
	assertMoneyKnown(t, "overview total", ov.Totals.Money.Total, 10_210_000)
	assertMoneyKnown(t, "overview orchestrator", ov.Orchestrator.Money.Total, 2_925_000)
}

func TestOverview_UnattributedUsageOnly_UnattributedShareIsTheWholeRun(t *testing.T) {
	in := unattrInput(unattrIn(usageRecordEvent("", "u-1", viewsModel, viewsUsage(1000, 0, 0, 100))))

	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	ov := requireRunReport(t, report, testRunID).Overview
	assertRatio(t, "unattributed share", ov.UnattributedShare, 1_500_000, 1_500_000)
	assertRatio(t, "orchestrator share", ov.OrchestratorShare, 0, 1_500_000)
}

func TestByAgentType_UnattributedUsage_IsNoDispatchAndLeavesGroupsUnchanged(t *testing.T) {
	baseline := viewsScenarioRun(t, viewsPricing(t))

	run := viewsUnattributedRun(t)

	if len(run.ByAgentType) != len(baseline.ByAgentType) {
		t.Errorf("agent-type groups = %d, want %d", len(run.ByAgentType), len(baseline.ByAgentType))
	}
	if run.Overview.Subagents.Invocations != baseline.Overview.Subagents.Invocations {
		t.Errorf("subagent invocations = %d, want %d", run.Overview.Subagents.Invocations, baseline.Overview.Subagents.Invocations)
	}
	assertMoneyKnown(t, "subagent total", run.Overview.Subagents.Totals.Money.Total, 5_785_000)
	if run.Unattributed == nil {
		t.Error("the by-agent view's unattributed row needs run.Unattributed")
	}
}

func TestOverview_RunWithoutUnattributedUsage_HasNoUnattributedFigures(t *testing.T) {
	report := analysis.Analyze(viewsScenarioInput(), viewsPricing(t), testSource(), nil)

	run := requireRunReport(t, report, testRunID)

	if run.Unattributed != nil {
		t.Errorf("run.Unattributed = %+v, want nil", run.Unattributed)
	}
	if run.Overview.Unattributed != nil {
		t.Errorf("overview.Unattributed = %+v, want nil", run.Overview.Unattributed)
	}
	if run.Overview.UnattributedShare != (domain.Ratio{}) {
		t.Errorf("unattributed share = %+v, want the zero Ratio", run.Overview.UnattributedShare)
	}
	assertMoneyKnown(t, "run total", run.Totals.Money.Total, 8_710_000)
	assertRatio(t, "orchestrator share", run.Overview.OrchestratorShare, 2_925_000, 8_710_000)
}
