package analysis_test

// Run-model tests for tool calls: pairing by call_id within one stream,
// attribution to the actor whose stream logged the call, and explicit
// unpaired states. Ambiguity stays visible; nothing is guessed.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func TestRunModel_OrchestratorToolCalls_PairWithStatusAndTimestamps(t *testing.T) {
	in := rmOrchestratorInput(
		rmToolStart("c1", "Read", rmAt(1)),
		rmToolEnd("c1", domain.ToolCallSuccess, rmAt(3)),
		rmToolStart("c2", "Bash", rmAt(4)),
		rmToolEnd("c2", domain.ToolCallError, rmAt(5)))

	act := rmActivity(t, in)

	if got := rmCallIDs(act.OrchestratorToolCalls); !rmEqualStrings(got, []string{"c1", "c2"}) {
		t.Fatalf("calls = %v, want [c1 c2]", got)
	}
	c1 := act.OrchestratorToolCalls[0]
	if c1.Pairing != domain.ToolCallPaired || c1.ToolName != "Read" || c1.Status != domain.ToolCallSuccess {
		t.Errorf("c1 = %+v, want paired Read success", c1)
	}
	if !c1.StartedAt.Equal(rmAt(1)) || !c1.EndedAt.Equal(rmAt(3)) {
		t.Errorf("c1 timestamps = %v .. %v, want %v .. %v", c1.StartedAt, c1.EndedAt, rmAt(1), rmAt(3))
	}
	if d, ok := c1.Duration(); !ok || d != 2*time.Second {
		t.Errorf("c1 Duration() = (%v, %v), want (2s, true)", d, ok)
	}
	c2 := act.OrchestratorToolCalls[1]
	if c2.Pairing != domain.ToolCallPaired || c2.ToolName != "Bash" || c2.Status != domain.ToolCallError {
		t.Errorf("c2 = %+v, want paired Bash error", c2)
	}
}

func TestRunModel_SubagentToolCalls_LandOnTheirInvocationNotTheOrchestrator(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{
			rmToolStart("o1", "Task", rmAt(0)), rmToolEnd("o1", domain.ToolCallSuccess, rmAt(1)),
		}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{
			rmInvStart("a#1", "coder", rmAt(2)),
			rmToolStart("a1", "Read", rmAt(3)), rmToolEnd("a1", domain.ToolCallSuccess, rmAt(4)),
			rmInvEnd("a#1", "SUCCESS", "model-a", rmAt(9)),
		}},
		{Ref: agentStream(testRunRef, "b#2"), Events: []domain.Event{
			rmInvStart("b#2", "tester", rmAt(2)),
			rmToolStart("b1", "Bash", rmAt(3)), rmToolEnd("b1", domain.ToolCallError, rmAt(5)),
		}},
	}}

	act := rmActivity(t, in)

	if got := rmCallIDs(act.OrchestratorToolCalls); !rmEqualStrings(got, []string{"o1"}) {
		t.Errorf("orchestrator calls = %v, want [o1]", got)
	}
	if got := rmCallIDs(rmInvocation(t, act, "a#1").ToolCalls); !rmEqualStrings(got, []string{"a1"}) {
		t.Errorf("a#1 calls = %v, want [a1]", got)
	}
	b := rmInvocation(t, act, "b#2").ToolCalls
	if got := rmCallIDs(b); !rmEqualStrings(got, []string{"b1"}) || b[0].Status != domain.ToolCallError {
		t.Errorf("b#2 calls = %+v, want one errored call b1", b)
	}
}

