package screens_test

// The comparison table: one row per agent type and a totals row with both runs'
// figures and the change between them, the only-in markers, absent figures and
// the sort order.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestCompareTable_TypeRowShowsBothRunsAndTheChange(t *testing.T) {
	colourOff(t)
	row := rowFor(t, stripStyling(compareView(tableW, tallH)), "builder")

	assertContains(t, row, "$10.00", "$15.00", "+$5.00", "1,000", "3,000", "+2,000", "100,000", "150,000", "+50,000")
	if !hasField(row, "+4") {
		t.Errorf("the invocation change +4 (5 to 9) is missing: %q", row)
	}
	if !percentUp50.MatchString(row) {
		t.Errorf("the cost change of +50%% is missing: %q", row)
	}
}

func TestCompareTable_DecreaseRowCarriesMinusSigns(t *testing.T) {
	colourOff(t)
	row := rowFor(t, stripStyling(compareView(tableW, tallH)), "reviewer")

	assertContains(t, row, "$8.00", "$2.00", "-$6.00", "-10,000")
	if !percentDown75.MatchString(row) {
		t.Errorf("the cost change of -75%% is missing: %q", row)
	}
	if !hasField(row, "+1") {
		t.Errorf("an invocation increase must read +1 (4 to 5): %q", row)
	}
}

func TestCompareTable_RowWithoutChangeCarriesNoSign(t *testing.T) {
	colourOff(t)
	row := rowFor(t, stripStyling(compareView(tableW, tallH)), "tester")

	if strings.ContainsAny(row, "+-") {
		t.Errorf("a row with no change must carry neither + nor -: %q", row)
	}
	if !hasField(row, "0") {
		t.Errorf("a row with no change must show the change 0 for invocations: %q", row)
	}
}

func TestCompareTable_TotalsRowShowsRunTotalsAndTheirChange(t *testing.T) {
	colourOff(t)

	line := totalsLine(t, compareView(tableW, tallH))

	assertContains(t, line, "$30.00", "$25.00", "-$5.00", "3,400", "4,850", "+1,450", "100,000", "110,000", "+10,000")
	if !regexpMatch(`-16(\.7)?%|-17%`, line) {
		t.Errorf("the totals cost change of -16.7%% is missing: %q", line)
	}
}

func TestCompareTable_OnlyInARowIsMarkedAndItsMissingSideIsAbsent(t *testing.T) {
	colourOff(t)
	row := rowFor(t, stripStyling(compareView(tableW, tallH)), "planner")

	assertContains(t, row, "only in A", "$4.00", "800", "95,000", screens.AbsentMarker)
	assertNotContains(t, row, "only in B", "$0.00")
	if hasField(row, "0") || strings.ContainsAny(row, "+-") {
		t.Errorf("an absent side must never read 0 and a row with no counterpart has no change sign: %q", row)
	}
}

func TestCompareTable_OnlyInBRowIsMarkedAndItsMissingSideIsAbsent(t *testing.T) {
	colourOff(t)
	row := rowFor(t, stripStyling(compareView(tableW, tallH)), "deployer")

	assertContains(t, row, "only in B", "$1.00", "250", "70,000", screens.AbsentMarker)
	assertNotContains(t, row, "only in A", "$0.00")
	if hasField(row, "0") || strings.ContainsAny(row, "+-") {
		t.Errorf("an absent side must never read 0 and a row with no counterpart has no change sign: %q", row)
	}
}

func TestCompareTable_TypesInBothRunsCarryNoOnlyInMarker(t *testing.T) {
	colourOff(t)
	view := stripStyling(compareView(tableW, tallH))

	for _, typ := range []string{"builder", "reviewer", "linter", "tester"} {
		assertNotContains(t, rowFor(t, view, typ), "only in")
	}
	assertNotContains(t, totalsLine(t, view), "only in")
}

