package chart_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/textwidth"
)

func quarterSegs() []chart.Segment {
	return []chart.Segment{
		{Label: "A", Value: 50, Role: tui.RoleSignalGood},
		{Label: "B", Value: 25, Role: tui.RoleSignalWarning},
		{Label: "C", Value: 25, Role: tui.RoleSignalBad},
	}
}

func assertLines(t *testing.T, got string, want ...string) {
	t.Helper()
	if g := trimmedLines(got); fmt.Sprintf("%q", g) != fmt.Sprintf("%q", want) {
		t.Errorf("got %q, want %q", g, want)
	}
}

func TestLegend_EntryIsSwatchLabelAndWholePercent(t *testing.T) {
	cases := []struct {
		mode tui.GlyphMode
		want []string
	}{
		{tui.GlyphASCII, []string{"# A 50%", "= B 25%", "+ C 25%"}},
		{tui.GlyphUnicode, []string{"█ A 50%", "▓ B 25%", "▒ C 25%"}},
	}
	for _, c := range cases {
		useMode(t, c.mode)
		assertLines(t, chart.Legend(quarterSegs(), 7, 10, theme()), c.want...)
	}
}

func TestLegend_SwatchesCycleThroughFourGlyphs(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	s := []chart.Segment{{Label: "a", Value: 1}, {Label: "b", Value: 1}, {Label: "c", Value: 1}, {Label: "d", Value: 1}, {Label: "e", Value: 1}}

	assertLines(t, chart.Legend(s, 9, 10, theme()), "# a 20%", "= b 20%", "+ c 20%", ": d 20%", "# e 20%")
}

func TestLegend_EmptyLabelDropsTheLabelAndOneSpace(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	s := []chart.Segment{{Value: 1}, {Label: "B", Value: 1}}

	assertLines(t, chart.Legend(s, 20, 5, theme()), "# 50%  = B 50%")
}

func TestLegend_EntriesShareLineWithTwoSpaceSeparator(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	assertLines(t, chart.Legend(quarterSegs(), 20, 5, theme()), "# A 50%  = B 25%", "+ C 25%")
	assertLines(t, chart.Legend(quarterSegs(), 25, 5, theme()), "# A 50%  = B 25%  + C 25%")
	assertLines(t, chart.Legend(quarterSegs(), 15, 5, theme()), "# A 50%", "= B 25%", "+ C 25%")
}

func TestLegend_LinesArePaddedToExactlyWidthWithoutTrailingNewline(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	out := chart.Legend(quarterSegs(), 20, 5, theme())

	if strings.HasSuffix(out, "\n") {
		t.Errorf("trailing newline in %q", out)
	}
	for i, l := range strings.Split(out, "\n") {
		if w := lipgloss.Width(l); w != 20 {
			t.Errorf("line %d is %d cells, want 20: %q", i, w, l)
		}
	}
}

func TestLegend_ListsSegmentsInGivenOrderNotByValue(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	s := []chart.Segment{{Label: "Small", Value: 10}, {Label: "Large", Value: 90}}

	out := chart.Legend(s, 80, 5, theme())

	small, large := strings.Index(out, "Small 10%"), strings.Index(out, "Large 90%")
	if small < 0 || large < 0 || small > large {
		t.Errorf("want \"Small 10%%\" before \"Large 90%%\", got %q", out)
	}
}

func TestLegend_WrapsOnlyBetweenEntries(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	s := []chart.Segment{{Label: "Alpha", Value: 1}, {Label: "Beta", Value: 1}, {Label: "Gamma", Value: 1}, {Label: "Delta", Value: 1}}
	for width := 12; width <= 60; width++ {
		out := chart.Legend(s, width, 10, theme())
		for _, entry := range []string{"# Alpha 25%", "= Beta 25%", "+ Gamma 25%", ": Delta 25%"} {
			if !strings.Contains(out, entry) {
				t.Fatalf("width %d: entry %q split or missing in %q", width, entry, out)
			}
		}
		for i, l := range strings.Split(out, "\n") {
			if w := lipgloss.Width(l); w != width {
				t.Fatalf("width %d: line %d is %d cells: %q", width, i, w, l)
			}
		}
	}
}

func TestLegend_EntryWiderThanWidthSitsAloneAndIsTruncated(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	s := []chart.Segment{{Label: "A very long label", Value: 1}, {Label: "B", Value: 1}}

	out := chart.Legend(s, 10, 5, theme())

	lines := strings.Split(out, "\n")
	want := textwidth.Truncate("# A very long label 50%", 10)
	if len(lines) != 2 || strings.TrimRight(lines[0], " ") != want || strings.TrimRight(lines[1], " ") != "= B 50%" {
		t.Errorf("got %q, want first line %q then the second entry", lines, want)
	}
}

