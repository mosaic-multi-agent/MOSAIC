package chart_test

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
)

func curveOf(c chart.Curve, width, height int) string {
	return chart.RenderCurve(c, width, height, theme())
}

func markerGlyphs() []rune {
	if tui.CurrentRenderMode().Glyphs == tui.GlyphASCII {
		return []rune{'|', ':', '*'}
	}
	return []rune{'┃', '│', '┊'}
}

// shape writes every cell that is neither blank nor a marker glyph as 'o', because the glyph
// of a plotted point is not fixed by the design. Trailing blanks are dropped.
func shape(line string) string {
	var b strings.Builder
	for _, r := range line {
		switch {
		case r == ' ':
			b.WriteRune(' ')
		case strings.ContainsRune(string(markerGlyphs()), r):
			b.WriteRune(r)
		default:
			b.WriteRune('o')
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func shapes(out string) []string {
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = shape(l)
	}
	return lines
}

func assertShapes(t *testing.T, out string, want ...string) {
	t.Helper()
	if got := shapes(out); !reflect.DeepEqual(got, want) {
		t.Errorf("shape differs\n got %q\nwant %q", got, want)
	}
}

func TestRenderCurve_PlacesPointsOnTheScaledGrid(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	out := curveOf(chart.Curve{Values: []float64{0, 2, 4, 2, 0}}, 9, 5)

	assertShapes(t, out,
		"    o",
		"",
		"  o   o",
		"",
		"o       o",
	)
}

func TestRenderCurve_MoreValuesThanColumnsShowTheLargestOfEachColumn(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	out := curveOf(chart.Curve{Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}, 5, 3)

	assertShapes(t, out,
		"   oo",
		" oo",
		"o",
	)
}

func TestRenderCurve_SmallSeriesCases(t *testing.T) {
	cases := []struct {
		name          string
		values        []float64
		width, height int
		want          []string
	}{
		{"single point sits in column 0 at the top", []float64{5}, 6, 3, []string{"o", "", ""}},
		{"all zero values sit on the bottom row", []float64{0, 0, 0}, 3, 4, []string{"", "", "", "ooo"}},
		{"negative values count as zero", []float64{-5, 4}, 2, 3, []string{" o", "", "o"}},
		{"empty series draws no point", nil, 5, 3, []string{"", "", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useMode(t, tui.GlyphASCII)
			assertShapes(t, curveOf(chart.Curve{Values: tc.values}, tc.width, tc.height), tc.want...)
		})
	}
}

func TestRenderCurve_OutputIsExactlyHeightLinesWithinWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	label := func(v float64) string {
		if v == 0 {
			return "0"
		}
		return "100"
	}
	curve := chart.Curve{
		Values:  []float64{3, 9, 1, 7, 5, 2, 8, 4},
		Guides:  []chart.Guide{{From: 2, To: 6, Role: tui.RoleSignalGood}, {From: 6, Role: tui.RoleSignalBad}},
		Markers: []chart.Marker{{Index: 1, Kind: chart.MarkerPrimary}, {Index: 6, Kind: chart.MarkerTertiary}},
		YLabel:  label,
	}

	for width := 0; width <= 25; width++ {
		for height := -1; height <= 7; height++ {
			out := curveOf(curve, width, height)
			if height < 2 {
				if out != "" {
					t.Fatalf("width %d height %d: want empty output, got %q", width, height, out)
				}
				continue
			}
			lines := strings.Split(out, "\n")
			if width <= 0 {
				if out != strings.Repeat("\n", height-1) {
					t.Fatalf("width %d height %d: want %d empty lines, got %q", width, height, height, out)
				}
				continue
			}
			if len(lines) != height {
				t.Fatalf("width %d height %d: got %d lines", width, height, len(lines))
			}
			for _, l := range lines {
				if runeCount(l) > width {
					t.Fatalf("width %d height %d: line %q is %d cells", width, height, l, runeCount(l))
				}
			}
		}
	}
}

func runeCount(s string) int { return len([]rune(s)) }

func TestRenderCurve_TooSmallAHeightGivesNothing(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	if out := curveOf(chart.Curve{Values: []float64{1, 2, 3}}, 10, 2); out == "" {
		t.Fatal("control: height 2 must render a curve")
	}
	for _, height := range []int{1, 0, -4} {
		if out := curveOf(chart.Curve{Values: []float64{1, 2, 3}}, 10, height); out != "" {
			t.Errorf("height %d: want empty output, got %q", height, out)
		}
	}
}

func TestRenderCurve_GutterShowsTopAndZeroLabelsOnly(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	label := func(v float64) string {
		switch v {
		case 4:
			return "top"
		case 0:
			return "0"
		}
		return "mid"
	}

	out := curveOf(chart.Curve{Values: []float64{0, 2, 4, 2, 0}, YLabel: label}, 13, 5)

	lines := strings.Split(out, "\n")
	if len(lines) != 5 || strings.Contains(out, "mid") {
		t.Fatalf("want 5 lines labelled only at the ends, got %q", out)
	}
	if strings.TrimSpace(string([]rune(lines[0])[:4])) != "top" || strings.TrimSpace(string([]rune(lines[4])[:4])) != "0" {
		t.Errorf("want top label on the first line and 0 on the last: %q", out)
	}
	for _, l := range lines[1:4] {
		if strings.TrimSpace(string([]rune(l)[:4])) != "" {
			t.Errorf("middle lines carry a blank gutter, got %q", l)
		}
	}
	plot := make([]string, len(lines))
	for i, l := range lines {
		plot[i] = shape(string([]rune(l)[4:]))
	}
	want := []string{"    o", "", "  o   o", "", "o       o"}
	if !reflect.DeepEqual(plot, want) {
		t.Errorf("the plot area follows the gutter\n got %q\nwant %q", plot, want)
	}
}

func TestRenderCurve_GutterLeavesNoPlotAreaWhenTooNarrow(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	curve := chart.Curve{Values: []float64{1, 2, 3}, YLabel: func(float64) string { return "abc" }}

	for _, width := range []int{1, 3, 4} {
		out := curveOf(curve, width, 4)
		blank := strings.Repeat(" ", width)
		if want := strings.Join([]string{blank, blank, blank, blank}, "\n"); out != want {
			t.Errorf("width %d: want 4 lines of exactly %d spaces, got %q", width, width, out)
		}
	}
}

func TestRenderCurve_AsciiModeDrawsOnlySevenBitGlyphs(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	curve := chart.Curve{
		Values: []float64{1, 5, 2, 9, 4, 6},
		Guides: []chart.Guide{{From: 2, To: 5, Role: tui.RoleSignalWarning, Label: "band"}},
		Markers: []chart.Marker{
			{Index: 0, Kind: chart.MarkerPrimary}, {Index: 2, Kind: chart.MarkerSecondary}, {Index: 4, Kind: chart.MarkerTertiary},
		},
	}

	out := curveOf(curve, 20, 6)

	if out == "" || !isASCII(out) || strings.Contains(out, "\x1b") {
		t.Errorf("want plain 7-bit text, got %q", out)
	}
}
