package chart_test

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
)

// fullHeightColumns lists the columns in which it fills every line.
func fullHeightColumns(out string, glyph rune) []int {
	lines := strings.Split(out, "\n")
	var cols []int
	for c := 0; c < runeCount(lines[0]); c++ {
		full := true
		for _, l := range lines {
			r := []rune(l)
			if c >= len(r) || r[c] != glyph {
				full = false
			}
		}
		if full {
			cols = append(cols, c)
		}
	}
	return cols
}

func TestRenderCurve_MarkersFillTheirColumnWithTheGlyphOfTheirKind(t *testing.T) {
	cases := []struct {
		mode  tui.GlyphMode
		kind  chart.MarkerKind
		glyph rune
	}{
		{tui.GlyphASCII, chart.MarkerPrimary, '|'},
		{tui.GlyphASCII, chart.MarkerSecondary, ':'},
		{tui.GlyphASCII, chart.MarkerTertiary, '*'},
		{tui.GlyphUnicode, chart.MarkerPrimary, '┃'},
		{tui.GlyphUnicode, chart.MarkerSecondary, '│'},
		{tui.GlyphUnicode, chart.MarkerTertiary, '┊'},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d-%q", tc.mode, tc.glyph), func(t *testing.T) {
			useMode(t, tc.mode)
			curve := chart.Curve{Values: []float64{1, 2, 3, 4, 5}, Markers: []chart.Marker{{Index: 2, Kind: tc.kind}}}

			out := curveOf(curve, 5, 4)

			if got := fullHeightColumns(out, tc.glyph); len(got) != 1 || got[0] != 2 {
				t.Errorf("want %q on every line of column 2 only, got columns %v in\n%s", tc.glyph, got, out)
			}
		})
	}
}

func TestRenderCurve_DistinctMarkersKeepDistinctGlyphsInTheirColumns(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	curve := chart.Curve{
		Values: []float64{1, 2, 3, 4, 5},
		Markers: []chart.Marker{
			{Index: 0, Kind: chart.MarkerTertiary}, {Index: 2, Kind: chart.MarkerPrimary}, {Index: 4, Kind: chart.MarkerSecondary},
		},
	}

	out := curveOf(curve, 5, 4)

	if a, b, c := fullHeightColumns(out, '*'), fullHeightColumns(out, '|'), fullHeightColumns(out, ':'); len(a) != 1 || a[0] != 0 || len(b) != 1 || b[0] != 2 || len(c) != 1 || c[0] != 4 {
		t.Errorf("want * in column 0, | in column 2 and : in column 4, got\n%s", out)
	}
}

func TestRenderCurve_MarkerColumnFollowsTheCompressedLayout(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	curve := chart.Curve{Values: ramp(10), Markers: []chart.Marker{{Index: 7, Kind: chart.MarkerTertiary}}}

	out := curveOf(curve, 5, 3) // column c holds points 2c and 2c+1

	if got := fullHeightColumns(out, '*'); len(got) != 1 || got[0] != 3 {
		t.Errorf("point 7 lives in column 3, got columns %v in\n%s", got, out)
	}
}

func TestRenderCurve_MarkerColumnFollowsTheSpreadLayout(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	curve := chart.Curve{Values: []float64{1, 2, 3, 4, 5}, Markers: []chart.Marker{{Index: 3, Kind: chart.MarkerPrimary}}}

	out := curveOf(curve, 9, 3) // point i is drawn at column i*8/4

	if got := fullHeightColumns(out, '|'); len(got) != 1 || got[0] != 6 {
		t.Errorf("point 3 lives in column 6, got columns %v in\n%s", got, out)
	}
}