func TestRunModel_ParallelCallsWithDistinctIDs_PairIndependentlyInFirstEventOrder(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		rmToolStart("p1", "Read", rmAt(10)),
		rmToolStart("p2", "Read", rmAt(10)),
		rmToolStart("p3", "Grep", rmAt(11)),
		rmToolEnd("p3", domain.ToolCallSuccess, rmAt(12)),
		rmToolEnd("p1", domain.ToolCallError, rmAt(15)),
		rmToolEnd("p2", domain.ToolCallSuccess, rmAt(13)))

	calls := rmInvocation(t, rmActivity(t, in), "a#1").ToolCalls

	if got := rmCallIDs(calls); !rmEqualStrings(got, []string{"p1", "p2", "p3"}) {
		t.Fatalf("calls = %v, want [p1 p2 p3] ordered by each call's first event", got)
	}
	want := map[string]struct {
		status domain.ToolCallStatus
		tool   string
		secs   time.Duration
	}{
		"p1": {domain.ToolCallError, "Read", 5},
		"p2": {domain.ToolCallSuccess, "Read", 3},
		"p3": {domain.ToolCallSuccess, "Grep", 1},
	}
	for _, c := range calls {
		w := want[c.CallID]
		d, ok := c.Duration()
		if c.Pairing != domain.ToolCallPaired || c.Status != w.status || c.ToolName != w.tool || !ok || d != w.secs*time.Second {
			t.Errorf("%s = %+v (duration %v, %v), want paired %s %s lasting %ds", c.CallID, c, d, ok, w.tool, w.status, w.secs)
		}
	}
}

func TestRunModel_StartWithoutEnd_IsStartOnly(t *testing.T) {
	in := rmOrchestratorInput(rmToolStart("c1", "Read", rmAt(1)))

	calls := rmActivity(t, in).OrchestratorToolCalls

	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	c := calls[0]
	if c.Pairing != domain.ToolCallStartOnly || c.ToolName != "Read" || c.Status != "" || !c.EndedAt.IsZero() || !c.StartedAt.Equal(rmAt(1)) {
		t.Errorf("call = %+v, want start-only Read with no status and no end time", c)
	}
	if _, ok := c.Duration(); ok {
		t.Error("an unpaired call must have no duration")
	}
}

func TestRunModel_EndWithoutStart_IsEndOnly(t *testing.T) {
	in := rmOrchestratorInput(rmToolEnd("c9", domain.ToolCallError, rmAt(7)))

	calls := rmActivity(t, in).OrchestratorToolCalls

	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	c := calls[0]
	if c.Pairing != domain.ToolCallEndOnly || c.ToolName != "" || c.Status != domain.ToolCallError || !c.StartedAt.IsZero() || !c.EndedAt.Equal(rmAt(7)) {
		t.Errorf("call = %+v, want end-only with no tool name, error status and no start time", c)
	}
	if _, ok := c.Duration(); ok {
		t.Error("an unpaired call must have no duration")
	}
}

func TestRunModel_PairedEndWithoutStatus_KeepsEmptyStatus(t *testing.T) {
	in := rmOrchestratorInput(
		rmToolStart("c1", "Read", rmAt(1)),
		rmToolEnd("c1", "", rmAt(2)),
		rmToolStart("c2", "Read", rmAt(3)),
		rmToolEnd("c2", domain.ToolCallStatus("cancelled"), rmAt(4)))

	calls := rmActivity(t, in).OrchestratorToolCalls

	c1, c2 := rmRequireCall(t, calls, "c1"), rmRequireCall(t, calls, "c2")
	if c1.Pairing != domain.ToolCallPaired || c1.Status != "" {
		t.Errorf("c1 = %+v, want paired with empty status", c1)
	}
	if c2.Pairing != domain.ToolCallPaired || c2.Status != "cancelled" {
		t.Errorf("c2 = %+v, want paired with status kept verbatim", c2)
	}
}

func TestRunModel_DuplicateCallID_NothingPairsAndEachEventCountsAsOneCall(t *testing.T) {
	in := rmOrchestratorInput(
		rmToolStart("dup", "Read", rmAt(1)),
		rmToolStart("dup", "Read", rmAt(2)),
		rmToolEnd("dup", domain.ToolCallSuccess, rmAt(3)))

	calls := rmActivity(t, in).OrchestratorToolCalls

	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3 (two start-only, one end-only)", len(calls))
	}
	starts, ends := 0, 0
	for _, c := range calls {
		switch c.Pairing {
		case domain.ToolCallStartOnly:
			starts++
		case domain.ToolCallEndOnly:
			ends++
		default:
			t.Errorf("call %+v is paired; an ambiguous id must never be paired", c)
		}
	}
	if starts != 2 || ends != 1 {
		t.Errorf("start-only = %d, end-only = %d; want 2 and 1", starts, ends)
	}
}

