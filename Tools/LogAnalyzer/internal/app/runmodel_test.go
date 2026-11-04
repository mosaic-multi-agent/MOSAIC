package app_test

// Tests that the run model is reachable through the app layer:
//   - report path: Result.Session.Aggregate().Runs[i].Activity
//   - total path: RunTotal.Activity, including streams merged from the
//     unknown-run bucket
//   - after re-pricing from the session, without re-reading any log.

import (
	"context"
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

var rmTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func rmToolStartEvent(callID, tool string, sec int) domain.Event {
	return domain.Event{
		Type:          domain.EventToolCallStart,
		Timestamp:     rmTime.Add(time.Duration(sec) * time.Second),
		ToolCallStart: &domain.ToolCallStartFields{CallID: callID, ToolName: tool},
	}
}

func rmToolEndEvent(callID string, status domain.ToolCallStatus, sec int) domain.Event {
	return domain.Event{
		Type:        domain.EventToolCallEnd,
		Timestamp:   rmTime.Add(time.Duration(sec) * time.Second),
		ToolCallEnd: &domain.ToolCallEndFields{CallID: callID, Status: status},
	}
}

func rmInvStartEvent(id domain.AgentInstanceID, agentType string, sec int) domain.Event {
	return domain.Event{
		Type:            domain.EventInvocationStart,
		Timestamp:       rmTime.Add(time.Duration(sec) * time.Second),
		InvocationStart: &domain.InvocationStartFields{AgentInstanceID: id, AgentType: agentType},
	}
}

func rmCompactionEvent(trigger string, sec int) domain.Event {
	return domain.Event{
		Type:       domain.EventCompaction,
		Timestamp:  rmTime.Add(time.Duration(sec) * time.Second),
		Compaction: &domain.CompactionFields{Trigger: trigger, TokensBefore: domain.Tokens(100), TokensAfter: domain.Tokens(10)},
	}
}

func rmCallIDs(calls []domain.ToolCall) map[string]domain.ToolCall {
	out := make(map[string]domain.ToolCall, len(calls))
	for _, c := range calls {
		out[c.CallID] = c
	}
	return out
}

func rmFindInvocation(t *testing.T, act domain.RunActivity, id domain.AgentInstanceID) domain.Invocation {
	t.Helper()
	for _, inv := range act.Invocations {
		if inv.Actor.Instance == id {
			return inv
		}
	}
	t.Fatalf("no invocation for %q among %d invocations", id, len(act.Invocations))
	return domain.Invocation{}
}

// rmOneRunService builds a service over one named run with an orchestrator
// stream and the two standard agent streams.
func rmOneRunService() (*app.Service, *fakeEventReader) {
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/test-logs"}
	logSource := &fakeLogSource{
		classifyFunc:  func(string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) { return oneRunTwoAgentInventory(s), nil },
	}
	reader := newFakeEventReader()
	reader.addEvents(appTestOrchFile,
		rmToolStartEvent("o1", "Read", 1), rmToolEndEvent("o1", domain.ToolCallSuccess, 2),
		rmCompactionEvent("auto", 50),
		appTestRunEndEvent())
	reader.addEvents(appTestAgent1File,
		rmInvStartEvent("TestAgent#1", "coder", 3),
		rmToolStartEvent("a1", "Bash", 4), rmToolEndEvent("a1", domain.ToolCallError, 6),
		appTestInvEndEvent("TestAgent#1", "model-a", domain.TokenUsage{Input: domain.Tokens(1000)}))
	reader.addEvents(appTestAgent2File,
		rmInvStartEvent("TestAgent#2", "tester", 3),
		rmToolStartEvent("b1", "Grep", 5)) // never ended; invocation never ends
	store := newFakePricingStore()
	store.table = domain.NewPricingTable([]domain.ModelPricing{flatPricing("model-a")})
	svc := app.New(app.Deps{
		Source: logSource, Reader: reader, Pricing: store, Clock: newFakeClock(),
		Interaction: &alwaysSkippingInteraction{}, WorkDir: "/workdir",
	})
	return svc, reader
}

func TestAnalyze_RunModelReachableFromSessionAggregate(t *testing.T) {
	svc, _ := rmOneRunService()

	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})

	if err != nil || result.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze = (%v, %v), want report outcome without error", result.Outcome, err)
	}
	runs := result.Session.Aggregate().Runs
	if len(runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(runs))
	}
	act := runs[0].Activity
	if c, ok := rmCallIDs(act.OrchestratorToolCalls)["o1"]; !ok || c.Pairing != domain.ToolCallPaired || c.ToolName != "Read" {
		t.Errorf("orchestrator calls = %+v, want one paired Read call o1", act.OrchestratorToolCalls)
	}
	one := rmFindInvocation(t, act, "TestAgent#1")
	if one.Actor.Type != "coder" || one.State != domain.InvocationClosed || len(one.Usage) != 1 {
		t.Errorf("TestAgent#1 = %+v, want closed coder with one usage entry", one)
	}
	if c := rmCallIDs(one.ToolCalls)["a1"]; c.Status != domain.ToolCallError || c.Pairing != domain.ToolCallPaired {
		t.Errorf("TestAgent#1 call a1 = %+v, want paired error", c)
	}
	two := rmFindInvocation(t, act, "TestAgent#2")
	if two.State != domain.InvocationOpen {
		t.Errorf("TestAgent#2 State = %v, want InvocationOpen", two.State)
	}
	if c := rmCallIDs(two.ToolCalls)["b1"]; c.Pairing != domain.ToolCallStartOnly {
		t.Errorf("TestAgent#2 call b1 = %+v, want start-only", c)
	}
	if len(act.Compactions) != 1 || act.Compactions[0].Actor.Kind != domain.ActorOrchestrator {
		t.Errorf("compactions = %+v, want one orchestrator compaction", act.Compactions)
	}
}

