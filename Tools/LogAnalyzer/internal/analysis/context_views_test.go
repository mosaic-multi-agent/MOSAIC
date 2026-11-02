package analysis_test

// Tests for the context figures in the views of a priced run.
//
// Rules under test:
//   - Per-call rows carry time, model, token categories, cost and the partial
//     marker; their costs add up to the actor's priced total.
//   - A dispatch carries the peak context of its invocation.
//   - An agent-type group carries the number of members with a peak, their
//     average (rounded half up) and the maximum; members without a peak are
//     excluded, never counted as 0.
//   - The same aggregate priced twice gives identical figures.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestCallRows_CarryTimeModelTokensAndCost(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxInputOnlyCall("r1", 10, 1000, 20),
	}}}})

	got := ctxRequireCall(t, ctxRequireAgent(t, run, ctxAgentID), 0)

	if !got.At.Equal(ctxAt(10)) || got.Model != ctxModel || got.RecordID != "r1" {
		t.Errorf("row = at %v model %q record %q", got.At, got.Model, got.RecordID)
	}
	ctxAssertTokens(t, "input", got.Usage.Input, 1000)
	ctxAssertTokens(t, "output", got.Usage.Output, 20)
	ctxAssertAbsent(t, "cache read", got.Usage.CacheRead)
	ctxAssertAbsent(t, "cache write", got.Usage.CacheCreation)
	// 1000 input tokens at 3000 nanodollars + 20 output tokens at 15000.
	assertMoneyKnown(t, "row cost", got.Cost, 3_000_000+300_000)
	if !got.Partial {
		t.Error("row is not marked partial, want partial (no cache categories)")
	}
}

func TestCallRows_CostsAddUpToTheActorsPricedTotal(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{
		ctxInputOnlyCall("r1", 10, 1000, 20),
		ctxInputOnlyCall("r2", 20, 2500, 40),
		ctxInputOnlyCall("r3", 30, 10, 1),
	}}}})
	got := ctxRequireAgent(t, run, ctxAgentID)

	var sum int64
	for _, c := range got.Calls {
		if c.Cost.State != domain.MoneyKnown {
			t.Fatalf("row %s cost state = %v, want known", c.RecordID, c.Cost.State)
		}
		sum += c.Cost.Amount.Nanos()
	}

	want := run.Agents[0].Totals.Money.Total
	assertMoneyKnown(t, "actor total", want, sum)
	if sum == 0 {
		t.Error("row costs sum to 0")
	}
}

func TestCallRows_UnpricedModel_CostIsUnpricedNotZero(t *testing.T) {
	call := ctxInputOnlyCall("r1", 10, 1000, 20)
	call.Model = "model-without-price"
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{{id: ctxAgentID, usage: []domain.InvocationUsage{call}}}})

	got := ctxRequireCall(t, ctxRequireAgent(t, run, ctxAgentID), 0)

	assertMoneyState(t, "row cost", got.Cost, domain.MoneyUnpriced)
}

func TestCallRows_OrchestratorRowsAreListedToo(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{orchUsage: []domain.InvocationUsage{
		ctxInputOnlyCall("o1", 10, 1000, 20),
		ctxInputOnlyCall("o2", 20, 1000, 20),
	}})

	got := run.Context.Orchestrator

	assertMoneyKnown(t, "orchestrator row cost", ctxRequireCall(t, got, 1).Cost, 3_300_000)
}

func requireDispatchOf(t *testing.T, run domain.RunReport, id domain.AgentInstanceID) domain.Dispatch {
	t.Helper()
	for _, g := range run.ByAgentType {
		for _, d := range g.Dispatches {
			if d.Invocation.Actor.Instance == id {
				return d
			}
		}
	}
	t.Fatalf("no dispatch for %s", id)
	return domain.Dispatch{}
}

func TestDispatch_PeakContext_EqualsTheInvocationsPeak(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{
		{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{
			ctxCall("r1", 10, 100_000, 0, 0, 1),
			ctxCall("r2", 20, 260_000, 0, 0, 1),
			ctxCall("r3", 30, 120_000, 0, 0, 1),
		}},
		{id: "coder#2", typ: "coder"},
	}})

	withCalls := requireDispatchOf(t, run, "coder#1")
	withoutCalls := requireDispatchOf(t, run, "coder#2")

	ctxAssertTokens(t, "dispatch peak", withCalls.PeakContext.Tokens, 260_000)
	if withCalls.PeakContext.Band != domain.BandAboveOptimal {
		t.Errorf("dispatch peak band = %v, want above optimal", withCalls.PeakContext.Band)
	}
	ctxAssertAbsent(t, "dispatch without calls", withoutCalls.PeakContext.Tokens)
}

