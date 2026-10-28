package analysis_test

// Tests for the per-actor context figures of a priced run.
//
// Rules under test:
//   - A call's context is input + cache read + cache write; only per-call
//     records (non-empty record id) count, in time order.
//   - First, peak and final context come from calls with a present context;
//     final is the last call's context as sent, without its output tokens.
//   - Legacy aggregate entries (no record id) are never used as a context size.
//   - An actor without per-call records has absent figures, never 0.
//   - A call missing input, cache read or cache write is marked partial.
//   - The orchestrator gets figures, but no first-write figure.
//   - The compaction count of an actor is the number of logged compactions of
//     that actor.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const ctxAgentID = domain.AgentInstanceID("coder#1")

func TestContext_FirstPeakFinalAndCallCount_FollowTimeOrderNotFileOrder(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, typ: "coder", usage: []domain.InvocationUsage{
		ctxCall("r2", 20, 100, 150, 50, 7), // context 300
		ctxCall("r1", 10, 50, 40, 10, 7),   // context 100
		ctxCall("r3", 30, 120, 60, 20, 7),  // context 200
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	if len(got.Calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(got.Calls))
	}
	for i, id := range []string{"r1", "r2", "r3"} {
		if got.Calls[i].RecordID != id {
			t.Errorf("call %d = %q, want %q (time order)", i, got.Calls[i].RecordID, id)
		}
	}
	ctxAssertTokens(t, "first", got.First.Tokens, 100)
	ctxAssertTokens(t, "peak", got.Peak.Tokens, 300)
	ctxAssertTokens(t, "final", got.Final.Tokens, 200)
}

func TestContext_CallContextIsInputPlusCacheReadPlusCacheWrite(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxCall("r1", 10, 11, 220, 3300, 44444),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	ctxAssertTokens(t, "call context", ctxRequireCall(t, got, 0).Context, 3531)
}

func TestContext_FinalContext_ExcludesThatCallsOutputTokens(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxCall("r1", 10, 1000, 2000, 3000, 5_000_000),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	ctxAssertTokens(t, "final", got.Final.Tokens, 6000)
	ctxAssertTokens(t, "peak", got.Peak.Tokens, 6000)
}

func TestContext_LegacyAggregateEntries_AreNeverUsedAsContext(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxLegacyEntry(5, 900_000, 0, 0, 10), // would be the peak and first
		ctxCall("r1", 10, 100, 0, 0, 1),
		ctxCall("r2", 20, 300, 0, 0, 1),
		ctxLegacyEntry(30, 800_000, 0, 0, 10), // would be the final
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	if len(got.Calls) != 2 {
		t.Errorf("calls = %d, want 2 (legacy entries are not calls)", len(got.Calls))
	}
	ctxAssertTokens(t, "first", got.First.Tokens, 100)
	ctxAssertTokens(t, "peak", got.Peak.Tokens, 300)
	ctxAssertTokens(t, "final", got.Final.Tokens, 300)
}

func TestContext_OnlyLegacyEntries_GiveAbsentFiguresNotZero(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxLegacyEntry(5, 900_000, 1, 1, 10),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	if len(got.Calls) != 0 {
		t.Errorf("calls = %d, want 0", len(got.Calls))
	}
	ctxAssertAbsent(t, "first", got.First.Tokens)
	ctxAssertAbsent(t, "peak", got.Peak.Tokens)
	ctxAssertAbsent(t, "final", got.Final.Tokens)
}

func TestContext_ActorWithoutAnyUsage_HasAbsentFigures(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	ctxAssertAbsent(t, "first", got.First.Tokens)
	ctxAssertAbsent(t, "peak", got.Peak.Tokens)
	ctxAssertAbsent(t, "final", got.Final.Tokens)
	ctxAssertAbsent(t, "first write", got.FirstWrite.Tokens)
	ctxAssertAbsent(t, "orchestrator peak", run.Context.Orchestrator.Peak.Tokens)
}

func TestContext_CallWithNoContributingCategory_IsListedButHasNoContext(t *testing.T) {
	outputOnly := domain.InvocationUsage{
		Model: ctxModel, Harness: domain.HarnessClaudeCode, RecordID: "r0", At: ctxAt(5),
		Usage: domain.TokenUsage{Output: domain.Tokens(900)},
	}
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		outputOnly,
		ctxCall("r1", 10, 100, 0, 0, 1),
		outputOnly2(),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	if len(got.Calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(got.Calls))
	}
	ctxAssertAbsent(t, "context of output-only call", ctxRequireCall(t, got, 0).Context)
	if !ctxRequireCall(t, got, 0).Partial {
		t.Error("output-only call is not marked partial")
	}
	ctxAssertTokens(t, "first", got.First.Tokens, 100)
	ctxAssertTokens(t, "final", got.Final.Tokens, 100)
}

// outputOnly2 is a later output-only call than the one in the test above.
func outputOnly2() domain.InvocationUsage {
	return domain.InvocationUsage{
		Model: ctxModel, Harness: domain.HarnessClaudeCode, RecordID: "r2", At: ctxAt(20),
		Usage: domain.TokenUsage{Output: domain.Tokens(1)},
	}
}

