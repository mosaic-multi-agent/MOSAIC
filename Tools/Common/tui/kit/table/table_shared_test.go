package table_test

// table_shared_test.go specifies the shared layout for tables stacked in one view: every table
// drawn with one layout has identical column start positions, identical hidden columns and the
// same header height.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

var sharedTitles = []string{"inst", "model", "alpha", "bravo", "charlie", "delta"}

// sharedColumns has six columns; priorities drop delta first. The flex first column cannot
// shrink below 25, so the widest table needs 142 cells and 120 cells force columns to be hidden.
func sharedColumns() []table.Column {
	cols := make([]table.Column, len(sharedTitles))
	for i, title := range sharedTitles {
		cols[i] = table.Column{Title: title, Priority: i}
	}
	cols[0].Flex, cols[0].MinWidth = true, 25
	return cols
}

// sharedRow builds a row whose cells are the column marker "c<i>" padded with x to the given widths.
func sharedRow(widths ...int) []table.Cell {
	texts := make([]string, len(widths))
	for i, w := range widths {
		texts[i] = fmt.Sprintf("c%d", i) + strings.Repeat("x", w-2)
	}
	return cells(texts...)
}

// wideShared needs 142 cells; narrowShared needs only 53 on its own.
func wideShared() table.Table {
	return table.Table{Columns: sharedColumns(), Cursor: table.NoCursor, SortColumn: table.NoSort,
		Rows: [][]table.Cell{sharedRow(30, 20, 20, 20, 20, 20)}}
}

func narrowShared() table.Table {
	return table.Table{Columns: sharedColumns(), Cursor: table.NoCursor, SortColumn: table.NoSort,
		Rows: [][]table.Cell{sharedRow(10, 6, 6, 6, 6, 6)}}
}

func middleShared() table.Table {
	return table.Table{Columns: sharedColumns(), Cursor: table.NoCursor, SortColumn: table.NoSort,
		Rows: [][]table.Cell{sharedRow(18, 12, 8, 14, 7, 9)}}
}

// columnStarts returns the start cell of every "c<i>" cell found in the row line.
func columnStarts(row string) []int {
	var out []int
	for i := range sharedTitles {
		out = append(out, strings.Index(row, fmt.Sprintf("c%dx", i)))
	}
	return out
}

func TestSharedLayout_TablesShareColumnPositionsAndHiddenColumns(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, width := range []int{120, 180} {
		tables := []table.Table{wideShared(), narrowShared(), middleShared()}
		layout := table.NewSharedLayout(tables, width)

		var headers []string
		var starts [][]int
		for i, tb := range tables {
			lines := splitLines(layout.Render(tb, 10, theme()))
			assertBounds(t, lines, width, 10)
			if len(lines) != layout.HeaderHeight()+1 {
				t.Fatalf("width %d table %d: got %d lines, want %d header lines and 1 row", width, i, len(lines), layout.HeaderHeight())
			}
			headers = append(headers, strings.TrimRight(lines[layout.HeaderHeight()-1], " "))
			starts = append(starts, columnStarts(lines[layout.HeaderHeight()]))
		}
		for i := 1; i < len(tables); i++ {
			if headers[i] != headers[0] {
				t.Errorf("width %d: header of table %d is %q, table 0 has %q", width, i, headers[i], headers[0])
			}
		}
		for i := 1; i < len(tables); i++ {
			for c := range starts[0] {
				if starts[i][c] != starts[0][c] {
					t.Errorf("width %d: table %d column %d starts at %d, table 0 at %d", width, i, c, starts[i][c], starts[0][c])
				}
			}
		}
	}
}

func TestSharedLayout_HiddenColumnsAreChosenOnceForEveryTable(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	if _, alone := table.Layout(narrowShared(), 120); alone != 0 {
		t.Fatalf("setup: the narrow table alone must fit at 120, but hides %d columns", alone)
	}
	tables := []table.Table{wideShared(), narrowShared()}

	atNarrow := table.NewSharedLayout(tables, 120)
	atWide := table.NewSharedLayout(tables, 180)

	for i, tb := range tables {
		lines := splitLines(atNarrow.Render(tb, 10, theme()))
		header := strings.TrimRight(lines[atNarrow.HeaderHeight()-1], " ")
		if !strings.Contains(header, "+") || !strings.HasSuffix(header, ">") {
			t.Errorf("width 120 table %d: header %q must carry the hidden-column marker shared by all tables", i, header)
		}
		if strings.Contains(header, "delta") {
			t.Errorf("width 120 table %d: header %q shows a column the wide table has to hide", i, header)
		}
		lines = splitLines(atWide.Render(tb, 10, theme()))
		header = strings.TrimRight(lines[atWide.HeaderHeight()-1], " ")
		if strings.Contains(header, "+") || !strings.Contains(header, "delta") {
			t.Errorf("width 180 table %d: header %q must show all six columns", i, header)
		}
	}
}

