package screens_test

// Behaviour of the timeline view: dispatch order, durations, parallel overlap,
// compactions with their actor, open invocations, scrolling and narrow widths.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func timelineView(run domain.RunReport, w, h int) string {
	return screens.NewTimelineScreen(run, w, h, plain()).View(w, h)
}

// lineIndex returns the index of the first line containing needle, or -1.
func lineIndex(view, needle string) int {
	for i, l := range strings.Split(view, "\n") {
		if strings.Contains(l, needle) {
			return i
		}
	}
	return -1
}

var openLabelToken = regexp.MustCompile(`(^|[^0-9A-Za-z#])` + regexp.QuoteMeta(screens.OpenLabel) + `($|[^0-9A-Za-z#])`)

func TestTimeline_ListsInvocationsInTimelineOrder(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	// Rows are located by their unique agent type.
	prev := -1
	for _, typ := range []string{"planner", "coder", "tester", "reviewer"} {
		i := lineIndex(view, typ)
		if i < 0 {
			t.Fatalf("view must list a row for type %s\n%s", typ, view)
		}
		if i <= prev {
			t.Errorf("row of type %s must come after the previous entry in timeline order planner, coder, tester, reviewer\n%s", typ, view)
		}
		prev = i
	}
	if d := lineIndex(view, "delta#5"); d <= prev {
		t.Errorf("delta#5 has no start and must be listed last\n%s", view)
	}
}

func TestTimeline_KeepsGivenOrderRegardlessOfDurationOrIdOrder(t *testing.T) {
	view := timelineView(orderRun(), wideW, tallH)

	mid, long, short := lineIndex(view, "mid-1"), lineIndex(view, "long-2"), lineIndex(view, "short-3")

	if mid < 0 || long < 0 || short < 0 || !(mid < long && long < short) {
		t.Errorf("entries must be rendered in the given order mid-1, long-2, short-3 (rows %d, %d, %d)\n%s", mid, long, short, view)
	}
}

func TestTimeline_ShowsEachDurationOnItsEntryRow(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	for typ, d := range map[string]time.Duration{
		"planner": 10 * time.Second, "coder": 30 * time.Second, "tester": 45 * time.Second,
	} {
		want := wantDuration(t, d)
		if row := rowFor(t, view, typ); !hasDuration(row, want) {
			t.Errorf("row of type %s %q must carry its duration %q", typ, row, want)
		}
	}
}

func TestTimeline_ShowsStartAndEndTimesAsLogged(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	for typ, times := range map[string][]string{
		"planner": {"10:00:00", "10:00:10"},
		"coder":   {"10:00:20", "10:00:50"},
		"tester":  {"10:00:30", "10:01:15"},
	} {
		row := rowFor(t, view, typ)
		for _, ts := range times {
			if !strings.Contains(row, ts) {
				t.Errorf("row of type %s %q must show the logged time %s", typ, row, ts)
			}
		}
		if strings.Contains(row, screens.AbsentMarker) {
			t.Errorf("row of type %s %q has all timestamps and must not show %q", typ, row, screens.AbsentMarker)
		}
	}
	if row := rowFor(t, view, "reviewer"); !strings.Contains(row, "10:00:40") {
		t.Errorf("open row %q must show its logged start 10:00:40", row)
	}
}

func TestTimeline_ParallelInvocationsShowEachOtherAsOverlaps(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	if row := rowFor(t, view, "coder"); !strings.Contains(row, "beta#3") {
		t.Errorf("alpha#2 row %q must name its overlap beta#3", row)
	}
	if row := rowFor(t, view, "tester"); !strings.Contains(row, "alpha#2") {
		t.Errorf("beta#3 row %q must name its overlap alpha#2", row)
	}
}

func TestTimeline_InvocationWithoutOverlapNamesNoOtherEntry(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	rows := map[string]string{"solo#1": rowFor(t, view, "planner"), "rev#4": rowFor(t, view, "reviewer"), "delta#5": rowFor(t, view, "delta#5")}
	for id, row := range rows {
		if !strings.Contains(row, id) {
			t.Errorf("row %q must name its own entry %s", row, id)
		}
		for _, other := range []string{"solo#1", "alpha#2", "beta#3", "rev#4", "delta#5"} {
			if other != id && strings.Contains(row, other) {
				t.Errorf("%s overlaps nothing but its row %q names %s", id, row, other)
			}
		}
	}
	if row := rowFor(t, view, "coder"); strings.Count(row, "alpha#2") > 1 {
		t.Errorf("alpha#2 must not be listed as its own overlap: %q", row)
	}
}

