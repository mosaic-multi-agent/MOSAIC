package analysis_test

// Tests that the run comparison never turns an absent figure into zero: no
// percent from an absent or zero base, no delta for unpriced cost, and runs
// loaded from a digest compare from the figures they carry.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func TestCompareRuns_UnpricedCostHasNoDeltaPercentOrDirection(t *testing.T) {
	unpriced := cmpOK("coder#1", "coder", 0)
	unpriced.usage = rwUnpricedUsage("coder#1", 1000)
	a := rwRun{dispatches: []rwDispatch{unpriced, cmpOK("reviewer#1", "reviewer", 1000)}}
	b := rwRun{dispatches: []rwDispatch{cmpOK("coder#1", "coder", 1000), cmpOK("reviewer#1", "reviewer", 2000)}}

	got := cmpCompare(t, a, b)

	row := cmpRow(t, got, "coder")
	assertMoneyState(t, "cost A", row.Cost.A, domain.MoneyUnpriced)
	assertMoneyKnown(t, "cost B", row.Cost.B, 1000*3000)
	if row.Cost.Delta.State == domain.MoneyKnown {
		t.Errorf("cost delta is Known (%d) with an unpriced side", row.Cost.Delta.Amount.Nanos())
	}
	if row.Cost.HasPercent || row.Cost.Direction != domain.ChangeUnknown || row.Cost.Complete {
		t.Errorf("cost: percent=%v direction=%v complete=%v, want no percent, unknown direction, incomplete",
			row.Cost.HasPercent, row.Cost.Direction, row.Cost.Complete)
	}
	cmpAssertQty(t, "tokens still compare", row.Tokens, 1000, 1000, domain.ChangeUnchanged)
	if order, want := cmpRowOrder(got), []string{"reviewer", "coder"}; !reflect.DeepEqual(order, want) {
		t.Errorf("row order = %v, want the row without a known delta last: %v", order, want)
	}
	if got.Totals.Cost.Complete || got.Totals.Cost.HasPercent {
		t.Errorf("totals cost: complete=%v percent=%v, want an incomplete figure without percent",
			got.Totals.Cost.Complete, got.Totals.Cost.HasPercent)
	}
}

func TestCompareRuns_ZeroBaseHasAChangeButNoPercent(t *testing.T) {
	a := rwRun{dispatches: []rwDispatch{cmpOK("coder#1", "coder", 0)}}
	b := rwRun{dispatches: []rwDispatch{cmpOK("coder#1", "coder", 1000)}}

	row := cmpRow(t, cmpCompare(t, a, b), "coder")

	cmpAssertQty(t, "tokens", row.Tokens, 0, 1000, domain.ChangeIncrease)
	cmpAssertCost(t, "cost", row.Cost, 0, 1000*3000, domain.ChangeIncrease)
}

func TestCompareRuns_AveragePeakContextIsAbsentWithoutPerCallRecords(t *testing.T) {
	legacy := cmpOK("coder#1", "coder", 0)
	legacy.usage = []domain.InvocationUsage{ctxLegacyEntry(0, 1000, 0, 0, 0)}
	a := rwRun{dispatches: []rwDispatch{legacy}}
	b := rwRun{dispatches: []rwDispatch{cmpOK("coder#1", "coder", 1000)}}

	got := cmpCompare(t, a, b)

	peak := cmpRow(t, got, "coder").AvgPeakContext
	if peak.HasA || !peak.HasB || peak.B != 1000 {
		t.Errorf("average peak: HasA=%v HasB=%v B=%d, want A absent and B=1000", peak.HasA, peak.HasB, peak.B)
	}
	cmpAssertNoChangeFigure(t, "average peak", peak)
	cmpAssertNoChangeFigure(t, "totals average peak", got.Totals.AvgPeakContext)
	if got.Totals.AvgPeakContext.HasA {
		t.Error("totals average peak of A is present, want absent")
	}
}

func TestCompareRuns_RunsLoadedFromADigestCompareFromTheirCarriedFigures(t *testing.T) {
	a := cmpDigestRun(cmpRunA, "coder", 1000, 3_000_000, 800)
	b := cmpDigestRun(cmpRunB, "coder", 2000, 6_000_000, 1200)

	got := analysis.CompareRuns(a, b)

	row := cmpRow(t, got, "coder")
	cmpAssertQty(t, "tokens", row.Tokens, 1000, 2000, domain.ChangeIncrease)
	cmpAssertCost(t, "cost", row.Cost, 3_000_000, 6_000_000, domain.ChangeIncrease)
	cmpAssertQty(t, "average peak context", row.AvgPeakContext, 800, 1200, domain.ChangeIncrease)
	cmpAssertQty(t, "totals invocations", got.Totals.Invocations, 1, 1, domain.ChangeUnchanged)
	cmpAssertCost(t, "totals cost", got.Totals.Cost, 3_000_000, 6_000_000, domain.ChangeIncrease)
	cmpAssertQty(t, "totals average peak", got.Totals.AvgPeakContext, 800, 1200, domain.ChangeIncrease)
}

func TestCompareRuns_DigestRunWithoutPeakContextLeavesItAbsentNeverZero(t *testing.T) {
	a := cmpDigestRun(cmpRunA, "coder", 1000, 3_000_000, -1)
	b := cmpDigestRun(cmpRunB, "coder", 2000, 6_000_000, 1200)

	got := analysis.CompareRuns(a, b)

	peak := cmpRow(t, got, "coder").AvgPeakContext
	if peak.HasA || !peak.HasB || peak.B != 1200 {
		t.Errorf("average peak: HasA=%v HasB=%v B=%d, want A absent and B=1200", peak.HasA, peak.HasB, peak.B)
	}
	cmpAssertNoChangeFigure(t, "average peak", peak)
	if got.Totals.AvgPeakContext.HasA {
		t.Error("totals average peak of A is present, want absent")
	}
	cmpAssertQty(t, "tokens", cmpRow(t, got, "coder").Tokens, 1000, 2000, domain.ChangeIncrease)
}
