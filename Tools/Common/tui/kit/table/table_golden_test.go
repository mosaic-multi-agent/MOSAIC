package table_test

// table_golden_test.go pins the exact render, colour off and ASCII glyphs, at several widths.
// Trailing spaces are ignored. Run with -update to rewrite the files after an intended change.

import (
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

// goldenTable has natural width 82: Name 15, Type 8, Size 4, Count 5, Notes 40.
func goldenTable() table.Table {
	cols := []table.Column{
		{Title: "Name", Flex: true, MinWidth: 6},
		{Title: "Type", Priority: 2},
		{Title: "Size", Align: table.AlignRight},
		{Title: "Count", Align: table.AlignRight, Priority: 1},
		{Title: "Notes", Priority: 3},
	}
	tb := plain(cols,
		cells("alpha-entry", "general", "1200", "12", "first draft accepted"),
		cells("beta", "coder", "35", "7", "retry after schema check failed twice ok"),
		cells("gamma-long-name", "reviewer", "410", "103", ""),
	)
	tb.Cursor = 1
	return tb
}

func TestRender_Golden(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name  string
		width int
	}{
		{"table_wide", 100},
		{"table_80", 80},
		{"table_narrow", 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := render(goldenTable(), tc.width, 20)

			assertBounds(t, lines, tc.width, 20)
			assertGolden(t, tc.name, lines)
		})
	}
}
