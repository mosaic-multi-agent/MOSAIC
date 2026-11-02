package screens_test

// The Gantt version of the timeline: one lane per invocation in dispatch order
// under a shared time axis, bars in the status roles, open invocations drawn
// open-ended, agent-coloured labels and no "parallel with" text on the lanes.

import (
	"sort"
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestGantt_OneLanePerInvocationInDispatchOrderBelowTheAxisAndOrchestratorLane(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	for id, typ := range timelineTypes {
		n := 0
		for _, l := range viewLines(view) {
			if isLaneLine(l, id, typ) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s must have exactly one lane, found %d\n%s", id, n, stripStyling(view))
		}
	}
	axis, orch := axisLineIndex(view), orchestratorLaneIndex(view)
	if axis < 0 || orch < 0 || axis >= orch {
		t.Fatalf("the axis (line %d) must come before the orchestrator lane (line %d)\n%s", axis, orch, stripStyling(view))
	}
	if first := laneIndex(view, "solo#1", "planner"); first <= orch {
		t.Errorf("entry lanes must follow the orchestrator lane\n%s", stripStyling(view))
	}
	assertLaneOrder(t, view, timelineTypes, "solo#1", "alpha#2", "beta#3", "rev#4", "delta#5")
}

func TestGantt_KeepsGivenOrderRegardlessOfDurationOrIdOrder(t *testing.T) {
	view := timelineView(t, orderRun(), wideW, tallH)

	assertLaneOrder(t, view, map[string]string{"mid-1": "coder", "long-2": "tester", "short-3": "planner"},
		"mid-1", "long-2", "short-3")
}

func TestGantt_AxisHasTickLabelsAcrossTheLoggedSpan(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	i := axisLineIndex(view)
	if i < 0 {
		t.Fatalf("the view has no axis line with tick labels\n%s", stripStyling(view))
	}
	labels := timeLabels(viewLines(view)[i])
	if labels[0] != "10:00:00" {
		t.Errorf("the first tick label = %q, want the start of the span 10:00:00: %v", labels[0], labels)
	}
	if last := labels[len(labels)-1]; last < "10:01:00" {
		t.Errorf("the last tick label = %q, want a tick in the last quarter of the span: %v", last, labels)
	}
	if !sort.StringsAreSorted(labels) {
		t.Errorf("tick labels must increase from left to right: %v", labels)
	}
}

func TestGantt_BarsShareTheAxisAndSpanTheirLoggedTimes(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)
	solo := glyphCols(laneLine(t, view, "solo#1", "planner"), barGlyph)
	alpha := glyphCols(laneLine(t, view, "alpha#2", "coder"), barGlyph)
	beta := glyphCols(laneLine(t, view, "beta#3", "tester"), barGlyph)
	if len(solo) == 0 || len(alpha) == 0 || len(beta) == 0 {
		t.Fatalf("every invocation with logged times needs a bar: %v %v %v", solo, alpha, beta)
	}

	first := func(c []int) int { return c[0] }
	end := func(c []int) int { return c[len(c)-1] }
	if end(solo) >= first(alpha) {
		t.Errorf("solo#1 (0-10s) ends before alpha#2 (20-50s) starts: columns %d..%d and %d..%d", first(solo), end(solo), first(alpha), end(alpha))
	}
	if !(first(alpha) < first(beta) && first(beta) <= end(alpha)) {
		t.Errorf("alpha#2 (20-50s) and beta#3 (30-75s) must overlap: columns %d..%d and %d..%d", first(alpha), end(alpha), first(beta), end(beta))
	}
	if !(len(solo) < len(alpha) && len(alpha) < len(beta)) {
		t.Errorf("bar lengths must follow the durations 10s < 30s < 45s: %d, %d, %d cells", len(solo), len(alpha), len(beta))
	}
}

func TestGantt_BarsAreColouredByStatus(t *testing.T) {
	colourOn(t)
	view := screens.NewTimelineScreen(timelineRun(), wideW, tallH, paletteStyles()).View(wideW, tallH)

	cases := []struct {
		id, typ string
		role    tuicommon.StyleRole
	}{
		{"solo#1", "planner", tuicommon.RoleSignalGood},
		{"alpha#2", "coder", tuicommon.RoleSignalBad},
		{"beta#3", "tester", tuicommon.RoleSignalWarning},
		{"rev#4", "reviewer", tuicommon.RoleSignalWarning},
	}
	for _, c := range cases {
		raw := rawLaneLine(t, view, c.id, c.typ)
		if !strings.Contains(raw, rolePrefix(c.role)+string(barGlyph)) {
			t.Errorf("the bar of %s must be drawn in role %q: %q", c.id, c.role, raw)
		}
	}
	alpha := rawLaneLine(t, view, "alpha#2", "coder")
	for _, other := range []tuicommon.StyleRole{tuicommon.RoleSignalGood, tuicommon.RoleSignalWarning} {
		if strings.Contains(alpha, rolePrefix(other)+string(barGlyph)) {
			t.Errorf("the BLOCKED bar of alpha#2 must not use role %q: %q", other, alpha)
		}
	}
}

