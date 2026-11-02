package table_test

// table_edge_test.go specifies degenerate inputs: no rows, no columns, titles wider than cells,
// and a sort column that was dropped on a narrow width.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

func TestRender_NoRowsDrawsTheHeaderOnly(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain([]table.Column{{Title: "Name"}, {Title: "Qty", Align: table.AlignRight}})

	lines := render(tb, 80, 10)

	if len(lines) != 1 || !strings.Contains(lines[0], "Name") || !strings.Contains(lines[0], "Qty") {
		t.Errorf("got %q, want the header line only", lines)
	}
}

func TestRender_NoColumnsStaysWithinBoundsAndDoesNotPanic(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := table.Table{Rows: [][]table.Cell{{}, {}}, Cursor: 0, SortColumn: 0}

	out := table.Render(tb, 20, 5, theme())

	for i, l := range splitLines(out) {
		if len(l) > 20 {
			t.Errorf("line %d too wide: %q", i, l)
		}
	}
	if n := len(splitLines(out)); n > 5 {
		t.Errorf("%d lines exceed height 5", n)
	}
}

func TestRender_HeaderWiderThanCellsSizesTheColumn(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{{Title: "Description"}, {Title: "Qty", Align: table.AlignRight}},
		cells("x", "1"),
	)

	lines := trimRight(render(tb, 80, 10))

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want header plus 1 row: %q", len(lines), lines)
	}
	want := "  x" + strings.Repeat(" ", 10) + "  " + "  1"
	if lines[1] != want {
		t.Errorf("row = %q, want %q (name column as wide as its 11-cell title)", lines[1], want)
	}
}

func TestRender_DroppedSortColumnDrawsNoMarkerElsewhere(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := priorityTable()
	tb.SortColumn = 3 // Delta, dropped at this width

	lines := render(tb, 47, 10)

	if !strings.Contains(lines[0], "Alpha") || !strings.Contains(lines[0], "Gamma") || strings.Contains(lines[0], "Delta") {
		t.Fatalf("header %q: want Delta dropped and the others kept", lines[0])
	}
	if strings.ContainsAny(lines[0], "v^") {
		t.Errorf("header %q: a dropped sort column must not draw its marker on another column", lines[0])
	}
}

func TestRender_CursorStaysOnItsRowWhenColumnsAreDropped(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := priorityTable()
	tb.Cursor = 1

	lines := render(tb, 24, 10)

	if len(lines) != 3 || !strings.HasPrefix(lines[2], "> ") || strings.Contains(lines[1], ">") {
		t.Errorf("got %q, want the marker on the second row only", lines)
	}
}
