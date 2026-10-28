package analysis_test

// Tests for the run comparison table: per-type rows, the totals row, change
// figures with direction, only-in markers and the default row order.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// cmpScenario compares two runs that differ in every way a row can:
//
//	type      run A (tokens)   run B (tokens)
//	coder     1000             1000 + 3000 (two dispatches)
//	reviewer  2000             1000
//	tester    1000             1100
//	docs      1000             1000
//	planner   4000             -          (only in A)
//	writer    -                4900       (only in B)
//
// Orchestrator tokens: A 500, B 700.
func cmpScenario(t *testing.T) domain.RunComparison {
	t.Helper()
	a := rwRun{
		orch: rwInputUsage("orchestrator", 500),
		dispatches: []rwDispatch{
			cmpOK("coder#1", "coder", 1000), cmpOK("reviewer#1", "reviewer", 2000),
			cmpOK("tester#1", "tester", 1000), cmpOK("docs#1", "docs", 1000),
			cmpOK("planner#1", "planner", 4000),
		},
	}
	b := rwRun{
		orch: rwInputUsage("orchestrator", 700),
		dispatches: []rwDispatch{
			cmpOK("coder#1", "coder", 1000), cmpOK("coder#2", "coder", 3000),
			cmpOK("reviewer#1", "reviewer", 1000), cmpOK("tester#1", "tester", 1100),
			cmpOK("docs#1", "docs", 1000), cmpOK("writer#1", "writer", 4900),
		},
	}
	return cmpCompare(t, a, b)
}

func TestCompareRuns_CarriesTheRunsAndTheirProvisionalFlags(t *testing.T) {
	a := rwRun{dispatches: []rwDispatch{cmpOK("coder#1", "coder", 1000)}}
	b := rwRun{provisional: true, dispatches: []rwDispatch{cmpOK("coder#1", "coder", 1000)}}

	got := cmpCompare(t, a, b)

	if got.A != domain.NamedRun(cmpRunA) || got.B != domain.NamedRun(cmpRunB) {
		t.Errorf("runs = %v / %v, want %s / %s", got.A, got.B, cmpRunA, cmpRunB)
	}
	if got.ProvisionalA || !got.ProvisionalB {
		t.Errorf("provisional = %v / %v, want false / true", got.ProvisionalA, got.ProvisionalB)
	}
}

func TestCompareRuns_RowHoldsBothRunsAndTheChangeBetweenThem(t *testing.T) {
	row := cmpRow(t, cmpScenario(t), "coder")

	if row.Presence != domain.PresentBoth {
		t.Errorf("presence = %v, want present in both", row.Presence)
	}
	cmpAssertQty(t, "invocations", row.Invocations, 1, 2, domain.ChangeIncrease)
	cmpAssertQty(t, "tokens", row.Tokens, 1000, 4000, domain.ChangeIncrease)
	cmpAssertCost(t, "cost", row.Cost, 1000*3000, 4000*3000, domain.ChangeIncrease)
	cmpAssertQty(t, "average peak context", row.AvgPeakContext, 1000, 2000, domain.ChangeIncrease)
}

func TestCompareRuns_DecreaseCarriesNegativeDeltaAndPercent(t *testing.T) {
	row := cmpRow(t, cmpScenario(t), "reviewer")

	cmpAssertQty(t, "invocations", row.Invocations, 1, 1, domain.ChangeUnchanged)
	cmpAssertQty(t, "tokens", row.Tokens, 2000, 1000, domain.ChangeDecrease)
	cmpAssertCost(t, "cost", row.Cost, 2000*3000, 1000*3000, domain.ChangeDecrease)
	cmpAssertQty(t, "average peak context", row.AvgPeakContext, 2000, 1000, domain.ChangeDecrease)
}

