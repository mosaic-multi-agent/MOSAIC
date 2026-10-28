package screens_test

// What the pricing and no-data pages have in common: the page frame (title, rule,
// the existing subtitle as first body line, short help line ending in the help
// hint), the ? overlay with its open, close and scroll rules, and staying inside
// the terminal.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// sharedPage describes one page that offers the help overlay.
type sharedPage struct {
	name      string
	make      func(width, height int) screens.Screen
	title     string
	firstBody string
	keys      []string // keys the help line and the overlay list
	ignored   []string // keys that act on the page, so the overlay must swallow them
}

func sharedPages() []sharedPage {
	return []sharedPage{
		{
			name: "pricing",
			make: func(w, h int) screens.Screen {
				return screens.NewPricingScreen(pricingModels(), "/cfg/pricing.yaml", w, h, paletteStyles())
			},
			title: "Log Analyzer — Pricing", firstBody: "3 model(s) in report",
			keys: []string{"enter", "esc"}, ignored: []string{"enter", "q"},
		},
		{
			name: "no data",
			make: func(w, h int) screens.Screen {
				src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/test/OrchestrationLogs"}
				return screens.NewNoDataScreen(src, w, h, paletteStyles())
			},
			title: "Log Analyzer — No Data Found", firstBody: "The source exists but contains no run data.",
			keys: []string{"s", "esc"}, ignored: []string{"s", "enter", "q"},
		},
	}
}

func forEachSharedPage(t *testing.T, run func(t *testing.T, p sharedPage)) {
	t.Helper()
	for _, p := range sharedPages() {
		t.Run(p.name, func(t *testing.T) { run(t, p) })
	}
}

func TestListPages_PageFrameHoldsTitleRuleSubtitleAndHelpLine(t *testing.T) {
	colourOff(t)
	forEachSharedPage(t, func(t *testing.T, p sharedPage) {
		lines := assertPageLayout(t, p.make(100, 24).View(100, 24), p.title, p.firstBody, append(p.keys, "? help")...)

		if !strings.HasSuffix(strings.TrimSpace(lines[len(lines)-1]), "? help") {
			t.Errorf("the help hint must end the help line: %q", lines[len(lines)-1])
		}
	})
}

func TestListPages_NeverDrawPastTheTerminal(t *testing.T) {
	colourOff(t)
	forEachSharedPage(t, func(t *testing.T, p sharedPage) {
		assertPageWithinEverySize(t, p.make(80, 24))
	})
}

func TestListPages_HelpKeyOpensAnOverlayInPlaceOfTheBodyAndKeepsTheHeader(t *testing.T) {
	colourOff(t)
	forEachSharedPage(t, func(t *testing.T, p sharedPage) {
		s := p.make(120, 40)
		before := s.View(120, 40)

		overlay := openOverlay(s, 120, 40)

		if !helpOpenOf(s) || overlay == before {
			t.Fatal("? must open the help overlay and change the view")
		}
		lines := viewLines(overlay)
		assertContains(t, lines[0], p.title)
		assertSeparator(t, lines[1])
		assertWithinBounds(t, overlay, 120, 40)
		assertNotContains(t, overlay, p.firstBody)
	})
}

func TestListPages_OverlayHasThisScreenAndNavigationGroupsButNoViews(t *testing.T) {
	colourOff(t)
	forEachSharedPage(t, func(t *testing.T, p sharedPage) {
		overlay := pageOverlayBody(p.make(120, 60), 120, 60)

		assertContains(t, overlay, "This screen", "Navigation")
		assertNotContains(t, overlay, "Views")
		for _, k := range append([]string{screens.KeyHelp}, p.keys...) {
			assertOverlayListsKey(t, overlay, k)
		}
	})
}

func TestListPages_HelpKeyAndEscCloseTheOverlayWithoutLeavingThePage(t *testing.T) {
	colourOff(t)
	forEachSharedPage(t, func(t *testing.T, p sharedPage) {
		for _, closer := range []string{screens.KeyHelp, "esc"} {
			s := p.make(120, 40)
			before := s.View(120, 40)

			openOverlay(s, 120, 40)
			press(s, closer)

			if helpOpenOf(s) || s.View(120, 40) != before {
				t.Errorf("%q must close the overlay and restore the page", closer)
			}
			if finished(s) {
				t.Errorf("%q closing the overlay must not leave or finish the page", closer)
			}
		}
	})
}

func TestListPages_EscLeavesThePageOnlyAfterTheOverlayIsClosed(t *testing.T) {
	forEachSharedPage(t, func(t *testing.T, p sharedPage) {
		s := p.make(120, 40)
		openOverlay(s, 120, 40)

		press(s, "esc")
		if finished(s) {
			t.Fatal("the first esc must only close the overlay")
		}
		press(s, "esc")

		if !finished(s) {
			t.Error("esc after closing the overlay must leave the page")
		}
	})
}

func TestListPages_OverlaySwallowsTheKeysThePageActsOn(t *testing.T) {
	colourOff(t)
	forEachSharedPage(t, func(t *testing.T, p sharedPage) {
		s := p.make(120, 40)
		before := s.View(120, 40)

		openOverlay(s, 120, 40)
		press(s, p.ignored...)
		if finished(s) {
			t.Errorf("keys %v must be ignored while the overlay is open", p.ignored)
		}
		press(s, screens.KeyHelp)

		if s.View(120, 40) != before {
			t.Error("keys pressed while the overlay was open must change nothing")
		}
	})
}

func TestListPages_OverlayScrollsOnAShortTerminalWithinBounds(t *testing.T) {
	colourOff(t)
	forEachSharedPage(t, func(t *testing.T, p sharedPage) {
		s := p.make(80, shortH)

		first := openOverlay(s, 80, shortH)
		assertWithinBounds(t, first, 80, shortH)
		m1 := moreLineRE.FindStringSubmatch(stripStyling(first))
		if m1 == nil {
			t.Fatalf("a cut overlay must end with a more line\n%s", first)
		}
		scrolled := press(s, repeat("down", 3)...).View(80, shortH)

		assertWithinBounds(t, scrolled, 80, shortH)
		if m2 := moreLineRE.FindStringSubmatch(stripStyling(scrolled)); m2 == nil || m2[1] == m1[1] {
			t.Errorf("down must scroll the overlay (first line %v -> %v)\n%s", m1, m2, scrolled)
		}
	})
}
