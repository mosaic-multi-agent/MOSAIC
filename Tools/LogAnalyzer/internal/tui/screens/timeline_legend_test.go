package screens_test

// The Gantt's legend: it names both shades in the Overview's terms and the
// compaction markers the screen draws, sits between the lanes and the detail
// strip, counts in the body budget and is left out when it leaves no room for a
// lane.

import (
	"strings"
	"testing"

	"mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	waitingName = "waiting on the user"
	betweenName = "between sessions"
)

// legendIndexes are the indexes of the lines that hold part of the legend: the
// shade names and the compaction label.
func legendIndexes(view string) []int {
	var out []int
	for i, l := range viewLines(view) {
		if strings.Contains(l, waitingName) || strings.Contains(l, betweenName) || strings.Contains(l, "compaction") {
			out = append(out, i)
		}
	}
	return out
}

func laneCount(view string) int {
	n := 0
	for id, typ := range timelineTypes {
		if laneIndex(view, id, typ) >= 0 {
			n++
		}
	}
	return n
}

func TestGanttLegend_NamesBothShadesInTheOverviewsTermsAndTheCompactionMarkers(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)
	text := strings.Join(viewLines(view), "\n")

	for _, want := range []string{waitingName, betweenName, "compaction"} {
		if !strings.Contains(text, want) {
			t.Errorf("the legend must name %q\n%s", want, stripStyling(view))
		}
	}
}

func TestGanttLegend_IsShownEvenWhenTheRunHasNoWaitsOrGaps(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	if len(legendIndexes(view)) == 0 {
		t.Errorf("the legend is part of the Gantt, not of the shading\n%s", stripStyling(view))
	}
}

func TestGanttLegend_SitsBetweenTheLastLaneAndTheDetailStrip(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	idx := legendIndexes(view)
	if len(idx) == 0 {
		t.Fatalf("no legend\n%s", stripStyling(view))
	}
	lastLane := laneIndex(view, "delta#5", screens.UnknownTypeLabel)
	strip := lineIndex(stripStyling(view), "solo#1")
	for i, l := range viewLines(view) {
		if isLaneLine(l, "solo#1", "planner") {
			continue
		}
		if strings.Contains(l, "solo#1") && strings.Contains(l, "planner") {
			strip = i
			break
		}
	}
	if idx[0] <= lastLane || idx[len(idx)-1] >= strip {
		t.Errorf("legend lines %v must lie after the last lane (line %d) and before the detail strip (line %d)\n%s",
			idx, lastLane, strip, stripStyling(view))
	}
}

func TestGanttLegend_SwatchesAreDrawnLikeTheShadesAndMarkersInAsciiMode(t *testing.T) {
	view := asciiTimeline(t, timelineRun(), wideW, tallH)
	text := strings.Join(viewLines(view), "\n")

	if !strings.Contains(text, string(asciiWait)+" "+waitingName) {
		t.Errorf("the waiting swatch is the shade's own character '-' before its name\n%s", text)
	}
	if !strings.Contains(text, string(asciiGap)+" "+betweenName) {
		t.Errorf("the between-sessions swatch is the shade's own character '_' before its name\n%s", text)
	}
	// The Gantt is the lane rows through the legend; the header, the detail
	// strip and the footer keys are other parts of the screen.
	lines := viewLines(view)
	first := len(lines)
	for id, typ := range timelineTypes {
		if i := laneIndex(view, id, typ); i >= 0 && i < first {
			first = i
		}
	}
	idx := legendIndexes(view)
	if len(idx) == 0 || first == len(lines) {
		t.Fatalf("no lanes or legend to check\n%s", text)
	}
	for _, l := range lines[first : idx[len(idx)-1]+1] {
		for _, r := range l {
			if r > 127 {
				t.Fatalf("the Gantt must be 7-bit in ASCII mode, found %q in %q", r, l)
			}
		}
	}
}