func TestContext_CallMissingACategory_IsPartial(t *testing.T) {
	full := ctxCall("full", 10, 1, 2, 3, 4)
	noInput := full
	noInput.RecordID, noInput.Usage.Input = "no-input", domain.AbsentTokens()
	noRead := full
	noRead.RecordID, noRead.Usage.CacheRead = "no-read", domain.AbsentTokens()
	noWrite := full
	noWrite.RecordID, noWrite.Usage.CacheCreation = "no-write", domain.AbsentTokens()
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		full, noInput, noRead, noWrite,
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	want := map[string]bool{"full": false, "no-input": true, "no-read": true, "no-write": true}
	for _, c := range got.Calls {
		if c.Partial != want[c.RecordID] {
			t.Errorf("call %s partial = %v, want %v", c.RecordID, c.Partial, want[c.RecordID])
		}
	}
}

func TestContext_PeakTie_TakesTheEarliestCall(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxInputOnlyCall("r1", 10, 300, 5),  // partial, context 300
		ctxCall("r2", 20, 100, 100, 100, 5), // complete, context 300
		ctxCall("r3", 30, 10, 10, 10, 5),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	ctxAssertTokens(t, "peak", got.Peak.Tokens, 300)
	if !got.Peak.Partial {
		t.Error("peak is not marked partial, want the earlier (input-only) call on a tie")
	}
}

func TestContext_FigureFromAPartialCall_CarriesThePartialMark(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxInputOnlyCall("r1", 10, 500, 5), // partial
		ctxCall("r2", 20, 100, 100, 100, 5),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	ctxAssertTokens(t, "first", got.First.Tokens, 500)
	if !got.First.Partial {
		t.Error("first is not marked partial, want partial (input only)")
	}
	if got.Final.Partial {
		t.Error("final is marked partial, want complete")
	}
	if !got.Peak.Partial {
		t.Error("peak is not marked partial, want partial (the peak is the input-only call)")
	}
}

func TestContext_Orchestrator_GetsFiguresButNoFirstWrite(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			ctxCall("o1", 10, 100, 100, 100, 5),
			ctxCall("o2", 30, 200, 200, 200, 5),
		},
		orchTools: []domain.ToolCall{ctxTool("w1", "Write", 15, 16)},
	})

	got := run.Context.Orchestrator

	ctxAssertTokens(t, "first", got.First.Tokens, 300)
	ctxAssertTokens(t, "peak", got.Peak.Tokens, 600)
	ctxAssertTokens(t, "final", got.Final.Tokens, 600)
	ctxAssertAbsent(t, "first write", got.FirstWrite.Tokens)
	for _, c := range got.Calls {
		if c.FirstWrite {
			t.Errorf("orchestrator call %s is marked first write", c.RecordID)
		}
	}
}

func TestContext_LoggedCompactionCount_IsPerActor(t *testing.T) {
	other := domain.AgentInstanceID("tester#2")
	run := ctxPriceDefault(t, ctxRunSpec{
		agents: []ctxAgent{
			{id: ctxAgentID, usage: []domain.InvocationUsage{ctxCall("r1", 10, 1, 1, 1, 1)}},
			{id: other, usage: []domain.InvocationUsage{ctxCall("r1", 10, 1, 1, 1, 1)}},
		},
		compactions: []domain.Compaction{
			{Actor: domain.Orchestrator(), At: ctxAt(5)},
			{Actor: domain.AgentInstance(ctxAgentID), At: ctxAt(12)},
			{Actor: domain.AgentInstance(ctxAgentID), At: ctxAt(14)},
		},
	})

	if n := run.Context.Orchestrator.LoggedCount(); n != 1 {
		t.Errorf("orchestrator logged compactions = %d, want 1", n)
	}
	if n := ctxRequireAgent(t, run, ctxAgentID).LoggedCount(); n != 2 {
		t.Errorf("%s logged compactions = %d, want 2", ctxAgentID, n)
	}
	if n := ctxRequireAgent(t, run, other).LoggedCount(); n != 0 {
		t.Errorf("%s logged compactions = %d, want 0", other, n)
	}
}

func TestContext_EveryInvocationHasFigures_SortedByInstanceId(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{
		{id: "zeta#9", usage: []domain.InvocationUsage{ctxCall("r1", 10, 1, 1, 1, 1)}},
		{id: "alpha#1"},
		{id: "mid#5", usage: []domain.InvocationUsage{ctxCall("r1", 10, 1, 1, 1, 1)}},
	}})

	var ids []domain.AgentInstanceID
	for _, a := range run.Context.Invocations {
		ids = append(ids, a.Actor.Instance)
	}

	want := []domain.AgentInstanceID{"alpha#1", "mid#5", "zeta#9"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
}

func TestContext_CallWithUnparseableTime_ComesAfterTimedCalls(t *testing.T) {
	undated := ctxCall("undated", 0, 999, 0, 0, 1)
	undated.At = domain.InvocationUsage{}.At
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		undated,
		ctxCall("r1", 10, 100, 0, 0, 1),
		ctxCall("r2", 20, 200, 0, 0, 1),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	if len(got.Calls) != 3 || got.Calls[2].RecordID != "undated" {
		t.Fatalf("call order = %+v, want the undated call last", got.Calls)
	}
	ctxAssertTokens(t, "first", got.First.Tokens, 100)
}
