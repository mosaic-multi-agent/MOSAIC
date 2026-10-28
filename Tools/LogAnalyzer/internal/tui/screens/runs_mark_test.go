package screens_test

// Marking runs of the runs list for a comparison: mark and unmark, at most two
// marks shown as A and B, the key that requests the comparison and the keys in
// the help line and overlay. The default sort lists runIDOne, runIDFour and
// runIDTwo (cost descending) as the first three rows.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var (
	markerA = regexp.MustCompile(`(^|\s)\[?A\]?(\s|$)`)
	markerB = regexp.MustCompile(`(^|\s)\[?B\]?(\s|$)`)
)

func newMarkRuns(t *testing.T) *screens.RunsScreen {
	t.Helper()
	colourOff(t)
	return newRuns(tableW, tallH)
}

// markedAs reports which of A and B mark the row of the run in the view.
func markedAs(t *testing.T, s *screens.RunsScreen, id string) (a, b bool) {
	t.Helper()
	row := rowFor(t, stripStyling(s.View(tableW, tallH)), id)
	return markerA.MatchString(row), markerB.MatchString(row)
}

func assertMarks(t *testing.T, s *screens.RunsScreen, id string, wantA, wantB bool) {
	t.Helper()
	if a, b := markedAs(t, s, id); a != wantA || b != wantB {
		t.Errorf("run %s: marked A=%v B=%v, want A=%v B=%v", id, a, b, wantA, wantB)
	}
}

// unmarkMentions counts the mentions of "unmark" in the view, in any case.
func unmarkMentions(s *screens.RunsScreen) int {
	return strings.Count(strings.ToLower(stripStyling(s.View(tableW, tallH))), "unmark")
}

func TestRunsMark_NoRunIsMarkedInitially(t *testing.T) {
	s := newMarkRuns(t)

	for _, id := range []string{runIDOne, runIDTwo, runIDThree, runIDFour, runIDLong} {
		assertMarks(t, s, id, false, false)
	}
}

func TestRunsMark_FirstMarkOfTheSelectedRunReadsAWithColourOff(t *testing.T) {
	s := newMarkRuns(t)

	press(s, screens.KeyMarkRun)

	assertMarks(t, s, runIDOne, true, false)
	assertMarks(t, s, runIDFour, false, false)
	assertMarks(t, s, runIDTwo, false, false)
}

func TestRunsMark_SecondMarkReadsBAndTheFirstStaysA(t *testing.T) {
	s := newMarkRuns(t)

	press(s, screens.KeyMarkRun, "down", "down", screens.KeyMarkRun)

	assertMarks(t, s, runIDOne, true, false)
	assertMarks(t, s, runIDTwo, false, true)
	assertMarks(t, s, runIDFour, false, false)
}

func TestRunsMark_MarkKeyOnAMarkedRunUnmarksIt(t *testing.T) {
	s := newMarkRuns(t)

	press(s, screens.KeyMarkRun)
	assertMarks(t, s, runIDOne, true, false)

	press(s, screens.KeyMarkRun)

	assertMarks(t, s, runIDOne, false, false)
}

func TestRunsMark_MarksStayWithTheirRunsWhenTheSortChanges(t *testing.T) {
	s := newMarkRuns(t)
	press(s, screens.KeyMarkRun, "down", "down", screens.KeyMarkRun)

	press(s, screens.KeySort, screens.KeySort, screens.KeySort) // name ascending: a different row order

	assertMarks(t, s, runIDOne, true, false)
	assertMarks(t, s, runIDTwo, false, true)
	assertMarks(t, s, runIDFour, false, false)
}

func TestRunsMark_AIsTheEarlierMarkOfTheTwoThatRemain(t *testing.T) {
	s := newMarkRuns(t)
	press(s, screens.KeyMarkRun, "down", screens.KeyMarkRun) // one = A, four = B

	press(s, "up", screens.KeyMarkRun)           // unmark one
	press(s, "down", "down", screens.KeyMarkRun) // mark two

	assertMarks(t, s, runIDOne, false, false)
	assertMarks(t, s, runIDFour, true, false)
	assertMarks(t, s, runIDTwo, false, true)
}

