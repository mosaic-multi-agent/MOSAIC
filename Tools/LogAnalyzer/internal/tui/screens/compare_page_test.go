package screens_test

// The comparison page: header with both run labels, the page frame without a
// tab bar, the help line and overlay, back navigation and bounds.

import (
	"fmt"
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestComparePage_HeaderNamesTheScreenAndBothRuns(t *testing.T) {
	colourOff(t)

	lines := viewLines(compareView(tableW, tallH))

	assertContains(t, headOf(lines), "Compare", "A "+compareRunA, "B "+compareRunB)
	assertNotContains(t, headOf(lines), screens.ProvisionalLabel)
	assertNotContains(t, strings.Join(lines, "\n"), "1 Dispatches")
}

func TestComparePage_ProvisionalRunIsMarkedNextToItsLabel(t *testing.T) {
	colourOff(t)
	for _, side := range []string{"A", "B"} {
		cmp := compareFixture()
		cmp.ProvisionalA, cmp.ProvisionalB = side == "A", side == "B"

		head := headOf(viewLines(newCompare(cmp, tableW, tallH).View(tableW, tallH)))

		if n := strings.Count(head, screens.ProvisionalLabel); n != 1 {
			t.Fatalf("provisional %s: header shows %q %d times, want once: %q", side, screens.ProvisionalLabel, n, head)
		}
		badge := side + " " + map[string]string{"A": compareRunA, "B": compareRunB}[side]
		if i := strings.Index(head, badge+" "+screens.ProvisionalLabel); i < 0 {
			t.Errorf("provisional %s: the marker must follow the run label of %s: %q", side, side, head)
		}
	}
}

func TestComparePage_RunLabelsAreDrawnInTheActorRoleAndProvisionalInTheWarningRole(t *testing.T) {
	colourOn(t)
	cmp := compareFixture()
	cmp.ProvisionalA = true

	head := rawLineWith(t, newCompare(cmp, tableW, tallH).View(tableW, tallH), "Compare")

	assertDrawnInRole(t, head, "A "+compareRunA+" "+screens.ProvisionalLabel, tuicommon.RoleSignalWarning)
	assertDrawnInRole(t, head, "B "+compareRunB, tuicommon.RoleDataActor)
}

func TestComparePage_ShortHelpLineListsSortBackAndTheHelpKey(t *testing.T) {
	colourOff(t)

	help := tailOf(viewLines(compareView(tableW, tallH)))

	assertContains(t, help, screens.KeySort+" sort", "esc", "? help")
}

func TestComparePage_HelpOverlayListsSortAndBackWithoutViews(t *testing.T) {
	colourOff(t)
	s := newCompare(compareFixture(), 100, 40)

	overlay := stripStyling(openOverlay(s, 100, 40))

	assertContains(t, overlay, "This screen", "Navigation")
	assertNotContains(t, overlay, "Views", "builder")
	if !regexpMatch(`(?m)^\s+`+screens.KeySort+`\s+.*(?i:sort)`, overlay) {
		t.Errorf("overlay must list the sort key with its purpose\n%s", overlay)
	}
	if !regexpMatch(`(?m)^\s+esc\s+\S`, overlay) {
		t.Errorf("overlay must list esc\n%s", overlay)
	}
	assertWithinBounds(t, overlay, 100, 40)
}

func TestComparePage_QuestionMarkAndEscCloseTheOverlayWithoutLeaving(t *testing.T) {
	colourOff(t)
	for _, closer := range []string{screens.KeyHelp, "esc"} {
		s := newCompare(compareFixture(), 100, 40)
		before := s.View(100, 40)

		overlay := stripStyling(openOverlay(s, 100, 40))
		assertContains(t, overlay, "This screen")
		press(s, closer)

		after := s.View(100, 40)
		if after != before {
			t.Errorf("%q must close the overlay and restore the comparison", closer)
		}
		assertContains(t, rowFor(t, stripStyling(after), "builder"), "$10.00", "$15.00")
		assertContains(t, totalsLine(t, after), "$30.00", "$25.00")
		if s.Back() {
			t.Errorf("%q closing the overlay must not leave the comparison", closer)
		}
	}
}

func TestComparePage_EscRequestsBack(t *testing.T) {
	s := newCompare(compareFixture(), tableW, tallH)
	if s.Back() {
		t.Fatal("setup: Back() is true before any key")
	}

	press(s, "esc")

	if !s.Back() {
		t.Error("esc must request the way back to the runs list")
	}
}

func TestComparePage_StaysWithinBoundsAtEightyColumns(t *testing.T) {
	colourOff(t)
	long := compareRows()
	long[0].AgentType = "an-extremely-long-agent-type-name-that-cannot-fit-in-eighty-columns"
	for _, size := range [][2]int{{80, 24}, {80, 10}, {80, 4}, {80, 3}, {60, 24}, {40, 12}} {
		view := newCompare(comparisonOf(long...), size[0], size[1]).View(size[0], size[1])

		assertWithinBounds(t, view, size[0], size[1])
	}
}

func TestComparePage_LongTypeNameIsTruncatedButKeepsItsFiguresAtEightyColumns(t *testing.T) {
	colourOff(t)
	long := compareRows()
	long[0].AgentType = "an-extremely-long-agent-type-name-that-cannot-fit-in-eighty-columns"

	view := stripStyling(newCompare(comparisonOf(long...), 80, 24).View(80, 24))

	assertWithinBounds(t, view, 80, 24)
	assertContains(t, rowFor(t, view, "builder"), "$10.00", "$15.00", "+$5.00")
	assertContains(t, totalsLine(t, view), "$30.00", "$25.00", "-$5.00")
	if n := strings.Count(view, "$3.00"); n < 2 {
		t.Errorf("the row of the long type name must still show its cost figures ($3.00 twice), found %d\n%s", n, view)
	}
}

func TestComparePage_KeepsTheCostFiguresOfEveryRowAtEightyColumns(t *testing.T) {
	colourOff(t)
	view := stripStyling(compareView(80, 24))

	assertContains(t, rowFor(t, view, "builder"), "$10.00", "$15.00", "+$5.00")
	assertContains(t, rowFor(t, view, "reviewer"), "$8.00", "$2.00", "-$6.00")
	assertContains(t, totalsLine(t, view), "$30.00", "$25.00", "-$5.00")
}

func TestComparePage_ManyTypesStayWithinTheHeight(t *testing.T) {
	colourOff(t)
	var rows []domain.ComparisonRow
	for i := 1; i <= 40; i++ {
		rows = append(rows, domain.ComparisonRow{AgentType: fmt.Sprintf("type-%02d", i), Invocations: cmpQty(1, 2),
			Tokens: cmpQty(100, 200), Cost: cmpCost(int64(100*i), int64(100*i+i)), AvgPeakContext: cmpQty(1000, 2000)})
	}

	view := newCompare(comparisonOf(rows...), 80, 12).View(80, 12)

	assertWithinBounds(t, view, 80, 12)
	assertContains(t, stripStyling(view), "type-40")
}
