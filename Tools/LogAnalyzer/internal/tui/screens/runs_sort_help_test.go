package screens_test

// Sorting and the ? help overlay of the runs list. The runs list is a page, not a
// per-run view, so its overlay has no Views group.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

func runsSortAscending() map[screens.SortKey][]string {
	return map[screens.SortKey][]string{
		screens.SortCost:        {runIDTwo, runIDFour, runIDOne},
		screens.SortInvocations: {runIDLong, runIDThree, runIDOne, runIDFour, runIDTwo},
		screens.SortDuration:    {runIDTwo, runIDFour, runIDOne, runIDThree},
		screens.SortName:        {runIDOne, runIDTwo, runIDThree, runIDFour, runIDLong},
	}
}

func newRuns(width, height int) *screens.RunsScreen {
	return screens.NewRunsScreen(runsReport(), width, height, paletteStyles())
}

func TestRuns_DefaultSortIsCostDescendingAndTheHeaderSaysSo(t *testing.T) {
	colourOff(t)

	key, desc := activeSort(t, newRuns(140, tallH).View(140, tallH))

	if key != screens.SortCost || !desc {
		t.Errorf("default sort = %q desc=%v, want cost descending", key, desc)
	}
}

func TestRuns_SortKeyCyclesThroughEveryOrderAndWrapsAround(t *testing.T) {
	colourOff(t)
	s := newRuns(140, tallH)

	seen := walkSorts(t, s, 140, tallH, runsSortAscending(),
		screens.SortCost, screens.SortInvocations, screens.SortDuration, screens.SortName)

	if len(seen) == 0 || seen[0] != screens.SortCost {
		t.Errorf("the cycle must start on cost, got %v", seen)
	}
}

func TestRuns_SortingNeverDropsARun(t *testing.T) {
	colourOff(t)
	s := newRuns(140, tallH)

	for i := 0; i < 5; i++ {
		view := s.View(140, tallH)
		for _, id := range []string{runIDOne, runIDTwo, runIDThree, runIDFour, runIDLong} {
			rowFor(t, view, id)
		}
		press(s, screens.KeySort)
	}
}

func TestRuns_ShortHelpLineListsTheSortKeyAndTheHelpKey(t *testing.T) {
	colourOff(t)

	lines := viewLines(newRuns(tableW, tallH).View(tableW, tallH))
	help := tailOf(lines)

	assertContains(t, help, screens.KeySort+" sort", "? help", "enter")
}

func runsOverlay(s screens.Screen, width, height int) string {
	return strings.Join(overlayLines(openOverlay(s, width, height)), "\n")
}

func TestRunsHelpOverlay_ReplacesTheBodyAndKeepsTheHeader(t *testing.T) {
	colourOff(t)
	s := newRuns(120, 40)
	before := s.View(120, 40)

	overlay := openOverlay(s, 120, 40)

	if overlay == before {
		t.Fatal("? must change the view")
	}
	lines := viewLines(overlay)
	assertContains(t, headOf(lines), "Log Analyzer")
	if len(lines) > 1 {
		assertSeparator(t, lines[1])
	}
	assertNotContains(t, overlay, runIDOne)
	assertWithinBounds(t, overlay, 120, 40)
}

func TestRunsHelpOverlay_ListsThisScreenNavigationAndExportKeysButNoViews(t *testing.T) {
	colourOff(t)

	overlay := runsOverlay(newRuns(120, 60), 120, 60)

	assertContains(t, overlay, "This screen", "Navigation", "Export")
	assertNotContains(t, overlay, "Views")
	for _, k := range []string{screens.KeySort, screens.KeyAcrossRuns, "s", "p", "enter", "esc", screens.KeyHelp} {
		if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(k) + `\s+\S`).MatchString(overlay) {
			t.Errorf("overlay must list key %q on its own entry\n%s", k, overlay)
		}
	}
	for _, k := range []string{screens.KeyExportRun, screens.KeyExportAll} {
		if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(k) + `\s+.*(?i:export)`).MatchString(overlay) {
			t.Errorf("overlay must list the export key %q\n%s", k, overlay)
		}
	}
	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeySort) + `\s+.*(?i:sort)`).MatchString(overlay) {
		t.Errorf("overlay must say the sort key sorts\n%s", overlay)
	}
}

func TestRunsHelpOverlay_QuestionMarkAndEscCloseItWithoutLeavingTheList(t *testing.T) {
	colourOff(t)
	for _, closer := range []string{screens.KeyHelp, "esc"} {
		s := newRuns(120, 40)
		before := s.View(120, 40)

		openOverlay(s, 120, 40)
		press(s, closer)

		if after := s.View(120, 40); after != before {
			t.Errorf("%q must close the overlay and restore the list\n%s\nvs\n%s", closer, after, before)
		}
		if s.Back() {
			t.Errorf("%q closing the overlay must not leave the runs list", closer)
		}
	}
}

func TestRunsHelpOverlay_EscLeavesTheListOnlyAfterTheOverlayIsClosed(t *testing.T) {
	s := newRuns(120, 40)

	openOverlay(s, 120, 40)
	press(s, "esc")
	if s.Back() {
		t.Fatal("the first esc must close the overlay without leaving the list")
	}
	press(s, "esc")

	if !s.Back() {
		t.Error("esc after closing the overlay must leave the list")
	}
}

func TestRunsHelpOverlay_IgnoresEveryOtherKeyWhileOpen(t *testing.T) {
	colourOff(t)
	s := newRuns(120, 40)
	before := s.View(120, 40)

	openOverlay(s, 120, 40)
	press(s, "enter", "q", screens.KeySort, "m")
	if s.Done() || s.Back() {
		t.Error("enter and q must be ignored while the overlay is open")
	}
	press(s, screens.KeyHelp)

	if after := s.View(120, 40); after != before {
		t.Errorf("keys pressed while the overlay was open must change nothing\n%s\nvs\n%s", after, before)
	}
}

func TestRunsHelpOverlay_ScrollsOnAShortTerminalWithinBounds(t *testing.T) {
	colourOff(t)
	s := newRuns(80, shortH)

	first := openOverlay(s, 80, shortH)
	assertWithinBounds(t, first, 80, shortH)
	m1 := moreLineRE.FindStringSubmatch(stripStyling(first))
	if m1 == nil {
		t.Fatalf("a cut overlay must end with a more line\n%s", first)
	}
	scrolled := press(s, repeat("down", 3)...).View(80, shortH)

	assertWithinBounds(t, scrolled, 80, shortH)
	if m2 := moreLineRE.FindStringSubmatch(stripStyling(scrolled)); m2 == nil || m2[1] == m1[1] {
		t.Errorf("down must scroll the overlay (first line %v -> %v)\n%s", m1, m2, scrolled)
	}
}

func TestRunsHelpOverlay_NeverTouchesTheRunListScroll(t *testing.T) {
	colourOff(t)
	s := newRuns(100, 9)
	scrolled := press(s, repeat("down", 3)...).View(100, 9)

	openOverlay(s, 100, 9)
	press(s, repeat("down", 4)...)
	press(s, screens.KeyHelp)

	if after := s.View(100, 9); after != scrolled {
		t.Errorf("scrolling the overlay must not move the list\n%s\nvs\n%s", after, scrolled)
	}
}
