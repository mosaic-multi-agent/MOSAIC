package screens_test

// The Session screen's column legend in the ? overlay and its fit on the screen:
// the page never grows past the terminal, the real header height of the call table
// is counted, and the selected call stays on screen.

import (
	"strconv"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// sessionLegendKeys are the columns of the Session call table the legend must describe.
var sessionLegendKeys = []string{
	"time", "model", "kind", "context", "input", "cache read", "cache write", "output", "cost", "mark",
	"requested tools", "context growth",
}

func TestSessionLegend_DescribesEveryColumnOfTheCallTableWithASentence(t *testing.T) {
	colourOff(t)
	s := sessOpenFrom(t, sessionRun(), "builder#1", screens.ScreenContext, headerW, legendOverlayH)
	body := s.View(headerW, legendOverlayH)
	titles := readHeader(t, body, "time").titles()
	overlay := openOverlay(s, headerW, legendOverlayH)
	described := map[string]string{}
	for _, g := range columnGroups(overlayGroupsOf(overlay)) {
		for _, e := range g.entries {
			described[e.key] = e.desc
		}
	}

	for _, key := range append(sessionLegendKeys, titles...) {
		desc, ok := described[key]
		if !ok {
			t.Errorf("the legend has no entry for the column %q\n%s", key, stripStyling(overlay))
		} else if len(strings.Fields(desc)) < 2 {
			t.Errorf("the legend entry %q has no sentence as description: %q", key, desc)
		}
	}
}

func TestSessionLegend_OverlayAlsoListsTheKeysOfTheScreen(t *testing.T) {
	colourOff(t)
	s := sessOpen(t, "builder#1")
	overlay := strings.Join(overlayLines(openOverlay(s, wideW, tallH)), "\n")

	assertContains(t, overlay, "esc", screens.KeySort, screens.KeyViewDispatches, screens.KeyViewContext)
}

func TestSessionLegend_OverlayOfAnActorWithoutCallsStillDescribesTheColumns(t *testing.T) {
	colourOff(t)
	s := sessOpen(t, "ghost#1")

	overlay := stripStyling(openOverlay(s, headerW, legendOverlayH))

	assertContains(t, overlay, "requested tools", "context growth")
}

func TestSessionLayout_ColumnTitlesWrapAtSpacesAndTheTableHasOneLineRows(t *testing.T) {
	colourOff(t)
	view := sessOpenFrom(t, sessionRun(), "builder#1", screens.ScreenContext, 100, tallH).View(100, tallH)

	block := readHeader(t, view, "time")

	for _, row := range block.rows {
		if strings.Count(row, ":") < 2 { // each row holds exactly one clock time
			t.Errorf("a call row must be one line holding its time: %q", row)
		}
	}
	if len(block.rows) != 4 {
		t.Errorf("%d call rows, want 4 one-line rows\n%s", len(block.rows), stripStyling(view))
	}
	assertWithinBounds(t, view, 100, tallH)
}

func TestSessionLayout_StaysWithinEveryTerminalSizeWithTheCursorAtTheBottom(t *testing.T) {
	sizes := [][2]int{{240, 14}, {240, 20}, {240, 30}, {120, 24}, {80, 24}, {80, 12}, {40, 24}, {120, 6}}
	lastCall := ctxAt(400).Format("15:04:05")
	for _, size := range sizes {
		w, h := size[0], size[1]
		t.Run(strconv.Itoa(w)+"x"+strconv.Itoa(h), func(t *testing.T) {
			colourOff(t)
			s := sessOpenFrom(t, manyCallsRun(), manyCallsID, screens.ScreenContext, w, h)
			assertWithinBounds(t, s.View(w, h), w, h)

			press(s, repeat("down", 200)...)
			view := s.View(w, h)

			assertWithinBounds(t, view, w, h)
			if h >= 14 && w >= 80 {
				if sel := selectedLine(view); !strings.Contains(sel, lastCall) {
					t.Errorf("the cursor sits on the last call (%s) and it must be on screen, selected line %q\n%s", lastCall, sel, stripStyling(view))
				}
			}
		})
	}
}

func TestSessionLayout_EveryCallShowsWhenTheTerminalIsTallEnough(t *testing.T) {
	colourOff(t)
	s := sessOpenFrom(t, manyCallsRun(), manyCallsID, screens.ScreenContext, headerW, headerH+40)

	view := s.View(headerW, headerH+40)
	block := readHeader(t, view, "time")

	if len(block.rows) != 40 {
		t.Errorf("%d call rows on screen, want all 40: a taller header must not cut the last rows\n%s", len(block.rows), stripStyling(view))
	}
	if block.headerLines < 2 {
		t.Errorf("the header has %d lines, want at least 2 for the wrapped titles", block.headerLines)
	}
	assertWithinBounds(t, view, headerW, headerH+40)
}

func TestSessionLayout_ScrollingKeepsTheCursorRowAndTheFirstRowReachable(t *testing.T) {
	colourOff(t)
	h := 40
	s := sessOpenFrom(t, manyCallsRun(), manyCallsID, screens.ScreenContext, heightW, h)

	press(s, repeat("down", 25)...)
	mid := selectedLine(s.View(heightW, h))
	press(s, repeat("up", 60)...)
	top := selectedLine(s.View(heightW, h))

	if !strings.Contains(mid, ctxAt(260).Format("15:04:05")) {
		t.Errorf("25 moves down select the 26th call (260s), got %q", mid)
	}
	if !strings.Contains(top, ctxAt(10).Format("15:04:05")) {
		t.Errorf("moving back up selects the first call (10s), got %q", top)
	}
}
