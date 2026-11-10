package tui

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/report"
)

// assertViewShowsNoCost fails when a rendered view carries a cost line, a
// currency unit or a cost figure.
func assertViewShowsNoCost(t *testing.T, view string, figures ...string) {
	t.Helper()
	if strings.Contains(view, "Cost:") {
		t.Errorf("view shows a cost line; displays must not show cost.\nview:\n%s", view)
	}
	if strings.Contains(view, "USD") || strings.Contains(view, "$") {
		t.Errorf("view shows a currency amount; displays must not show cost.\nview:\n%s", view)
	}
	for _, f := range figures {
		if strings.Contains(view, f) {
			t.Errorf("view shows the cost figure %q.\nview:\n%s", f, view)
		}
	}
}

func TestSessionRollup_ShowsNoCost(t *testing.T) {
	resultA := rollupPassingResult("suite-a", 3)
	resultB := rollupFailingResult("suite-b", 2, 2)
	runner := newPerCallRunner(resultA, resultB)
	m := driveTwoSuitesToCompletion(t, []string{"suite-a.yaml", "suite-b.yaml"}, runner)
	if m.Screen() != ScreenResults {
		t.Fatalf("expected ScreenResults, got %q", m.Screen())
	}

	view := safeView(t, m)

	if !strings.Contains(strings.ToLower(view), "session") {
		t.Fatalf("results view has no session roll-up to check.\nview:\n%s", view)
	}
	assertViewShowsNoCost(t, view)
}

func TestDetail_ShowsNoCost(t *testing.T) {
	classes := []report.OutcomeClass{report.ClassAssertionFailure, report.ClassTimeout, report.ClassPass}
	for _, class := range classes {
		t.Run(string(class), func(t *testing.T) {
			run := fixtureRunReport("test-under-test", class)
			m := modelWithFinishedResult(t, run)

			view := safeView(t, m)

			if view == "" {
				t.Fatalf("detail view is empty")
			}
			assertViewShowsNoCost(t, view, "0.15")
		})
	}
}