func TestSessionReprice_KeepsRunModelWithoutReReadingLogs(t *testing.T) {
	svc, reader := rmOneRunService()
	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	before := result.Session.Aggregate().Runs[0].Activity
	if len(before.Invocations) == 0 {
		t.Fatal("precondition: the run model must be populated before re-pricing")
	}
	readsBefore := reader.readCounts[appTestOrchFile] + reader.readCounts[appTestAgent1File] + reader.readCounts[appTestAgent2File]

	report := result.Session.Reprice(flatPricing("model-new"))

	readsAfter := reader.readCounts[appTestOrchFile] + reader.readCounts[appTestAgent1File] + reader.readCounts[appTestAgent2File]
	if readsAfter != readsBefore {
		t.Errorf("Reprice re-read logs: %d reads before, %d after", readsBefore, readsAfter)
	}
	if len(report.Runs) != 1 {
		t.Fatalf("repriced report has %d runs, want 1", len(report.Runs))
	}
	after := result.Session.Aggregate().Runs[0].Activity
	if !reflect.DeepEqual(before, after) {
		t.Errorf("run model changed across Reprice:\n before %+v\n after  %+v", before, after)
	}
}

func TestTotalForRun_SingleRunWithoutUnknownBucket_ActivityEqualsSessionActivity(t *testing.T) {
	svc, _ := rmOneRunService()
	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	sessionAct := result.Session.Aggregate().Runs[0].Activity
	if len(sessionAct.Invocations) == 0 {
		t.Fatal("precondition: the session run model must be populated")
	}

	total, outcome, err := svc.TotalForRun(context.Background(), app.Request{ExplicitPath: "/test-logs"}, appTestRunID)

	if err != nil || outcome != app.OutcomeReport {
		t.Fatalf("TotalForRun = (%v, %v), want report outcome without error", outcome, err)
	}
	if !reflect.DeepEqual(total.Activity, sessionAct) {
		t.Errorf("total activity differs from session activity:\n total   %+v\n session %+v", total.Activity, sessionAct)
	}
}

