package screens_test

// The band colours of the Context view, its sparkline with the first write
// marker, the default sort and the sort cycle, and the bounds at narrow and
// short terminals.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// ctxColourView renders the Context view of the standard run with colour on.
func ctxColourView(t *testing.T) string {
	t.Helper()
	colourOn(t)
	return screens.NewContextScreen(contextRun(), tableW, tallH, paletteStyles()).View(tableW, tallH)
}

func TestContext_PeakFinalAndFirstWriteAreDrawnInTheirBandRole(t *testing.T) {
	view := ctxColourView(t)

	builder := rawLineWith(t, view, "builder#1")
	assertDrawnInRole(t, builder, "380,000", screens.BandRole(domain.BandDegraded))
	assertDrawnInRole(t, builder, "360,000", screens.BandRole(domain.BandDegraded))
	assertDrawnInRole(t, builder, "260,000", screens.BandRole(domain.BandAboveOptimal))
	orch := rawLineWith(t, view, "model-o")
	assertDrawnInRole(t, orch, "180,000", screens.BandRole(domain.BandOptimal))
	assertDrawnInRole(t, orch, "130,000", screens.BandRole(domain.BandOptimal))
	assertDrawnInRole(t, rawLineWith(t, view, "tester#1"), "95,000", screens.BandRole(domain.BandOptimal))
}

func TestContext_SparklineIsColouredPerPointByBand(t *testing.T) {
	view := ctxColourView(t)

	builder := rawLineWith(t, view, "builder#1")
	assertDrawnInRole(t, builder, "▂", screens.BandRole(domain.BandOptimal))
	assertDrawnInRole(t, builder, "█", screens.BandRole(domain.BandDegraded))
	assertDrawnInRole(t, rawLineWith(t, view, "model-o"), "█", screens.BandRole(domain.BandOptimal))
}

func TestContext_SparklineDrawsOnePointPerCallAndMarksTheFirstWrite(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	// Levels scale to each actor's own largest context; the first write call is
	// drawn with the marker instead of its level.
	want := map[string]string{
		"builder#1":    "▂┃██",
		"reviewer#1":   "▅┃█",
		"orchestrator": "▃█▅▆",
		"coder#1":      "▃▆█▃▃",
	}
	for id, spark := range want {
		if got := ctxSparkline(t, view, id); got != spark {
			t.Errorf("%s sparkline = %q, want %q", id, got, spark)
		}
	}
}

func TestContext_AnInvocationWithoutCallsDrawsNoSparkline(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	if got := ctxSparkline(t, view, "ghost#1"); got != "" {
		t.Errorf("ghost#1 sparkline = %q, want none", got)
	}
}

func TestContext_DefaultSortIsPeakContextDescending(t *testing.T) {
	view := ctxView(t, contextRun(), tableW, tallH)

	key, desc := activeSort(t, view)
	if key != screens.SortPeakContext || !desc {
		t.Errorf("default sort = %q desc=%v, want peak context descending", key, desc)
	}
	assertAppearInOrder(t, view, []string{"builder#1", "coder#1", "reviewer#1", "tester#1"}, true)
	if !regexp.MustCompile(`peak context\s?(↓|v)`).MatchString(strings.ToLower(stripStyling(view))) {
		t.Errorf("the peak column header must show the descending sort marker\n%s", stripStyling(view))
	}
}

func TestContext_SortCycleOffersPeakFinalCallsAndNameInTheirOrder(t *testing.T) {
	s := ctxOpen(t)
	finalOrder := []string{"coder#1", "tester#1", "reviewer#1", "builder#1"}
	ascending := map[screens.SortKey][]string{
		screens.SortPeakContext:  {"tester#1", "reviewer#1", "coder#1", "builder#1"},
		screens.SortCalls:        {"tester#1", "reviewer#1", "builder#1", "coder#1"},
		screens.SortName:         {"builder#1", "coder#1", "reviewer#1", "tester#1"},
		screens.SortContext:      finalOrder,
		screens.SortKey("final"): finalOrder,
	}

	seen := walkSorts(t, s, tableW, tallH, ascending, screens.SortPeakContext, screens.SortCalls, screens.SortName)

	if len(seen) < 4 || seen[0] != screens.SortPeakContext {
		t.Errorf("the cycle must start at peak context and offer peak, final, calls and name; offered %v", seen)
	}
}

func TestContext_NarrowTerminalsStayWithinTheirBoundsAndKeepThePeak(t *testing.T) {
	for _, w := range []int{80, 60, 40} {
		view := ctxView(t, contextRun(), w, tallH)

		assertWithinBounds(t, view, w, tallH)
		// the row's identity outlasts the figures, so the peak is only guaranteed at the widest of these.
		assertContains(t, stripStyling(view), "builder#1", "model-b")
	}
	assertContains(t, stripStyling(ctxView(t, contextRun(), 80, tallH)), "380,000")
}

func TestContext_StaysWithinVeryNarrowAndVeryShortTerminals(t *testing.T) {
	for _, size := range [][2]int{{24, tallH}, {80, 24}, {80, 10}, {80, 6}, {40, 8}} {
		view := ctxView(t, contextRun(), size[0], size[1])

		if strings.TrimSpace(view) == "" {
			t.Errorf("%dx%d: the view drew nothing", size[0], size[1])
		}
		assertWithinBounds(t, view, size[0], size[1])
	}
}

func TestContext_ShortTerminalKeepsTheSelectedRowVisible(t *testing.T) {
	colourOff(t)
	s := screens.NewContextScreen(contextRun(), 80, shortH, paletteStyles())
	selected := func() string {
		for _, l := range viewLines(s.View(80, shortH)) {
			if strings.HasPrefix(strings.TrimSpace(l), ">") {
				return l
			}
		}
		return ""
	}
	first := selected()

	press(s, repeat("down", 4)...)

	if now := selected(); now == "" || now == first {
		t.Errorf("after moving down the selected row must stay visible and change: %q -> %q", first, now)
	}
	assertWithinBounds(t, s.View(80, shortH), 80, shortH)
}
