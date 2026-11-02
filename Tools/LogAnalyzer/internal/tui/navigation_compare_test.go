package tui

// Navigation from the runs list to the run comparison and back. The comparison
// figures come from the comparison of the two runs computed outside the screens;
// these tests only check that the right runs reach the screen in the right order.
// navViewsRun gives each run one builder dispatch costing half the run total and
// no context figures, like a run loaded from a digest without per-call records.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// compareReport has runA ($7.00, builder $3.50) listed before runB ($3.00,
// builder $1.50) under the default cost sort.
func compareReport() domain.Report {
	return navViewsReport(navViewsRun(runA, 700), navViewsRun(runB, 300))
}

// markBoth marks the first row, then the second.
func markBoth(m Model) Model {
	return send(m, screens.KeyMarkRun, "down", screens.KeyMarkRun)
}

func TestCompare_TwoMarkedRunsOpenTheComparison(t *testing.T) {
	m := markBoth(onRunsList(compareReport()))

	m = send(m, screens.KeyCompare)

	assertOn(t, m, screens.ScreenCompare)
	view := m.View()
	for _, want := range []string{"Compare", runA, runB, "builder"} {
		if !strings.Contains(view, want) {
			t.Errorf("the comparison must show %q\n%s", want, view)
		}
	}
}

func TestCompare_RunMarkedFirstIsRunA(t *testing.T) {
	m := markBoth(onRunsList(compareReport()))

	view := send(m, screens.KeyCompare).View()

	// Run A is runA (builder $3.50), run B is runB (builder $1.50): the change is a decrease.
	for _, want := range []string{"$3.50", "$1.50", "-$2.00"} {
		if !strings.Contains(view, want) {
			t.Errorf("run A first: the comparison must show %q\n%s", want, view)
		}
	}
	if strings.Contains(view, "+$2.00") {
		t.Errorf("run A first: the change must not read as an increase\n%s", view)
	}
}

func TestCompare_MarkOrderDecidesWhichRunIsA(t *testing.T) {
	// Mark runB first (second row), then runA (first row).
	m := send(onRunsList(compareReport()), "down", screens.KeyMarkRun, "up", screens.KeyMarkRun)

	view := send(m, screens.KeyCompare).View()

	for _, want := range []string{"$1.50", "$3.50", "+$2.00"} {
		if !strings.Contains(view, want) {
			t.Errorf("run B marked first: the comparison must show %q\n%s", want, view)
		}
	}
	if strings.Contains(view, "-$2.00") {
		t.Errorf("run B marked first: the change must not read as a decrease\n%s", view)
	}
}

func TestCompare_CompareKeyDoesNothingWithoutTwoMarks(t *testing.T) {
	for name, keys := range map[string][]string{
		"no mark":  {screens.KeyCompare},
		"one mark": {screens.KeyMarkRun, screens.KeyCompare},
	} {
		m := send(onRunsList(compareReport()), keys...)

		assertOn(t, m, screens.ScreenRuns)
		if t.Failed() {
			t.Fatalf("case %q left the runs list", name)
		}
	}
}

func TestCompare_EscReturnsToTheRunsList(t *testing.T) {
	m := send(markBoth(onRunsList(compareReport())), screens.KeyCompare)
	assertOn(t, m, screens.ScreenCompare)

	m = send(m, "esc")

	assertOn(t, m, screens.ScreenRuns)
}

func TestCompare_ProvisionalRunIsMarkedInProgress(t *testing.T) {
	provisional := navViewsRun(runA, 700)
	provisional.Provisional = true
	m := markBoth(onRunsList(navViewsReport(provisional, navViewsRun(runB, 300))))

	m = send(m, screens.KeyCompare)

	assertOn(t, m, screens.ScreenCompare)
	view := m.View()
	if !strings.Contains(view, screens.ProvisionalLabel) {
		t.Errorf("a provisional run in the comparison must be marked %q\n%s", screens.ProvisionalLabel, view)
	}
}

func TestCompare_FiguresTheRunsLackAreAbsentNotZero(t *testing.T) {
	m := send(markBoth(onRunsList(compareReport())), screens.KeyCompare)

	var row string
	for _, l := range strings.Split(m.View(), "\n") {
		if strings.Contains(l, "builder") {
			row = l
		}
	}

	if row == "" || !strings.Contains(row, screens.AbsentMarker) {
		t.Errorf("neither run has context figures: the average peak context must read %q\n%s", screens.AbsentMarker, m.View())
	}
}

func TestCompare_ViewStaysWithinTheTerminal(t *testing.T) {
	m := send(markBoth(onRunsList(compareReport())), screens.KeyCompare)
	assertOn(t, m, screens.ScreenCompare)

	lines := strings.Split(m.View(), "\n")

	if len(lines) > m.height {
		t.Errorf("the comparison has %d lines on a %d-line terminal", len(lines), m.height)
	}
}