func TestRunModel_TwoStartsAndTwoEndsOfOneID_AreFourUnpairedCalls(t *testing.T) {
	in := rmOrchestratorInput(
		rmToolStart("dup", "Read", rmAt(1)), rmToolEnd("dup", domain.ToolCallSuccess, rmAt(2)),
		rmToolStart("dup", "Read", rmAt(3)), rmToolEnd("dup", domain.ToolCallSuccess, rmAt(4)))

	calls := rmActivity(t, in).OrchestratorToolCalls

	if len(calls) != 4 {
		t.Fatalf("got %d calls, want 4", len(calls))
	}
	for _, c := range calls {
		if c.Pairing == domain.ToolCallPaired {
			t.Errorf("call %+v is paired; reused ids are ambiguous and must stay unpaired", c)
		}
	}
}

func TestRunModel_EmptyCallID_NeverPairs(t *testing.T) {
	in := rmOrchestratorInput(
		rmToolStart("", "Read", rmAt(1)),
		rmToolEnd("", domain.ToolCallSuccess, rmAt(2)))

	calls := rmActivity(t, in).OrchestratorToolCalls

	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2 unpaired calls", len(calls))
	}
	if calls[0].Pairing != domain.ToolCallStartOnly || calls[1].Pairing != domain.ToolCallEndOnly {
		t.Errorf("pairings = %v, %v; want start-only then end-only", calls[0].Pairing, calls[1].Pairing)
	}
}

func TestRunModel_SameCallIDOnDifferentStreams_DoesNotPairAcrossStreams(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{rmToolStart("c1", "Task", rmAt(1))}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{rmToolEnd("c1", domain.ToolCallSuccess, rmAt(2))}},
	}}

	act := rmActivity(t, in)

	if o := act.OrchestratorToolCalls; len(o) != 1 || o[0].Pairing != domain.ToolCallStartOnly {
		t.Errorf("orchestrator calls = %+v, want one start-only call", o)
	}
	if a := rmInvocation(t, act, "a#1").ToolCalls; len(a) != 1 || a[0].Pairing != domain.ToolCallEndOnly {
		t.Errorf("a#1 calls = %+v, want one end-only call", a)
	}
}

func TestRunModel_SameCallIDOnTwoAgentStreams_PairsWithinEachStreamOnly(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{
			rmToolStart("c1", "Read", rmAt(1)), rmToolEnd("c1", domain.ToolCallSuccess, rmAt(2))}},
		{Ref: agentStream(testRunRef, "b#2"), Events: []domain.Event{
			rmToolStart("c1", "Bash", rmAt(5)), rmToolEnd("c1", domain.ToolCallError, rmAt(9))}},
	}}

	act := rmActivity(t, in)

	a, b := rmInvocation(t, act, "a#1").ToolCalls, rmInvocation(t, act, "b#2").ToolCalls
	if len(a) != 1 || a[0].Pairing != domain.ToolCallPaired || a[0].ToolName != "Read" || a[0].Status != domain.ToolCallSuccess {
		t.Errorf("a#1 calls = %+v, want one paired Read success", a)
	}
	if len(b) != 1 || b[0].Pairing != domain.ToolCallPaired || b[0].ToolName != "Bash" || b[0].Status != domain.ToolCallError {
		t.Errorf("b#2 calls = %+v, want one paired Bash error", b)
	}
}

