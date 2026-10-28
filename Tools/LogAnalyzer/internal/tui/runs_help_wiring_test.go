package tui

// runs_help_wiring_test.go verifies that the help overlay of the runs list takes
// every key while it is open, including the keys the model itself handles before
// the screen sees them (source, across runs, pricing, export), and that esc closes
// the overlay instead of quitting.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/tui/screens"
)

// runsHelpOpen reports whether the runs screen shows its help overlay.
func runsHelpOpen(m Model) bool {
	help, ok := any(m.runsScreen).(interface{ HelpOpen() bool })
	return ok && help.HelpOpen()
}

func TestRunsHelpOverlay_OpensWithTheHelpKey(t *testing.T) {
	m := send(newExportModel(t, newTestDigestStore(false)), screens.KeyHelp)

	assertOn(t, m, screens.ScreenRuns)
	if !runsHelpOpen(m) {
		t.Error("the help key must open the overlay of the runs list")
	}
}

func TestRunsHelpOverlay_TakesTheKeysTheModelHandlesBeforeTheScreen(t *testing.T) {
	for _, key := range []string{"s", screens.KeyAcrossRuns, "p", screens.KeyExportRun, screens.KeyExportAll} {
		t.Run(key, func(t *testing.T) {
			m := send(newExportModel(t, newTestDigestStore(false)), screens.KeyHelp)

			m = send(m, key)

			assertOn(t, m, screens.ScreenRuns)
			if !runsHelpOpen(m) {
				t.Errorf("key %q must be ignored while the overlay is open, and the overlay must stay open", key)
			}
		})
	}
}

func TestRunsHelpOverlay_KeysWorkAgainOnceItIsClosed(t *testing.T) {
	m := send(newExportModel(t, newTestDigestStore(false)), screens.KeyHelp, screens.KeyHelp, "s")

	assertOn(t, m, screens.ScreenSource)
}

func TestRunsHelpOverlay_EscClosesItWithoutQuitting(t *testing.T) {
	m := send(newExportModel(t, newTestDigestStore(false)), screens.KeyHelp)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = asModel(updated)

	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Error("esc must close the overlay, not quit the program")
		}
	}
	if runsHelpOpen(m) {
		t.Error("esc must close the overlay")
	}
	assertOn(t, m, screens.ScreenRuns)
}

func TestRunsHelpOverlay_QuestionMarkOnAPerRunViewStillOpensThatViewsOverlay(t *testing.T) {
	m := send(onRunDetail(t, navViewsReport(navViewsRun(runA, 1000))), screens.KeyHelp)

	assertOn(t, m, screens.ScreenRunDetail)
	help, ok := m.runView.(interface{ HelpOpen() bool })
	if !ok || !help.HelpOpen() {
		t.Error("the per-run view keeps its own help overlay")
	}
}
