package screens_test

// Behaviour of the tools view: counts, errors, durations, unpaired calls, the
// sections of the run, the orchestrator and each agent type, the absence of any
// token or money attribution, and scrolling.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// toolsView renders the tools view with colour off and the application's palette.
func toolsView(t *testing.T, run domain.RunReport, w, h int) string {
	t.Helper()
	colourOff(t)
	return screens.NewToolsScreen(run, w, h, paletteStyles()).View(w, h)
}

// wantDuration returns the formatted duration and fails when the format helper
// yields nothing to look for.
func wantDuration(t *testing.T, d time.Duration) string {
	t.Helper()
	s := screens.FormatDuration(d, true)
	if s == "" || s == screens.AbsentMarker {
		t.Fatalf("FormatDuration(%v, true) = %q, want a rendered duration", d, s)
	}
	return s
}

// hasDuration reports whether the line holds the duration as a whole token, so
// "4s" is not found inside "44s".
func hasDuration(line, want string) bool {
	return regexp.MustCompile(`(^|[^0-9A-Za-z.])` + regexp.QuoteMeta(want) + `($|[^0-9A-Za-z.])`).MatchString(stripStyling(line))
}

func TestFormatDuration_AbsentWhenNotOkAndDistinctWhenOk(t *testing.T) {
	if got := screens.FormatDuration(1500*time.Millisecond, false); got != screens.AbsentMarker {
		t.Errorf("FormatDuration(_, false) = %q, want the absent marker", got)
	}
	one, two := wantDuration(t, time.Second), wantDuration(t, 2*time.Second)
	if one == two {
		t.Errorf("different durations must render differently, both gave %q", one)
	}
}

func TestTools_ShowsCallsAndErrorsPerTool(t *testing.T) {
	view := toolsView(t, toolsRun(), tableW, tallH)

	read := rowFor(t, view, "Read")
	assertFields(t, read, "11", "3")
	assertFields(t, rowFor(t, view, "Bash"), "6")
}

func TestTools_ShowsMinMaxAndAverageDurations(t *testing.T) {
	view := toolsView(t, toolsRun(), tableW, tallH)

	read := rowFor(t, view, "Read")
	for _, d := range []time.Duration{ms(1250), ms(7500), 4 * time.Second} {
		if want := wantDuration(t, d); !hasDuration(read, want) {
			t.Errorf("Read row %q must show duration %q", read, want)
		}
	}
	bash := rowFor(t, view, "Bash")
	for _, d := range []time.Duration{time.Second, 6 * time.Second, 2500 * time.Millisecond} {
		if want := wantDuration(t, d); !hasDuration(bash, want) {
			t.Errorf("Bash row %q must show duration %q", bash, want)
		}
	}
}

func TestTools_UnpairedCallsAreVisibleAndCarryNoDuration(t *testing.T) {
	view := toolsView(t, toolsRun(), tableW, tallH)

	unnamed := rowFor(t, view, screens.StartMissingLabel)
	assertContains(t, unnamed, screens.UnpairedLabel, screens.AbsentMarker)
	assertFields(t, unnamed, "3", "2")
	bash := rowFor(t, view, "Bash")
	assertContains(t, bash, screens.UnpairedLabel)
	assertFields(t, bash, "2")
}

func TestTools_FullyPairedToolIsNotLabelledUnpaired(t *testing.T) {
	view := toolsView(t, toolsRun(), tableW, tallH)

	for _, tool := range []string{"Read", "Grep", "Edit"} {
		assertNotContains(t, rowFor(t, view, tool), screens.UnpairedLabel)
	}
}