func TestTimeline_OpenInvocationIsLabelledOpenWithoutDuration(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	row := rowFor(t, view, "reviewer")

	if !openLabelToken.MatchString(row) {
		t.Errorf("rev#4 row %q must carry %q as a separate token", row, screens.OpenLabel)
	}
	for _, d := range []time.Duration{10 * time.Second, 30 * time.Second, 45 * time.Second} {
		if hasDuration(row, wantDuration(t, d)) {
			t.Errorf("rev#4 row %q must not invent a duration", row)
		}
	}
	for _, typ := range []string{"planner", "coder", "tester"} {
		if r := rowFor(t, view, typ); openLabelToken.MatchString(r) {
			t.Errorf("row of type %s %q is closed and must not carry %q", typ, r, screens.OpenLabel)
		}
	}
}

func TestTimeline_InvocationWithoutTimestampsStaysVisibleWithAbsentMarker(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	row := rowFor(t, view, "delta#5")

	if !strings.Contains(row, screens.AbsentMarker) {
		t.Errorf("delta#5 row %q has no usable timestamps and must show %q", row, screens.AbsentMarker)
	}
}

func TestTimeline_ShowsCompactionsWithActorTriggerTimeAndTokens(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	auto := rowFor(t, view, "auto")
	if !strings.Contains(strings.ToLower(auto), "orchestrator") {
		t.Errorf("auto compaction row %q must name the orchestrator", auto)
	}
	if !strings.Contains(auto, "10:00:25") {
		t.Errorf("auto compaction row %q must show its logged time 10:00:25", auto)
	}
	for _, n := range []domain.TokenCount{domain.Tokens(100000), domain.Tokens(20000)} {
		if want := screens.FormatTokens(n); !strings.Contains(auto, want) {
			t.Errorf("auto compaction row %q must show token count %q", auto, want)
		}
	}
	manual := rowFor(t, view, "manual")
	if !strings.Contains(manual, "ghost#9") || !strings.Contains(manual, screens.AbsentMarker) {
		t.Errorf("manual compaction row %q must name ghost#9 and show %q for its unknown time", manual, screens.AbsentMarker)
	}
	if lineIndex(view, "auto") >= lineIndex(view, "manual") {
		t.Errorf("compactions must keep timeline order, auto before manual\n%s", view)
	}
}

func TestTimeline_ShowsNoMoneyOrPricingMarkers(t *testing.T) {
	view := timelineView(timelineRun(), wideW, tallH)

	assertContains(t, view, "alpha#2")
	assertNotContains(t, view, "$", screens.UnpricedMarker, "(partial)", screens.AssumedTierLabel)
}

func TestTimeline_RunWithoutInvocationsStillRenders(t *testing.T) {
	view := timelineView(overviewRun(), wideW, tallH)

	if strings.TrimSpace(view) == "" {
		t.Error("a run without a timeline must still render a page")
	}
}

func TestTimeline_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	s := screens.NewTimelineScreen(manyTimelineRun(30), wideW, shortH, plain())

	first := s.View(wideW, shortH)
	assertFits(t, first, shortH)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	assertContains(t, first, "step-01")
	assertNotContains(t, first, "step-30")

	var scrolled screens.Screen = s
	scrolled = press(scrolled, repeat("down", 120)...)
	last := scrolled.View(wideW, shortH)
	assertFits(t, last, shortH)
	assertContains(t, last, "step-30")
	assertNotContains(t, last, "step-01")

	if back := press(scrolled, repeat("up", 120)...).View(wideW, shortH); back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestTimeline_TallTerminalShowsEverythingWithoutHint(t *testing.T) {
	view := timelineView(manyTimelineRun(30), wideW, 200)

	assertContains(t, view, "step-01", "step-30")
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestTimeline_NarrowTerminalNeverExceedsItsWidth(t *testing.T) {
	const narrow = 40

	view := timelineView(timelineRun(), narrow, tallH)

	assertContains(t, view, "solo#1")
	assertFitsWidth(t, view, narrow)
}
