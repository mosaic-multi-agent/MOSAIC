package decision_test

// A wide executable path keeps the end of the path (and the cursor) visible in
// the executable-override input, before and after a terminal resize.

import (
	"testing"

	"mosaic-run/internal/tui/screens"
	"mosaic-run/internal/tui/screens/decision"
	"mosaic-run/internal/tui/screens/inputtest"
)

func TestExecOverrideScreen_WideValueKeepsTailVisibleAcrossResize(t *testing.T) {
	s := decision.NewExecOverrideScreen("claude-code", "/bin/claude", 1, 100, 24, screens.Styles{})
	inputtest.Type(s.Update, inputtest.WideValue())

	inputtest.AssertTailLineWithin(t, s.View(), 100)

	for _, w := range []int{60, 40} {
		s.Resize(w, 24)
		inputtest.AssertTailLineWithin(t, s.View(), w)
	}
}