func TestTools_ShowsOrchestratorAndAgentTypeSections(t *testing.T) {
	view := toolsView(t, toolsRun(), tableW, tallH)

	orch := strings.Index(strings.ToLower(view), "orchestrator")
	grep := strings.Index(view, "Grep")
	builder := strings.Index(view, "builder")
	edit := strings.Index(view, "Edit")
	unknown := strings.Index(view, screens.UnknownTypeLabel)
	write := strings.Index(view, "Write")
	if orch < 0 || grep < 0 || builder < 0 || edit < 0 || unknown < 0 || write < 0 {
		t.Fatalf("view must show orchestrator, Grep, builder, Edit, %q and Write\n%s", screens.UnknownTypeLabel, view)
	}
	if !(orch < grep) {
		t.Errorf("Grep must be listed under the orchestrator heading\n%s", view)
	}
	if !(builder < edit && edit < unknown && unknown < write) {
		t.Errorf("Edit must be listed under builder and Write under the unknown type, in report order\n%s", view)
	}
	assertFields(t, rowFor(t, view, "Edit"), "9")
	write2 := rowFor(t, view, "Write")
	assertFields(t, write2, "5", "2")
	assertContains(t, write2, screens.UnpairedLabel)
}

func TestTools_ShowsNoTokenOrMoneyAttribution(t *testing.T) {
	view := toolsView(t, toolsRun(), tableW, tallH)

	assertContains(t, view, "Read")
	// The frame header carries the run's total cost; the body itself shows none.
	body := strings.Join(overlayLines(view), "\n")
	assertNotContains(t, body, "$", screens.UnpricedMarker, "(partial)", screens.AssumedTierLabel(nil))
	assertNotContains(t, strings.ToLower(body), "cost", "token")
}

func TestTools_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(manyToolsRun(30), wideW, shortH, paletteStyles())

	// Calls sort puts the busiest tool (tool-30) first and the quietest (tool-01) last.
	first := s.View(wideW, shortH)
	assertWithinBounds(t, first, wideW, shortH)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	assertContains(t, first, "tool-30")
	assertNotContains(t, first, "tool-01")

	var scrolled screens.Screen = s
	scrolled = press(scrolled, repeat("down", 80)...)
	last := scrolled.View(wideW, shortH)
	assertWithinBounds(t, last, wideW, shortH)
	assertContains(t, last, "tool-01")
	assertNotContains(t, last, "tool-30")

	back := press(scrolled, repeat("up", 80)...).View(wideW, shortH)
	if back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestTools_TallTerminalShowsEverythingWithoutHint(t *testing.T) {
	view := toolsView(t, manyToolsRun(30), wideW, 200)

	assertContains(t, view, "tool-01", "tool-30")
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestTools_NarrowTerminalNeverExceedsItsWidth(t *testing.T) {
	const narrow = 40

	view := toolsView(t, toolsRun(), narrow, tallH)

	assertContains(t, view, "Read")
	assertWithinBounds(t, view, narrow, tallH)
}

func TestTools_RunWithoutToolCallsStillRenders(t *testing.T) {
	view := toolsView(t, overviewRun(), wideW, tallH)

	if strings.TrimSpace(view) == "" {
		t.Error("a run without tool calls must still render a page")
	}
	assertWithinBounds(t, view, wideW, tallH)
}

func TestTools_OrchestratorSectionUnpairedToolShowsLabelAndAbsentDuration(t *testing.T) {
	run := overviewRun()
	run.Tools.Orchestrator = []domain.ToolFigures{{Tool: "Glob", Calls: 4, Unpaired: 3}}

	line := rowFor(t, toolsView(t, run, tableW, tallH), "Glob")

	assertContains(t, line, screens.UnpairedLabel, screens.AbsentMarker)
}

func TestTools_ToolWithErrorsAndNoPairedCallsShowsAbsentDuration(t *testing.T) {
	run := overviewRun()
	run.Tools.Run = []domain.ToolFigures{{Tool: "Fetch", Calls: 7, Errors: 7, Unpaired: 7}}

	line := rowFor(t, toolsView(t, run, tableW, tallH), "Fetch")

	assertContains(t, line, screens.AbsentMarker)
	if zero := screens.FormatDuration(0, true); zero != screens.AbsentMarker && hasDuration(line, zero) {
		t.Errorf("Fetch row %q must not show a zero duration %q", line, zero)
	}
}
