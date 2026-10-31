package screens_test

// The Timeline draws collapsed stretches (long waits and breaks) as small markers,
// so a run with a long break spends its plot width on the work. In ASCII mode a
// bar is '=' and a marker is ':' at its first and last cell of ten.

import (
	"regexp"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestTimelineCollapse_BreakBetweenTwoInvocations_GivesTheWorkTheWholePlotWidth(t *testing.T) {
	view := asciiTimeline(t, brokenRun(), brokenWidth, tallH)

	first, second := plotOfLane(t, view, "w1#1"), plotOfLane(t, view, "w2#2")

	if want := repStr("=", 35) + ":" + repStr(" ", 8) + ":" + repStr(" ", 35); first != want {
		t.Errorf("first invocation's lane = %q, want %q", first, want)
	}
	if want := repStr(" ", 35) + ":" + repStr(" ", 8) + ":" + repStr("=", 35); second != want {
		t.Errorf("second invocation's lane = %q, want %q", second, want)
	}
}

func TestTimelineCollapse_MarkerCrossesEveryLaneIncludingTheOrchestrators(t *testing.T) {
	view := asciiTimeline(t, brokenRun(), brokenWidth, tallH)

	orchestrator := plotPart(orchestratorLane(t, view))

	if want := repStr(" ", 35) + ":" + repStr(" ", 8) + ":" + repStr(" ", 35); orchestrator != want {
		t.Errorf("orchestrator lane = %q, want %q", orchestrator, want)
	}
}

func TestTimelineCollapse_AxisRowShowsTheLengthOfTheBreak(t *testing.T) {
	view := asciiTimeline(t, brokenRun(), brokenWidth, tallH)

	axis := lineWithAll(view, "3d 2h")

	if !regexp.MustCompile(`:( {1,2})3d 2h( {1,2}):`).MatchString(axis) {
		t.Errorf("axis row has no \"3d 2h\" centred between two break glyphs\n%s", stripStyling(view))
	}
	if cols := laneGlyphCols(axis, ':'); len(cols) < 2 || !containsInt(cols, 35) || !containsInt(cols, 44) {
		t.Errorf("axis marker is not at plot cells 35..44: %q", plotPart(axis))
	}
}

func TestTimelineCollapse_BreakIsNotAlsoShadedAsBetweenSessions(t *testing.T) {
	view := asciiTimeline(t, brokenRun(), brokenWidth, tallH)

	assertNoGlyph(t, view, asciiGap, "the break is collapsed, not shaded", "orchestrator", "w1#1", "w2#2")
}

func TestTimelineCollapse_WaitingAgentsOwnBarContinuesOnBothSidesOfItsCollapsedWait(t *testing.T) {
	view := asciiTimeline(t, waitingRun(), brokenWidth, tallH)

	got := plotOfLane(t, view, "long#1")

	if want := repStr("=", 35) + ":" + repStr(" ", 8) + ":" + repStr("=", 35); got != want {
		t.Errorf("waiting agent's lane = %q, want %q", got, want)
	}
}

func TestTimelineCollapse_StretchWithoutACollapsibleEntry_StaysToScale(t *testing.T) {
	run := brokenRun()
	run.WallTime.Collapsible = nil

	view := asciiTimeline(t, run, brokenWidth, tallH)

	first := plotOfLane(t, view, "w1#1")
	if n := countGlyph(first, asciiBar); n >= 3 {
		t.Errorf("without a collapsible stretch the first bar is %d cells, want to scale (under 3 of 80)", n)
	}
	if countGlyph(first, ':') != 0 {
		t.Errorf("without a collapsible stretch no marker is drawn: %q", first)
	}
}

func TestTimelineCollapse_DetailStripKeepsTheRealDurationsOfBothSidesOfTheBreak(t *testing.T) {
	s := newCollapsedBrokenScreen(t)

	first := s.View(brokenWidth, tallH)
	second := press(s, "down").View(brokenWidth, tallH)

	real := screens.FormatDuration(secs(collapseWork), true)
	if lineWithAll(first, "w1#1", "coder") == "" || lineWithAll(first, real, "SUCCESS") == "" {
		t.Errorf("strip of the first invocation lacks its real duration %s\n%s", real, stripStyling(first))
	}
	if lineWithAll(second, "w2#2", "tester") == "" || lineWithAll(second, real, "SUCCESS") == "" {
		t.Errorf("strip of the second invocation lacks its real duration %s\n%s", real, stripStyling(second))
	}
}

func newCollapsedBrokenScreen(t *testing.T) screens.Screen {
	t.Helper()
	asciiOn(t)
	return screens.NewTimelineScreen(brokenRun(), brokenWidth, tallH, paletteStyles())
}

func TestTimelineCollapse_ThreeHoursOfWorkWithAThreeDayBreak_UsesThePlotWidthForTheWork(t *testing.T) {
	run := longBreakReport(t)
	view := asciiTimeline(t, run, wideW, tallH)

	for _, id := range []string{"worker#1", "tester#2"} {
		lane := plotPart(laneLine(t, view, id, timelineTypes2[id]))
		if n := countGlyph(lane, asciiBar); n < 15 {
			t.Errorf("%s bar is %d cells, want the work to use the plot width (at least 15)\n%s", id, n, stripStyling(view))
		}
		if cols := glyphCols(lane, ':'); len(cols) != 2 || cols[1]-cols[0] != 9 {
			t.Errorf("%s lane break glyphs at %v, want two, ten cells apart", id, cols)
		}
	}
	if lineWithAll(view, "3d 2h") == "" {
		t.Errorf("axis row does not show the break length 3d 2h\n%s", stripStyling(view))
	}
}

func TestTimelineCollapse_LongBreakFixture_CollapsesExactlyTheBreak(t *testing.T) {
	got := longBreakReport(t).WallTime.Collapsible

	if len(got) != 1 {
		t.Fatalf("Collapsible = %v, want the one 3d 2h break", got)
	}
	if want := time.Date(2026, 10, 31, 10, 30, 0, 0, time.UTC); !got[0].Start.Equal(want) {
		t.Errorf("collapsed break starts %v, want the end of the first session %v", got[0].Start, want)
	}
	if d := got[0].End.Sub(got[0].Start); d != secs(collapseGap) {
		t.Errorf("collapsed break is %v long, want 3d 2h (%v)", d, secs(collapseGap))
	}
}

func TestTimelineCollapse_OverviewFiguresAndRunDurationKeepTheirRealValues(t *testing.T) {
	run := longBreakReport(t)
	asciiOn(t)

	overview := screens.NewOverviewScreen(run, wideW, 150, paletteStyles()).View(wideW, 150)

	if run.WallTime.BetweenSessions != secs(collapseGap) {
		t.Errorf("BetweenSessions = %v, want the real %v", run.WallTime.BetweenSessions, secs(collapseGap))
	}
	if span := run.WallTime.Span.End.Sub(run.WallTime.Span.Start); span != secs(277_200) {
		t.Errorf("Span = %v, want the real 3d 5h", span)
	}
	if want := "Wall time  " + screens.FormatDuration(secs(277_200), true); lineWithAll(overview, want) == "" {
		t.Errorf("Overview does not show the real run duration %q\n%s", want, stripStyling(overview))
	}
}

func TestTimelineCollapse_SampleRun_CollapsesNothing(t *testing.T) {
	run := sampleRun(t)

	if len(run.WallTime.Collapsible) != 0 {
		t.Errorf("the sample run collapses %v, want nothing (its longest break is %v)",
			run.WallTime.Collapsible, longestInterval(run.WallTime.BetweenSessionIntervals))
	}
	asciiOn(t)
	view := screens.NewTimelineScreen(run, wideW, tallH, paletteStyles()).View(wideW, tallH)
	if lineWithAll(view, "3d") != "" || regexp.MustCompile(`:\s{8}:`).MatchString(stripStyling(view)) {
		t.Errorf("the sample run's Timeline shows a collapse marker\n%s", stripStyling(view))
	}
}

func longestInterval(ivs []domain.TimeInterval) time.Duration {
	var longest time.Duration
	for _, iv := range ivs {
		longest = max(longest, iv.End.Sub(iv.Start))
	}
	return longest
}
