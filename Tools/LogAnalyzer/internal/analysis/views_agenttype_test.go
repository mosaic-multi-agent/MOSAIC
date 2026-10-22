package analysis_test

// Tests for the by-agent-type view (RunReport.ByAgentType) over the shared
// scenario in views_scenario_test.go.
//
// Rules under test:
//   - Groups are ordered by agent type ascending with the unknown type ("") last
//     and the orchestrator never included; an invocation without a type is
//     grouped under "", not dropped.
//   - Per group: invocation count, summed totals, and minimum / maximum / average
//     money per invocation, computed from the members' known amounts.
//   - Partial figures are marked: a group whose members are not all fully priced
//     is not Complete, and an unpriced or usage-less member never disappears.
//   - Dispatches of a group are listed by instance id with their own totals,
//     state, status and tool-call count.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestByAgentType_GroupsOrderedByTypeWithUnknownLast(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	var types []string
	for _, g := range run.ByAgentType {
		types = append(types, g.AgentType)
	}

	want := []string{"coder", "reviewer", "tester", ""}
	if !rmEqualStrings(types, want) {
		t.Errorf("agent types = %q, want %q", types, want)
	}
}

func TestByAgentType_PerInvocationStats_UseKnownAmountsAndMarkPartial(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	coder := requireGroup(t, run, "coder")

	if coder.Figures.Invocations != 3 {
		t.Errorf("coder invocations = %d, want 3", coder.Figures.Invocations)
	}
	assertMoneyKnown(t, "coder group total", coder.Figures.Totals.Money.Total, 5_425_000)
	stats := coder.PerInvocation
	if stats.Basis != 2 {
		t.Errorf("basis = %d, want 2 (coder#3 has no known amount)", stats.Basis)
	}
	assertMoneyKnown(t, "min", stats.Min, 1_000_000)
	assertMoneyKnown(t, "max", stats.Max, 4_425_000)
	assertMoneyKnown(t, "average", stats.Average, 2_712_500)
	if stats.Complete {
		t.Error("stats are complete, want partial because coder#3 is unpriced")
	}
	assertUnpricedModels(t, "coder group", coder.Figures.UnpricedModels, viewsUnpriced)
}

func TestByAgentType_FullyPricedGroup_StatsAreComplete(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	tester := requireGroup(t, run, "tester")

	stats := tester.PerInvocation
	if stats.Basis != 1 || !stats.Complete {
		t.Errorf("basis = %d, complete = %v, want 1 and true", stats.Basis, stats.Complete)
	}
	assertMoneyKnown(t, "min", stats.Min, 350_000)
	assertMoneyKnown(t, "max", stats.Max, 350_000)
	assertMoneyKnown(t, "average", stats.Average, 350_000)
}

func TestByAgentType_GroupWithoutAnyKnownAmount_HasNoDataStatsAndIsPartial(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	reviewer := requireGroup(t, run, "reviewer")

	stats := reviewer.PerInvocation
	if stats.Basis != 0 || stats.Complete {
		t.Errorf("basis = %d, complete = %v, want 0 and false", stats.Basis, stats.Complete)
	}
	assertMoneyState(t, "min", stats.Min, domain.MoneyNoData)
	assertMoneyState(t, "max", stats.Max, domain.MoneyNoData)
	assertMoneyState(t, "average", stats.Average, domain.MoneyNoData)
	if reviewer.Figures.Invocations != 1 || reviewer.Figures.MissingUsage != 1 {
		t.Errorf("reviewer figures = %+v, want 1 invocation with missing usage", reviewer.Figures)
	}
}

func TestByAgentType_UnknownType_IsGroupedNotDropped(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	unknown := requireGroup(t, run, "")

	if unknown.Figures.Invocations != 1 || len(unknown.Dispatches) != 1 {
		t.Fatalf("unknown-type group = %+v, want 1 invocation (x#6)", unknown.Figures)
	}
	assertMoneyKnown(t, "unknown-type total", unknown.Figures.Totals.Money.Total, 10_000)
	if got := unknown.Dispatches[0].Invocation.Actor.Instance; got != "x#6" {
		t.Errorf("unknown-type dispatch = %s, want x#6", got)
	}
}

func TestByAgentType_Dispatches_ListedByInstanceIDWithTheirOwnFigures(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	coder := requireGroup(t, run, "coder")

	var ids []domain.AgentInstanceID
	for _, d := range coder.Dispatches {
		ids = append(ids, d.Invocation.Actor.Instance)
	}
	if len(ids) != 3 || ids[0] != "coder#1" || ids[1] != "coder#2" || ids[2] != "coder#3" {
		t.Fatalf("dispatch order = %v, want coder#1, coder#2, coder#3", ids)
	}
	first := requireDispatch(t, coder, "coder#1")
	assertMoneyKnown(t, "coder#1 total", first.Totals.Money.Total, 4_425_000)
	if first.Invocation.State != domain.InvocationClosed || first.Invocation.StatusCode != "SUCCESS" {
		t.Errorf("coder#1 invocation = %+v, want closed SUCCESS", first.Invocation)
	}
	if len(first.Invocation.ToolCalls) != 3 || first.MissingUsage {
		t.Errorf("coder#1 has %d tool calls, missing usage %v, want 3 and false", len(first.Invocation.ToolCalls), first.MissingUsage)
	}
	if len(first.Models) != 1 || first.Models[0] != viewsModel || len(first.UnpricedModels) != 0 {
		t.Errorf("coder#1 models = %v / unpriced %v, want [m-a] and none", first.Models, first.UnpricedModels)
	}
}

func TestByAgentType_Dispatch_CarriesUnpricedMissingUsageAndAssumedLabels(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	blocked := requireDispatch(t, requireGroup(t, run, "coder"), "coder#3")
	assertUnpricedModels(t, "coder#3", blocked.UnpricedModels, viewsUnpriced)
	assertMoneyState(t, "coder#3 total", blocked.Totals.Money.Total, domain.MoneyUnpriced)
	if blocked.Totals.Money.Complete {
		t.Error("coder#3 totals are complete, want partial")
	}

	silent := requireDispatch(t, requireGroup(t, run, "reviewer"), "reviewer#5")
	if !silent.MissingUsage {
		t.Error("reviewer#5 MissingUsage = false, want true")
	}
	assertMoneyState(t, "reviewer#5 total", silent.Totals.Money.Total, domain.MoneyNoData)

	open := requireDispatch(t, requireGroup(t, run, "tester"), "tester#4")
	assertAssumedTokens(t, "tester#4", open.Totals, 200)
	if open.Invocation.State != domain.InvocationOpen {
		t.Errorf("tester#4 state = %v, want open", open.Invocation.State)
	}
}

func TestByAgentType_OrchestratorIsNeverAGroup(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	total := 0
	for _, g := range run.ByAgentType {
		total += g.Figures.Invocations
	}

	if total != 6 {
		t.Errorf("sum of group invocations = %d, want 6 subagent invocations only", total)
	}
}