// rmMergeService builds a single-run source plus an unknown-run bucket whose
// orchestrator and agent streams carry run-model events.
func rmMergeService() *app.Service {
	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/logs"}
	inv := mergeTestInventoryOneRunWithUnknown(src)
	inv.Unattributable.Agents = []domain.AgentEntry{{
		Dir: "/logs/unknown-run/Agent#1", InstanceHint: "Agent#1", EventFile: mergeTestUnknownAgent1File,
	}}
	logSource := &fakeLogSource{
		classifyFunc:  func(string) domain.Source { return src },
		enumerateFunc: func(domain.Source) (domain.Inventory, []domain.Finding) { return inv, nil },
	}
	reader := newFakeEventReader()
	reader.addEvents(mergeTestOrchFile,
		rmToolStartEvent("named-o", "Read", 1), rmToolEndEvent("named-o", domain.ToolCallSuccess, 2),
		appTestRunEndEvent())
	reader.addEvents(mergeTestUnknownOrchFile,
		rmToolStartEvent("unknown-o", "Bash", 3), rmToolEndEvent("unknown-o", domain.ToolCallError, 4),
		rmCompactionEvent("manual", 9))
	reader.addEvents(mergeTestUnknownAgent1File,
		rmInvStartEvent("Agent#1", "coder", 5),
		rmToolStartEvent("unknown-a", "Grep", 6))
	store := newFakePricingStore()
	return app.New(app.Deps{
		Source: logSource, Reader: reader, Pricing: store, Clock: newFakeClock(),
		Interaction: &alwaysSkippingInteraction{}, WorkDir: "/workdir",
	})
}

func TestTotalForRun_RunModelIncludesStreamsMergedFromUnknownRun(t *testing.T) {
	svc := rmMergeService()

	total, outcome, err := svc.TotalForRun(context.Background(), app.Request{ExplicitPath: "/logs"}, mergeTestRunID)

	if err != nil || outcome != app.OutcomeReport {
		t.Fatalf("TotalForRun = (%v, %v), want report outcome without error", outcome, err)
	}
	calls := rmCallIDs(total.Activity.OrchestratorToolCalls)
	if len(calls) != 2 || calls["named-o"].Pairing != domain.ToolCallPaired || calls["unknown-o"].Status != domain.ToolCallError {
		t.Errorf("orchestrator calls = %+v, want named-o and merged unknown-o, both paired", total.Activity.OrchestratorToolCalls)
	}
	inv := rmFindInvocation(t, total.Activity, "Agent#1")
	if inv.Actor.Type != "coder" || rmCallIDs(inv.ToolCalls)["unknown-a"].Pairing != domain.ToolCallStartOnly {
		t.Errorf("merged invocation = %+v, want coder with start-only call unknown-a", inv)
	}
	if len(total.Activity.Compactions) != 1 || total.Activity.Compactions[0].Trigger != "manual" {
		t.Errorf("compactions = %+v, want the merged manual compaction", total.Activity.Compactions)
	}
}

func TestAnalyze_UnknownRunBucketRunModel_StaysOutOfNamedRun(t *testing.T) {
	svc := rmMergeService()

	result, err := svc.Analyze(context.Background(), app.Request{ExplicitPath: "/logs"})

	if err != nil || result.Outcome != app.OutcomeReport {
		t.Fatalf("Analyze = (%v, %v), want report outcome without error", result.Outcome, err)
	}
	agg := result.Session.Aggregate()
	if agg.Unattributable == nil {
		t.Fatal("Unattributable bucket is nil on the report path")
	}
	if _, ok := rmCallIDs(agg.Unattributable.Activity.OrchestratorToolCalls)["unknown-o"]; !ok {
		t.Errorf("unattributable calls = %+v, want unknown-o", agg.Unattributable.Activity.OrchestratorToolCalls)
	}
	named := agg.Runs[0].Activity
	if _, ok := rmCallIDs(named.OrchestratorToolCalls)["unknown-o"]; ok || len(named.Compactions) != 0 {
		t.Errorf("named run activity = %+v, want nothing from the unknown-run bucket", named)
	}
	if _, ok := rmCallIDs(named.OrchestratorToolCalls)["named-o"]; !ok {
		t.Errorf("named run calls = %+v, want named-o", named.OrchestratorToolCalls)
	}
}
