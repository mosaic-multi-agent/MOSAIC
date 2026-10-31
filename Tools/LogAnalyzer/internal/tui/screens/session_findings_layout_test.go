package screens_test

// The fit of the Session screen once it holds findings and tools: the page never
// grows past the terminal, the real header heights of both tables are counted,
// every row is one line, the page does not pan, and the selected call stays on screen.

import (
	"strconv"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// manyCallsFindingsRun is the many-calls run with findings and tools on its actor.
func manyCallsFindingsRun() domain.RunReport {
	run := manyCallsRun()
	run.ActorSessions = []domain.ActorSession{
		{Actor: ctxAgent(manyCallsID, "scroller"), Findings: builderFindings(), Tools: builderTools()},
	}
	return run
}

func TestSessionFindingsLayout_StaysWithinEveryTerminalSizeWithTheCursorAtTheBottom(t *testing.T) {
	sizes := [][2]int{{240, 14}, {240, 20}, {240, 30}, {240, 60}, {120, 24}, {80, 24}, {80, 12}, {40, 24}, {120, 6}}
	lastCall := ctxAt(400).Format("15:04:05")
	for _, size := range sizes {
		w, h := size[0], size[1]
		t.Run(strconv.Itoa(w)+"x"+strconv.Itoa(h), func(t *testing.T) {
			colourOff(t)
			s := sessOpenFrom(t, manyCallsFindingsRun(), manyCallsID, screens.ScreenContext, w, h)
			assertWithinBounds(t, s.View(w, h), w, h)

			press(s, repeat("down", 200)...)
			view := s.View(w, h)

			assertWithinBounds(t, view, w, h)
			if h >= 30 && w >= 80 {
				if sel := selectedLine(view); !strings.Contains(sel, lastCall) {
					t.Errorf("the cursor sits on the last call (%s) and it must be on screen, selected line %q\n%s", lastCall, sel, stripStyling(view))
				}
			}
		})
	}
}

func TestSessionFindingsLayout_ScrollingKeepsTheCursorRowAndTheFirstRowReachable(t *testing.T) {
	colourOff(t)
	h := 60
	s := sessOpenFrom(t, manyCallsFindingsRun(), manyCallsID, screens.ScreenContext, heightW, h)

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

func TestSessionFindingsLayout_EverythingShowsWhenTheTerminalIsTallEnough(t *testing.T) {
	colourOff(t)
	h := 150
	view := sessOpenFrom(t, manyCallsFindingsRun(), manyCallsID, screens.ScreenContext, headerW, h).View(headerW, h)

	calls := readHeader(t, view, "time")
	tools := readHeader(t, view, "tool")

	if len(calls.rows) != 40 {
		t.Errorf("%d call rows on screen, want all 40: the findings and the tools must not cut calls on a tall terminal\n%s", len(calls.rows), stripStyling(view))
	}
	if len(tools.rows) != 5 {
		t.Errorf("%d tool rows on screen, want all 5", len(tools.rows))
	}
	if len(findingLines(view)) != 10 {
		t.Errorf("%d finding lines on screen, want all 10", len(findingLines(view)))
	}
	if calls.headerLines < 2 || tools.headerLines < 2 {
		t.Errorf("headers of %d and %d lines, want at least 2 for the wrapped titles", calls.headerLines, tools.headerLines)
	}
	assertWithinBounds(t, view, headerW, h)
}

func TestSessionFindingsLayout_RowsOfBothTablesAreOneLineEach(t *testing.T) {
	colourOff(t)
	h := 150
	view := sessOpenFrom(t, manyCallsFindingsRun(), manyCallsID, screens.ScreenContext, 100, h).View(100, h)

	for _, row := range readHeader(t, view, "time").rows {
		if strings.Count(row, ":") < 2 {
			t.Errorf("a call row must be one line holding its time: %q", row)
		}
	}
	for _, row := range readHeader(t, view, "tool").rows {
		if strings.TrimSpace(row) == "" {
			t.Errorf("empty tool row line %q", row)
		}
	}
	assertWithinBounds(t, view, 100, h)
}

func TestSessionFindingsLayout_NarrowTerminalDropsColumnsInsteadOfPanning(t *testing.T) {
	colourOff(t)
	h := 150
	wide := stripStyling(sessOpenFrom(t, sessionFindingsRun(), "builder#1", screens.ScreenContext, headerW, h).View(headerW, h))
	narrow := stripStyling(sessOpenFrom(t, sessionFindingsRun(), "builder#1", screens.ScreenContext, 80, h).View(80, h))

	wideTools := readHeader(t, wide, "tool")
	narrowTools := readHeader(t, narrow, "tool")

	if len(narrowTools.titles()) >= len(wideTools.titles()) {
		t.Errorf("tools columns at 80: %q, at %d: %q; a narrow terminal drops columns", narrowTools.titles(), headerW, wideTools.titles())
	}
	assertWithinBounds(t, narrow, 80, h)
}

func TestSessionFindingsLayout_ConstrainedHeightStillCountsFindingsAndTheWrappedToolsHeader(t *testing.T) {
	lastCall := ctxAt(400).Format("15:04:05")
	for _, size := range [][2]int{{240, 30}, {100, 40}} {
		w, h := size[0], size[1]
		t.Run(strconv.Itoa(w)+"x"+strconv.Itoa(h), func(t *testing.T) {
			colourOff(t)
			s := sessOpenFrom(t, manyCallsFindingsRun(), manyCallsID, screens.ScreenContext, w, h)
			press(s, repeat("down", 200)...)

			view := s.View(w, h)

			assertWithinBounds(t, view, w, h)
			if len(findingLines(view)) == 0 {
				t.Errorf("no finding line on screen\n%s", stripStyling(view))
			}
			tools := readHeader(t, view, "tool")
			if tools.headerLines < 1 || len(tools.rows) == 0 {
				t.Errorf("tools table with %d header lines and %d rows, want its header and at least one row", tools.headerLines, len(tools.rows))
			}
			if sel := selectedLine(view); !strings.Contains(sel, lastCall) {
				t.Errorf("the cursor sits on the last call (%s) and it must be on screen, selected line %q\n%s", lastCall, sel, stripStyling(view))
			}
		})
	}
}