func TestCompareTable_AFigureARunLacksIsAbsentNeverZero(t *testing.T) {
	colourOff(t)
	// linter has no average peak context in run A, as for a run that carries no context figures.
	row := rowFor(t, stripStyling(compareView(tableW, tallH)), "linter")

	assertContains(t, row, "90,000", screens.AbsentMarker)
	if hasField(row, "0") {
		t.Errorf("the absent average peak context of run A must not read 0: %q", row)
	}
}

func TestCompareTable_UnpricedCostShowsTheMarkerAndNoMoneyChange(t *testing.T) {
	colourOff(t)
	legacy := domain.ComparisonRow{AgentType: "legacy", Invocations: cmpQty(1, 1), Tokens: cmpQty(100, 200),
		Cost: domain.ComparedMoney{A: domain.UnpricedMoney(), B: known(200), Delta: domain.NoMoneyData()}}

	row := rowFor(t, stripStyling(newCompare(comparisonOf(legacy), tableW, tallH).View(tableW, tallH)), "legacy")

	assertContains(t, row, screens.UnpricedMarker, "$2.00")
	assertNotContains(t, row, "$0.00", "-$", "+$")
}

func TestCompareTable_ChangeFromZeroCostHasNoPercent(t *testing.T) {
	colourOff(t)
	fresh := domain.ComparisonRow{AgentType: "fresh", Invocations: cmpQty(1, 2), Tokens: cmpQty(100, 200), Cost: cmpCost(0, 200),
		AvgPeakContext: cmpQty(1000, 2000)}

	row := rowFor(t, stripStyling(newCompare(comparisonOf(fresh), tableW, tallH).View(tableW, tallH)), "fresh")

	assertContains(t, row, "$0.00", "$2.00", "+$2.00", screens.AbsentMarker)
	assertNotContains(t, row, "Inf", "NaN")
	// The only percent on the row is the average peak context change (+100%); the cost percent is absent.
	if n := strings.Count(row, "%"); n != 1 {
		t.Errorf("the row must show exactly one percent (peak context), the cost percent must be absent: %q", row)
	}
}

func TestCompareTable_DefaultSortIsLargestCostChangeAndTheHeaderSaysSo(t *testing.T) {
	colourOff(t)
	view := compareView(tableW, tallH)

	key, desc := activeSort(t, view)

	if key != screens.SortCostChange || !desc {
		t.Errorf("default sort = %q desc=%v, want cost change descending", key, desc)
	}
	// Largest change regardless of sign first, no change next, rows without a known change after.
	assertAppearInOrder(t, view, compareTypes, true)
	assertTotalsLast(t, view, compareTypes...)
}

func TestCompareTable_SortKeyCyclesThroughEveryOrderAndWrapsAround(t *testing.T) {
	colourOff(t)
	s := newCompare(compareFixture(), tableW, tallH)
	ascending := map[screens.SortKey][]string{
		screens.SortCostChange:  {"tester", "linter", "builder", "reviewer"},
		screens.SortTokens:      {"tester", "reviewer", "builder"},
		screens.SortInvocations: {"tester", "reviewer", "builder"},
		screens.SortName:        {"builder", "deployer", "linter", "planner", "reviewer", "tester"},
	}

	seen := walkSorts(t, s, tableW, tallH, ascending,
		screens.SortCostChange, screens.SortTokens, screens.SortInvocations, screens.SortName)

	if len(seen) == 0 || seen[0] != screens.SortCostChange {
		t.Errorf("the cycle must start on cost change, got %v", seen)
	}
}

func TestCompareTable_SortingNeverDropsARowAndTheTotalsRowStaysLast(t *testing.T) {
	colourOff(t)
	s := newCompare(compareFixture(), tableW, tallH)

	for i := 0; i < 5; i++ {
		view := stripStyling(s.View(tableW, tallH))
		for _, typ := range compareTypes {
			rowFor(t, view, typ)
		}
		assertTotalsLast(t, view, compareTypes...)
		press(s, screens.KeySort)
	}
}
