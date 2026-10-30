package analysis_test

// Tests that the context figures use main calls only, while usage and cost
// count every call.
//
// Rules under test:
//   - First, peak and final context, the context at first write and detected
//     compactions come from calls of kind main; helper calls (compaction,
//     auxiliary) never shift them.
//   - ActorContext.Calls still lists every per-call record, with its kind and
//     label; a record without a kind is a main call.
//   - Usage and cost totals include calls of every kind.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// ctxKinded marks a call as the given kind with a label.
func ctxKinded(u domain.InvocationUsage, kind domain.CallKind, label string) domain.InvocationUsage {
	u.Kind = kind
	u.Label = label
	return u
}

// ctxMixedAgent has main calls of 100k and 200k around helper calls whose
// contexts would change every figure if they counted.
func ctxMixedAgent(tools ...domain.ToolCall) ctxAgent {
	return ctxAgent{id: ctxAgentID, tools: tools, usage: []domain.InvocationUsage{
		ctxKinded(ctxCall("a1", 5, 800, 0, 0, 1), domain.CallAuxiliary, "title"),
		ctxCall("m1", 10, 100_000, 0, 0, 1),
		ctxKinded(ctxCall("a2", 15, 900_000, 0, 0, 1), domain.CallAuxiliary, "progressMessages"),
		ctxCall("m2", 20, 200_000, 0, 0, 1),
		ctxKinded(ctxCall("c1", 25, 950_000, 0, 0, 1), domain.CallCompaction, "summarizeConversationHistory"),
		ctxKinded(ctxCall("a3", 30, 700, 0, 0, 1), domain.CallAuxiliary, "title"),
	}}
}

func TestContextKinds_FirstPeakAndFinal_UseMainCallsOnly(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{ctxMixedAgent()}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	ctxAssertTokens(t, "first", got.First.Tokens, 100_000)
	ctxAssertTokens(t, "peak", got.Peak.Tokens, 200_000)
	ctxAssertTokens(t, "final", got.Final.Tokens, 200_000)
}

func TestContextKinds_Orchestrator_FiguresUseMainCallsOnly(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{orchUsage: ctxMixedAgent().usage})

	got := run.Context.Orchestrator

	ctxAssertTokens(t, "first", got.First.Tokens, 100_000)
	ctxAssertTokens(t, "peak", got.Peak.Tokens, 200_000)
	ctxAssertTokens(t, "final", got.Final.Tokens, 200_000)
}

func TestContextKinds_CallsListEveryCallWithItsKindAndLabel(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{ctxMixedAgent()}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	want := []struct {
		id    string
		kind  domain.CallKind
		label string
	}{
		{"a1", domain.CallAuxiliary, "title"},
		{"m1", domain.CallMain, ""},
		{"a2", domain.CallAuxiliary, "progressMessages"},
		{"m2", domain.CallMain, ""},
		{"c1", domain.CallCompaction, "summarizeConversationHistory"},
		{"a3", domain.CallAuxiliary, "title"},
	}
	if len(got.Calls) != len(want) {
		t.Fatalf("calls = %d, want %d (every kind is listed)", len(got.Calls), len(want))
	}
	for i, w := range want {
		c := got.Calls[i]
		if c.RecordID != w.id || c.Kind != w.kind || c.Label != w.label {
			t.Errorf("call %d = {%s %q %q}, want {%s %q %q}", i, c.RecordID, c.Kind, c.Label, w.id, w.kind, w.label)
		}
	}
}

func TestContextKinds_CallWithoutKind_IsMainForFiguresAndRows(t *testing.T) {
	// ctxCall leaves Kind unset: the call must count as a main call.
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxCall("r1", 10, 100, 0, 0, 1),
		ctxCall("r2", 20, 300, 0, 0, 1),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	ctxAssertTokens(t, "peak", got.Peak.Tokens, 300)
	if k := ctxRequireCall(t, got, 0).Kind; k != domain.CallMain {
		t.Errorf("row kind = %q, want %q", k, domain.CallMain)
	}
}

func TestContextKinds_ActorWithOnlyHelperCalls_HasAbsentFiguresButListsTheCalls(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxKinded(ctxCall("a1", 10, 500, 0, 0, 1), domain.CallAuxiliary, "title"),
		ctxKinded(ctxCall("c1", 20, 900, 0, 0, 1), domain.CallCompaction, "summarizeConversationHistory"),
	}}}})

	got := ctxRequireAgent(t, run, ctxAgentID)

	ctxAssertAbsent(t, "first", got.First.Tokens)
	ctxAssertAbsent(t, "peak", got.Peak.Tokens)
	ctxAssertAbsent(t, "final", got.Final.Tokens)
	if len(got.Calls) != 2 {
		t.Errorf("calls = %d, want 2", len(got.Calls))
	}
}

