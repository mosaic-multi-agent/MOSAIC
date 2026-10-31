package table_test

// table_group_test.go specifies group labels: one label line above adjacent related columns,
// centred over their span, never widening a column, and ignoring hidden columns.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

// groupTable has Name (4 wide, starts at 2), then grouped columns A and B (8 wide each).
// A starts at 8 and B at 18, so the group span is cells 8..25.
func groupTable() table.Table {
	return plain(
		[]table.Column{
			{Title: "Name"},
			{Title: "Before", Group: "compactions"},
			{Title: "After", Group: "compactions"},
		},
		cells("nnnn", "aaaaaaaa", "bbbbbbbb"),
	)
}

// span returns the cells [from, to) of line, padded with spaces when the line is shorter.
func span(line string, from, to int) string {
	r := []rune(line + strings.Repeat(" ", to))
	return string(r[from:to])
}

func TestRender_GroupLabelIsCentredOverItsColumnsWithRuleFill(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	lines := render(groupTable(), 80, 10)

	if len(lines) != 3 {
		t.Fatalf("got %d lines, want group line, title line and one row: %q", len(lines), lines)
	}
	if strings.TrimSpace(span(lines[0], 0, 8)) != "" || strings.TrimSpace(span(lines[0], 26, 80)) != "" {
		t.Errorf("group line %q: nothing may be drawn outside the group span 8..25", lines[0])
	}
	got := span(lines[0], 8, 26)
	label := strings.Index(got, "compactions")
	if label < 0 || !strings.HasPrefix(got, "-") || !strings.HasSuffix(got, "-") {
		t.Fatalf("span %q: want the label filled on both sides with - rules", got)
	}
	left, right := label, len(got)-label-len("compactions")
	if d := left - right; d < -1 || d > 1 {
		t.Errorf("span %q: label is not centred (%d cells left, %d right)", got, left, right)
	}
	if rest := strings.NewReplacer("compactions", "", "-", "", " ", "").Replace(got); rest != "" {
		t.Errorf("span %q holds unexpected characters %q", got, rest)
	}
	if !strings.Contains(lines[1], "Before") || !strings.Contains(lines[1], "After") {
		t.Errorf("title line %q lost the column titles", lines[1])
	}
}

func TestRender_GroupRuleUsesBoxDrawingLineInUnicodeMode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)

	lines := render(groupTable(), 80, 10)

	if !strings.Contains(lines[0], "─") || strings.Contains(lines[0], "-") {
		t.Errorf("group line %q: want U+2500 rules and no ASCII hyphen", lines[0])
	}
}

func TestHeaderHeight_CountsTheGroupLine(t *testing.T) {
	tb := groupTable()
	if got := table.HeaderHeight(tb, 80); got != 2 {
		t.Errorf("one title line plus the group line: HeaderHeight = %d, want 2", got)
	}
	tb.WrapTitles = true
	tb.Columns[1].Title = "Before the change"
	if got := table.HeaderHeight(tb, 80); got != 4 {
		t.Errorf("three title lines (before/the/change) plus the group line: HeaderHeight = %d, want 4", got)
	}
}

// Guard test: protects existing sizing, not evidence for the new group line.
func TestRender_GroupLabelNeverWidensAColumn(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	grouped := groupTable()
	grouped.Columns[1].Group = "a very long group label that cannot fit"
	grouped.Columns[2].Group = grouped.Columns[1].Group
	ungrouped := groupTable()
	ungrouped.Columns[1].Group, ungrouped.Columns[2].Group = "", ""

	g, u := render(grouped, 80, 10), render(ungrouped, 80, 10)

	assertBounds(t, g, 80, 10)
	if g[len(g)-1] != u[len(u)-1] {
		t.Errorf("row with group %q differs from row without %q: the label must not move columns", g[len(g)-1], u[len(u)-1])
	}
}

func TestRender_GroupLabelThatDoesNotFitIsCutWithAnEllipsis(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain([]table.Column{{Title: "A", Group: "compactions"}}, cells("aaaaa"))

	lines := render(tb, 80, 10)

	got := span(lines[0], 2, 7)
	if !strings.Contains(got, "...") || strings.Contains(lines[0], "compactions") {
		t.Errorf("group line %q: want the label cut with ... inside its 5-cell span", lines[0])
	}
	if w := len(strings.TrimRight(lines[0], " ")); w > 7 {
		t.Errorf("group line %q spills past the column (ends at %d)", lines[0], w)
	}
}

func TestRender_GroupLabelUsesUnicodeEllipsisInUnicodeMode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	tb := plain([]table.Column{{Title: "A", Group: "compactions"}}, cells("aaaaa"))

	lines := render(tb, 80, 10)

	if !strings.Contains(lines[0], "…") || strings.Contains(lines[0], "...") {
		t.Errorf("group line %q: want the single-cell ellipsis", lines[0])
	}
}

func TestRender_HiddenGroupedColumnsAreNotSpanned(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{
			{Title: "Name"},
			{Title: "Before", Group: "compactions"},
			{Title: "After", Group: "compactions"},
			{Title: "Third", Group: "compactions", Priority: 1},
		},
		cells("nnnn", "aaaaaaaa", "bbbbbbbb", "cccccccc"),
	)

	lines := render(tb, 32, 10)
	visible, hidden := table.Layout(tb, 32)

	assertBounds(t, lines, 32, 10)
	if len(visible) != 3 || hidden != 1 {
		t.Fatalf("Layout = %v hidden %d, want the third group column hidden", visible, hidden)
	}
	if strings.TrimSpace(span(lines[0], 26, 32)) != "" || strings.Contains(lines[0], "+") {
		t.Errorf("group line %q: the label must stop at the last visible grouped column", lines[0])
	}
	if got := span(lines[0], 8, 26); !strings.Contains(got, "compactions") || !strings.HasSuffix(got, "-") {
		t.Errorf("group span %q: want the label centred over the two visible columns", got)
	}
	if !strings.HasSuffix(strings.TrimRight(lines[1], " "), "+1 >") {
		t.Errorf("title line %q: want the hidden marker on the bottom title line", lines[1])
	}
}

// Guard test: passes without group support, kept as a regression check.
func TestRender_NoGroupLineWhenEveryGroupedColumnIsHidden(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{
			{Title: "Name"},
			{Title: "Before", Group: "compactions", Priority: 1},
			{Title: "After", Group: "compactions", Priority: 1},
		},
		cells("nnnn", "aaaaaaaa", "bbbbbbbb"),
	)

	lines := render(tb, 14, 10)
	_, hidden := table.Layout(tb, 14)

	if hidden != 2 || len(lines) != 2 || strings.Contains(strings.Join(lines, ""), "compactions") {
		t.Errorf("hidden %d, lines %q: want no group line and a one-line header", hidden, lines)
	}
	if got := table.HeaderHeight(tb, 14); got != 1 {
		t.Errorf("HeaderHeight = %d, want 1", got)
	}
}

func TestRender_NonAdjacentColumnsOfOneGroupGetSeparateLabels(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{
			{Title: "One", Group: "grp"}, {Title: "Mid"}, {Title: "Two", Group: "grp"},
		},
		cells("aaaaaaaa", "bbbbbbbb", "cccccccc"),
	)

	lines := render(tb, 80, 10)

	if n := strings.Count(lines[0], "grp"); n != 2 {
		t.Errorf("group line %q: want the label once over each adjacent run, got %d", lines[0], n)
	}
}
