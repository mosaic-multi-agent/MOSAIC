package screens_test

// The cursor and the detail strip of the Gantt: three fixed lines with the
// selected invocation's instance and type; logged start, end, duration and
// status; cost, the invocations it ran in parallel with, and peak and final
// context with their band names. Open invocations follow the overlap rule of the
// analysis, so the strip agrees with the bars.

import (
	"strings"
	"testing"
	"time"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func tokens(n int64) string { return screens.FormatTokens(domain.Tokens(n)) }

func TestGanttDetail_StripShowsInstanceAndTypeOfTheCursorEntry(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)

	stripBlock(t, s.View(wideW, tallH), "solo#1", "planner", "SUCCESS", "$3.17")
	press(s, "down")
	stripBlock(t, s.View(wideW, tallH), "alpha#2", "coder", "BLOCKED", "$2.28")
}

func TestGanttDetail_StripShowsLoggedStartEndDurationAndStatus(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)
	cases := []struct {
		start, end, status string
		d                  time.Duration
	}{
		{"10:00:00", "10:00:10", "SUCCESS", 10 * time.Second},
		{"10:00:20", "10:00:50", "BLOCKED", 30 * time.Second},
		{"10:00:30", "10:01:15", "PARTIALLY_DONE", 45 * time.Second},
	}
	for _, c := range cases {
		line := stripLine(t, s.View(wideW, tallH), c.start, c.end, c.status)
		if !hasDuration(line, wantDuration(t, c.d)) {
			t.Errorf("the line %q must carry the duration %q", line, wantDuration(t, c.d))
		}
		press(s, "down")
	}
}

func TestGanttDetail_OpenInvocationShowsOpenWithoutAnInventedDuration(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)
	press(s, repeat("down", 3)...)

	block := stripBlock(t, s.View(wideW, tallH), "rev#4", "reviewer", "10:00:40", "$0.54")
	line := block[1]

	if !openLabelToken.MatchString(line) {
		t.Errorf("rev#4 must carry %q as a separate token: %q", screens.OpenLabel, line)
	}
	for _, d := range []time.Duration{10 * time.Second, 30 * time.Second, 35 * time.Second, 45 * time.Second} {
		if hasDuration(line, wantDuration(t, d)) {
			t.Errorf("an open invocation must not show a duration: %q", line)
		}
	}
}

func TestGanttDetail_InvocationWithoutTimestampsShowsAbsentMarkersAndNoStatus(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)
	press(s, repeat("down", 4)...)
	view := s.View(wideW, tallH)

	stripLine(t, view, "delta#5", screens.UnknownTypeLabel)
	line := stripLine(t, view, screens.NoStatusLabel)

	if n := strings.Count(line, screens.AbsentMarker); n < 3 {
		t.Errorf("start, end and duration of delta#5 are unknown and must each show %q: %q", screens.AbsentMarker, line)
	}
}

func TestGanttDetail_StripShowsCostParallelWithAndPeakAndFinalContextWithBandNames(t *testing.T) {
	cases := []struct {
		down                      int
		cost                      string
		parallel, notParallel     []string
		peak, final, peakBand, fb string
	}{
		{0, "$3.17", nil, []string{"alpha#2", "beta#3", "rev#4"}, tokens(120_000), tokens(60_000), "optimal", "optimal"},
		{1, "$2.28", []string{"beta#3", "rev#4"}, []string{"solo#1"}, tokens(300_000), tokens(90_000), "above optimal", "optimal"},
		{2, "$1.39", []string{"alpha#2", "rev#4"}, []string{"solo#1"}, tokens(380_000), tokens(210_000), "degraded", "optimal"},
		{3, "$0.54", []string{"alpha#2", "beta#3"}, []string{"solo#1"}, tokens(40_000), tokens(40_000), "optimal", "optimal"},
	}
	for _, c := range cases {
		s := openTimeline(t, timelineRun(), wideW, tallH)
		press(s, repeat("down", c.down)...)

		line := stripLine(t, s.View(wideW, tallH), c.cost)

		assertContains(t, line, c.peak, c.final, c.peakBand, c.fb)
		assertNotContains(t, line, "below optimal")
		assertContains(t, line, c.parallel...)
		assertNotContains(t, line, c.notParallel...)
	}
}

func TestGanttDetail_OpenInvocationCountsAsRunningToTheEndOfTheAxisInTheParallelList(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)

	press(s, "down", "down") // beta#3 ends at the axis end, alpha#2 ends at 50s
	line := stripLine(t, s.View(wideW, tallH), "$1.39")
	assertContains(t, line, "rev#4")

	press(s, "down") // the open rev#4 ran in parallel with both
	line = stripLine(t, s.View(wideW, tallH), "$0.54")
	assertContains(t, line, "alpha#2", "beta#3")
}

