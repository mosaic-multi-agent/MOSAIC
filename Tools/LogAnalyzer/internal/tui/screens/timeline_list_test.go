package screens_test

// The list version of the timeline (key v): a table of instance, type, start,
// end, duration, status and overlap count, followed by the compactions table
// with one row per compaction. Open invocations count as running to the end of
// the axis in the overlap count, so the list agrees with the bars.

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// listView opens the list version of the timeline.
func listView(t *testing.T, run domain.RunReport, w, h int) string {
	t.Helper()
	s := openTimeline(t, run, w, h)
	press(s, screens.KeyToggleView)
	return s.View(w, h)
}

// assertListHeader fails unless the view has the invocation table header (a line
// naming instance and overlap), which only the list version draws.
func assertListHeader(t *testing.T, view string) {
	t.Helper()
	for _, l := range viewLines(view) {
		low := strings.ToLower(l)
		if strings.Contains(low, "instance") && strings.Contains(low, "overlap") {
			return
		}
	}
	t.Errorf("the view has no table header naming instance and overlap\n%s", view)
}

// invocationsTable is the part of the list view above the compactions table
// header (the line naming actor and trigger).
func invocationsTable(view string) string {
	var out []string
	for _, l := range viewLines(view) {
		if low := strings.ToLower(l); strings.Contains(low, "actor") && strings.Contains(low, "trigger") {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func TestTimelineList_KeyTogglesBetweenTheGanttAndTheListAndBack(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)
	gantt := s.View(wideW, tallH)

	press(s, screens.KeyToggleView)
	list := s.View(wideW, tallH)
	press(s, screens.KeyToggleView)

	if list == gantt {
		t.Error("v must switch to the list version")
	}
	if strings.ContainsRune(stripStyling(list), barGlyph) {
		t.Errorf("the list version draws no bars\n%s", list)
	}
	if back := s.View(wideW, tallH); back != gantt {
		t.Errorf("a second v must return to the Gantt\n%s\nvs\n%s", back, gantt)
	}
}

func TestTimelineList_HeaderNamesTheColumns(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)

	header := ""
	for _, l := range viewLines(view) {
		if strings.Contains(strings.ToLower(l), "instance") {
			header = strings.ToLower(l)
			break
		}
	}
	if header == "" {
		t.Fatalf("the table has no header line with an instance column\n%s", view)
	}
	for _, col := range []string{"start", "end", "duration", "status", "overlap"} {
		if !strings.Contains(header, col) {
			t.Errorf("header %q must name the column %q", header, col)
		}
	}
}

func TestTimelineList_EveryInvocationHasOneRowWithItsFields(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)
	cases := []struct {
		id, start, end, status string
		d                      time.Duration
	}{
		{"solo#1", "10:00:00", "10:00:10", "SUCCESS", 10 * time.Second},
		{"alpha#2", "10:00:20", "10:00:50", "BLOCKED", 30 * time.Second},
		{"beta#3", "10:00:30", "10:01:15", "PARTIALLY_DONE", 45 * time.Second},
	}
	for _, c := range cases {
		// the invocations table comes before the compactions table, which may
		// name an instance again as the actor of a compaction
		if n := countOf(invocationsTable(view), c.id); n != 1 {
			t.Errorf("%s must have exactly one invocation row, found it %d times\n%s", c.id, n, view)
		}
		row := rowFor(t, view, c.id)
		assertContains(t, row, c.start, c.end, c.status)
		if !hasDuration(row, wantDuration(t, c.d)) {
			t.Errorf("row %q must carry its duration %q", row, wantDuration(t, c.d))
		}
	}
}

func TestTimelineList_OpenInvocationIsLabelledOpenWithoutDuration(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)

	row := rowFor(t, view, "rev#4")

	assertContains(t, row, "10:00:40")
	if !openLabelToken.MatchString(row) {
		t.Errorf("rev#4 row %q must carry %q as a separate token", row, screens.OpenLabel)
	}
	for _, d := range []time.Duration{10 * time.Second, 30 * time.Second, 35 * time.Second, 45 * time.Second} {
		if hasDuration(row, wantDuration(t, d)) {
			t.Errorf("rev#4 row %q must not invent a duration", row)
		}
	}
	for id := range timelineTypes {
		if id == "rev#4" {
			continue
		}
		if r := rowFor(t, view, id); openLabelToken.MatchString(r) {
			t.Errorf("row %q is closed and must not carry %q", r, screens.OpenLabel)
		}
	}
}

func TestTimelineList_InvocationWithoutTimestampsStaysListedWithAbsentMarkers(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)

	assertListHeader(t, view)
	row := rowFor(t, view, "delta#5")

	if n := strings.Count(row, screens.AbsentMarker); n < 3 {
		t.Errorf("delta#5 has no usable start, end or duration and must show %q for each: %q", screens.AbsentMarker, row)
	}
}

func TestTimelineList_OverlapCountIncludesTheOpenInvocationAsRunningToTheAxisEnd(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)

	for id, want := range map[string]string{"solo#1": "0", "alpha#2": "2", "beta#3": "2", "rev#4": "2", "delta#5": "0"} {
		row := withoutID(rowFor(t, view, id), id)
		assertFields(t, row, want)
		for _, other := range []string{"0", "1", "2", "3"} {
			if other != want {
				assertNoFields(t, row, other)
			}
		}
	}
	for id := range timelineTypes {
		if row := rowFor(t, view, id); strings.Contains(row, "parallel with") {
			t.Errorf("a list row must not say \"parallel with\": %q", row)
		}
	}
}

