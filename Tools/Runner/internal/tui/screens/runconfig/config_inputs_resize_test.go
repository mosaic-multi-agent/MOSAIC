package runconfig

// A wide value keeps its end (and the cursor) visible in the wizard's two
// single-line inputs, the invocation timeout and the review loop limit, before
// and after a terminal resize.

import (
	"testing"

	"mosaic-run/internal/harness"
	"mosaic-run/internal/tui/screens"
	"mosaic-run/internal/tui/screens/inputtest"
)

// newWideScreen returns a 100-column wizard shown at the prompt whose label
// contains marker.
func newWideScreen(t *testing.T, marker string) *ConfigScreen {
	t.Helper()
	s := NewConfigScreen(100, 24, screens.Styles{})
	s.SetIsNewRun(true)
	s.SetPreselectedHarness(harness.CLISelections()[0].ID)
	if !driveUntilShowing(s, marker) {
		t.Fatalf("the wizard never showed the %q prompt", marker)
	}
	return s
}

func assertWideValueFollowsResize(t *testing.T, s *ConfigScreen) {
	t.Helper()
	inputtest.Type(s.Update, inputtest.WideValue())
	inputtest.AssertTailLineWithin(t, s.View(), 100)

	for _, w := range []int{60, 40} {
		s.Resize(w, 24)
		inputtest.AssertTailLineWithin(t, s.View(), w)
	}
}

func TestConfigScreen_TimeoutInput_WideValueKeepsTailVisibleAcrossResize(t *testing.T) {
	assertWideValueFollowsResize(t, newWideScreen(t, "invocation timeout"))
}

func TestConfigScreen_ReviewLimitInput_WideValueKeepsTailVisibleAcrossResize(t *testing.T) {
	assertWideValueFollowsResize(t, newWideScreen(t, limitPromptMarker))
}
