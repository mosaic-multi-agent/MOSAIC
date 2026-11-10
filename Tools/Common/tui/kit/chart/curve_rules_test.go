package chart_test

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
)

func TestRenderCurve_PointGlyphDependsOnTheGlyphMode(t *testing.T) {
	cases := []struct {
		name  string
		mode  tui.GlyphMode
		glyph string
	}{{"unicode", tui.GlyphUnicode, "•"}, {"ascii", tui.GlyphASCII, "o"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useMode(t, tc.mode)

			out := curveOf(chart.Curve{Values: []float64{3}}, 3, 2)

			lines := strings.Split(out, "\n")
			if len(lines) != 2 || strings.TrimRight(lines[0], " ") != tc.glyph || strings.TrimSpace(lines[1]) != "" {
				t.Errorf("want only %q in the top left cell, got %q", tc.glyph, out)
			}
		})
	}
}

// linesFor splits out into lines and stops the test when the count differs from n.
func linesFor(t *testing.T, out string, n int) []string {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) != n || out == "" {
		t.Fatalf("want %d lines, got %q", n, out)
	}
	return lines
}

// bandCurve plots one point per row of a 5 row grid: the point in column c is on row c, which
// stands for the value c.
func bandCurve(guides ...chart.Guide) chart.Curve {
	return chart.Curve{Values: []float64{0, 1, 2, 3, 4}, Guides: guides}
}

func TestRenderCurve_RowInsideAGuideColoursEveryPlotCellIncludingBlanks(t *testing.T) {
	useColourOn(t)

	lines := linesFor(t, curveOf(bandCurve(chart.Guide{From: 1, To: 3, Role: tui.RoleSignalWarning}), 5, 5), 5)

	for _, row := range []int{1, 2} {
		styles := cellStyles(lines[4-row])
		if len(styles) != 5 {
			t.Fatalf("row %d: want 5 coloured plot cells, got %d in %q", row, len(styles), lines[4-row])
		}
		for c, st := range styles {
			if st != rolePrefix(tui.RoleSignalWarning) {
				t.Errorf("row %d cell %d: want the guide role, got %q", row, c, st)
			}
		}
	}
}

func TestRenderCurve_RowOutsideEveryGuideKeepsBlanksUnstyledAndGlyphsInBody(t *testing.T) {
	useColourOn(t)

	lines := linesFor(t, curveOf(bandCurve(chart.Guide{From: 1, To: 3, Role: tui.RoleSignalWarning}), 5, 5), 5)

	for _, row := range []int{0, 3, 4} {
		plain, styles := stripANSI(lines[4-row]), cellStyles(lines[4-row])
		for c, r := range []rune(plain) {
			want := ""
			if r != ' ' {
				want = rolePrefix(tui.RoleBody)
			}
			if styles[c] != want {
				t.Errorf("row %d cell %d (%q): want style %q, got %q", row, c, r, want, styles[c])
			}
		}
	}
}

func TestRenderCurve_MarkerTakesTheRoleOfTheRowItCrosses(t *testing.T) {
	useColourOn(t)
	curve := bandCurve(chart.Guide{From: 1, To: 3, Role: tui.RoleSignalWarning})
	curve.Markers = []chart.Marker{{Index: 2, Kind: chart.MarkerPrimary}}

	lines := linesFor(t, curveOf(curve, 5, 5), 5)

	want := map[int]tui.StyleRole{0: tui.RoleBody, 1: tui.RoleSignalWarning, 2: tui.RoleSignalWarning, 3: tui.RoleBody, 4: tui.RoleBody}
	for row, role := range want {
		if got := styleAt(lines[4-row], 2); got != rolePrefix(role) {
			t.Errorf("row %d: want the marker cell in role %q, got style %q", row, role, got)
		}
	}
}

func TestRenderCurve_GutterIsNotColouredByGuides(t *testing.T) {
	useColourOn(t)
	curve := bandCurve(chart.Guide{From: 0, To: 0, Role: tui.RoleSignalWarning})
	curve.YLabel = func(float64) string { return "ab" }

	lines := strings.Split(curveOf(curve, 8, 5), "\n")

	for i, line := range lines {
		styles := cellStyles(line)
		if len(styles) != 8 {
			t.Fatalf("line %d: want 8 cells, got %d in %q", i, len(styles), line)
		}
		for c, st := range styles {
			if inBand := st == rolePrefix(tui.RoleSignalWarning); inBand != (c >= 3) {
				t.Errorf("line %d cell %d: the guide colours the plot area (cells 3 to 7) and not the gutter, got style %q", i, c, st)
			}
		}
	}
}

func TestRenderCurve_GuideLabelIsNeverDrawn(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	labelled := bandCurve(chart.Guide{From: 1, To: 3, Role: tui.RoleSignalWarning, Label: "BANDNAME"})

	with, without := curveOf(labelled, 12, 5), curveOf(bandCurve(), 12, 5)

	if len(strings.Split(without, "\n")) != 5 || !strings.Contains(without, "o") {
		t.Fatalf("control: the curve must render 5 lines with points, got %q", without)
	}
	if strings.Contains(with, "BAND") || with != without {
		t.Errorf("a guide changes nothing with colour off and its label is not drawn:\n%s\nvs\n%s", with, without)
	}
}

func TestRenderCurve_EmptySeriesDrawsOnlyTheGuideColouring(t *testing.T) {
	t.Run("markers draw nothing", func(t *testing.T) {
		useMode(t, tui.GlyphASCII)

		out := curveOf(chart.Curve{Markers: []chart.Marker{{Index: 0, Kind: chart.MarkerPrimary}}}, 5, 3)

		if lines := linesFor(t, out, 3); strings.TrimSpace(strings.Join(lines, "")) != "" {
			t.Errorf("want blank lines, got %q", out)
		}
	})
	t.Run("guides colour their rows", func(t *testing.T) {
		useColourOn(t)

		out := curveOf(chart.Curve{Guides: []chart.Guide{{From: 0, To: 0, Role: tui.RoleSignalBad}}}, 5, 3)

		for i, line := range strings.Split(out, "\n") {
			styles := cellStyles(line)
			if len(styles) != 5 {
				t.Fatalf("line %d: want 5 coloured cells, got %d in %q", i, len(styles), line)
			}
			for c, st := range styles {
				if st != rolePrefix(tui.RoleSignalBad) {
					t.Errorf("line %d cell %d: want the guide role, got %q", i, c, st)
				}
			}
		}
	})
}

func TestRenderCurve_NegativeValuesAreTreatedAsZero(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	negative := curveOf(chart.Curve{Values: []float64{-3, 2, -1, 4}}, 10, 4)
	zeroed := curveOf(chart.Curve{Values: []float64{0, 2, 0, 4}}, 10, 4)

	linesFor(t, zeroed, 4)
	if negative != zeroed {
		t.Errorf("negatives must draw as zeros:\n%s\nvs\n%s", negative, zeroed)
	}
}

func TestRenderCurve_AllZeroValuesSitOnTheBottomRow(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	lines := linesFor(t, curveOf(chart.Curve{Values: []float64{0, 0, 0}}, 10, 4), 4)

	if strings.Count(shape(lines[3]), "o") != 3 {
		t.Errorf("want 3 points on the bottom row, got %q", lines[3])
	}
	for _, l := range lines[:3] {
		if strings.Contains(shape(l), "o") {
			t.Errorf("no point may be above the bottom row, got %q", l)
		}
	}
}
