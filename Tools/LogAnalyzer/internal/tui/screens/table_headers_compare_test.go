package screens_test

// The comparison view with more agent types than fit: its wrapped header is
// counted in the row budget, so scrolled to the end the last type row and the
// always-shown totals row are both on screen and the hint agrees with the rows.

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

var typeRowRE = regexp.MustCompile(`type-\d\d`)

// manyTypesComparison compares two runs that each hold n agent types.
func manyTypesComparison(n int) domain.RunComparison {
	var rows []domain.ComparisonRow
	for i := 1; i <= n; i++ {
		rows = append(rows, domain.ComparisonRow{AgentType: fmt.Sprintf("type-%02d", i), Invocations: cmpQty(1, 2),
			Tokens: cmpQty(100, 200), Cost: cmpCost(int64(100*i), int64(100*i+i)), AvgPeakContext: cmpQty(1000, 2000)})
	}
	return comparisonOf(rows...)
}

// drawnTypeRows counts the lines of the view that hold an agent type row.
func drawnTypeRows(view string) int {
	n := 0
	for _, l := range viewLines(view) {
		if typeRowRE.MatchString(l) {
			n++
		}
	}
	return n
}

func TestTableHeight_ScrolledCompareShowsTheLastTypeAndTheTotalsRow(t *testing.T) {
	const types = 30
	for _, h := range shortHeights {
		t.Run(strconv.Itoa(h), func(t *testing.T) {
			colourOff(t)
			s := newCompare(manyTypesComparison(types), heightW, h)
			press(s, repeat("down", 500)...)
			view := s.View(heightW, h)

			assertWithinBounds(t, view, heightW, h)
			first, last, total := hintOf(t, view)
			if last != total || total != types {
				t.Errorf("the window ends at row %d of %d, want the last of %d\n%s", last, total, types, stripStyling(view))
			}
			if got, want := drawnTypeRows(view), last-first+1; got != want {
				t.Errorf("the hint reports rows %d-%d (%d rows) but %d rows are on screen\n%s", first, last, want, got, stripStyling(view))
			}
			// The default sort is the largest cost change first, so the last row is type-01.
			assertContains(t, stripStyling(view), "type-01")
			totalsLine(t, view)
		})
	}
}

func TestTableHeight_CompareAtTheTopShowsTheTotalsRowAndMatchesTheHint(t *testing.T) {
	for _, h := range shortHeights {
		t.Run(strconv.Itoa(h), func(t *testing.T) {
			colourOff(t)
			view := newCompare(manyTypesComparison(30), heightW, h).View(heightW, h)

			assertWithinBounds(t, view, heightW, h)
			first, last, _ := hintOf(t, view)
			if got, want := drawnTypeRows(view), last-first+1; got != want {
				t.Errorf("the hint reports rows %d-%d (%d rows) but %d rows are on screen\n%s", first, last, want, got, stripStyling(view))
			}
			totalsLine(t, view)
		})
	}
}