func TestCompareRuns_EqualFiguresAreUnchangedWithAZeroPercent(t *testing.T) {
	row := cmpRow(t, cmpScenario(t), "docs")

	cmpAssertQty(t, "tokens", row.Tokens, 1000, 1000, domain.ChangeUnchanged)
	cmpAssertCost(t, "cost", row.Cost, 1000*3000, 1000*3000, domain.ChangeUnchanged)
}

func TestCompareRuns_TypeInOnlyOneRunIsMarkedAndItsOtherSideIsAbsent(t *testing.T) {
	got := cmpScenario(t)

	onlyA := cmpRow(t, got, "planner")
	onlyB := cmpRow(t, got, "writer")

	if onlyA.Presence != domain.OnlyInA || onlyB.Presence != domain.OnlyInB {
		t.Fatalf("presence = %v / %v, want only in A / only in B", onlyA.Presence, onlyB.Presence)
	}
	if !onlyA.Invocations.HasA || onlyA.Invocations.HasB || onlyA.Invocations.A != 1 {
		t.Errorf("only-in-A invocations: %+v, want A=1 present and B absent", onlyA.Invocations)
	}
	if onlyB.Tokens.HasA || !onlyB.Tokens.HasB || onlyB.Tokens.B != 4900 {
		t.Errorf("only-in-B tokens: %+v, want A absent and B=4900 present", onlyB.Tokens)
	}
	assertMoneyKnown(t, "only-in-A cost A", onlyA.Cost.A, 4000*3000)
	assertMoneyState(t, "only-in-A cost B", onlyA.Cost.B, domain.MoneyNoData)
	assertMoneyState(t, "only-in-B cost A", onlyB.Cost.A, domain.MoneyNoData)
	assertMoneyState(t, "only-in-A cost delta", onlyA.Cost.Delta, domain.MoneyNoData)
	for name, q := range map[string]domain.ComparedQuantity{
		"A invocations": onlyA.Invocations, "A tokens": onlyA.Tokens, "B tokens": onlyB.Tokens,
	} {
		cmpAssertNoChangeFigure(t, name, q)
	}
	if onlyA.Cost.HasPercent || onlyB.Cost.HasPercent || onlyA.Cost.Direction != domain.ChangeUnknown {
		t.Error("an only-in-one-run cost carries a percent or a direction")
	}
}

func TestCompareRuns_DefaultOrderIsLargestCostChangeFirstThenSinglesByCost(t *testing.T) {
	got := cmpScenario(t)

	want := []string{"coder", "reviewer", "tester", "docs", "writer", "planner"}
	if order := cmpRowOrder(got); !reflect.DeepEqual(order, want) {
		t.Errorf("row order = %v, want %v", order, want)
	}
}

func TestCompareRuns_EqualCostChangesOrderByTypeName(t *testing.T) {
	a := rwRun{dispatches: []rwDispatch{cmpOK("beta#1", "beta", 1000), cmpOK("alpha#1", "alpha", 1000)}}
	b := rwRun{dispatches: []rwDispatch{cmpOK("beta#1", "beta", 2000), cmpOK("alpha#1", "alpha", 2000)}}

	got := cmpCompare(t, a, b)

	if order, want := cmpRowOrder(got), []string{"alpha", "beta"}; !reflect.DeepEqual(order, want) {
		t.Errorf("row order = %v, want %v", order, want)
	}
}

func TestCompareRuns_TotalsRowIncludesTheOrchestratorAndCountsDispatches(t *testing.T) {
	got := cmpScenario(t).Totals

	if got.Presence != domain.PresentBoth {
		t.Errorf("totals presence = %v, want present in both", got.Presence)
	}
	cmpAssertQty(t, "invocations", got.Invocations, 5, 6, domain.ChangeIncrease)
	cmpAssertQty(t, "tokens", got.Tokens, 9500, 12700, domain.ChangeIncrease)
	cmpAssertCost(t, "cost", got.Cost, 9500*3000, 12700*3000, domain.ChangeIncrease)
	cmpAssertQty(t, "average peak context", got.AvgPeakContext, 1800, 2000, domain.ChangeIncrease)
}
