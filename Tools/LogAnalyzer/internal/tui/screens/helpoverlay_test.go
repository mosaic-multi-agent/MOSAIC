package screens_test

// The ? help overlay of the per-run views: opens in place of the body, lists
// every key of the screen, scrolls, ignores other keys, and closes again.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

var moreLineRE = regexp.MustCompile(`-- more: (\d+)-(\d+) of (\d+) --`)

func TestHelpOverlay_ReplacesTheBodyAndKeepsTheFrame(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		t.Run(string(fv.id), func(t *testing.T) {
			s := fv.make(fv.run, 120, 40, paletteStyles())
			before := s.View(120, 40)

			overlay := openOverlay(s, 120, 40)

			if overlay == before {
				t.Fatal("? must change the view")
			}
			lines := framedLines(t, overlay)
			assertContains(t, lines[0], fv.run.Run.Label())
			assertContains(t, lines[1], "["+fv.tab+"]")
			assertWithinBounds(t, overlay, 120, 40)
		})
	}
}

func TestHelpOverlay_ListsViewExportAndNavigationKeys(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		t.Run(string(fv.id), func(t *testing.T) {
			s := fv.make(fv.run, 120, 60, paletteStyles())

			overlay := strings.Join(overlayLines(openOverlay(s, 120, 60)), "\n")

			assertContains(t, overlay, "Views", "Export", "Navigation")
			for _, k := range []string{
				screens.KeyViewDispatches, screens.KeyViewOverview, screens.KeyViewAgentTypes,
				screens.KeyViewStatus, screens.KeyViewTools, screens.KeyViewTimeline, "esc", "enter",
			} {
				if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(k) + `\s+\S`).MatchString(overlay) {
					t.Errorf("overlay must list key %q on its own entry\n%s", k, overlay)
				}
			}
			if !regexp.MustCompile(`(?m)^\s+e\s+.*(?i:export)`).MatchString(overlay) {
				t.Errorf("overlay must list the export key e\n%s", overlay)
			}
			if !regexp.MustCompile(`(?m)^\s+E\s+.*(?i:export)`).MatchString(overlay) {
				t.Errorf("overlay must list the export key E\n%s", overlay)
			}
		})
	}
}

func TestHelpOverlay_QuestionMarkAndEscCloseIt(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		for _, closer := range []string{screens.KeyHelp, "esc"} {
			s := fv.make(fv.run, 120, 40, paletteStyles())
			before := s.View(120, 40)

			openOverlay(s, 120, 40)
			press(s, closer)

			if after := s.View(120, 40); after != before {
				t.Errorf("%s: %q must close the overlay and restore the view\n%s\nvs\n%s", fv.id, closer, after, before)
			}
			if s.Back() {
				t.Errorf("%s: %q closing the overlay must not leave the view", fv.id, closer)
			}
		}
	}
}

func TestHelpOverlay_EscLeavesTheViewOnlyAfterTheOverlayIsClosed(t *testing.T) {
	for _, fv := range allFrameViews() {
		s := fv.make(fv.run, 120, 40, paletteStyles())

		openOverlay(s, 120, 40)
		press(s, "esc")
		if s.Back() {
			t.Errorf("%s: the first esc must close the overlay without leaving the view", fv.id)
		}
		press(s, "esc")

		if !s.Back() {
			t.Errorf("%s: esc after closing the overlay must leave the view", fv.id)
		}
	}
}

func TestHelpOverlay_IgnoresViewKeysAndLeaveKeysWhileOpen(t *testing.T) {
	for _, fv := range allFrameViews() {
		s := fv.make(fv.run, 120, 40, paletteStyles())

		openOverlay(s, 120, 40)
		press(s, fv.otherKey, "q")
		if _, ok := s.Requested(); ok {
			t.Errorf("%s: view key %q must be ignored while the overlay is open", fv.id, fv.otherKey)
		}
		if s.Back() {
			t.Errorf("%s: q must be ignored while the overlay is open", fv.id)
		}

		press(s, screens.KeyHelp, fv.otherKey)
		if got, ok := s.Requested(); !ok || got == fv.id {
			t.Errorf("%s: view key %q must work again after the overlay closed", fv.id, fv.otherKey)
		}
	}
}

func TestHelpOverlay_ScrollsOnAShortTerminalWithinBounds(t *testing.T) {
	colourOff(t)
	for _, fv := range allFrameViews() {
		t.Run(string(fv.id), func(t *testing.T) {
			s := fv.make(fv.run, 80, shortH, paletteStyles())

			first := openOverlay(s, 80, shortH)
			assertWithinBounds(t, first, 80, shortH)
			m1 := moreLineRE.FindStringSubmatch(stripStyling(first))
			if m1 == nil {
				t.Fatalf("a cut overlay must end with a more line\n%s", first)
			}

			scrolled := press(s, repeat("down", 3)...).View(80, shortH)
			assertWithinBounds(t, scrolled, 80, shortH)
			m2 := moreLineRE.FindStringSubmatch(stripStyling(scrolled))
			if m2 == nil || m2[1] == m1[1] {
				t.Errorf("down must scroll the overlay (first line %v -> %v)\n%s", m1, m2, scrolled)
			}
		})
	}
}

func TestHelpOverlay_NeverTouchesTheBodyScroll(t *testing.T) {
	colourOff(t)
	run := manyModelsRun(40)
	s := screens.NewOverviewScreen(run, 100, 12, paletteStyles())
	scrolled := press(s, repeat("down", 5)...).View(100, 12)

	openOverlay(s, 100, 12)
	press(s, repeat("down", 4)...)
	press(s, screens.KeyHelp)

	if after := s.View(100, 12); after != scrolled {
		t.Errorf("scrolling the overlay must not move the body\n%s\nvs\n%s", after, scrolled)
	}
}

func TestHelpKey_DoesNotCollideWithExistingKeys(t *testing.T) {
	existing := []string{
		"1", "2", "3", "4", "5", "6", "a", "s", "p", "q", "e", "E",
		"up", "down", "j", "k", "enter", "esc",
	}
	for _, k := range existing {
		if screens.KeyHelp == k || screens.KeySort == k {
			t.Errorf("new key %q collides with an existing key", k)
		}
	}
	if screens.KeyHelp != "?" {
		t.Errorf("KeyHelp = %q, want ?", screens.KeyHelp)
	}
}
