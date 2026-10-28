package screens_test

// Compaction markers and shading on the Gantt: one marker per compaction on the
// orchestrator lane or on the owning agent's own lane, shaded wait and
// between-session intervals, and markers that stay text with colour off.

import (
	"sort"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// markerOrder lists the marker glyphs of a lane from left to right.
func markerOrder(line string) string {
	type at struct {
		col int
		r   rune
	}
	var found []at
	for _, g := range []rune{loggedGlyph, detectedGlyph} {
		for _, c := range glyphCols(line, g) {
			found = append(found, at{c, g})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].col < found[j].col })
	var sb strings.Builder
	for _, f := range found {
		sb.WriteRune(f.r)
	}
	return sb.String()
}

func TestGanttMarkers_OrchestratorLaneHasOneMarkerPerCompactionInTimeOrder(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)
	orch := orchestratorLane(t, view)

	// detected at 10s; logged at 25s, the paired one (50s-60s) once, and 72s.
	want := string(detectedGlyph) + string(loggedGlyph) + string(loggedGlyph) + string(loggedGlyph)
	if got := markerOrder(orch); got != want {
		t.Errorf("orchestrator lane markers = %q, want %q (a paired compaction is one marker)\n%s", got, want, orch)
	}
}

func TestGanttMarkers_AnAgentsCompactionsAreOnItsOwnRowOnly(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	want := map[string]string{
		"solo#1":  "",
		"alpha#2": string(loggedGlyph),
		"beta#3":  string(detectedGlyph) + string(loggedGlyph), // detected at 45s, pre-only at 65s
		"rev#4":   "",
		"delta#5": "",
	}
	for id, markers := range want {
		if got := markerOrder(laneLine(t, view, id, timelineTypes[id])); got != markers {
			t.Errorf("markers on the lane of %s = %q, want %q", id, got, markers)
		}
	}
}

func TestGanttMarkers_UnplaceableCompactionsDrawNoMarker(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	logged, detected := 0, 0
	for _, l := range laneAndOrchestratorLines(view) {
		logged += countGlyph(l, loggedGlyph)
		detected += countGlyph(l, detectedGlyph)
	}

	// ghost#10 has no timeline entry and ghost#9 has no time: neither is drawn.
	if logged != 5 || detected != 2 {
		t.Errorf("markers on the lanes = %d logged, %d detected; want 5 and 2 (orchestrator 3+1, alpha#2 1, beta#3 1+1)", logged, detected)
	}
}

func TestGanttMarkers_MarkerSitsInTheColumnOfItsTime(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)
	orch := orchestratorLane(t, view)
	alpha := laneLine(t, view, "alpha#2", "coder")
	bars := glyphCols(alpha, barGlyph) // 20-50s
	if len(bars) == 0 {
		t.Fatalf("alpha#2 has no bar: %q", alpha)
	}
	marker := firstCol(t, alpha, loggedGlyph)

	// The marker at 35s replaces a bar cell: it lies inside the bar, and the
	// orchestrator's markers at 25s and 50s lie at the same axis columns.
	if marker <= bars[0] || marker >= bars[len(bars)-1] {
		t.Errorf("alpha#2's compaction at 35s must sit inside its 20-50s bar: marker %d, bar %d..%d", marker, bars[0], bars[len(bars)-1])
	}
	cols := glyphCols(orch, loggedGlyph)
	if len(cols) < 2 || !(cols[0] < marker && marker < cols[1]) {
		t.Errorf("the marker at 35s must lie between the orchestrator's markers at 25s and 50s: %v, %d", cols, marker)
	}
}

func TestGanttMarkers_StayOnTheirLaneWhenGrouped(t *testing.T) {
	s := openTimeline(t, groupRun(), wideW, tallH)
	before := s.View(wideW, tallH)

	press(s, "g")
	after := s.View(wideW, tallH)

	for _, v := range []string{before, after} {
		if got := countGlyph(laneLine(t, v, "b-2", "builder"), loggedGlyph); got != 1 {
			t.Errorf("b-2 keeps its one compaction marker, got %d\n%s", got, stripStyling(v))
		}
		if got := countGlyph(laneLine(t, v, "b-1", "builder"), loggedGlyph); got != 0 {
			t.Errorf("b-1 has no compaction, got %d markers", got)
		}
	}
}

