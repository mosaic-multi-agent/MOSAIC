package table_test

// table_height_test.go specifies the real header height: header lines plus body rows never exceed
// the given height, HeaderHeight reports what Render draws, and tables using none of the new
// features keep their one-line header.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

// tallTable has wrapped titles of up to three lines and a group over its last two columns.
func tallTable() table.Table {
	cols := []table.Column{
		{Title: "Alpha one two", Priority: 0},
		{Title: "Bravo", Priority: 1},
		{Title: "Charlie three", Priority: 2, Group: "grp"},
		{Title: "Delta four", Priority: 3, Group: "grp"},
	}
	var rows [][]table.Cell
	for i := range 8 {
		rows = append(rows, cells(fmt.Sprintf("row-%d", i), "bb", "cc", "dd"))
	}
	tb := plain(cols, rows...)
	tb.WrapTitles = true
	return tb
}

// headerLines counts the lines before the first body row.
func headerLines(lines []string) int {
	for i, l := range lines {
		if strings.Contains(l, "row-0") {
			return i
		}
	}
	return -1
}

// Guard test: a bound check that the one-line header already satisfies; not RED evidence.
func TestRender_HeaderPlusBodyRowsNeverExceedHeightOrWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := tallTable()
	tb.Cursor = 2
	for w := 1; w <= 70; w += 3 {
		for h := 1; h <= 12; h++ {
			assertBounds(t, render(tb, w, h), w, h)
		}
	}
}

func TestRender_ReportedHeaderHeightMatchesTheRenderedHeader(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := tallTable()
	for _, w := range []int{20, 30, 40, 60, 80} {
		lines := render(tb, w, 30)
		got := table.HeaderHeight(tb, w)

		if rendered := headerLines(lines); rendered != got {
			t.Errorf("width %d: HeaderHeight = %d, rendered header has %d lines: %q", w, got, rendered, lines)
		}
		if got < 3 {
			t.Errorf("width %d: HeaderHeight = %d, want at least the 3 wrapped title lines", w, got)
		}
		if want := got + len(tb.Rows); len(lines) != want {
			t.Errorf("width %d: %d lines, want %d header lines plus all %d rows", w, len(lines), got, len(tb.Rows))
		}
	}
}

func TestBodyRowsFor_SubtractsTheRealHeaderHeight(t *testing.T) {
	tb := tallTable()
	h := table.HeaderHeight(tb, 80)

	cases := map[int]int{h + 5: 5, h + 1: 1, h: 0, h - 1: 0, 0: 0, -3: 0}
	for height, want := range cases {
		if got := table.BodyRowsFor(tb, 80, height); got != want {
			t.Errorf("BodyRowsFor(height %d) = %d, want %d (header %d lines)", height, got, want, h)
		}
	}
	if h < 3 {
		t.Errorf("HeaderHeight = %d, want at least 3", h)
	}
}

// Guard test: self-consistent with HeaderHeight; not RED evidence.
func TestRender_ShowsExactlyBodyRowsForRows(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := tallTable()
	h := table.HeaderHeight(tb, 80)

	for _, height := range []int{h + 1, h + 3, h + 20} {
		lines := render(tb, 80, height)
		rows := len(lines) - h
		if want := min(table.BodyRowsFor(tb, 80, height), len(tb.Rows)); rows != want {
			t.Errorf("height %d: %d body rows shown, want %d", height, rows, want)
		}
		if len(lines) > height {
			t.Errorf("height %d: %d lines", height, len(lines))
		}
	}
}

func TestRender_LayoutAgreesWithWhatWrappedAndGroupedRenderDraws(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := tallTable()
	for w := 16; w <= 80; w++ {
		visible, hidden := table.Layout(tb, w)
		lines := render(tb, w, 30)
		header := strings.Join(lines[:max(headerLines(lines), 1)], "\n")

		first := []string{"Alpha", "Bravo", "Charlie", "Delta"}
		shown := map[int]bool{}
		for _, i := range visible {
			shown[i] = true
		}
		for i, word := range first {
			if got := strings.Contains(header, word); got != shown[i] {
				t.Errorf("width %d: column %d (%s) shown=%v in header but Layout visible=%v:\n%s", w, i, word, got, shown[i], header)
			}
		}
		if (hidden > 0) != strings.Contains(header, "+") {
			t.Errorf("width %d: hidden=%d but marker presence differs:\n%s", w, hidden, header)
		}
	}
}

func TestHeaderHeight_IgnoresColumnsLeftOutOfTheLayoutForTheGroupLine(t *testing.T) {
	// Both grouped columns have the highest priorities, so they are hidden at 20 cells.
	tb := tallTable()
	if got := table.HeaderHeight(tb, 20); got != 3 {
		t.Errorf("HeaderHeight at 20 = %d, want 3 title lines and no group line", got)
	}
	if got := table.HeaderHeight(tb, 80); got != 4 {
		t.Errorf("HeaderHeight at 80 = %d, want 3 title lines plus the group line", got)
	}
}

func TestHeaderHeight_IsOneForTablesUsingNoNewFeature(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{{Title: "Cache read tokens"}, {Title: "Name", Flex: true, MinWidth: 4}},
		cells("1", "abc"),
	)

	for _, w := range []int{10, 40, 80} {
		if got := table.HeaderHeight(tb, w); got != 1 {
			t.Errorf("width %d: HeaderHeight = %d, want 1", w, got)
		}
	}
	for _, height := range []int{0, 1, 2, 10} {
		if got, want := table.BodyRowsFor(tb, 80, height), table.BodyRows(height); got != want {
			t.Errorf("BodyRowsFor(%d) = %d, want BodyRows %d", height, got, want)
		}
	}
	if lines := render(tb, 80, 10); len(lines) != 2 {
		t.Errorf("got %d lines, want one header line and one row: %q", len(lines), lines)
	}
}

func TestRender_JustAboveTheHeaderShowsExactlyOneBodyRow(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := tallTable()

	// At 80 cells the header is 3 title lines plus the group line, so height 5 leaves one row.
	lines := render(tb, 80, 5)

	if got := headerLines(lines); got != 4 || len(lines) != 5 {
		t.Errorf("got %d header lines in %d lines, want 4 header lines and 1 row: %q", got, len(lines), lines)
	}
}