func TestAgentTypeGroup_PeakStats_AverageRoundsHalfUpAndMaxCarriesItsBand(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{
		{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{ctxCall("r1", 10, 100_000, 0, 0, 1)}},
		{id: "coder#2", typ: "coder", usage: []domain.InvocationUsage{ctxCall("r1", 10, 150_001, 0, 0, 1)}},
	}})

	got := requireGroup(t, run, "coder").PeakContext

	if got.Basis != 2 {
		t.Errorf("basis = %d, want 2", got.Basis)
	}
	ctxAssertTokens(t, "average", got.Average, 125_001) // 125000.5 rounds up
	ctxAssertTokens(t, "max", got.Max.Tokens, 150_001)
	if got.Max.Band != domain.BandOptimal {
		t.Errorf("max band = %v, want optimal", got.Max.Band)
	}
}

func TestAgentTypeGroup_PeakStats_MembersWithoutAPeakAreExcludedNotZero(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{
		{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{ctxCall("r1", 10, 200_000, 0, 0, 1)}},
		{id: "coder#2", typ: "coder"},
		{id: "coder#3", typ: "coder", usage: []domain.InvocationUsage{ctxLegacyEntry(10, 900_000, 0, 0, 1)}},
	}})

	got := requireGroup(t, run, "coder").PeakContext

	if got.Basis != 1 {
		t.Errorf("basis = %d, want 1 (two members have no per-call records)", got.Basis)
	}
	ctxAssertTokens(t, "average", got.Average, 200_000)
	ctxAssertTokens(t, "max", got.Max.Tokens, 200_000)
}

func TestAgentTypeGroup_PeakStats_GroupWithoutAnyPeakIsAbsent(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{
		{id: "tester#1", typ: "tester"},
		{id: "tester#2", typ: "tester"},
	}})

	got := requireGroup(t, run, "tester").PeakContext

	if got.Basis != 0 {
		t.Errorf("basis = %d, want 0", got.Basis)
	}
	ctxAssertAbsent(t, "average", got.Average)
	ctxAssertAbsent(t, "max", got.Max.Tokens)
}

func TestContext_PricingTheSameAggregateTwice_GivesIdenticalFigures(t *testing.T) {
	agg := ctxAggregate(ctxRunSpec{
		orchUsage: []domain.InvocationUsage{ctxCall("o1", 10, 100, 100, 100, 5)},
		agents: []ctxAgent{{id: ctxAgentID, typ: "coder", tools: []domain.ToolCall{ctxTool("w", "Write", 25, 26)},
			usage: []domain.InvocationUsage{ctxCall("r1", 10, 100, 0, 0, 1), ctxCall("r2", 20, 200, 0, 0, 1)}}},
	})

	first := ctxPrice(t, agg, domain.DefaultAnalysisSettings())
	second := ctxPrice(t, agg, domain.DefaultAnalysisSettings())

	if !reflect.DeepEqual(first.Context, second.Context) {
		t.Errorf("context differs between identical pricings:\n%+v\n%+v", first.Context, second.Context)
	}
	if len(first.Context.Invocations) != 1 {
		t.Fatalf("invocations = %d, want 1", len(first.Context.Invocations))
	}
	ctxAssertTokens(t, "first write", first.Context.Invocations[0].FirstWrite.Tokens, 200) // last call at or before the write at 25s
}

func TestAgentTypeGroup_PeakStats_AverageRoundsDownBelowHalf(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{
		{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{ctxCall("r1", 10, 100_000, 0, 0, 1)}},
		{id: "coder#2", typ: "coder", usage: []domain.InvocationUsage{ctxCall("r1", 10, 100_000, 0, 0, 1)}},
		{id: "coder#3", typ: "coder", usage: []domain.InvocationUsage{ctxCall("r1", 10, 100_001, 0, 0, 1)}},
	}})

	got := requireGroup(t, run, "coder").PeakContext

	ctxAssertTokens(t, "average", got.Average, 100_000) // 100000.33 rounds down
}

func TestAgentTypeGroup_PeakStats_AreGroupedPerTypeNotRunWide(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{
		{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{ctxCall("r1", 10, 300_000, 0, 0, 1)}},
		{id: "tester#1", typ: "tester", usage: []domain.InvocationUsage{ctxCall("r1", 10, 50_000, 0, 0, 1)}},
	}})

	coder := requireGroup(t, run, "coder").PeakContext
	tester := requireGroup(t, run, "tester").PeakContext

	ctxAssertTokens(t, "coder max", coder.Max.Tokens, 300_000)
	ctxAssertTokens(t, "tester max", tester.Max.Tokens, 50_000)
	ctxAssertTokens(t, "tester average", tester.Average, 50_000)
}

func TestDispatch_PeakContext_CarriesThePartialMarkOfItsSourceCall(t *testing.T) {
	run := ctxPriceDefault(t, ctxRunSpec{agents: []ctxAgent{
		{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{ctxInputOnlyCall("r1", 10, 1000, 1)}},
		{id: "coder#2", typ: "coder", usage: []domain.InvocationUsage{ctxCall("r1", 10, 1000, 0, 0, 1)}},
	}})

	partial := requireDispatchOf(t, run, "coder#1")
	complete := requireDispatchOf(t, run, "coder#2")

	if !partial.PeakContext.Partial {
		t.Error("dispatch peak from an input-only call is not marked partial")
	}
	if complete.PeakContext.Partial {
		t.Error("dispatch peak from a complete call is marked partial")
	}
}
