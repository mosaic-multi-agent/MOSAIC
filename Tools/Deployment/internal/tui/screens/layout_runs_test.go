package screens_test

// layout_runs_test.go holds the reusable layout scenarios. Each screen's layout test file calls
// them with its own layoutScreen. Nothing in this file is a test itself.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// layoutRunFitsWhateverSelected: the view fits the terminal and starts with the title for 0, 3
// and 30 selected rows at several sizes, including the narrow width 40.
func layoutRunFitsWhateverSelected(t *testing.T, sc layoutScreen) {
	for _, size := range layoutSizes {
		for _, k := range []int{0, 3, layoutManyRows} {
			ctx := fmt.Sprintf("%dx%d with %d selected", size.w, size.h, k)
			d := sc.build(size.w, size.h, layoutManyRows, false)
			layoutSelect(d, k, layoutManyRows)
			view := d.View()
			assertLayoutFits(t, view, sc, size, ctx)
			if sc.hasSummary && k > 0 && !strings.Contains(view, fmt.Sprintf("Selected %d", k)) {
				t.Errorf("%s: %s: summary must state the selected count\n%s", sc.name, ctx, view)
			}
		}
	}
}

// layoutRunCursorStaysVisible: scrolling down while selecting every row, then back up, always
// renders exactly one cursor row, ends on the last (first) row, and keeps fitting.
func layoutRunCursorStaysVisible(t *testing.T, sc layoutScreen) {
	for _, size := range []layoutSize{{80, 12}, {40, 10}} {
		d := sc.build(size.w, size.h, layoutManyRows, false)
		var line string
		for i := 0; i < layoutManyRows+3; i++ {
			layoutToggle(d)
			layoutKey(d, tea.KeyDown)
			view := d.View()
			ctx := fmt.Sprintf("%dx%d down step %d", size.w, size.h, i)
			assertLayoutFits(t, view, sc, size, ctx)
			line = assertCursorVisible(t, view, sc, ctx)
		}
		if !strings.Contains(line, layoutRowName(layoutManyRows-1)) {
			t.Errorf("%s %dx%d: cursor row after scrolling to the end = %q; want %s",
				sc.name, size.w, size.h, line, layoutRowName(layoutManyRows-1))
		}
		for i := 0; i < layoutManyRows+3; i++ {
			layoutKey(d, tea.KeyUp)
			view := d.View()
			ctx := fmt.Sprintf("%dx%d up step %d", size.w, size.h, i)
			assertLayoutFits(t, view, sc, size, ctx)
			line = assertCursorVisible(t, view, sc, ctx)
		}
		if !containsAny(line, sc.topMarks) {
			t.Errorf("%s %dx%d: cursor row after scrolling back to the top = %q; want one of %v",
				sc.name, size.w, size.h, line, sc.topMarks)
		}
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// layoutRunResizeKeepsFit: shrinking and growing a screen that has everything selected keeps
// the view within the new height.
func layoutRunResizeKeepsFit(t *testing.T, sc layoutScreen) {
	d := sc.build(120, 40, layoutManyRows, false)
	layoutSelect(d, layoutManyRows, layoutManyRows)
	for _, size := range []layoutSize{{40, 10}, {80, 12}, {120, 40}, {40, 10}} {
		d.Resize(size.w, size.h)
		view := d.View()
		ctx := fmt.Sprintf("after resize to %dx%d", size.w, size.h)
		assertLayoutFits(t, view, sc, size, ctx)
		assertCursorVisible(t, view, sc, ctx)
	}
}

// layoutRunProvenanceKeepsFit: relabelling the rows with catalog labels keeps the view within
// the terminal height.
func layoutRunProvenanceKeepsFit(t *testing.T, sc layoutScreen) {
	for _, size := range []layoutSize{{80, 12}, {40, 10}} {
		d := sc.build(size.w, size.h, layoutManyRows, true)
		layoutSelect(d, layoutManyRows, layoutManyRows)
		view := d.View()
		ctx := fmt.Sprintf("%dx%d with provenance labels", size.w, size.h)
		assertLayoutFits(t, view, sc, size, ctx)
		assertCursorVisible(t, view, sc, ctx)
	}
}

// layoutRunShortListIsNotPadded: on a tall terminal the content block is as tall as the rows
// need (at least 3 lines), shows every row, and the tall detail text is cut to that height.
func layoutRunShortListIsNotPadded(t *testing.T, sc layoutScreen) {
	for _, n := range []int{1, 3, 5} {
		d := sc.build(80, 40, n, false)
		view := d.View()
		want := n
		if want < 3 {
			want = 3
		}
		want += sc.slack
		if got := contentHeight(view); got < 0 || got > want {
			t.Errorf("%s with %d rows: content block is %d lines tall; want at most %d\n%s",
				sc.name, n, got, want, view)
		}
		col := strings.Join(listColumn(view), "\n")
		for i := 0; i < n; i++ {
			if !strings.Contains(col, layoutRowName(i)) {
				t.Errorf("%s with %d rows: %s is not rendered in the list\n%s",
					sc.name, n, layoutRowName(i), view)
			}
		}
	}
}

// layoutRunValidationFits: confirming an empty selection on the narrowest terminal shows the
// validation message without pushing the title off screen.
func layoutRunValidationFits(t *testing.T, sc layoutScreen) {
	size := layoutSize{40, 10}
	d := sc.build(size.w, size.h, layoutManyRows, false)
	layoutKey(d, tea.KeyEnter)
	assertLayoutFits(t, d.View(), sc, size, "after enter with nothing selected")
}