func TestSharedLayout_WidthsAreTheWidestNaturalWidthAcrossTables(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tables := []table.Table{narrowShared(), wideShared()}
	layout := table.NewSharedLayout(tables, 180)

	lines := splitLines(layout.Render(narrowShared(), 10, theme()))

	// inst is 30 wide in the wide table: model starts at 2 + 30 + 2.
	if got := strings.Index(lines[layout.HeaderHeight()], "c1x"); got != 34 {
		t.Errorf("model starts at %d in the narrow table, want 34 (the wide table's inst width)", got)
	}
}

func TestSharedLayout_HeaderHeightAndBodyRowsAgreeWithTheRender(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	mk := func(sort int) table.Table {
		tb := narrowShared()
		tb.WrapTitles = true
		tb.Columns[2].Title = "alpha beta gamma delta"
		tb.SortColumn = sort
		return tb
	}
	tables := []table.Table{mk(table.NoSort), mk(2)}
	layout := table.NewSharedLayout(tables, 180)

	h := layout.HeaderHeight()

	if h < 2 {
		t.Fatalf("HeaderHeight = %d, want at least the two lines the longest title needs", h)
	}
	if got := layout.BodyRows(10); got != 10-h {
		t.Errorf("BodyRows(10) = %d, want %d", got, 10-h)
	}
	if got := layout.BodyRows(h - 1); got != 0 {
		t.Errorf("BodyRows(%d) = %d, want 0", h-1, got)
	}
	for i, tb := range tables {
		if n := len(splitLines(layout.Render(tb, 20, theme()))); n != h+1 {
			t.Errorf("table %d: %d lines, want the shared %d header lines plus 1 row", i, n, h)
		}
	}
}

func TestSharedLayout_RenderStaysWithinWidthAndHeight(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tables := []table.Table{wideShared(), narrowShared()}
	for _, width := range []int{20, 60, 120, 180} {
		layout := table.NewSharedLayout(tables, width)
		for _, height := range []int{1, 2, 5} {
			for _, tb := range tables {
				lines := splitLines(layout.Render(tb, height, theme()))
				if len(lines) > 0 {
					assertBounds(t, lines, width, height)
				}
			}
		}
	}
}

func TestSharedLayout_TableWithDifferentColumnCountIsDrawnAlone(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	other := plain([]table.Column{{Title: "Only"}}, cells("one"))
	layout := table.NewSharedLayout([]table.Table{wideShared(), narrowShared()}, 120)

	got := layout.Render(other, 10, theme())

	if want := table.Render(other, 120, 10, theme()); got != want {
		t.Errorf("got %q, want the plain render %q", got, want)
	}
}

func TestSharedLayout_SingleTableMatchesThePlainRender(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, width := range []int{120, 180} {
		tb := wideShared()
		layout := table.NewSharedLayout([]table.Table{tb}, width)

		if got, want := layout.Render(tb, 10, theme()), table.Render(tb, width, 10, theme()); got != want {
			t.Errorf("width %d: shared render of one table differs from the plain render\n%s\n%s", width, got, want)
		}
	}
}

func TestSharedLayout_HeaderHeightIsTheTallestAcrossTablesAndEveryTableUsesIt(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	short := narrowShared()
	short.WrapTitles = true
	tall := narrowShared()
	tall.WrapTitles = true
	tall.Columns[2].Title = "alpha beta gamma delta"
	tall.Columns[3].Group, tall.Columns[4].Group = "grp", "grp"
	layout := table.NewSharedLayout([]table.Table{short, tall}, 180)

	h := layout.HeaderHeight()

	if h < 5 {
		t.Fatalf("HeaderHeight = %d, want at least 4 title lines plus the group line of the tall table", h)
	}
	for i, tb := range []table.Table{short, tall} {
		lines := splitLines(layout.Render(tb, 20, theme()))
		if len(lines) != h+1 {
			t.Errorf("table %d: %d lines, want the shared %d header lines plus 1 row", i, len(lines), h)
		}
	}
	tallLines := splitLines(layout.Render(tall, 20, theme()))
	if !strings.Contains(tallLines[0], "grp") {
		t.Errorf("group line missing from the tall table: %q", tallLines[0])
	}
}

func TestSharedLayout_TableWithoutRowsContributesItsTitlesOnly(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	empty := narrowShared()
	empty.Rows = nil
	layout := table.NewSharedLayout([]table.Table{empty, narrowShared()}, 180)

	lines := splitLines(layout.Render(narrowShared(), 10, theme()))

	// inst is 10 wide in the narrow table (value width beats the 4-cell title): model at 2 + 10 + 2.
	if got := strings.Index(lines[layout.HeaderHeight()], "c1x"); got != 14 {
		t.Errorf("model starts at %d, want 14; lines %q", got, lines)
	}
}
