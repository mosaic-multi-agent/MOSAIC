package runconfig_test

// A wide path keeps the end of the path (and the cursor) visible in the seed
// input, before and after a terminal resize.

import (
	"testing"

	"mosaic-run/internal/tui/screens"
	"mosaic-run/internal/tui/screens/inputtest"
	"mosaic-run/internal/tui/screens/runconfig"
)

func TestSeedInputScreen_WideValueKeepsTailVisibleAcrossResize(t *testing.T) {
	s := runconfig.NewSeedInputScreen(100, 24, screens.Styles{})
	inputtest.Type(s.Update, inputtest.WideValue())

	inputtest.AssertTailLineWithin(t, s.View(), 100)

	for _, w := range []int{60, 40} {
		s.Resize(w, 24)
		inputtest.AssertTailLineWithin(t, s.View(), w)
	}
}