func TestGantt_OpenInvocationRunsOpenEndedToTheEndOfTheAxisInTheWarningRole(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)
	rev := laneLine(t, view, "rev#4", "reviewer")
	beta := laneLine(t, view, "beta#3", "tester")

	if !strings.HasSuffix(strings.TrimRight(rev, " "), string(openEndGlyph)) {
		t.Errorf("an open bar must end in %q: %q", string(openEndGlyph), rev)
	}
	if lastCol(rev) != lastCol(beta) {
		t.Errorf("the open bar must reach the axis end, where beta#3 (ends at the axis end) stops: column %d vs %d", lastCol(rev), lastCol(beta))
	}
	rc, ac := glyphCols(rev, barGlyph), glyphCols(laneLine(t, view, "alpha#2", "coder"), barGlyph)
	if len(rc) == 0 || len(ac) == 0 || rc[0] <= ac[0] {
		t.Errorf("the open bar starts at its logged start 40s, after alpha#2 (20s): %v vs %v", rc, ac)
	}
	for id, typ := range map[string]string{"solo#1": "planner", "alpha#2": "coder", "beta#3": "tester"} {
		if strings.Contains(laneLine(t, view, id, typ), string(openEndGlyph)) {
			t.Errorf("%s is closed and must not be drawn open-ended", id)
		}
	}

	colourOn(t)
	raw := rawLaneLine(t, screens.NewTimelineScreen(timelineRun(), wideW, tallH, paletteStyles()).View(wideW, tallH), "rev#4", "reviewer")
	if !strings.Contains(raw, rolePrefix(tuicommon.RoleSignalWarning)+string(barGlyph)) {
		t.Errorf("the open bar must use the warning role: %q", raw)
	}
}

func TestGantt_LabelsAreTheInstanceInTheAgentColourAndTheCursorLaneIsSelected(t *testing.T) {
	colourOn(t)
	s := screens.NewTimelineScreen(timelineRun(), wideW, tallH, paletteStyles())
	view := s.View(wideW, tallH)

	if raw := rawLaneLine(t, view, "alpha#2", "coder"); !strings.Contains(raw, rolePrefix(tuicommon.RoleDataActor)+"alpha#2") {
		t.Errorf("a lane label must be the instance in the agent colour: %q", raw)
	}
	if raw := rawLaneLine(t, view, "solo#1", "planner"); !strings.Contains(raw, rolePrefix(tuicommon.RoleSelected)+"solo#1") {
		t.Errorf("the label of the cursor lane must use the selected role: %q", raw)
	}
	press(s, "down")
	moved := s.View(wideW, tallH)
	if raw := rawLaneLine(t, moved, "alpha#2", "coder"); !strings.Contains(raw, rolePrefix(tuicommon.RoleSelected)+"alpha#2") {
		t.Errorf("after down the label of alpha#2 must use the selected role: %q", raw)
	}
	if raw := rawLaneLine(t, moved, "solo#1", "planner"); !strings.Contains(raw, rolePrefix(tuicommon.RoleDataActor)+"solo#1") {
		t.Errorf("the label of solo#1 returns to the agent colour: %q", raw)
	}
}

func TestGantt_LanesCarryNoParallelWithText(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	for id, typ := range timelineTypes {
		if l := laneLine(t, view, id, typ); strings.Contains(strings.ToLower(l), "parallel with") {
			t.Errorf("the lane of %s must not say \"parallel with\": %q", id, l)
		}
	}
}

func TestGantt_InvocationWithoutTimestampsKeepsALaneWithoutABar(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	lane := laneLine(t, view, "delta#5", screens.UnknownTypeLabel)

	if strings.ContainsRune(lane, barGlyph) || strings.ContainsRune(lane, openEndGlyph) {
		t.Errorf("delta#5 has no logged times and must be drawn without a bar: %q", lane)
	}
}

func TestGantt_RunWithoutAnAxisStillListsTheLanesWithinBounds(t *testing.T) {
	run := timelineRun()
	run.Timeline.Axis, run.Timeline.HasAxis = domain.TimeInterval{}, false

	view := timelineView(t, run, 80, tallH)

	for id, typ := range timelineTypes {
		laneLine(t, view, id, typ)
	}
	assertWithinBounds(t, view, 80, tallH)
}

func TestGantt_RunWithoutInvocationsStillRenders(t *testing.T) {
	view := timelineView(t, overviewRun(), wideW, tallH)

	if strings.TrimSpace(view) == "" {
		t.Error("a run without a timeline must still render a page")
	}
	assertWithinBounds(t, view, wideW, tallH)
}

func TestGantt_NothingIsDrawnPastTheTerminalAtAnySize(t *testing.T) {
	colourOff(t)
	for _, run := range []domain.RunReport{timelineRun(), manyTimelineRun(150), groupRun()} {
		s := screens.NewTimelineScreen(run, 80, 30, paletteStyles())
		for height := 0; height <= 30; height++ {
			for _, width := range []int{120, 80, 60, 40, 30, 20, 10} {
				assertWithinBounds(t, s.View(width, height), width, height)
			}
		}
	}
}

func TestGantt_LanesShowNoMoneyOrPricingMarkers(t *testing.T) {
	view := timelineView(t, timelineRun(), wideW, tallH)

	for id, typ := range timelineTypes {
		assertNotContains(t, laneLine(t, view, id, typ), "$", screens.UnpricedMarker, "(partial)", screens.AssumedTierLabel(nil))
	}
	if i := orchestratorLaneIndex(view); i >= 0 {
		assertNotContains(t, viewLines(view)[i], "$", screens.UnpricedMarker)
	}
}

func TestGantt_NarrowTerminalNeverExceedsItsWidth(t *testing.T) {
	const narrow = 40

	view := timelineView(t, timelineRun(), narrow, tallH)

	assertWithinBounds(t, view, narrow, tallH)
	assertNameShown(t, view, "solo#1", 4)
}