func TestRunsMark_ThirdMarkIsRefusedWithAStatusAskingToUnmarkOne(t *testing.T) {
	s := newMarkRuns(t)
	press(s, screens.KeyMarkRun, "down", screens.KeyMarkRun, "down")
	before := unmarkMentions(s)

	press(s, screens.KeyMarkRun)

	assertMarks(t, s, runIDOne, true, false)
	assertMarks(t, s, runIDFour, false, true)
	assertMarks(t, s, runIDTwo, false, false)
	if after := unmarkMentions(s); after <= before {
		t.Errorf("a refused third mark must show a status asking the user to unmark one (mentions %d -> %d)\n%s",
			before, after, stripStyling(s.View(tableW, tallH)))
	}
}

func TestRunsMark_RefusedMarkLeavesTheTwoMarksAndTheirComparison(t *testing.T) {
	s := newMarkRuns(t)
	press(s, screens.KeyMarkRun, "down", screens.KeyMarkRun, "down", screens.KeyMarkRun, screens.KeyCompare)

	a, b, ok := s.CompareRequested()

	if !ok || a != runIDOne || b != runIDFour {
		t.Errorf("CompareRequested() = %q, %q, %v; want %q, %q, true", a, b, ok, runIDOne, runIDFour)
	}
}

func TestRunsMark_CompareKeyWithTwoMarksRequestsTheComparisonAThenB(t *testing.T) {
	s := newMarkRuns(t)
	press(s, "down", screens.KeyMarkRun, "up", screens.KeyMarkRun) // four = A, one = B

	press(s, screens.KeyCompare)

	a, b, ok := s.CompareRequested()
	if !ok || a != runIDFour || b != runIDOne {
		t.Errorf("CompareRequested() = %q, %q, %v; want %q, %q, true", a, b, ok, runIDFour, runIDOne)
	}
	if s.Done() || s.Back() {
		t.Error("the compare key must neither open a run nor leave the list")
	}
}

func TestRunsMark_CompareKeyIsIneffectiveWithoutTwoMarks(t *testing.T) {
	cases := map[string][]string{
		"no mark":                {screens.KeyCompare},
		"one mark":               {screens.KeyMarkRun, screens.KeyCompare},
		"two marks, one removed": {screens.KeyMarkRun, "down", screens.KeyMarkRun, screens.KeyMarkRun, screens.KeyCompare},
	}
	for name, keys := range cases {
		s := newMarkRuns(t)

		press(s, keys...)

		if a, b, ok := s.CompareRequested(); ok {
			t.Errorf("%s: CompareRequested() = %q, %q, true; want no request", name, a, b)
		}
	}
}

func TestRunsMark_NoRequestUntilTheCompareKeyIsPressed(t *testing.T) {
	s := newMarkRuns(t)

	press(s, screens.KeyMarkRun, "down", screens.KeyMarkRun)

	if _, _, ok := s.CompareRequested(); ok {
		t.Error("two marks alone must not request the comparison")
	}
}

func TestRunsMark_ProvisionalRunCanBeMarkedAndCompared(t *testing.T) {
	s := newMarkRuns(t)
	press(s, screens.KeyMarkRun, "down", "down", screens.KeyMarkRun) // two is provisional

	press(s, screens.KeyCompare)

	a, b, ok := s.CompareRequested()
	if !ok || a != runIDOne || b != runIDTwo {
		t.Errorf("CompareRequested() = %q, %q, %v; want %q, %q, true", a, b, ok, runIDOne, runIDTwo)
	}
}