func TestRunModel_MixedCalls_AreOrderedByFirstEventPosition(t *testing.T) {
	in := rmOrchestratorInput(
		rmToolStart("c1", "Read", rmAt(1)),
		rmToolEnd("c9", domain.ToolCallSuccess, rmAt(2)), // end-only, second event
		rmToolStart("c2", "Bash", rmAt(3)),               // start-only, third event
		rmToolEnd("c1", domain.ToolCallSuccess, rmAt(4))) // completes c1

	calls := rmActivity(t, in).OrchestratorToolCalls

	if got := rmCallIDs(calls); !rmEqualStrings(got, []string{"c1", "c9", "c2"}) {
		t.Errorf("order = %v, want [c1 c9 c2]", got)
	}
}

func TestRunModel_ToolOnlyInstance_AppearsInActivityButNotInAgents(t *testing.T) {
	in := oneAgentStream(testRunRef, "ghost#1", rmToolStart("c1", "Read", rmAt(1)))

	agg, _ := analysis.Aggregate(in)

	run := requireOneRun(t, agg)
	inv := rmInvocation(t, run.Activity, "ghost#1")
	if inv.State != domain.InvocationOpen || len(inv.ToolCalls) != 1 {
		t.Errorf("invocation = %+v, want an open invocation holding its one tool call", inv)
	}
	if len(run.Agents) != 0 {
		t.Errorf("got %d agents, want 0: a tool-only instance stays out of RunAggregate.Agents", len(run.Agents))
	}
}

func TestRunModel_ToolCallsAttributedToStreamHint_WhenEventsCarryAnotherID(t *testing.T) {
	in := oneAgentStream(testRunRef, "hint#1",
		rmInvEnd("real#1", "SUCCESS", "model-a", rmAt(9)),
		rmToolStart("c1", "Read", rmAt(1)),
		rmToolEnd("c1", domain.ToolCallSuccess, rmAt(2)))

	act := rmActivity(t, in)

	if got := rmCallIDs(rmInvocation(t, act, "hint#1").ToolCalls); !rmEqualStrings(got, []string{"c1"}) {
		t.Errorf("hint#1 calls = %v, want [c1]: tool calls follow the stream hint", got)
	}
	if got := rmInvocation(t, act, "real#1").ToolCalls; len(got) != 0 {
		t.Errorf("real#1 calls = %+v, want none: the attribution is visible, not guessed", got)
	}
}

func TestRunModel_ToolCallOnAgentStreamWithoutIdentity_IsNotAttributedToAnyInvocation(t *testing.T) {
	// A populated control stream proves the model is built; the anonymous
	// stream must add no invocation and no compaction to it.
	in := analysis.Input{Streams: []analysis.Stream{
		{
			Ref:    domain.StreamRef{Run: testRunRef, Kind: domain.StreamAgentInstance, InstanceHint: "known#1"},
			Events: []domain.Event{rmToolStart("k1", "Read", rmAt(1))},
		},
		{
			Ref:    domain.StreamRef{Run: testRunRef, Kind: domain.StreamAgentInstance, InstanceHint: ""},
			Events: []domain.Event{rmToolStart("c1", "Read", rmAt(1)), rmCompaction("auto", domain.Tokens(2), domain.Tokens(1), rmAt(2))},
		},
	}}

	agg, findings := analysis.Aggregate(in)
	act := requireOneRun(t, agg).Activity

	var unidentified int
	for _, f := range findings {
		if f.Kind == domain.FindingUnidentifiedEvent {
			unidentified++
		}
	}
	if unidentified != 2 {
		t.Errorf("got %d unidentified_event findings, want 2 (tool call and compaction): %v", unidentified, findings)
	}

	if len(act.Invocations) != 1 || act.Invocations[0].ToolCalls == nil || len(act.Invocations[0].ToolCalls) != 1 {
		t.Fatalf("invocations = %+v, want only known#1 holding its one tool call", act.Invocations)
	}
	if got := rmCallIDs(act.Invocations[0].ToolCalls); !rmEqualStrings(got, []string{"k1"}) {
		t.Errorf("known#1 calls = %v, want [k1]: the anonymous stream's call must not land here", got)
	}
	if len(act.Compactions) != 0 {
		t.Errorf("compactions = %+v, want none from an unattributable stream", act.Compactions)
	}
}
