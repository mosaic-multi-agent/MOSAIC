package screens_test

// Behaviour of the tools view: counts, errors, durations, unpaired calls, the
// absence of any token or money attribution, and scrolling.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func toolsView(run domain.RunReport, w, h int) string {
	return screens.NewToolsScreen(run, w, h, plain()).View(w, h)
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
	return regexp.MustCompile(`(^|[^0-9A-Za-z.])` + regexp.QuoteMeta(want) + `($|[^0-9A-Za-z.])`).MatchString(line)
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
	view := toolsView(toolsRun(), wideW, tallH)

	read := rowFor(t, view, "Read")
	if !hasToken(read, "11") || !hasToken(read, "3") {
		t.Errorf("Read row %q must show 11 calls and 3 errors", read)
	}
	bash := rowFor(t, view, "Bash")
	if !hasToken(bash, "6") {
		t.Errorf("Bash row %q must show 6 calls", bash)
	}
}

func TestTools_ShowsMinMaxAndAverageDurations(t *testing.T) {
	view := toolsView(toolsRun(), wideW, tallH)

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
	view := toolsView(toolsRun(), wideW, tallH)

	unnamed := rowFor(t, view, screens.UnnamedToolLabel)
	if !strings.Contains(unnamed, screens.UnpairedLabel) || !hasToken(unnamed, "3") || !hasToken(unnamed, "2") {
		t.Errorf("unnamed row %q must show 3 calls and 2 %q", unnamed, screens.UnpairedLabel)
	}
	if !strings.Contains(unnamed, screens.AbsentMarker) {
		t.Errorf("unnamed row %q has no duration and must show the absent marker", unnamed)
	}
	bash := rowFor(t, view, "Bash")
	if !strings.Contains(bash, screens.UnpairedLabel) || !hasToken(bash, "2") {
		t.Errorf("Bash row %q must show 2 %q calls", bash, screens.UnpairedLabel)
	}
}

func TestTools_FullyPairedToolIsNotLabelledUnpaired(t *testing.T) {
	view := toolsView(toolsRun(), wideW, tallH)

	for _, tool := range []string{"Read", "Grep", "Edit"} {
		if line := rowFor(t, view, tool); strings.Contains(line, screens.UnpairedLabel) {
			t.Errorf("%s row %q must not carry %q", tool, line, screens.UnpairedLabel)
		}
	}
}

func TestTools_ShowsOrchestratorAndAgentTypeSections(t *testing.T) {
	view := toolsView(toolsRun(), wideW, tallH)

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
	if line := rowFor(t, view, "Edit"); !hasToken(line, "9") {
		t.Errorf("Edit row %q must show 9 calls", line)
	}
	if line := rowFor(t, view, "Write"); !hasToken(line, "5") || !hasToken(line, "2") {
		t.Errorf("Write row %q must show 5 calls and 2 errors", line)
	}
	if line := rowFor(t, view, "Write"); !strings.Contains(line, screens.UnpairedLabel) {
		t.Errorf("Write row %q in an agent-type section must carry %q", line, screens.UnpairedLabel)
	}
}

func TestTools_ShowsNoTokenOrMoneyAttribution(t *testing.T) {
	view := toolsView(toolsRun(), wideW, tallH)

	assertContains(t, view, "Read")
	assertNotContains(t, view, "$", screens.UnpricedMarker, "(partial)", screens.AssumedTierLabel)
	assertNotContains(t, strings.ToLower(view), "cost", "token")
}

func TestTools_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	s := screens.NewToolsScreen(manyToolsRun(30), wideW, shortH, plain())

	first := s.View(wideW, shortH)
	assertFits(t, first, shortH)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	assertNotContains(t, first, "tool-30")

	var scrolled screens.Screen = s
	scrolled = press(scrolled, repeat("down", 80)...)
	last := scrolled.View(wideW, shortH)
	assertFits(t, last, shortH)
	assertContains(t, last, "tool-30")
	assertNotContains(t, last, "tool-01")

	back := press(scrolled, repeat("up", 80)...).View(wideW, shortH)
	if back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestTools_TallTerminalShowsEverythingWithoutHint(t *testing.T) {
	view := toolsView(manyToolsRun(30), wideW, 200)

	assertContains(t, view, "tool-01", "tool-30")
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestTools_NarrowTerminalNeverExceedsItsWidth(t *testing.T) {
	const narrow = 40

	view := toolsView(toolsRun(), narrow, tallH)

	assertContains(t, view, "Read")
	assertFitsWidth(t, view, narrow)
}

func TestTools_RunWithoutToolCallsStillRenders(t *testing.T) {
	view := toolsView(overviewRun(), wideW, tallH)

	if strings.TrimSpace(view) == "" {
		t.Error("a run without tool calls must still render a page")
	}
}

func TestTools_OrchestratorSectionUnpairedToolShowsLabelAndAbsentDuration(t *testing.T) {
	run := overviewRun()
	run.Tools.Orchestrator = []domain.ToolFigures{{Tool: "Glob", Calls: 4, Unpaired: 3}}

	line := rowFor(t, toolsView(run, wideW, tallH), "Glob")

	if !strings.Contains(line, screens.UnpairedLabel) || !strings.Contains(line, screens.AbsentMarker) {
		t.Errorf("Glob row %q must carry %q and the absent marker", line, screens.UnpairedLabel)
	}
}

func TestTools_ToolWithErrorsAndNoPairedCallsShowsAbsentDuration(t *testing.T) {
	run := overviewRun()
	run.Tools.Run = []domain.ToolFigures{{Tool: "Fetch", Calls: 7, Errors: 7, Unpaired: 7}}

	line := rowFor(t, toolsView(run, wideW, tallH), "Fetch")

	if !strings.Contains(line, screens.AbsentMarker) {
		t.Errorf("Fetch row %q has no paired call and must show the absent marker", line)
	}
	if zero := screens.FormatDuration(0, true); zero != screens.AbsentMarker && hasDuration(line, zero) {
		t.Errorf("Fetch row %q must not show a zero duration %q", line, zero)
	}
}