func TestTimelineList_SortsByStartDurationAndName(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)
	press(s, screens.KeyToggleView)

	if key, _ := activeSort(t, s.View(wideW, tallH)); key != screens.SortStart {
		t.Errorf("the default sort = %q, want %q", key, screens.SortStart)
	}
	walkSorts(t, s, wideW, tallH, map[screens.SortKey][]string{
		screens.SortStart:    {"solo#1", "alpha#2", "beta#3", "rev#4"},
		screens.SortDuration: {"solo#1", "alpha#2", "beta#3"},
	}, screens.SortStart, screens.SortDuration, screens.SortName)
}

func TestTimelineList_CompactionsTableHasOneRowPerCompactionInTimelineOrder(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)

	header := ""
	for _, l := range viewLines(view) {
		if low := strings.ToLower(l); strings.Contains(low, "actor") && strings.Contains(low, "trigger") {
			header = low
		}
	}
	if header == "" {
		t.Errorf("the compactions table has no header naming actor and trigger\n%s", view)
	}
	triggers := []string{"stray", "autocompact", "overflow", "scheduled", "interrupted", "lateness", "manual"}
	assertAppearInOrder(t, view, triggers, true)
	for _, trig := range triggers {
		if n := countOf(view, trig); n != 1 {
			t.Errorf("trigger %q must be on exactly one row, found %d\n%s", trig, n, view)
		}
	}
}

func TestTimelineList_CompactionRowsShowTimeActorTriggerTokensAndShape(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)

	auto := rowFor(t, view, "autocompact")
	if !strings.Contains(strings.ToLower(auto), "orchestrator") || !strings.Contains(auto, "10:00:25") {
		t.Errorf("autocompact row %q must name the orchestrator and its logged time 10:00:25", auto)
	}
	for _, n := range []int64{100000, 20000} {
		if want := tokens(n); !strings.Contains(auto, want) {
			t.Errorf("autocompact row %q must show token count %q", auto, want)
		}
	}
	paired := rowFor(t, view, "scheduled")
	assertContains(t, paired, "10:00:50", tokens(150000), tokens(30000), "paired")
	pre := rowFor(t, view, "interrupted")
	assertContains(t, pre, "beta#3", "10:01:05", tokens(70000), "pre only", screens.AbsentMarker)
	post := rowFor(t, view, "lateness")
	assertContains(t, post, "10:01:12", tokens(25000), "post only", screens.AbsentMarker)
	manual := rowFor(t, view, "manual")
	assertContains(t, manual, "ghost#9", screens.AbsentMarker)
	assertNotContains(t, auto, "pre only", "post only", "paired")
}

func TestTimelineList_ShowsNoMoneyOrPricingMarkers(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)

	assertContains(t, view, "alpha#2")
	body := strings.Join(overlayLines(view), "\n")
	assertNotContains(t, body, "$", screens.UnpricedMarker, "(partial)", screens.AssumedTierLabel(nil))
}

func TestTimelineList_RunWithoutInvocationsStillRenders(t *testing.T) {
	view := listView(t, overviewRun(), wideW, tallH)

	if strings.TrimSpace(view) == "" {
		t.Error("a run without a timeline must still render a page")
	}
	assertWithinBounds(t, view, wideW, tallH)
}

func TestTimelineList_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	s := openTimeline(t, manyTimelineRun(30), wideW, shortH)
	press(s, screens.KeyToggleView)

	first := s.View(wideW, shortH)
	assertWithinBounds(t, first, wideW, shortH)
	assertListHeader(t, first)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	assertContains(t, first, "step-01")
	assertNotContains(t, first, "step-30")

	press(s, repeat("down", 120)...)
	last := s.View(wideW, shortH)
	assertWithinBounds(t, last, wideW, shortH)
	assertContains(t, last, "step-30")
	assertNotContains(t, last, "step-01")
	if back := press(s, repeat("up", 120)...).View(wideW, shortH); back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestTimelineList_TallTerminalShowsEverythingWithoutHint(t *testing.T) {
	view := listView(t, manyTimelineRun(30), wideW, 200)

	assertListHeader(t, view)
	assertContains(t, view, "step-01", "step-30")
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestTimelineList_NothingIsDrawnPastTheTerminalAtAnySize(t *testing.T) {
	colourOff(t)
	s := screens.NewTimelineScreen(timelineRun(), 80, 30, paletteStyles())
	press(s, screens.KeyToggleView)

	for height := 0; height <= 40; height++ {
		for _, width := range []int{120, 80, 60, 40, 30, 20} {
			assertWithinBounds(t, s.View(width, height), width, height)
		}
	}
}

func TestTimelineList_MarkersStayTextWithColourOff(t *testing.T) {
	view := listView(t, timelineRun(), wideW, tallH)

	if view != stripStyling(view) {
		t.Errorf("with colour off the list must contain no escape sequences: %q", view)
	}
	assertContains(t, view, "paired", "pre only", "post only", screens.OpenLabel)
}
