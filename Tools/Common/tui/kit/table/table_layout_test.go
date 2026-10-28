package table_test

// table_layout_test.go specifies content-sized columns, alignment, the cursor gutter, the sort
// marker, offsets and the height bound.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

func qtyTable() table.Table {
	return plain(
		[]table.Column{{Title: "Name"}, {Title: "Qty", Align: table.AlignRight}},
		cells("ab", "5"), cells("abcdef", "1234"),
	)
}

func TestRender_ColumnsAreContentSizedAndRightAlignedNumbers(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	lines := trimRight(render(qtyTable(), 80, 10))

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want header plus 2 rows: %q", len(lines), lines)
	}
	if lines[1] != "  ab         5" || lines[2] != "  abcdef  1234" {
		t.Errorf("rows = %q, %q; want gutter, name padded to 6, two spaces, number right-aligned to 4",
			lines[1], lines[2])
	}
	if strings.Index(lines[0], "Name") != 2 || !strings.Contains(lines[0], "Qty") {
		t.Errorf("header %q: want Name at the first column after the gutter, and Qty", lines[0])
	}
}

func TestRender_CursorRowShowsMarkerAndOthersShowGutter(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := qtyTable()
	tb.Cursor = 1

	lines := render(tb, 80, 10)

	if !strings.HasPrefix(lines[2], "> abcdef") {
		t.Errorf("cursor row = %q, want it to start with \"> \"", lines[2])
	}
	if !strings.HasPrefix(lines[1], "  ab") || strings.Contains(lines[0]+lines[1], ">") {
		t.Errorf("non-cursor lines %q, %q must have a blank gutter and no marker", lines[0], lines[1])
	}
}

func TestRender_ZeroValueCursorAndSortUseRowAndColumnZero(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := table.Table{
		Columns: []table.Column{{Title: "Name"}, {Title: "Qty"}},
		Rows:    [][]table.Cell{cells("ab", "5"), cells("cd", "6")},
	}

	lines := render(tb, 80, 10)

	if !strings.HasPrefix(lines[1], "> ab") || strings.HasPrefix(lines[2], ">") {
		t.Errorf("rows %q, %q: want the cursor on row 0 only", lines[1], lines[2])
	}
	if !strings.Contains(lines[0], "^") {
		t.Errorf("header %q: want an ascending marker on column 0", lines[0])
	}
}

func TestRender_OutOfRangeCursorShowsNoMarker(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, cur := range []int{table.NoCursor, 99, -7} {
		tb := qtyTable()
		tb.Cursor = cur
		out := table.Render(tb, 80, 10, theme())
		if out == "" || strings.Contains(out, ">") {
			t.Errorf("cursor %d: want rows without a cursor marker, got:\n%s", cur, out)
		}
	}
}

func TestRender_SortMarkerOnSortColumnOnlyAndNeverReorders(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := qtyTable()
	tb.SortColumn = 1

	asc := render(tb, 80, 10)
	tb.SortDesc = true
	desc := render(tb, 80, 10)
	none := render(qtyTable(), 80, 10)

	if !strings.Contains(asc[0], "^") || strings.Contains(asc[0], "v") {
		t.Errorf("ascending header %q: want ^ and no v", asc[0])
	}
	if !strings.Contains(desc[0], "v") || strings.Contains(desc[0], "^") {
		t.Errorf("descending header %q: want v and no ^", desc[0])
	}
	if strings.ContainsAny(none[0], "v^") {
		t.Errorf("no-sort header %q must carry no marker", none[0])
	}
	if strings.Index(asc[0], "^") < strings.Index(asc[0], "Qty") {
		t.Errorf("marker in %q must follow the sort column's title, not the other column", asc[0])
	}
	if !strings.HasPrefix(desc[1], "  ab") || !strings.HasPrefix(desc[2], "  abcdef") {
		t.Errorf("rows were reordered: %q", desc)
	}
}

func TestRender_SortMarkerGlyphsInUnicodeMode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	tb := qtyTable()
	tb.SortColumn = 0
	up := table.Render(tb, 80, 10, theme())
	tb.SortDesc = true
	down := table.Render(tb, 80, 10, theme())

	if !strings.Contains(up, "↑") || !strings.Contains(down, "↓") {
		t.Errorf("want up and down arrows; got up=%q down=%q", up, down)
	}
}

func TestRender_HeightBoundsLinesAndOffsetScrolls(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	rows := [][]table.Cell{cells("row-0"), cells("row-1"), cells("row-2"), cells("row-3"), cells("row-4")}
	base := plain([]table.Column{{Title: "Name"}}, rows...)

	short := render(base, 80, 3)
	headerOnly := render(base, 80, 1)
	if len(short) != 3 || !strings.Contains(short[2], "row-1") {
		t.Errorf("height 3: got %q, want header + row-0 + row-1", short)
	}
	if len(headerOnly) != 1 || !strings.Contains(headerOnly[0], "Name") {
		t.Errorf("height 1: got %q, want the header only", headerOnly)
	}
	if out := table.Render(base, 80, 0, theme()); out != "" {
		t.Errorf("height 0: got %q, want empty", out)
	}

	scrolled := base
	scrolled.Offset = 2
	lines := render(scrolled, 80, 3)
	if len(lines) != 3 || !strings.Contains(lines[1], "row-2") || !strings.Contains(lines[2], "row-3") {
		t.Errorf("offset 2: got %q, want header, row-2, row-3", lines)
	}
}

