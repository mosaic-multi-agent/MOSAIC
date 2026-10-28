package screens_test

// Fixture for the comparison view: two runs, six agent types and a totals row.
// Every figure is distinct so a figure on a row identifies its column, and the
// types are listed in an order that is none of the sort orders.
//
//	type      cost A -> B (change)       tokens A -> B   invocations A -> B  avg peak A -> B
//	reviewer  $8.00  -> $2.00  (-$6.00)  800 -> 900      4 -> 5              120,000 -> 110,000
//	builder   $10.00 -> $15.00 (+$5.00)  1,000 -> 3,000  5 -> 9              100,000 -> 150,000
//	linter    $5.00  -> $4.00  (-$1.00)  500 -> 400      3 -> 2              absent -> 90,000
//	tester    $3.00  -> $3.00  (none)    300 -> 300      2 -> 2              80,000 -> 80,000
//	planner   $4.00 only in A            800              2                  95,000
//	deployer  $1.00 only in B            250              1                  70,000

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	compareRunA = "20261028T100000Z-aaaa"
	compareRunB = "20261028T110000Z-bbbb"
)

var compareTypes = []string{"reviewer", "builder", "linter", "tester", "planner", "deployer"}

func cmpDirection(delta int64) domain.ChangeDirection {
	switch {
	case delta > 0:
		return domain.ChangeIncrease
	case delta < 0:
		return domain.ChangeDecrease
	}
	return domain.ChangeUnchanged
}

// cmpQty is a quantity present in both runs.
func cmpQty(a, b int64) domain.ComparedQuantity {
	q := domain.ComparedQuantity{A: a, B: b, HasA: true, HasB: true, Delta: b - a, Direction: cmpDirection(b - a)}
	if a != 0 {
		q.HasPercent = true
		q.Percent = domain.Ratio{Numerator: b - a, Denominator: a}
	}
	return q
}

func cmpQtyOnlyA(a int64) domain.ComparedQuantity { return domain.ComparedQuantity{A: a, HasA: true} }
func cmpQtyOnlyB(b int64) domain.ComparedQuantity { return domain.ComparedQuantity{B: b, HasB: true} }

// cmpCost is a complete cost present in both runs, given in cents.
func cmpCost(a, b int64) domain.ComparedMoney {
	m := domain.ComparedMoney{A: known(a), B: known(b), Delta: known(b - a), Complete: true, Direction: cmpDirection(b - a)}
	if a != 0 {
		m.HasPercent = true
		m.Percent = domain.Ratio{Numerator: b - a, Denominator: a}
	}
	return m
}

func cmpCostOnlyA(a int64) domain.ComparedMoney {
	return domain.ComparedMoney{A: known(a), B: domain.NoMoneyData(), Delta: domain.NoMoneyData(), Complete: true}
}

func cmpCostOnlyB(b int64) domain.ComparedMoney {
	return domain.ComparedMoney{A: domain.NoMoneyData(), B: known(b), Delta: domain.NoMoneyData(), Complete: true}
}

func compareRows() []domain.ComparisonRow {
	linter := domain.ComparisonRow{AgentType: "linter", Invocations: cmpQty(3, 2), Tokens: cmpQty(500, 400),
		Cost: cmpCost(500, 400), AvgPeakContext: cmpQtyOnlyB(90_000)}
	return []domain.ComparisonRow{
		{AgentType: "tester", Invocations: cmpQty(2, 2), Tokens: cmpQty(300, 300), Cost: cmpCost(300, 300), AvgPeakContext: cmpQty(80_000, 80_000)},
		{AgentType: "planner", Presence: domain.OnlyInA, Invocations: cmpQtyOnlyA(2), Tokens: cmpQtyOnlyA(800),
			Cost: cmpCostOnlyA(400), AvgPeakContext: cmpQtyOnlyA(95_000)},
		{AgentType: "builder", Invocations: cmpQty(5, 9), Tokens: cmpQty(1000, 3000), Cost: cmpCost(1000, 1500), AvgPeakContext: cmpQty(100_000, 150_000)},
		{AgentType: "deployer", Presence: domain.OnlyInB, Invocations: cmpQtyOnlyB(1), Tokens: cmpQtyOnlyB(250),
			Cost: cmpCostOnlyB(100), AvgPeakContext: cmpQtyOnlyB(70_000)},
		linter,
		{AgentType: "reviewer", Invocations: cmpQty(4, 5), Tokens: cmpQty(800, 900), Cost: cmpCost(800, 200), AvgPeakContext: cmpQty(120_000, 110_000)},
	}
}

func compareTotals() domain.ComparisonRow {
	return domain.ComparisonRow{Invocations: cmpQty(16, 17), Tokens: cmpQty(3400, 4850), Cost: cmpCost(3000, 2500),
		AvgPeakContext: cmpQty(100_000, 110_000)}
}

func comparisonOf(rows ...domain.ComparisonRow) domain.RunComparison {
	return domain.RunComparison{A: domain.NamedRun(compareRunA), B: domain.NamedRun(compareRunB), Rows: rows, Totals: compareTotals()}
}

func compareFixture() domain.RunComparison { return comparisonOf(compareRows()...) }

func newCompare(cmp domain.RunComparison, width, height int) *screens.CompareScreen {
	return screens.NewCompareScreen(cmp, width, height, paletteStyles())
}

func compareView(width, height int) string {
	return newCompare(compareFixture(), width, height).View(width, height)
}

// totalsLine is the single line of the view that holds the run-totals row.
func totalsLine(t *testing.T, view string) string {
	t.Helper()
	var found []string
	for _, l := range viewLines(view) {
		if strings.Contains(strings.ToLower(l), "total") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one run-totals line, found %d\n%s", len(found), stripStyling(view))
	}
	return found[0]
}

// assertTotalsLast fails unless the totals row is below every agent type row.
func assertTotalsLast(t *testing.T, view string, types ...string) {
	t.Helper()
	at := lineIndex(stripStyling(view), totalsLine(t, view))
	for _, typ := range types {
		if i := lineIndex(stripStyling(view), typ); i < 0 || i > at {
			t.Errorf("row %q must be listed above the totals row (line %d vs %d)\n%s", typ, i, at, stripStyling(view))
		}
	}
}

var (
	// plusFifty and the like match a signed percent with or without a decimal.
	percentUp50   = regexp.MustCompile(`\+50(\.0)?%`)
	percentDown75 = regexp.MustCompile(`-75(\.0)?%`)
	percentDown20 = regexp.MustCompile(`-20(\.0)?%`)
)

// regexpMatch reports whether the pattern matches anywhere in s.
func regexpMatch(pattern, s string) bool { return regexp.MustCompile(pattern).MatchString(s) }