func TestLegend_ShareRoundsEachEntryOnItsOwnHalfUp(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name   string
		values []float64
		want   []string
	}{
		{"equal thirds", []float64{1, 1, 1}, []string{"# a 33%", "= b 33%", "+ c 33%"}},
		{"half rounds up", []float64{1, 7}, []string{"# a 13%", "= b 88%"}},
		{"negative is zero share", []float64{-3, 10}, []string{"# a 0%", "= b 100%"}},
		{"zero value reads 0%", []float64{0, 10}, []string{"# a 0%", "= b 100%"}},
		{"tiny positive reads <1%", []float64{1000, 1}, []string{"# a 100%", "= b <1%"}},
		{"all zero reads 0%", []float64{0, 0}, []string{"# a 0%", "= b 0%"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := make([]chart.Segment, len(tc.values))
			for i, v := range tc.values {
				s[i] = chart.Segment{Label: string(rune('a' + i)), Value: v}
			}
			assertLines(t, chart.Legend(s, 8, 10, theme()), tc.want...)
		})
	}
}

func TestLegend_OverflowEndsWithMoreIndicatorWithinHeight(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	assertLines(t, chart.Legend(quarterSegs(), 8, 2, theme()), "# A 50%", "+2 more")
	assertLines(t, chart.Legend(quarterSegs(), 20, 1, theme()), "# A 50%  +2 more")
	assertLines(t, chart.Legend(quarterSegs(), 30, 1, theme()), "# A 50%  = B 25%  + C 25%")
	assertLines(t, chart.Legend(quarterSegs(), 8, 3, theme()), "# A 50%", "= B 25%", "+ C 25%")
	assertLines(t, chart.Legend(quarterSegs(), 8, 1, theme()), "+3 more")
}

func TestLegend_OverflowInUnicodeModeUsesSameIndicatorText(t *testing.T) {
	useMode(t, tui.GlyphUnicode)

	assertLines(t, chart.Legend(quarterSegs(), 20, 1, theme()), "█ A 50%  +2 more")
	assertLines(t, chart.Legend(quarterSegs(), 8, 2, theme()), "█ A 50%", "+2 more")
}

func TestLegend_OverflowLastLineHoldsPartialRunThenIndicator(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	s := make([]chart.Segment, 7)
	for i := range s {
		s[i] = chart.Segment{Label: string(rune('a' + i)), Value: 1}
	}

	assertLines(t, chart.Legend(s, 20, 3, theme()), "# a 14%  = b 14%", "+ c 14%  : d 14%", "# e 14%  +2 more")
}

func TestLegend_MoreIndicatorIsTruncatedWhenWiderThanWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	out := chart.Legend(quarterSegs(), 4, 1, theme())

	assertLines(t, out, textwidth.Truncate("+3 more", 4))
}

func TestLegend_NeverExceedsHeightLines(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for height := 1; height <= 3; height++ {
		lines := trimmedLines(chart.Legend(quarterSegs(), 8, height, theme()))
		if len(lines) != height {
			t.Errorf("height %d: got %d lines %q, want %d", height, len(lines), lines, height)
		}
	}
}

func TestLegend_NonPositiveSizeOrNoSegmentsGivesEmptyString(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, wh := range [][2]int{{0, 5}, {-1, 5}, {20, 0}, {20, -2}} {
		if got := chart.Legend(quarterSegs(), wh[0], wh[1], theme()); got != "" {
			t.Errorf("width %d height %d: got %q, want empty", wh[0], wh[1], got)
		}
	}
	if got := chart.Legend(nil, 20, 5, theme()); got != "" {
		t.Errorf("no segments: got %q, want empty", got)
	}
}

func TestLegend_AsciiOutputIsSevenBitAndReadsWithColourOff(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	for _, height := range []int{1, 2, 5} {
		out := chart.Legend(quarterSegs(), 20, height, theme())
		if out == "" || !isASCII(out) {
			t.Errorf("height %d: want non-empty ASCII output, got %q", height, out)
		}
		if out != stripANSI(out) {
			t.Errorf("colour off must emit no escape sequences: %q", out)
		}
	}
}

func TestLegend_SwatchUsesSegmentRoleAndTextUsesBody(t *testing.T) {
	useColourOn(t)

	styles := cellStyles(chart.Legend(quarterSegs(), 30, 1, theme()))

	good, warn, body := rolePrefix(tui.RoleSignalGood), rolePrefix(tui.RoleSignalWarning), rolePrefix(tui.RoleBody)
	if len(styles) != 30 || styles[0] != good || styles[9] != warn {
		t.Fatalf("unexpected styles %q", styles)
	}
	for i := 2; i < 7; i++ {
		if styles[i] != body {
			t.Errorf("cell %d style %q, want the body style", i, styles[i])
		}
	}
	for i := 25; i < 30; i++ {
		if styles[i] != "" {
			t.Errorf("padding cell %d style %q, want none", i, styles[i])
		}
	}
}

func TestLegend_Golden(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode tui.GlyphMode
	}{{"legend_unicode", tui.GlyphUnicode}, {"legend_ascii", tui.GlyphASCII}} {
		t.Run(tc.name, func(t *testing.T) {
			useMode(t, tc.mode)
			var b strings.Builder
			for _, w := range []int{7, 12, 20, 30} {
				fmt.Fprintf(&b, "width %d\n%s\n", w, chart.Legend(quarterSegs(), w, 10, theme()))
			}
			assertGolden(t, tc.name, strings.TrimSuffix(b.String(), "\n"))
		})
	}
}