func TestRunsMark_MarksAreClearedWhenTheReportIsReplaced(t *testing.T) {
	s := newMarkRuns(t)
	press(s, screens.KeyMarkRun, "down", screens.KeyMarkRun)
	assertMarks(t, s, runIDOne, true, false)
	assertMarks(t, s, runIDFour, false, true)

	s.SetReport(runsReport())

	assertMarks(t, s, runIDOne, false, false)
	assertMarks(t, s, runIDFour, false, false)
	press(s, screens.KeyCompare)
	if _, _, ok := s.CompareRequested(); ok {
		t.Error("cleared marks must not allow a comparison")
	}
}

func TestRunsMark_MarkKeyOnAnEmptyListDoesNothing(t *testing.T) {
	colourOff(t)
	empty := domain.Report{Currency: domain.Currency, Quality: domain.NewQualitySummary(nil)}
	s := screens.NewRunsScreen(empty, 100, 20, paletteStyles())

	press(s, screens.KeyMarkRun, screens.KeyMarkRun, screens.KeyCompare)

	if _, _, ok := s.CompareRequested(); ok {
		t.Error("an empty list has nothing to compare")
	}
	assertWithinBounds(t, s.View(100, 20), 100, 20)
}

func TestRunsMark_MarkerIsVisibleAtEightyColumns(t *testing.T) {
	colourOff(t)
	s := newRuns(80, 30)

	press(s, screens.KeyMarkRun, "down", screens.KeyMarkRun)

	view := stripStyling(s.View(80, 30))
	if !markerA.MatchString(rowFor(t, view, runIDOne)) || !markerB.MatchString(rowFor(t, view, runIDFour)) {
		t.Errorf("A and B must stay visible on an 80-column terminal\n%s", view)
	}
	assertWithinBounds(t, view, 80, 30)
}

func TestRunsMark_KeysDoNotCollideWithTheExistingRunsListKeys(t *testing.T) {
	taken := []string{screens.KeyAcrossRuns, screens.KeySort, screens.KeyExportRun, screens.KeyExportAll, screens.KeyHelp,
		"s", "p", "q", "j", "k", "enter", "esc", "up", "down", "ctrl+c"}
	for name, key := range map[string]string{"mark": screens.KeyMarkRun, "compare": screens.KeyCompare} {
		if key == "" {
			t.Errorf("the %s key is empty", name)
		}
		for _, other := range taken {
			if key == other {
				t.Errorf("the %s key %q collides with an existing runs-list key", name, key)
			}
		}
	}
	if screens.KeyMarkRun == screens.KeyCompare {
		t.Errorf("mark and compare must be different keys, both are %q", screens.KeyMarkRun)
	}
}

func entryRE(key, descPattern string) *regexp.Regexp {
	return regexp.MustCompile(`(?mi)(^|\s)` + regexp.QuoteMeta(key) + `\s+.*` + descPattern)
}

func TestRunsMark_KeysAreListedInTheHelpText(t *testing.T) {
	s := newMarkRuns(t)

	help := s.Help()

	if !entryRE(screens.KeyMarkRun, `mark`).MatchString(help) || !entryRE(screens.KeyCompare, `compar`).MatchString(help) {
		t.Errorf("help text %q must list the mark and compare keys as entries", help)
	}
}

func TestRunsMark_KeysAreListedInTheShortHelpLineOfTheView(t *testing.T) {
	colourOff(t)

	help := tailOf(viewLines(newRuns(tableW, tallH).View(tableW, tallH)))

	if !entryRE(screens.KeyMarkRun, `mark`).MatchString(help) || !entryRE(screens.KeyCompare, `compar`).MatchString(help) {
		t.Errorf("the help line of the view %q must list the mark and compare keys", help)
	}
}

func TestRunsMark_KeysAreListedInTheOverlayWithTheirPurpose(t *testing.T) {
	colourOff(t)

	overlay := runsOverlay(newRuns(120, 60), 120, 60)

	if !entryRE(screens.KeyMarkRun, `mark`).MatchString(overlay) {
		t.Errorf("overlay must list the mark key with its purpose\n%s", overlay)
	}
	if !entryRE(screens.KeyCompare, `compar`).MatchString(overlay) {
		t.Errorf("overlay must list the compare key with its purpose\n%s", overlay)
	}
}