func TestRender_OffsetIsClamped(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	rows := [][]table.Cell{cells("row-0"), cells("row-1"), cells("row-2")}
	for _, off := range []int{-5, 99} {
		tb := plain([]table.Column{{Title: "Name"}}, rows...)
		tb.Offset = off
		lines := render(tb, 80, 3)
		if len(lines) == 0 || !strings.Contains(lines[0], "Name") {
			t.Fatalf("offset %d: header missing: %q", off, lines)
		}
		joined := strings.Join(lines, "\n")
		if off < 0 && !strings.Contains(joined, "row-0") {
			t.Errorf("negative offset must behave as 0: %q", lines)
		}
		if off > 0 && !strings.Contains(joined, "row-2") {
			t.Errorf("too-large offset must clamp so the last row shows: %q", lines)
		}
	}
}

func TestRender_CursorOnScrolledRow(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	rows := [][]table.Cell{cells("row-0"), cells("row-1"), cells("row-2"), cells("row-3")}
	tb := plain([]table.Column{{Title: "Name"}}, rows...)
	tb.Offset, tb.Cursor = 2, 3

	lines := render(tb, 80, 10)

	if len(lines) != 3 || !strings.HasPrefix(lines[2], "> row-3") || strings.Contains(lines[1], ">") {
		t.Errorf("got %q, want the marker on row-3 only", lines)
	}
}

func TestBodyRows(t *testing.T) {
	cases := map[int]int{10: 9, 2: 1, 1: 0, 0: 0, -4: 0}
	for height, want := range cases {
		if got := table.BodyRows(height); got != want {
			t.Errorf("BodyRows(%d) = %d, want %d", height, got, want)
		}
	}
}

func TestRender_FillCellIsRenderedAtColumnWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	var asked []int
	fill := func(w int) string { asked = append(asked, w); return strings.Repeat("#", w) }
	tb := plain(
		[]table.Column{{Title: "Name"}, {Title: "Share", Width: 8}},
		[]table.Cell{{Text: "ab"}, {Fill: fill}},
	)

	lines := trimRight(render(tb, 80, 10))

	if len(asked) == 0 || asked[0] != 8 {
		t.Errorf("Fill asked for widths %v, want the column width 8", asked)
	}
	if len(lines) < 2 || lines[1] != "  ab    ########" {
		t.Errorf("rows = %q, want the bar after the padded name", lines)
	}
}

func TestRender_CellRoleColoursOnlyWhenColourIsOn(t *testing.T) {
	build := func(role tui.StyleRole) table.Table {
		return plain([]table.Column{{Title: "Name"}}, []table.Cell{{Text: "boom", Role: role}})
	}

	useMode(t, tui.GlyphASCII)
	off := table.Render(build(tui.RoleSignalBad), 80, 10, theme())
	useColourOn(t)
	bad := table.Render(build(tui.RoleSignalBad), 80, 10, theme())
	good := table.Render(build(tui.RoleSignalGood), 80, 10, theme())

	if off == "" || strings.Contains(off, "\x1b[") {
		t.Errorf("colour off must render plain text without escape sequences: %q", off)
	}
	if pre := styleBefore(bad, "boom"); pre == "" || pre == "<text not found>" {
		t.Errorf("colour on: the escape sequence must wrap the cell text: %q", bad)
	}
	if styleBefore(bad, "boom") == styleBefore(good, "boom") {
		t.Errorf("colour on: two different roles must style the cell differently: %q vs %q", bad, good)
	}
}

func TestRender_CursorRowUsesSelectedRoleWhenColourIsOn(t *testing.T) {
	build := func(cursor int) table.Table {
		tb := plain([]table.Column{{Title: "Name"}}, cells("item"), cells("item"))
		tb.Cursor = cursor
		return tb
	}
	useColourOn(t)

	withCursor := splitLines(table.Render(build(1), 80, 10, theme()))
	without := splitLines(table.Render(build(table.NoCursor), 80, 10, theme()))

	if len(withCursor) != 3 || len(without) != 3 {
		t.Fatalf("want header plus 2 rows, got %q and %q", withCursor, without)
	}
	if !strings.Contains(stripANSI(withCursor[2]), "> item") {
		t.Errorf("cursor row %q must show the marker", withCursor[2])
	}
	if got, other := styleBefore(withCursor[2], "item"), styleBefore(withCursor[1], "item"); got == other {
		t.Errorf("cursor row text style %q equals the non-cursor row style %q; want RoleSelected on the cursor row", got, other)
	}
	if styleBefore(withCursor[1], "item") != styleBefore(without[1], "item") {
		t.Errorf("a non-cursor row must be styled the same with and without a cursor elsewhere")
	}
}