func TestRenderCurve_HighestKindWinsWhenMarkersShareAColumn(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name    string
		markers []chart.Marker
		glyph   rune
	}{
		{"same index", []chart.Marker{{Index: 2, Kind: chart.MarkerPrimary}, {Index: 2, Kind: chart.MarkerTertiary}}, '*'},
		{"same index, reverse order", []chart.Marker{{Index: 2, Kind: chart.MarkerTertiary}, {Index: 2, Kind: chart.MarkerPrimary}}, '*'},
		{"primary against secondary", []chart.Marker{{Index: 2, Kind: chart.MarkerSecondary}, {Index: 2, Kind: chart.MarkerPrimary}}, ':'},
		{"different indices of one compressed column", []chart.Marker{{Index: 6, Kind: chart.MarkerSecondary}, {Index: 7, Kind: chart.MarkerPrimary}}, ':'},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := []float64{1, 2, 3, 4, 5}
			width := 5
			if strings.Contains(tc.name, "compressed") {
				values, width = ramp(10), 5
			}
			out := curveOf(chart.Curve{Values: values, Markers: tc.markers}, width, 3)

			col := 2
			if width == 5 && len(values) == 10 {
				col = 3
			}
			if got := fullHeightColumns(out, tc.glyph); len(got) != 1 || got[0] != col {
				t.Errorf("want %q to win in column %d, got columns %v in\n%s", tc.glyph, col, got, out)
			}
		})
	}
}

func TestRenderCurve_MarkersOutsideTheSeriesAreNotDrawn(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	curve := chart.Curve{
		Values:  []float64{1, 2, 3},
		Markers: []chart.Marker{{Index: -1, Kind: chart.MarkerPrimary}, {Index: 3, Kind: chart.MarkerSecondary}, {Index: 40, Kind: chart.MarkerTertiary}},
	}

	out := curveOf(curve, 6, 4)

	if out == "" || strings.ContainsAny(out, "|:*") {
		t.Errorf("no marker may be drawn for an index outside [0, n), got\n%s", out)
	}
}

func TestRenderCurve_MarkerReplacesThePointInItsColumn(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	curve := chart.Curve{Values: []float64{0, 4, 0}, Markers: []chart.Marker{{Index: 1, Kind: chart.MarkerPrimary}}}

	out := curveOf(curve, 3, 3)

	assertShapes(t, out, " |", " |", "o|o")
}

func TestRenderCurve_GuideBandsColourTheirRows(t *testing.T) {
	useColourOn(t)
	// five points on a 5 row grid: the point in column c is on row c, standing for value c
	curve := chart.Curve{
		Values: []float64{0, 1, 2, 3, 4},
		Guides: []chart.Guide{
			{From: 1, To: 3, Role: tui.RoleSignalWarning},
			{From: 2, To: 4, Role: tui.RoleSignalGood},
			{From: 3, To: 0, Role: tui.RoleSignalBad},
		},
	}

	lines := strings.Split(curveOf(curve, 5, 5), "\n")

	want := map[int]tui.StyleRole{
		0: tui.RoleBody,          // value 0 is in no band
		1: tui.RoleSignalWarning, // [1,3)
		2: tui.RoleSignalWarning, // the first band wins over [2,4)
		3: tui.RoleSignalGood,    // [2,4), not the open-ended band behind it
		4: tui.RoleSignalBad,     // [3, inf)
	}
	if len(lines) != 5 {
		t.Fatalf("want 5 lines, got %d", len(lines))
	}
	for col, role := range want {
		line := lines[4-col] // line 0 is the top row
		if got := styleAt(line, col); got != rolePrefix(role) {
			t.Errorf("point of row %d: want role %q, got style %q", col, role, got)
		}
	}
}

func styleAt(out string, i int) string {
	s := cellStyles(out)
	if i >= len(s) {
		return "<missing cell>"
	}
	return s[i]
}

func TestRenderCurve_GuidesDoNotWidenTheScale(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	withGuide := chart.Curve{Values: []float64{0, 2, 4}, Guides: []chart.Guide{{From: 100, To: 0, Role: tui.RoleSignalBad}}}
	without := chart.Curve{Values: []float64{0, 2, 4}}

	if a, b := curveOf(withGuide, 3, 5), curveOf(without, 3, 5); a != b || !strings.Contains(a, "\n") {
		t.Errorf("a band above the data must not change the layout:\n%s\nvs\n%s", a, b)
	}
}

func TestRenderCurve_ColourOffAddsNoEscapeSequences(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	curve := chart.Curve{Values: []float64{1, 2, 3}, Guides: []chart.Guide{{From: 1, To: 0, Role: tui.RoleSignalGood}}}

	out := curveOf(curve, 6, 4)

	if out == "" || strings.Contains(out, "\x1b") {
		t.Errorf("colour off must render plain text, got %q", out)
	}
}
