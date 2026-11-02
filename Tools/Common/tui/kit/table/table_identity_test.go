package table_test

// table_identity_test.go specifies identity columns: the last flex column to shrink, cut in the
// middle so the tail of the identity (such as #N) stays visible.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/table"
)

const (
	identityText = "writer-agent-name-long#12" // 25 cells
	modelText    = "claude-sonnet-long-name"    // 23 cells
)

// identityTable has natural width 2 + 25 + 2 + 23 = 52; instance may shrink to 8, model to 6.
func identityTable(identity bool) table.Table {
	return plain(
		[]table.Column{
			{Title: "instance", Flex: true, MinWidth: 8, Identity: identity},
			{Title: "model", Flex: true, MinWidth: 6},
		},
		cells(identityText, modelText),
	)
}

func TestRender_IdentityColumnKeepsItsFullTextWhileOtherFlexColumnsCanShrink(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	lines := render(identityTable(true), 40, 10)

	assertBounds(t, lines, 40, 10)
	if !strings.Contains(lines[1], identityText) {
		t.Errorf("row %q: the identity text must stay whole while model can still shrink", lines[1])
	}
	if !strings.Contains(lines[1], "...") || strings.Contains(lines[1], modelText) {
		t.Errorf("row %q: the model column must be the one that shrank", lines[1])
	}
}

func TestRender_WithoutIdentityTheFirstFlexColumnShrinksFirst(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	lines := render(identityTable(false), 40, 10)

	if strings.Contains(lines[1], identityText) || !strings.Contains(lines[1], modelText) {
		t.Errorf("row %q: without Identity the left-most flex column shrinks first", lines[1])
	}
}

func TestRender_IdentityCutKeepsTheTailAfterTheLastHashAndFillsFromTheHead(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	// model is at its minimum 6, so instance gets 30 - 2 - 2 - 6 = 20 cells.
	lines := render(identityTable(true), 30, 10)

	assertBounds(t, lines, 30, 10)
	if !strings.HasPrefix(lines[1], "  writer-agent-n...#12  ") {
		t.Errorf("row %q: want head text, the ellipsis, then the #12 tail in a 20-cell column", lines[1])
	}
}

func TestRender_IdentityCutUsesTheUnicodeEllipsisInUnicodeMode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)

	lines := render(identityTable(true), 30, 10)

	if !strings.HasPrefix(lines[1], "  writer-agent-nam…#12  ") {
		t.Errorf("row %q: want the single-cell ellipsis between head and #12 tail", lines[1])
	}
}

func TestRender_IdentityTailSurvivesDownToItsMinimumWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	// Both columns are at their minimum: 2 + 8 + 2 + 6 = 18.
	lines := render(identityTable(true), 18, 10)

	assertBounds(t, lines, 18, 10)
	if !strings.Contains(lines[1], "#12") {
		t.Errorf("row %q: the #12 tail must stay visible at the minimum width", lines[1])
	}
}

func TestRender_IdentityColumnShrinksOnlyAfterEveryOtherFlexColumnIsAtItsMinimum(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := plain(
		[]table.Column{
			{Title: "first", Flex: true, MinWidth: 6},
			{Title: "instance", Flex: true, MinWidth: 8, Identity: true},
			{Title: "last", Flex: true, MinWidth: 6},
		},
		cells("first-column-value", identityText, "last-column-value"),
	)

	// natural 2 + 18 + 2 + 25 + 2 + 17 = 66; first and last can give up 12 + 11 = 23 cells.
	atEdge := render(tb, 43, 10)
	oneMore := render(tb, 42, 10)

	assertBounds(t, atEdge, 43, 10)
	if !strings.Contains(atEdge[1], identityText) {
		t.Errorf("width 43: row %q: the identity text must be whole when the other columns absorb the cut", atEdge[1])
	}
	if strings.Contains(oneMore[1], identityText) || !strings.Contains(oneMore[1], "#12") {
		t.Errorf("width 42: row %q: the identity column shrinks only now, keeping its #12 tail", oneMore[1])
	}
}

func TestRender_IdentityColumnNeverExceedsWidthOrHeightAtAnySize(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	tb := identityTable(true)
	for w := 1; w <= 60; w += 3 {
		for _, h := range []int{1, 2, 5} {
			assertBounds(t, render(tb, w, h), w, h)
		}
	}
}

func TestLayout_IdentityColumnShrinkingMatchesRender(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for w := 18; w <= 60; w++ {
		tb := identityTable(true)
		visible, hidden := table.Layout(tb, w)
		lines := render(tb, w, 10)

		if len(visible)+hidden != 2 {
			t.Errorf("width %d: %d visible + %d hidden != 2", w, len(visible), hidden)
		}
		if len(visible) == 2 && !strings.Contains(lines[0], "model") {
			t.Errorf("width %d: Layout shows both columns but header is %q", w, lines[0])
		}
	}
}
