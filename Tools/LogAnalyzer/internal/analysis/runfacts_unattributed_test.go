package analysis_test

// The unattributed stream is a usage-only stream of its own. Every consumer of
// the stream kind other than pricing (run facts, sessions, turns,
// notifications, the run model with invocations, tool calls and compactions,
// and the status, tool and timeline views) must neither pick up anything from
// it nor drop or relabel what the other streams hold.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func unattributedUsageStream(extra ...domain.Event) analysis.Stream {
	events := append([]domain.Event{usageRecordEvent("", "u-1", viewsModel, viewsUsage(1000, 0, 0, 100))}, extra...)
	return unattrIn(events...)
}

// factsStreams holds an orchestrator and one agent stream with run, session,
// turn and notification facts.
func factsStreams() []analysis.Stream {
	agent := analysis.Stream{Ref: agentStream(testRunRef, "coder#1"), Events: []domain.Event{
		rmInvStart("coder#1", "coder", rmAt(10)),
		rmToolStart("p1", "Read", rmAt(11)), rmToolEnd("p1", domain.ToolCallSuccess, rmAt(12)),
		rmInvEnd("coder#1", "SUCCESS", viewsModel, rmAt(20)),
	}}
	orch := rfOrch(
		rfRunStart(rmAt(0), domain.RunStartFields{AdapterVersion: "0.9.1", Model: "orch-model", Provider: "anthropic"}),
		rfSessionStart("s-1", rmAt(1), domain.SessionStartFields{}),
		rfTurn(domain.TurnRole("assistant"), rmAt(2), viewsUsage(10, 0, 0, 5)),
		rfNotification(rmAt(3), domain.NotificationFields{}),
		rfSessionEnd("s-1", rmAt(30), "done"),
		rfRunEnd(rmAt(40), "completed"),
	)
	return []analysis.Stream{orch, agent}
}

func withoutUnattributed(run domain.RunAggregate) domain.RunAggregate {
	run.Unattributed = nil
	return run
}

func TestRunConsumers_UnattributedStreamNextToOtherStreams_LeavesFactsAndModelUnchanged(t *testing.T) {
	baseline := rfRun(t, factsStreams()...)
	if baseline.Facts.Outcome != "completed" || len(baseline.Sessions) == 0 || len(baseline.Turns) == 0 ||
		len(baseline.Notifications) == 0 || len(baseline.Activity.Invocations) == 0 {
		t.Fatalf("baseline is missing the facts this test compares: %+v", baseline)
	}

	with := rfRun(t, append(factsStreams(), unattributedUsageStream())...)

	if with.Unattributed == nil {
		t.Fatal("run.Unattributed is nil, want the stream's usage kept")
	}
	if !reflect.DeepEqual(withoutUnattributed(with), withoutUnattributed(baseline)) {
		t.Errorf("run with the unattributed stream differs from the same run without it (beyond the unattributed usage):\nwith:     %+v\nbaseline: %+v", withoutUnattributed(with), withoutUnattributed(baseline))
	}
}

func TestRunConsumers_LoneUnattributedStream_CreatesARunWithEmptyFactsAndModel(t *testing.T) {
	run := rfRun(t, unattributedUsageStream())

	if run.Unattributed == nil {
		t.Fatal("run.Unattributed is nil, want the stream's usage")
	}
	if !reflect.DeepEqual(run.Facts, domain.RunFacts{}) {
		t.Errorf("Facts = %+v, want empty: the stream carries no run facts", run.Facts)
	}
	if len(run.Sessions) != 0 || len(run.Turns) != 0 || len(run.Notifications) != 0 {
		t.Errorf("sessions/turns/notifications = %d/%d/%d, want none", len(run.Sessions), len(run.Turns), len(run.Notifications))
	}
	if len(run.Activity.Invocations) != 0 || len(run.Activity.OrchestratorToolCalls) != 0 || len(run.Activity.Compactions) != 0 {
		t.Errorf("activity = %+v, want no invocations, orchestrator tool calls or compactions", run.Activity)
	}
	if len(run.Agents) != 0 || len(run.Orchestrator.Invocations) != 0 {
		t.Errorf("agents/orchestrator usage = %d/%d, want none: the stream is neither", len(run.Agents), len(run.Orchestrator.Invocations))
	}
}

func TestRunViews_UnattributedStream_LeavesStatusToolsAndTimelineViewsUnchanged(t *testing.T) {
	baseline := viewsScenarioRun(t, viewsPricing(t))

	run := viewsUnattributedRun(t)

	if !reflect.DeepEqual(run.ByStatus, baseline.ByStatus) {
		t.Errorf("ByStatus differs with the unattributed stream:\nwith:     %+v\nbaseline: %+v", run.ByStatus, baseline.ByStatus)
	}
	if !reflect.DeepEqual(run.Tools, baseline.Tools) {
		t.Errorf("Tools differ with the unattributed stream:\nwith:     %+v\nbaseline: %+v", run.Tools, baseline.Tools)
	}
	if !reflect.DeepEqual(run.Timeline, baseline.Timeline) {
		t.Errorf("Timeline differs with the unattributed stream:\nwith:     %+v\nbaseline: %+v", run.Timeline, baseline.Timeline)
	}
	if !reflect.DeepEqual(run.ByAgentType, baseline.ByAgentType) {
		t.Errorf("ByAgentType differs with the unattributed stream")
	}
}

func TestRunViews_LoneUnattributedStream_HasNoStatusToolOrAgentTypeRows(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{unattributedUsageStream()}}

	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	run := requireRunReport(t, report, testRunID)
	if len(run.ByAgentType) != 0 || len(run.ByStatus.Run) != 0 || len(run.Tools.Run) != 0 {
		t.Errorf("by-agent-type/by-status/tools = %d/%d/%d groups, want none for a usage-only stream",
			len(run.ByAgentType), len(run.ByStatus.Run), len(run.Tools.Run))
	}
	if run.Unattributed == nil {
		t.Error("run.Unattributed is nil, want the unattributed actor")
	}
}
