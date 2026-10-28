package screens_test

// The Context view as one of the per-run views: its tab, its key from every other
// view, the keys it offers and its entries in the help overlay.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// Compile-time proof that the Context view is a per-run view with findings in its header.
var (
	_ screens.RunView = (*screens.ContextScreen)(nil)
	_ qualitySetter   = (*screens.ContextScreen)(nil)
)

func TestContext_TabBarHighlightsTheContextTab(t *testing.T) {
	view := ctxView(t, contextRun(), wideW, tallH)

	assertContains(t, stripStyling(view), "[7 Context]")
}

func TestContext_EveryOtherPerRunViewShowsTheContextTabInactive(t *testing.T) {
	colourOff(t)
	for _, v := range allFrameViews() {
		view := stripStyling(v.make(v.run, wideW, tallH, paletteStyles()).View(wideW, tallH))

		assertContains(t, view, " 7 Context ")
		assertNotContains(t, view, "[7 Context]")
	}
}

func TestContext_EveryOtherPerRunViewReachesItByItsKey(t *testing.T) {
	colourOff(t)
	for _, v := range allFrameViews() {
		s := v.make(v.run, wideW, tallH, paletteStyles())

		press(s, screens.KeyViewContext)

		if id, ok := s.Requested(); !ok || id != screens.ScreenContext {
			t.Errorf("%s: pressing %s requests (%q, %v), want the Context view", v.id, screens.KeyViewContext, id, ok)
		}
	}
}

func TestContext_EveryOtherPerRunViewListsItsKeyInTheHelpOverlay(t *testing.T) {
	colourOff(t)
	for _, v := range allFrameViews() {
		s := v.make(v.run, wideW, tallH, paletteStyles())

		assertOverlayListsViewKeys(t, s, screens.KeyViewContext)
	}
}

func TestContext_ReachesEveryOtherPerRunViewByItsKey(t *testing.T) {
	want := map[string]screens.ScreenID{
		screens.KeyViewDispatches: screens.ScreenRunDetail,
		screens.KeyViewOverview:   screens.ScreenOverview,
		screens.KeyViewAgentTypes: screens.ScreenAgentTypes,
		screens.KeyViewStatus:     screens.ScreenStatus,
		screens.KeyViewTools:      screens.ScreenTools,
		screens.KeyViewTimeline:   screens.ScreenTimeline,
	}
	for key, id := range want {
		s := ctxOpen(t)

		press(s, key)

		if got, ok := s.Requested(); !ok || got != id {
			t.Errorf("key %s requests (%q, %v), want %q", key, got, ok, id)
		}
	}
}

func TestContext_EscLeavesThePerRunViewsAndResetClearsTheRequest(t *testing.T) {
	s := ctxOpen(t)
	if s.Back() {
		t.Fatal("a fresh Context view must not ask to go back")
	}

	press(s, "esc")
	if !s.Back() {
		t.Error("esc on the table must leave the per-run views")
	}
	s.Reset()
	if s.Back() {
		t.Error("Reset must clear the back request")
	}
	press(s, screens.KeyViewTools)
	s.Reset()
	if _, ok := s.Requested(); ok {
		t.Error("Reset must clear the view request")
	}
}

func TestContext_HelpOverlayListsViewKeysSortAndDrillKeys(t *testing.T) {
	s := ctxOpen(t)
	overlay := strings.Join(overlayLines(openOverlay(s, wideW, tallH)), "\n")

	for _, k := range []string{
		screens.KeyViewDispatches, screens.KeyViewOverview, screens.KeyViewAgentTypes, screens.KeyViewStatus,
		screens.KeyViewTools, screens.KeyViewTimeline, screens.KeyViewContext, "esc", screens.KeySort, "enter",
	} {
		if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(k) + `\s+\S`).MatchString(overlay) {
			t.Errorf("the Context help overlay must list key %q\n%s", k, overlay)
		}
	}
}
