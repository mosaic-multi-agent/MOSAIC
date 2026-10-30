package table_test

// table_marker_test.go specifies shrink-before-drop and the hidden-column marker: flex columns
// shrink to their minimum before any column is dropped, and whenever columns are dropped the
// header ends with "+N >" (N = number of hidden columns).

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

// shrinkTable has natural width 2 + 26 + 2 + 6 + 2 + 6 = 44; the flex Name column can shrink to 4.
// Dropping order is Size (priority 2), then Kind (priority 1); Name is never dropped.
func shrinkTable() table.Table {
	cols := []table.Column{
		{Title: "Name", Flex: true, MinWidth: 4},
		{Title: "Kind", Priority: 1},
		{Title: "Size", Align: table.AlignRight, Priority: 2},
	}
	return plain(cols,
		cells("abcdefghijklmnopqrstuvwxyz", "kind-x", "123456"),
		cells("short", "kind-y", "7"),
	)
}

func TestRender_FlexColumnsShrinkBeforeAnyColumnIsDropped(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	lines := render(shrinkTable(), 32, 10)
	visible, hidden := table.Layout(shrinkTable(), 32)

	assertBounds(t, lines, 32, 10)
	if !reflect.DeepEqual(visible, []int{0, 1, 2}) || hidden != 0 {
		t.Errorf("Layout = %v hidden %d, want all three columns and none hidden", visible, hidden)
	}
	if !strings.Contains(lines[0], "Kind") || !strings.Contains(lines[0], "Size") {
		t.Errorf("header %q: want no column dropped when shrinking Name is enough", lines[0])
	}
	if strings.Contains(lines[0], "+") || strings.Contains(lines[0], ">") {
		t.Errorf("header %q: no hidden-column marker when nothing is dropped", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  abcdefghijk...") || !strings.Contains(lines[1], "kind-x") {
		t.Errorf("row %q: want the long name truncated with an ellipsis and the other cells intact", lines[1])
	}
}

func TestRender_FlexColumnsShrinkOnlyAsFarAsNeeded(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	// width 33 leaves one cell more than the 32 needed with Name at 14
	lines := render(shrinkTable(), 33, 10)

	if !strings.HasPrefix(lines[1], "  abcdefghijkl...") {
		t.Errorf("row %q: Name should be 15 cells wide (12 characters plus the ellipsis)", lines[1])
	}
}

func TestRender_DropsByPriorityOnlyWhenShrinkingIsNotEnough(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	// Name at its minimum plus Kind plus Size needs 22 cells, so 21 forces a drop.
	lines := render(shrinkTable(), 21, 10)
	visible, hidden := table.Layout(shrinkTable(), 21)

	assertBounds(t, lines, 21, 10)
	if !reflect.DeepEqual(visible, []int{0, 1}) || hidden != 1 {
		t.Errorf("Layout = %v hidden %d, want Size dropped first (priority 2) and one hidden", visible, hidden)
	}
	if strings.Contains(lines[0], "Size") || !strings.Contains(lines[0], "Kind") {
		t.Errorf("header %q: want Size dropped and Kind kept", lines[0])
	}
	if !strings.HasSuffix(strings.TrimRight(lines[0], " "), "+1 >") {
		t.Errorf("header %q: want it to end with the marker +1 >", lines[0])
	}
	if !strings.Contains(lines[1], "...") {
		t.Errorf("row %q: the kept flex column must still be shrunk to fit beside the marker", lines[1])
	}
}

func TestRender_MarkerCountsEveryHiddenColumn(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	// Only Name (priority 0) can stay: Kind and Size are both hidden.
	lines := render(shrinkTable(), 14, 10)
	visible, hidden := table.Layout(shrinkTable(), 14)

	assertBounds(t, lines, 14, 10)
	if !reflect.DeepEqual(visible, []int{0}) || hidden != 2 {
		t.Errorf("Layout = %v hidden %d, want only Name and two hidden", visible, hidden)
	}
	if !strings.HasSuffix(strings.TrimRight(lines[0], " "), "+2 >") {
		t.Errorf("header %q: want it to end with the marker +2 >", lines[0])
	}
}

func TestRender_MarkerIsSeparatedFromTheLastHeaderByTwoSpaces(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	lines := render(priorityTable(), 42, 10)

	// 10-cell columns: Gamma padded to 10, then two spaces, then the marker.
	want := "  Alpha" + strings.Repeat(" ", 7) + "Beta" + strings.Repeat(" ", 8) +
		"Gamma" + strings.Repeat(" ", 7) + "+1 >"
	if lines[0] != want {
		t.Errorf("header %q, want %q (marker two spaces after the last visible column)", lines[0], want)
	}
}

func TestRender_MarkerUsesSingleAngleGlyphInUnicodeMode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)

	lines := render(shrinkTable(), 21, 10)

	if !strings.HasSuffix(strings.TrimRight(lines[0], " "), "+1 ›") {
		t.Errorf("header %q: want it to end with +1 followed by the single right-pointing angle", lines[0])
	}
	if strings.Contains(lines[0], ">") {
		t.Errorf("header %q: unicode mode must not use the ASCII angle", lines[0])
	}
}

func TestRender_NoMarkerWhenTheTableFits(t *testing.T) {
	for _, mode := range []tui.GlyphMode{tui.GlyphASCII, tui.GlyphUnicode} {
		useMode(t, mode)

		lines := render(priorityTable(), 80, 10)
		_, hidden := table.Layout(priorityTable(), 80)

		if hidden != 0 || strings.ContainsAny(lines[0], "+>›") {
			t.Errorf("mode %v: header %q hidden %d, want no marker when every column fits", mode, lines[0], hidden)
		}
	}
}

func TestRender_BodyRowsCarryNoMarkerAndStayOneLine(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := shrinkTable()
	tb.Cursor = 1

	lines := render(tb, 21, 10)

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want header plus 2 rows (no wrapping): %q", len(lines), lines)
	}
	for _, l := range lines[1:] {
		if strings.Contains(l, "+") {
			t.Errorf("body row %q must not carry the hidden-column marker", l)
		}
	}
}

func TestRender_MarkerFitsAtEveryWidthAndMatchesLayout(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := priorityTable()
	// Alpha (priority 0) plus the "+3 >" marker needs 2 + 10 + 6 = 18 cells.
	for w := 18; w <= 60; w++ {
		lines := render(tb, w, 10)
		visible, hidden := table.Layout(tb, w)

		assertBounds(t, lines, w, 10)
		if len(visible)+hidden != len(tb.Columns) {
			t.Errorf("width %d: %d visible + %d hidden != %d columns", w, len(visible), hidden, len(tb.Columns))
		}
		if got := len(shownTitles(lines[0])); got != len(visible) {
			t.Errorf("width %d: header shows %d columns, Layout says %d: %q", w, got, len(visible), lines[0])
		}
		header := strings.TrimRight(lines[0], " ")
		if want := fmt.Sprintf("+%d >", hidden); hidden > 0 && !strings.HasSuffix(header, want) {
			t.Errorf("width %d: header %q must end with %q", w, header, want)
		}
		if hidden == 0 && strings.Contains(header, "+") {
			t.Errorf("width %d: header %q carries a marker although nothing is hidden", w, header)
		}
	}
}
