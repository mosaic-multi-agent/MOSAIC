package screens_test

// Scrolling of the timeline: the cursor moves lane by lane through at least 150
// invocations and its lane (and detail strip) is always in view; a short
// terminal shows a position hint and a tall one shows every lane without it.

import (
	"fmt"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

func stepID(i int) string   { return fmt.Sprintf("step-%02d", i) }
func stepType(i int) string { return fmt.Sprintf("kind-%02d", i) }

func TestGanttScroll_CursorLaneAndStripStayVisibleThroughAll150Entries(t *testing.T) {
	for _, height := range []int{14, 24, 60} {
		s := openTimeline(t, manyTimelineRun(150), wideW, height)
		for i := 1; i <= 150; i++ {
			view := s.View(wideW, height)

			assertWithinBounds(t, view, wideW, height)
			if laneIndex(view, stepID(i), stepType(i)) < 0 {
				t.Fatalf("height %d: the lane of the cursor entry %s must be visible\n%s", height, stepID(i), stripStyling(view))
			}
			if lineWithAll(view, stepID(i), stepType(i)) == "" {
				t.Fatalf("height %d: the detail strip of %s must stay visible\n%s", height, stepID(i), stripStyling(view))
			}
			press(s, "down")
		}
	}
}

func TestGanttScroll_UpAndKAndJMoveTheCursorBackAndForth(t *testing.T) {
	s := openTimeline(t, manyTimelineRun(150), wideW, 20)

	press(s, repeat("j", 99)...)
	if view := s.View(wideW, 20); laneIndex(view, stepID(100), stepType(100)) < 0 || lineWithAll(view, stepType(100)) == "" {
		t.Errorf("99 presses of j must put the cursor on step-100\n%s", stripStyling(view))
	}
	press(s, repeat("k", 40)...)
	if view := s.View(wideW, 20); lineWithAll(view, stepType(60)) == "" || laneIndex(view, stepID(60), stepType(60)) < 0 {
		t.Errorf("40 presses of k must move the cursor back to step-60\n%s", stripStyling(view))
	}
	press(s, repeat("up", 5)...)
	if view := s.View(wideW, 20); lineWithAll(view, stepType(55)) == "" {
		t.Errorf("5 presses of up must move the cursor to step-55\n%s", stripStyling(view))
	}
}

func TestGanttScroll_CursorStopsAtTheFirstAndLastEntry(t *testing.T) {
	s := openTimeline(t, manyTimelineRun(150), wideW, 20)

	press(s, repeat("up", 10)...)
	if view := s.View(wideW, 20); lineWithAll(view, stepType(1)) == "" {
		t.Errorf("up on the first entry must stay on it\n%s", stripStyling(view))
	}
	press(s, repeat("down", 400)...)
	last := s.View(wideW, 20)
	if lineWithAll(last, stepType(150)) == "" || laneIndex(last, stepID(150), stepType(150)) < 0 {
		t.Errorf("down past the end must stay on step-150\n%s", stripStyling(last))
	}
	assertNotContains(t, stripStyling(last), "step-01 ")
}

func TestGanttScroll_ScrollingBackToTheTopRestoresTheFirstView(t *testing.T) {
	s := openTimeline(t, manyTimelineRun(150), wideW, 20)
	first := s.View(wideW, 20)

	press(s, repeat("down", 300)...)
	press(s, repeat("up", 300)...)

	if back := s.View(wideW, 20); back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestGanttScroll_ShortTerminalScrollsAndShowsHint(t *testing.T) {
	s := openTimeline(t, manyTimelineRun(30), wideW, 14)

	first := s.View(wideW, 14)
	assertFits(t, first, 14)
	if !scrollHintRE.MatchString(first) {
		t.Errorf("short terminal must show a scroll hint\n%s", first)
	}
	if laneIndex(first, "step-01", "kind-01") < 0 || laneIndex(first, "step-30", "kind-30") >= 0 {
		t.Errorf("the first view shows the first lanes and not the last\n%s", stripStyling(first))
	}

	var scrolled screens.Screen = s
	scrolled = press(scrolled, repeat("down", 120)...)
	last := scrolled.View(wideW, 14)
	assertFits(t, last, 14)
	if laneIndex(last, "step-30", "kind-30") < 0 || laneIndex(last, "step-01", "kind-01") >= 0 {
		t.Errorf("scrolled to the end the view shows the last lane and not the first\n%s", stripStyling(last))
	}
	if back := press(scrolled, repeat("up", 120)...).View(wideW, 14); back != first {
		t.Errorf("scrolling back to the top must restore the first view\n%s\nvs\n%s", back, first)
	}
}

func TestGanttScroll_TallTerminalShowsEveryLaneWithoutHint(t *testing.T) {
	view := timelineView(t, manyTimelineRun(150), wideW, 200)

	for _, i := range []int{1, 75, 150} {
		if laneIndex(view, stepID(i), stepType(i)) < 0 && lineWithAll(view, stepID(i)) == "" {
			t.Errorf("%s must be listed when everything fits\n%s", stepID(i), stripStyling(view))
		}
	}
	if scrollHintRE.MatchString(view) {
		t.Errorf("no scroll hint expected when everything fits\n%s", view)
	}
}

func TestGanttScroll_NothingIsDrawnPastTheTerminalWhileScrolling150Entries(t *testing.T) {
	for _, width := range []int{80, 60, 40, 20} {
		s := openTimeline(t, manyTimelineRun(150), width, 18)
		for i := 0; i < 150; i += 7 {
			assertWithinBounds(t, s.View(width, 18), width, 18)
			press(s, repeat("down", 7)...)
		}
	}
}
