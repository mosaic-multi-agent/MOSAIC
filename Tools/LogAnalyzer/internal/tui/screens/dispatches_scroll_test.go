package screens_test

// Scrolling of the Dispatches view: a run with more dispatches than the terminal
// has lines shows a window of rows with a position hint, never draws past the
// height, and keeps the window valid when the terminal is resized.

import (
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

func TestDispatchesScroll_ShortTerminalShowsTheFirstRowsAndAPositionHint(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(withDispatchCount(40), 100, shortH, paletteStyles())

	view := s.View(100, shortH)

	assertWithinBounds(t, view, 100, shortH)
	assertContains(t, view, "builder#01")
	assertNotContains(t, view, "builder#40")
	if !scrollHintRE.MatchString(view) {
		t.Errorf("a clipped table must show a position hint\n%s", view)
	}
}

func TestDispatchesScroll_ScrollingDownRevealsTheLastRowAndStopsThere(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(withDispatchCount(40), 100, 12, paletteStyles())

	press(s, repeat("down", 200)...)
	view := s.View(100, 12)

	assertWithinBounds(t, view, 100, 12)
	assertContains(t, view, "builder#40")
	assertNotContains(t, view, "builder#01", "orchestrator")
}

func TestDispatchesScroll_ScrollingBackUpReturnsToTheFirstRows(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(withDispatchCount(40), 100, 12, paletteStyles())

	press(s, repeat("down", 30)...)
	press(s, repeat("up", 60)...)
	view := s.View(100, 12)

	assertContains(t, view, "orchestrator", "builder#01")
	assertNotContains(t, view, "builder#40")
}

func TestDispatchesScroll_ResizingKeepsTheSelectionInTheWindow(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(withDispatchCount(40), 100, 40, paletteStyles())
	press(s, repeat("down", 5)...)

	view := s.View(100, 10)

	assertWithinBounds(t, view, 100, 10)
	if countOf(view, "builder#05")+countOf(view, "builder#06") == 0 {
		t.Errorf("the rows around the fifth scroll step must stay in the window after the resize\n%s", view)
	}
	assertNotContains(t, view, "builder#40")
}

func TestDispatchesScroll_NothingIsDrawnPastTheTerminalAtAnySize(t *testing.T) {
	colourOff(t)
	s := screens.NewRunDetailScreen(dispatchRun(), 100, tallH, paletteStyles())

	for height := 0; height <= 25; height++ {
		for _, width := range []int{100, 60, 30} {
			assertWithinBounds(t, s.View(width, height), width, height)
		}
	}
}