func TestGanttMarkers_AreTextWithColourOffAndSevenBitInAsciiMode(t *testing.T) {
	asciiOn(t)
	s := newTimelineScreen(timelineRun())

	view := s.View(wideW, tallH)

	orch := orchestratorLane(t, view)
	if got := strings.Count(orch, ":"); got != 3 {
		t.Errorf("ASCII logged markers on the orchestrator lane = %d, want 3: %q", got, orch)
	}
	if got := strings.Count(orch, "*"); got != 1 {
		t.Errorf("ASCII detected markers on the orchestrator lane = %d, want 1: %q", got, orch)
	}
	for _, l := range laneAndOrchestratorLines(view) {
		for _, r := range l {
			if r > 127 {
				t.Errorf("a lane must be 7-bit in ASCII mode, found %q in %q", r, l)
				break
			}
		}
	}
	rev := laneLine(t, view, "rev#4", "reviewer")
	if !strings.HasSuffix(strings.TrimRight(rev, " "), ">") || !strings.Contains(rev, "=") {
		t.Errorf("an ASCII open bar is = cells ending in >: %q", rev)
	}
}

func TestGanttShading_NothingIsShadedWithoutWaitsOrGaps(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	for _, l := range laneAndOrchestratorLines(view) {
		if strings.ContainsRune(l, shadeGlyph) {
			t.Errorf("no interval is shaded without waits or gaps: %q", l)
		}
	}
}

func TestGanttShading_UserWaitIsShadedWhereNoBarCoversIt(t *testing.T) {
	run := withShades(timelineRun(), []domain.TimeInterval{tlSpan(55, 70)}, nil)
	view := timelineView(t, run, wideW, tallH)

	shade := glyphCols(laneLine(t, view, "solo#1", "planner"), shadeGlyph)
	alphaEnd := lastBar(laneLine(t, view, "alpha#2", "coder"))
	betaEnd := lastBar(laneLine(t, view, "beta#3", "tester"))

	if len(shade) == 0 {
		t.Fatalf("the wait from 55s to 70s must be shaded on the solo#1 lane\n%s", stripStyling(view))
	}
	if shade[0] <= alphaEnd || shade[len(shade)-1] > betaEnd {
		t.Errorf("shade columns %d..%d must lie after alpha#2's end (%d) and within the axis (%d)", shade[0], shade[len(shade)-1], alphaEnd, betaEnd)
	}
}

func TestGanttShading_BetweenSessionGapIsShadedOnTheAxis(t *testing.T) {
	run := withShades(timelineRun(), nil, []domain.TimeInterval{tlSpan(12, 18)})
	view := timelineView(t, run, wideW, tallH)

	solo := laneLine(t, view, "solo#1", "planner")
	shade := glyphCols(solo, shadeGlyph)
	alphaStart := firstCol(t, laneLine(t, view, "alpha#2", "coder"), barGlyph)

	if len(shade) == 0 {
		t.Fatalf("the gap from 12s to 18s must be shaded\n%s", stripStyling(view))
	}
	if shade[0] <= lastBar(solo) || shade[len(shade)-1] >= alphaStart {
		t.Errorf("the gap lies between solo#1's end (%d) and alpha#2's start (%d): shade %d..%d", lastBar(solo), alphaStart, shade[0], shade[len(shade)-1])
	}
}

func TestGanttShading_NeverCoversABar(t *testing.T) {
	plain := timelineView(t, timelineRun(), wideW, tallH)
	shaded := timelineView(t, withShades(timelineRun(), []domain.TimeInterval{tlSpan(0, 75)}, nil), wideW, tallH)

	for id, typ := range timelineTypes {
		if id == "delta#5" {
			continue
		}
		a, b := glyphCols(laneLine(t, plain, id, typ), barGlyph), glyphCols(laneLine(t, shaded, id, typ), barGlyph)
		if len(a) == 0 || len(a) != len(b) || a[0] != b[0] || a[len(a)-1] != b[len(b)-1] {
			t.Errorf("shading must not change the bar of %s: %v vs %v", id, a, b)
		}
	}
	if !strings.ContainsRune(laneLine(t, shaded, "alpha#2", "coder"), shadeGlyph) {
		t.Errorf("the span outside alpha#2's bar must be shaded when the whole run is a wait")
	}
}

// lastBar is the column of the last bar cell of a lane, or -1.
func lastBar(line string) int {
	cols := glyphCols(line, barGlyph)
	if len(cols) == 0 {
		return -1
	}
	return cols[len(cols)-1]
}
