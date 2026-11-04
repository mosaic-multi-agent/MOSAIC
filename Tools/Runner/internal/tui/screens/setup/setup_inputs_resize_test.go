package setup_test

// A wide value keeps its end (and the cursor) visible in the orchestrator file
// path and task description inputs, before and after a terminal resize.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/tui/screens"
	"mosaic-run/internal/tui/screens/inputtest"
	"mosaic-run/internal/tui/screens/setup"
)

// assertWideValueFollowsResize types the wide value into a 100-column screen
// and checks the input line at 100, 60 and 40 columns.
func assertWideValueFollowsResize(t *testing.T, update func(tea.Msg) tea.Cmd, view func() string, resize func(w, h int)) {
	t.Helper()
	inputtest.Type(update, inputtest.WideValue())
	inputtest.AssertTailLineWithin(t, view(), 100)

	for _, w := range []int{60, 40} {
		resize(w, 24)
		inputtest.AssertTailLineWithin(t, view(), w)
	}
}

func TestOrchestratorFileScreen_WideValueKeepsTailVisibleAcrossResize(t *testing.T) {
	s := setup.NewOrchestratorFileScreen(100, 24, screens.Styles{})

	assertWideValueFollowsResize(t, s.Update, s.View, s.Resize)
}

func TestTaskScreen_WideValueKeepsTailVisibleAcrossResize(t *testing.T) {
	s := setup.NewTaskScreen(100, 24, screens.Styles{})

	assertWideValueFollowsResize(t, s.Update, s.View, s.Resize)
}
