package analysis_test

// Regression: carrying the new facts must not change any priced report. The
// same run is priced twice, once with every fact-bearing event present and once
// with only the usage-bearing events; the reports must be identical, and the
// totals must match fixed expectations.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// rfUsageEvents are the events that determine usage, views and findings.
func rfUsageEvents() (orch, agent []domain.Event) {
	orch = []domain.Event{
		rfUsageRecord("", "o-1", "priority", rmAt(10), 1000),
		rfUsageRecord("", "o-1", "standard", rmAt(11), 1500),
		rfUsageRecord("", "o-2", "", rmAt(12), 500),
	}
	agent = []domain.Event{
		rmInvStart("w#1", "worker", rmAt(20)),
		invEndEvent("w#1", "model-a", domain.TokenUsage{Input: domain.Tokens(300), Output: domain.Tokens(100)}),
		rmInvEnd("w#1", "success", "model-a", rmAt(40)),
	}
	orch = append(orch, rfRunEnd(rmAt(100), ""))
	return orch, agent
}

// rfFactEvents are events and envelope data that only the new facts read.
func rfFactEvents() (orch, agent []domain.Event) {
	orch = []domain.Event{
		rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "0.9.1", Model: "orch-model", Provider: "anthropic"}),
		rfSessionStart("s1", rmAt(1), domain.SessionStartFields{Resumed: domain.BoolTrue, Model: "m", Provider: "p"}),
		rfTurn(domain.TurnUser, rmAt(2), domain.TokenUsage{}),
		rfNotification(rmAt(3), domain.NotificationFields{NotificationType: "idle", RequiresResponse: domain.BoolFalse}),
		rfSessionEnd("s1", rmAt(90), "user_exit"),
		rfRunEnd(rmAt(100), "completed"),
	}
	agent = []domain.Event{
		rfTurn(domain.TurnUser, rmAt(21), domain.TokenUsage{}),
		rfTurn(domain.TurnAssistant, rmAt(22), domain.TokenUsage{Input: domain.Tokens(9999)}),
		rfNotification(rmAt(23), domain.NotificationFields{NotificationType: "permission_prompt"}),
	}
	return orch, agent
}

func rfPricedReport(t *testing.T, orch, agent []domain.Event) domain.Report {
	t.Helper()
	in := analysis.Input{Streams: []analysis.Stream{
		rfOrch(orch...),
		rfAgent("w#1", agent...),
	}}
	agg, _ := analysis.Aggregate(in)
	table := domain.NewPricingTable([]domain.ModelPricing{flatModelPricing(t, "model-a")})
	report, _ := analysis.Price(agg, table, domain.DefaultAnalysisSettings(), domain.Source{})
	return report
}

// rfIgnoreTimestampSpan returns a copy of the report without the figures that
// legitimately depend on run_start, run_end, session and notification
// timestamps: the timeline axis and the wall-time split.
func rfIgnoreTimestampSpan(r domain.Report) domain.Report {
	runs := make([]domain.RunReport, len(r.Runs))
	copy(runs, r.Runs)
	for i := range runs {
		runs[i].Timeline.Axis = domain.TimeInterval{}
		runs[i].Timeline.HasAxis = false
		runs[i].WallTime = domain.WallTimeSplit{}
	}
	r.Runs = runs
	return r
}

func TestPrice_ReportUnchangedByCarriedFacts(t *testing.T) {
	baseOrch, baseAgent := rfUsageEvents()
	extraOrch, extraAgent := rfFactEvents()
	// Fact events surround the usage events, keeping the relative order of the
	// usage-bearing events identical in both inputs.
	withOrch := append(append([]domain.Event{}, extraOrch[:5]...), baseOrch...)
	withOrch = append(withOrch, extraOrch[5:]...)
	withAgent := append(append([]domain.Event{}, extraAgent...), baseAgent...)

	without := rfPricedReport(t, baseOrch, baseAgent)
	with := rfPricedReport(t, withOrch, withAgent)

	with, without = rfIgnoreTimestampSpan(with), rfIgnoreTimestampSpan(without)

	if !reflect.DeepEqual(with, without) {
		t.Errorf("report changed by carried facts:\n with    %+v\n without %+v", with, without)
	}
}

func TestPrice_CarriedFactsDoNotChangeTotals(t *testing.T) {
	baseOrch, baseAgent := rfUsageEvents()
	extraOrch, extraAgent := rfFactEvents()
	events := append(append([]domain.Event{}, extraOrch...), baseOrch...)
	agentEvents := append(append([]domain.Event{}, extraAgent...), baseAgent...)

	report := rfPricedReport(t, events, agentEvents)

	run, ok := report.FindRun(testRunID)
	if !ok {
		t.Fatalf("run %s missing from report", testRunID)
	}
	// Orchestrator: o-1 superseded (1500) + o-2 (500). Agent: 300 only; its
	// turn token_usage (9999) must stay excluded.
	assertTokenPresent(t, "orchestrator input", run.Orchestrator.Totals.Tokens.Input, 2000)
	if len(run.Agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(run.Agents))
	}
	assertTokenPresent(t, "agent input", run.Agents[0].Totals.Tokens.Input, 300)
	assertTokenPresent(t, "run input", run.Totals.Tokens.Input, 2300)
}
