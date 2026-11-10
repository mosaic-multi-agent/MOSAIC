package screens_test

// The page frame of the screens that are not per-run views: a header line with
// the title and badges, a separator, the body cut to the room left, and a short
// help line. Nothing is ever drawn past the width or height.

import (
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/tui/screens"
)

func samplePage() screens.Page {
	return screens.Page{
		Title:  "Log Analyzer — Runs",
		Badges: []screens.PageBadge{{Text: "3 runs", Role: good}, {Text: "2 findings", Role: warning}},
		Keys:   []tuicommon.HelpEntry{{Key: "o", Desc: "sort"}, {Key: "enter", Desc: "open"}},
		Help:   true,
	}
}

func renderedPage(p screens.Page, body []string, width, height int) []string {
	return viewLines(screens.RenderPage(p, body, width, height, paletteStyles()))
}

func TestPageBodyHeight_IsHeightMinusChromeAndNeverNegative(t *testing.T) {
	cases := []struct{ height, want int }{{10, 10 - screens.PageChromeLines}, {screens.PageChromeLines, 0}, {1, 0}, {0, 0}, {-4, 0}}
	for _, c := range cases {
		if got := screens.PageBodyHeight(c.height); got != c.want {
			t.Errorf("PageBodyHeight(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}

func TestRenderPage_ReturnsExactlyTheChromePlusTheBodyThatFits(t *testing.T) {
	colourOff(t)
	for height := 0; height <= 12; height++ {
		lines := renderedPage(samplePage(), bodyLines(5), 100, height)

		want := min(height, screens.PageChromeLines+5)
		if len(lines) != want {
			t.Errorf("height %d: %d lines, want %d\n%s", height, len(lines), want, strings.Join(lines, "\n"))
		}
	}
}

func TestRenderPage_PutsHeaderSeparatorBodyAndHelpInOrder(t *testing.T) {
	colourOff(t)

	lines := renderedPage(samplePage(), bodyLines(5), 100, 6)

	if len(lines) != 6 {
		t.Fatalf("%d lines, want 6\n%s", len(lines), strings.Join(lines, "\n"))
	}
	assertContains(t, lines[0], "Log Analyzer — Runs")
	assertSeparator(t, lines[1])
	assertContains(t, lines[2], "body-01")
	assertContains(t, lines[4], "body-03")
	assertContains(t, lines[5], "? help")
	assertNotContains(t, strings.Join(lines, "\n"), "body-04")
}

func TestRenderPage_AtHeightThreeOnlyTheChromeIsDrawn(t *testing.T) {
	colourOff(t)

	lines := renderedPage(samplePage(), bodyLines(5), 100, screens.PageChromeLines)

	if len(lines) != 3 {
		t.Fatalf("%d lines, want 3\n%s", len(lines), strings.Join(lines, "\n"))
	}
	assertContains(t, lines[0], "Log Analyzer")
	assertSeparator(t, lines[1])
	assertContains(t, lines[2], "? help")
	assertNotContains(t, strings.Join(lines, "\n"), "body-01")
}

func TestRenderPage_BadgesFollowTheTitleOnTheHeaderLine(t *testing.T) {
	colourOff(t)

	header := headOf(renderedPage(samplePage(), bodyLines(2), 100, 10))

	title, runs, findings := strings.Index(header, "Log Analyzer"), strings.Index(header, "3 runs"), strings.Index(header, "2 findings")
	if title < 0 || runs < title || findings < runs {
		t.Errorf("header must read title, then badges in order: %q", header)
	}
}

func TestRenderPage_BadgeIsDrawnInItsRole(t *testing.T) {
	for _, role := range []tuicommon.StyleRole{good, warning, bad} {
		colourOn(t)
		p := screens.Page{Title: "T", Badges: []screens.PageBadge{{Text: "2 findings", Role: role}}, Help: true}

		view := screens.RenderPage(p, bodyLines(1), 100, 10, paletteStyles())

		header := strings.SplitN(view, "\n", 2)[0]
		assertContains(t, header, styledBadge("2 findings", role))
	}
}

func TestRenderPage_HelpLineListsKeysHintAndTheHelpKeyOnlyWhenOffered(t *testing.T) {
	colourOff(t)
	p := samplePage()
	p.Hint = "1-5 of 9"

	with := renderedPage(p, bodyLines(2), 100, 10)
	helpLine := tailOf(with)

	assertContains(t, helpLine, "o sort", "enter open", "1-5 of 9")
	if !strings.HasSuffix(strings.TrimSpace(helpLine), "? help") {
		t.Errorf("the help hint must end the help line: %q", helpLine)
	}

	p.Help = false
	without := renderedPage(p, bodyLines(2), 100, 10)
	assertNotContains(t, tailOf(without), "? help")
	assertContains(t, tailOf(without), "o sort")
}

func TestRenderPage_NeverDrawsPastWidthOrHeight(t *testing.T) {
	colourOff(t)
	long := []string{strings.Repeat("x", 300), strings.Repeat("ä", 200)}
	p := samplePage()
	p.Title = strings.Repeat("Very long title ", 12)

	for _, width := range []int{200, 80, 40, 20, 8, 3, 1} {
		for _, height := range []int{20, 5, 3, 2, 1} {
			view := screens.RenderPage(p, long, width, height, paletteStyles())

			assertWithinBounds(t, view, width, height)
			if width >= 20 && height >= screens.PageChromeLines && view == "" {
				t.Errorf("a %dx%d page must draw its chrome", width, height)
			}
		}
	}
}

func TestRenderPage_NarrowWidthKeepsTheHelpKey(t *testing.T) {
	colourOff(t)
	p := samplePage()
	p.Keys = []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "navigate and scroll through everything"}, {Key: "enter", Desc: "open the selected row"}}

	lines := renderedPage(p, bodyLines(2), 30, 8)

	assertContains(t, tailOf(lines), "? help")
}

func TestRenderPage_DegenerateSizesDrawNothing(t *testing.T) {
	colourOff(t)
	for _, c := range [][2]int{{0, 10}, {-5, 10}, {80, 0}, {80, -1}} {
		if view := screens.RenderPage(samplePage(), bodyLines(3), c[0], c[1], paletteStyles()); view != "" {
			t.Errorf("size %v: want empty view, got %q", c, view)
		}
	}
}