func TestGanttLegend_SwatchesCarryTheShadeRolesInColour(t *testing.T) {
	colourOn(t)
	view := screens.NewTimelineScreen(timelineRun(), wideW, tallH, paletteStyles()).View(wideW, tallH)
	theme := tui.PaletteTheme()
	neutral, cool := theme.Style(tui.RoleShadeNeutral).Render(" "), theme.Style(tui.RoleShadeCool).Render(" ")

	var waitLine, gapLine string
	for _, l := range strings.Split(view, "\n") {
		plain := stripStyling(l)
		if strings.Contains(plain, waitingName) {
			waitLine = l
		}
		if strings.Contains(plain, betweenName) {
			gapLine = l
		}
	}

	if !strings.Contains(waitLine, neutral+" "+waitingName) {
		t.Errorf("the waiting swatch is a blank cell in the neutral shade role right before its name: %q", waitLine)
	}
	if !strings.Contains(gapLine, cool+" "+betweenName) {
		t.Errorf("the between-sessions swatch is a blank cell in the cool shade role right before its name: %q", gapLine)
	}
}

func TestGanttLegend_ShowsOnlyTheMarkerKindsTheScreenDraws(t *testing.T) {
	both := timelineView(t, timelineRun(), wideW, tallH)
	onlyLogged := timelineView(t, groupRun(), wideW, tallH)
	legend := func(view string) string {
		var parts []string
		for _, i := range legendIndexes(view) {
			parts = append(parts, viewLines(view)[i])
		}
		return strings.Join(parts, "\n")
	}

	if l := legend(both); !strings.ContainsRune(l, loggedGlyph) || !strings.ContainsRune(l, detectedGlyph) {
		t.Errorf("a screen drawing logged and detected markers explains both: %q", l)
	}
	if l := legend(onlyLogged); !strings.ContainsRune(l, loggedGlyph) || strings.ContainsRune(l, detectedGlyph) {
		t.Errorf("a screen drawing only logged markers explains only that kind: %q", l)
	}
}

func TestGanttLegend_WrapsBetweenEntriesAtNarrowWidthsAndStaysInsideTheTerminal(t *testing.T) {
	for _, w := range []int{70, 50, 36} {
		view := timelineView(t, timelineRun(), w, tallH)
		text := strings.Join(viewLines(view), "\n")

		for _, want := range []string{waitingName, betweenName, "compaction"} {
			if !strings.Contains(text, want) {
				t.Errorf("width %d: the legend must still name %q whole\n%s", w, want, stripStyling(view))
			}
		}
		assertWithinBounds(t, view, w, tallH)
	}
}

func TestGanttLegend_CountsInTheBodyBudgetAndTakesLanesFirstWhenShort(t *testing.T) {
	// The frame takes 4 lines; the axis, orchestrator lane and detail strip take 5 more.
	cases := []struct {
		height     int
		wantLanes  int
		wantLegend bool
	}{
		{height: 4 + 5 + 1, wantLanes: 1, wantLegend: false}, // one lane fits, no room for the legend
		{height: 4 + 5 + 2, wantLanes: 1, wantLegend: true},  // one lane plus the one-line legend
		{height: 4 + 5 + 3, wantLanes: 2, wantLegend: true},
	}
	for _, tc := range cases {
		view := timelineView(t, timelineRun(), wideW, tc.height)

		if got := laneCount(view); got != tc.wantLanes {
			t.Errorf("height %d: %d entry lanes, want %d\n%s", tc.height, got, tc.wantLanes, stripStyling(view))
		}
		if got := len(legendIndexes(view)) > 0; got != tc.wantLegend {
			t.Errorf("height %d: legend shown = %v, want %v\n%s", tc.height, got, tc.wantLegend, stripStyling(view))
		}
		assertWithinBounds(t, view, wideW, tc.height)
	}
}

func TestGanttLegend_IsNotShownWithoutAnyInvocationOrCompaction(t *testing.T) {
	run := timelineRun()
	run.Timeline = domain.Timeline{}

	view := timelineView(t, run, wideW, tallH)

	text := strings.Join(viewLines(view), "\n")
	if strings.Contains(text, waitingName) || strings.Contains(text, betweenName) {
		t.Errorf("an empty timeline shows its notice only\n%s", stripStyling(view))
	}
}
