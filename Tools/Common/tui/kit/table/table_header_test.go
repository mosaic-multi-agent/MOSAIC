package table_test

// table_header_test.go specifies wrapped column titles: wrapping at spaces, columns as narrow as
// their widest value or longest title word, the sort marker on the last title line, and a header
// height that grows with the tallest title.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

func wrapTable(cols []table.Column, rows ...[]table.Cell) table.Table {
	tb := plain(cols, rows...)
	tb.WrapTitles = true
	return tb
}

func TestRender_WrapTitlesSplitAtSpacesAndAlignToTheBottom(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := wrapTable(
		[]table.Column{{Title: "Name"}, {Title: "Cache read tokens"}},
		cells("abcdefgh", "12"),
	)

	lines := trimRight(render(tb, 80, 10))

	// Column 2 starts at 2 (gutter) + 8 (Name column) + 2 (gap) = 12 and is as wide as "tokens".
	pad := strings.Repeat(" ", 12)
	want := []string{pad + "Cache", pad + "read", "  Name      tokens", "  abcdefgh  12"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", lines, want)
	}
}

func TestRender_WrappedColumnIsAsNarrowAsItsLongestTitleWord(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := wrapTable(
		[]table.Column{{Title: "Total input tokens"}, {Title: "Next"}},
		cells("1", "x"),
	)

	lines := trimRight(render(tb, 80, 10))

	// "tokens" is the longest word (6), so Next starts at 2 + 6 + 2.
	if got := strings.Index(lines[len(lines)-2], "Next"); got != 10 {
		t.Errorf("Next starts at %d, want 10; lines %q", got, lines)
	}
	if got := strings.Index(lines[len(lines)-1], "x"); got != 10 {
		t.Errorf("row cell starts at %d, want 10 (same as its title); lines %q", got, lines)
	}
}

func TestRender_WrappedColumnIsAsWideAsItsWidestValueWhenThatIsLonger(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := wrapTable(
		[]table.Column{{Title: "Total input tokens"}, {Title: "Next"}},
		cells("1234567890", "x"),
	)

	lines := trimRight(render(tb, 80, 10))

	if got := strings.Index(lines[len(lines)-2], "Next"); got != 14 {
		t.Errorf("Next starts at %d, want 14 (value width 10); lines %q", got, lines)
	}
}

func TestRender_WithoutWrapTitlesTheColumnKeepsTheFullTitleWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{{Title: "Total input tokens"}, {Title: "Next"}},
		cells("1", "x"),
	)

	lines := trimRight(render(tb, 80, 10))

	if len(lines) != 2 || strings.Index(lines[0], "Next") != 22 {
		t.Errorf("got %q, want one header line with Next at 22", lines)
	}
}

func TestRender_ExplicitWidthWinsOverTitleWrapping(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := wrapTable([]table.Column{{Title: "Cache read tokens", Width: 20}}, cells("1"))

	lines := trimRight(render(tb, 80, 10))

	if len(lines) != 2 || lines[0] != "  Cache read tokens" {
		t.Errorf("got %q, want the whole title on one line in the 20-cell column", lines)
	}
	if got := table.HeaderHeight(tb, 80); got != 1 {
		t.Errorf("HeaderHeight = %d, want 1", got)
	}
}

func TestRender_SortMarkerFollowsTheLastTitleLine(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := wrapTable(
		[]table.Column{{Title: "Cache read tokens"}, {Title: "Next"}},
		cells("12", "x"),
	)
	tb.SortColumn = 0

	lines := trimRight(render(tb, 80, 10))

	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 3 header lines and 1 row: %q", len(lines), lines)
	}
	if strings.Contains(lines[0], "^") || strings.Contains(lines[1], "^") {
		t.Errorf("marker on an upper title line: %q", lines)
	}
	tokens, marker := strings.Index(lines[2], "tokens"), strings.Index(lines[2], "^")
	if tokens < 0 || marker < tokens {
		t.Errorf("last title line %q: want the marker after \"tokens\"", lines[2])
	}
	// The column holds the last word plus the marker, and nothing wider.
	if next := strings.Index(lines[2], "Next"); next < marker || next > 12 {
		t.Errorf("Next at %d (marker at %d): want the column no wider than \"tokens ^\"", next, marker)
	}
}