func TestContextKinds_FirstWrite_UsesTheLastMainCallBeforeTheWrite(t *testing.T) {
	// The auxiliary call at 20s is the latest call before the write at 25s.
	agent := ctxAgent{id: ctxAgentID, tools: []domain.ToolCall{ctxTool("t1", "Write", 25, 26)},
		usage: []domain.InvocationUsage{
			ctxCall("m1", 10, 100_000, 0, 0, 1),
			ctxKinded(ctxCall("a1", 20, 900, 0, 0, 1), domain.CallAuxiliary, "title"),
			ctxCall("m2", 40, 300_000, 0, 0, 1),
		}}

	got := ctxRequireAgent(t, ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}}), ctxAgentID)

	ctxAssertTokens(t, "first write", got.FirstWrite.Tokens, 100_000)
	for _, c := range got.Calls {
		if c.FirstWrite != (c.RecordID == "m1") {
			t.Errorf("call %s FirstWrite = %v, want only m1 marked", c.RecordID, c.FirstWrite)
		}
	}
}

func TestContextKinds_HelperCallBetweenMainCalls_DoesNotCreateADetectedCompaction(t *testing.T) {
	// 100k -> 500 -> 100k: with the helper call counted this is a drop of 99.5%.
	run, _ := detPrice(t, ctxRunSpec{orchUsage: []domain.InvocationUsage{
		ctxCall("m1", 0, 100_000, 0, 0, 1),
		ctxKinded(ctxCall("a1", 30, 500, 0, 0, 1), domain.CallAuxiliary, "title"),
		ctxCall("m2", 60, 100_000, 0, 0, 1),
	}})

	if n := run.Context.Orchestrator.DetectedCount(); n != 0 {
		t.Errorf("detected = %d, want 0", n)
	}
}

func TestContextKinds_DropBetweenMainCallsAroundAHelperCall_IsDetectedOnTheMainCalls(t *testing.T) {
	run, _ := detPrice(t, ctxRunSpec{orchUsage: []domain.InvocationUsage{
		ctxCall("m1", 0, 100_000, 0, 0, 1),
		ctxKinded(ctxCall("a1", 30, 99_000, 0, 0, 1), domain.CallAuxiliary, "title"),
		ctxCall("m2", 60, 60_000, 0, 0, 1),
	}})

	marks := detDetected(run.Context.Orchestrator)
	if len(marks) != 1 {
		t.Fatalf("detected marks = %d, want 1", len(marks))
	}
	m := marks[0]
	if m.BeforeTokens != 100_000 || m.AfterTokens != 60_000 {
		t.Errorf("before/after = %d/%d, want 100000/60000 (main calls only)", m.BeforeTokens, m.AfterTokens)
	}
	calls := run.Context.Orchestrator.Calls
	if m.Before < 0 || m.After < 0 || m.Before >= len(calls) || m.After >= len(calls) {
		t.Fatalf("mark indexes %d/%d outside %d calls", m.Before, m.After, len(calls))
	}
	if calls[m.Before].RecordID != "m1" || calls[m.After].RecordID != "m2" {
		t.Errorf("mark points at %s/%s, want m1/m2", calls[m.Before].RecordID, calls[m.After].RecordID)
	}
	if !m.At.Equal(ctxAt(60)) {
		t.Errorf("mark at %v, want %v", m.At, ctxAt(60))
	}
}

func TestContextKinds_UsageAndCost_CountEveryKind(t *testing.T) {
	agent := ctxAgent{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxCall("m1", 10, 1000, 0, 0, 0),
		ctxKinded(ctxCall("c1", 20, 2000, 0, 0, 0), domain.CallCompaction, "summarizeConversationHistory"),
		ctxKinded(ctxCall("a1", 30, 3000, 0, 0, 0), domain.CallAuxiliary, "title"),
	}}

	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{agent}})

	if len(run.Agents) != 1 {
		t.Fatalf("agents = %d, want 1", len(run.Agents))
	}
	assertTokenPresent(t, "agent input", run.Agents[0].Totals.Tokens.Input, 6000)
	assertMoneyKnown(t, "agent cost", run.Agents[0].Totals.Money.Total, 6000*3000)
	assertTokenPresent(t, "run input", run.Totals.Tokens.Input, 6000)
	assertMoneyKnown(t, "run cost", run.Totals.Money.Total, 6000*3000)
	got := ctxRequireAgent(t, run, ctxAgentID)
	assertMoneyKnown(t, "auxiliary call cost", ctxRequireCall(t, got, 2).Cost, 3000*3000)
}