func TestGanttDetail_BandNamesAreDrawnInTheBandRoles(t *testing.T) {
	colourOn(t)
	s := screens.NewTimelineScreen(timelineRun(), wideW, tallH, paletteStyles())
	press(s, "down", "down") // beta#3: peak degraded, final optimal
	raw := rawLineWith(t, s.View(wideW, tallH), "$1.39")

	assertContains(t, raw, rolePrefix(tuicommon.RoleSignalBad), rolePrefix(tuicommon.RoleSignalGood))

	press(s, "up") // alpha#2: peak above optimal, final optimal
	raw = rawLineWith(t, s.View(wideW, tallH), "$2.28")
	assertContains(t, raw, rolePrefix(tuicommon.RoleSignalWarning), rolePrefix(tuicommon.RoleSignalGood))
	assertNotContains(t, raw, rolePrefix(tuicommon.RoleSignalBad))
}

func TestGanttDetail_StripIsThreeFixedLinesForEveryCursorPosition(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)
	want := lineCount(s.View(wideW, tallH))

	for i := 0; i < 5; i++ {
		if got := lineCount(s.View(wideW, tallH)); got != want {
			t.Errorf("cursor on entry %d: the view has %d lines, want the same %d as on the first entry", i, got, want)
		}
		press(s, "down")
	}
	view := newTimelineScreen(timelineRun()).View(wideW, tallH)
	first := lineIndex(stripStyling(view), "planner")
	second := lineIndex(stripStyling(view), "SUCCESS")
	third := lineIndex(stripStyling(view), "$3.17")
	if first < 0 || second != first+1 || third != first+2 {
		t.Errorf("the strip is three consecutive lines (instance and type, times and status, cost and context): lines %d, %d, %d", first, second, third)
	}
}

func TestGanttDetail_CursorMovesWithDownAndStopsAtTheEnds(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)

	press(s, repeat("up", 3)...)
	stripBlock(t, s.View(wideW, tallH), "solo#1", "planner", "SUCCESS", "$3.17")
	press(s, "j", "j", "down")
	stripBlock(t, s.View(wideW, tallH), "rev#4", "reviewer", "10:00:40", "$0.54")
	press(s, "k")
	stripBlock(t, s.View(wideW, tallH), "beta#3", "tester", "PARTIALLY_DONE", "$1.39")
	press(s, repeat("down", 20)...)
	stripBlock(t, s.View(wideW, tallH), "delta#5", screens.UnknownTypeLabel, screens.NoStatusLabel, "")
}

func TestGanttDetail_TimesAreShownAsLoggedWithoutZoneConversion(t *testing.T) {
	zone := time.FixedZone("plus2", 2*3600)
	at := func(sec int) time.Time { return time.Date(2026, 10, 1, 10, 0, sec, 0, zone) }
	run := overviewRun()
	e := tlWithStatus(tlClosed("zoned#1", "worker", at(21), at(53)), "SUCCESS")
	run.Timeline = domain.Timeline{Entries: []domain.TimelineEntry{e}, Axis: domain.TimeInterval{Start: at(0), End: at(60)}, HasAxis: true}

	view := timelineView(t, run, wideW, tallH)

	stripLine(t, view, "10:00:21", "10:00:53", "SUCCESS")
	assertNotContains(t, stripStyling(view), "08:00:")
	axis := axisLineIndex(view)
	if axis < 0 {
		t.Fatalf("the view has no axis line\n%s", stripStyling(view))
	}
	if labels := timeLabels(viewLines(view)[axis]); len(labels) == 0 || labels[0] != "10:00:00" {
		t.Errorf("axis labels are shown in the logged zone, starting 10:00:00: %v", labels)
	}
}

func TestGantt_AxisOfZeroSpanStillDrawsTheLaneWithoutPanicking(t *testing.T) {
	run := overviewRun()
	e := tlWithStatus(tlClosed("only#1", "worker", tlAt(5), tlAt(5)), "SUCCESS")
	run.Timeline = domain.Timeline{Entries: []domain.TimelineEntry{e}, Axis: tlSpan(5, 5), HasAxis: true}

	view := timelineView(t, run, wideW, tallH)

	assertWithinBounds(t, view, wideW, tallH)
	lane := laneLine(t, view, "only#1", "worker")
	if countGlyph(lane, barGlyph) < 1 {
		t.Errorf("an instant invocation on a zero-span axis still shows at least one bar cell: %q", lane)
	}
	stripBlock(t, view, "only#1", "worker", "SUCCESS", "")
}

func TestGanttDetail_AgentWithoutContextDataShowsAbsentMarkersOnTheCostAndContextLine(t *testing.T) {
	run := overviewRun()
	e := tlWithStatus(tlClosed("bare#1", "worker", tlAt(0), tlAt(10)), "SUCCESS")
	run.Timeline = domain.Timeline{Entries: []domain.TimelineEntry{e}, Axis: tlSpan(0, 10), HasAxis: true}

	view := timelineView(t, run, wideW, tallH)

	block := stripBlock(t, view, "bare#1", "worker", "SUCCESS", screens.AbsentMarker)
	if n := strings.Count(block[2], screens.AbsentMarker); n < 2 {
		t.Errorf("cost and context are unknown and must show %q each: %q", screens.AbsentMarker, block[2])
	}
}