func TestRender_DescendingSortMarkerOnWrappedTitleInUnicodeMode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	tb := wrapTable([]table.Column{{Title: "Cache read tokens"}}, cells("12"))
	tb.SortColumn, tb.SortDesc = 0, true

	lines := render(tb, 80, 10)

	if len(lines) != 4 || !strings.Contains(lines[2], "↓") || strings.Contains(lines[1], "↓") {
		t.Errorf("got %q, want the down arrow on the last title line only", lines)
	}
}

func TestHeaderHeight_GrowsWithTheTallestTitle(t *testing.T) {
	cases := []struct {
		name  string
		title []string
		want  int
	}{
		{"single words", []string{"Name", "Qty"}, 1},
		{"tallest has two lines", []string{"Name", "Total tokens"}, 2},
		{"tallest has three lines", []string{"Cache read tokens", "Total tokens", "Name"}, 3},
	}
	for _, tc := range cases {
		var cols []table.Column
		row := make([]string, len(tc.title))
		for i, title := range tc.title {
			cols = append(cols, table.Column{Title: title})
			row[i] = "1"
		}
		tb := wrapTable(cols, cells(row...))

		if got := table.HeaderHeight(tb, 80); got != tc.want {
			t.Errorf("%s: HeaderHeight = %d, want %d", tc.name, got, tc.want)
		}
		if got := len(render(tb, 80, 10)); got != tc.want+1 {
			t.Errorf("%s: rendered %d lines, want %d header lines plus 1 row", tc.name, got, tc.want)
		}
	}
}

func TestRender_HiddenColumnMarkerSitsOnTheBottomTitleLine(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := wrapTable(
		[]table.Column{{Title: "Alpha beta gamma"}, {Title: "Delta eps", Priority: 1}},
		cells("aaaaaaaaaa", "bbbbbbbbbb"),
	)

	lines := trimRight(render(tb, 20, 10))

	assertBounds(t, lines, 20, 10)
	if len(lines) != 3 || lines[0] != "  Alpha beta" || !strings.HasPrefix(lines[1], "  gamma") {
		t.Fatalf("got %q, want two title lines then the row", lines)
	}
	if !strings.HasSuffix(lines[1], "+1 >") || strings.Contains(lines[0], "+") {
		t.Errorf("got %q, want the +1 > marker on the bottom title line only", lines)
	}
}

func TestRender_WrappedTitleIsFilledGreedilyIntoTheColumnWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := wrapTable(
		[]table.Column{{Title: "Total input tokens"}, {Title: "Next"}},
		cells("12345678901", "x"),
	)

	lines := trimRight(render(tb, 80, 10))

	// The 11-cell value width lets "Total input" share a line; only "tokens" goes below.
	want := []string{"  Total input", "  tokens       Next", "  12345678901  x"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", lines, want)
	}
}

func TestRender_WordLongerThanAShrunkFlexColumnIsHardCutWithinWidthAndHeight(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := wrapTable(
		[]table.Column{{Title: "Supercalifragilistic", Flex: true, MinWidth: 3}, {Title: "Next", Width: 10}},
		cells("x", "y"),
	)

	lines := trimRight(render(tb, 20, 10))

	// The flex column gets 20 - 2 (gutter) - 2 (gap) - 10 = 6 cells, so the word is cut every 6 cells.
	assertBounds(t, lines, 20, 10)
	if len(lines) != 5 || lines[0] != "  Superc" || lines[1] != "  alifra" || lines[2] != "  gilist" {
		t.Fatalf("got %q, want the word cut into 6-cell chunks over 4 title lines and a row", lines)
	}
	if !strings.HasPrefix(lines[3], "  ic") || !strings.HasSuffix(lines[3], "Next") {
		t.Errorf("last title line %q: want the final chunk \"ic\" with Next", lines[3])
	}
	if got := table.HeaderHeight(tb, 20); got != 4 {
		t.Errorf("HeaderHeight = %d, want 4", got)
	}
}
