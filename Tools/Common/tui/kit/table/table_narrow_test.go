package table_test

// table_narrow_test.go specifies behaviour when the natural width exceeds the terminal width:
// priority-based column dropping, flex shrinking, display-width truncation and the final cut.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

// priorityTable has four 10-cell-wide columns, natural width 2 + 4*10 + 3*2 = 48.
func priorityTable() table.Table {
	cols := []table.Column{
		{Title: "Alpha"}, {Title: "Beta", Priority: 1},
		{Title: "Gamma", Priority: 2}, {Title: "Delta", Priority: 2},
	}
	return plain(cols,
		cells("aaaaaaaaaa", "bbbbbbbbbb", "cccccccccc", "dddddddddd"),
		cells("a", "b", "c", "d"),
	)
}

func shownTitles(header string) []string {
	var out []string
	for _, title := range []string{"Alpha", "Beta", "Gamma", "Delta"} {
		if strings.Contains(header, title) {
			out = append(out, title)
		}
	}
	return out
}

func TestRender_DropsColumnsByDescendingPriorityRightmostFirst(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		width int
		want  []string
	}{
		{48, []string{"Alpha", "Beta", "Gamma", "Delta"}},
		{47, []string{"Alpha", "Beta", "Gamma"}},
		{42, []string{"Alpha", "Beta", "Gamma"}},
		{41, []string{"Alpha", "Beta"}},
		{30, []string{"Alpha", "Beta"}},
		{29, []string{"Alpha"}},
		{18, []string{"Alpha"}},
	}
	for _, tc := range cases {
		lines := render(priorityTable(), tc.width, 10)
		assertBounds(t, lines, tc.width, 10)
		got := shownTitles(lines[0])
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("width %d: columns %v, want %v", tc.width, got, tc.want)
		}
	}
}

func TestRender_RowsKeepIdenticalColumnPositions(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := priorityTable()
	tb.Rows = append(tb.Rows, cells("aaaa", "bbbbbbbbbb", "cc", "dddddd"))

	lines := render(tb, 42, 10)

	if len(lines) != 4 {
		t.Fatalf("got %d lines, want header plus 3 rows: %q", len(lines), lines)
	}
	betaAt, gammaAt := strings.Index(lines[0], "Beta"), strings.Index(lines[0], "Gamma")
	if betaAt < 0 || gammaAt < 0 {
		t.Fatalf("header %q lacks the Beta or Gamma column", lines[0])
	}
	for i, l := range lines[1:] {
		// each body cell starts at the same cell offset as its column title
		if got := strings.Index(l, "b"); got != betaAt {
			t.Errorf("row %d: Beta cell at %d, header at %d: %q", i, got, betaAt, l)
		}
		if got := strings.Index(l, "c"); got != gammaAt {
			t.Errorf("row %d: Gamma cell at %d, header at %d: %q", i, got, gammaAt, l)
		}
	}
}

func TestRender_PriorityZeroColumnsAreNeverDropped(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{{Title: "Alpha"}, {Title: "Beta"}},
		cells("aaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbb"),
	)

	lines := render(tb, 12, 10)

	assertBounds(t, lines, 12, 10)
	if !strings.Contains(lines[0], "Alpha") {
		t.Errorf("header %q lost a priority-0 column", lines[0])
	}
}

func TestRender_FlexColumnShrinksWithEllipsisBeforeTheLineIsCut(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{{Title: "Name", Flex: true, MinWidth: 4}, {Title: "Size", Align: table.AlignRight}},
		cells("abcdefghijklmnopqrstuvwxyz", "99"),
	)

	lines := render(tb, 20, 10)

	assertBounds(t, lines, 20, 10)
	row := strings.TrimRight(lines[1], " ")
	if !strings.HasPrefix(row, "  a") || !strings.Contains(row, "...") || !strings.HasSuffix(row, "99") {
		t.Errorf("row %q: want a truncated name with ellipsis and the size intact", row)
	}
}

func TestRender_FlexTruncationUsesUnicodeEllipsisInUnicodeMode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	tb := plain(
		[]table.Column{{Title: "Name", Flex: true, MinWidth: 4}, {Title: "Size", Align: table.AlignRight}},
		cells("abcdefghijklmnopqrstuvwxyz", "99"),
	)

	lines := render(tb, 20, 10)

	assertBounds(t, lines, 20, 10)
	if !strings.Contains(lines[1], "…") || strings.Contains(lines[1], "...") {
		t.Errorf("row %q: want the single-cell ellipsis", lines[1])
	}
}

func TestRender_WideRunesAreMeasuredByDisplayWidth(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	wide := strings.Repeat("日本語", 3)
	tb := plain(
		[]table.Column{{Title: "Name", Flex: true, MinWidth: 3}, {Title: "Qty", Align: table.AlignRight}},
		cells(wide, "7"), cells("ab", "12"),
	)

	for _, w := range []int{40, 14, 10} {
		lines := render(tb, w, 10)
		assertBounds(t, lines, w, 10)
		if len(lines) != 3 {
			t.Fatalf("width %d: got %d lines, want header plus 2 rows: %q", w, len(lines), lines)
		}
		for _, l := range lines {
			if l != strings.ToValidUTF8(l, "?") {
				t.Errorf("width %d: line %q is not valid UTF-8", w, l)
			}
		}
		if truncated := strings.Contains(lines[1], "…"); truncated == (w == 40) {
			t.Errorf("width %d: row %q truncated=%v; the wide name is cut with the ellipsis only when it does not fit", w, lines[1], truncated)
		}
		if !strings.Contains(lines[2], "ab") {
			t.Errorf("width %d: second row lost its content: %q", w, lines[2])
		}
		// the right-aligned Qty column ends at the same cell on the header and both rows
		end := lipgloss.Width(strings.TrimRight(lines[0], " "))
		for _, l := range lines[1:] {
			if got := lipgloss.Width(strings.TrimRight(l, " ")); got != end {
				t.Errorf("width %d: Qty ends at cell %d, header ends at %d: %q", w, got, end, l)
			}
		}
	}
}

func TestRender_FinalCutKeepsEveryLineWithinWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{{Title: "Alpha"}, {Title: "Beta"}},
		cells("aaaaaaaaaaaaaaaaaaaa", "b"),
	)
	for _, w := range []int{1, 2, 5, 10} {
		assertBounds(t, render(tb, w, 5), w, 5)
	}
}

func TestRender_NeverExceedsWidthOrHeightAtAnySize(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := priorityTable()
	tb.Cursor = 1
	tb.SortColumn = 2
	for w := 1; w <= 60; w += 3 {
		for _, h := range []int{1, 2, 5} {
			assertBounds(t, render(tb, w, h), w, h)
		}
	}
}

func TestRender_WidthZeroDrawsNothing(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lines := splitLines(table.Render(priorityTable(), 0, 5, theme()))
	for _, l := range lines {
		if l != "" {
			t.Errorf("width 0 produced %q", l)
		}
	}
	if len(lines) == 0 {
		return
	}
	if len(lines) > 5 {
		t.Errorf("too many lines: %d", len(lines))
	}
}
