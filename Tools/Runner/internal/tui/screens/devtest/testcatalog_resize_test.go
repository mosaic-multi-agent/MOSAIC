package devtest_test

// A wide path keeps the end of the path (and the cursor) visible in the test
// catalog path input, before and after a terminal resize.

import (
	"testing"

	"mosaic-run/internal/tui/screens"
	"mosaic-run/internal/tui/screens/devtest"
	"mosaic-run/internal/tui/screens/inputtest"
)

func TestTestCatalogScreen_WideValueKeepsTailVisibleAcrossResize(t *testing.T) {
	s := devtest.NewTestCatalogScreen(100, 24, screens.Styles{})
	inputtest.Type(s.Update, inputtest.WideValue())

	inputtest.AssertTailLineWithin(t, s.View(), 100)

	for _, w := range []int{60, 40} {
		s.Resize(w, 24)
		inputtest.AssertTailLineWithin(t, s.View(), w)
	}
}
