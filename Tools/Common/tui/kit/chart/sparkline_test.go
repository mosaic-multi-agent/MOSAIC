package chart_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
)

var bothModes = []struct {
	name string
	mode tui.GlyphMode
}{{"unicode", tui.GlyphUnicode}, {"ascii", tui.GlyphASCII}}

func spark(values []float64, roles []tui.StyleRole, marker, width int) string {
	return chart.Sparkline(values, roles, marker, width, theme())
}

func constant(n int, v float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

// differingColumns lists the rune positions at which a and b differ.
func differingColumns(a, b string) []int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) != len(rb) {
		return []int{-1}
	}
	var cols []int
	for i := range ra {
		if ra[i] != rb[i] {
			cols = append(cols, i)
		}
	}
	return cols
}

func TestSparkline_IsExactlyWidthCellsForAnySeriesLength(t *testing.T) {
	for _, m := range bothModes {
		useMode(t, m.mode)
		for _, n := range []int{1, 2, 5, 8, 9, 30, 200} {
			values := make([]float64, n)
			for i := range values {
				values[i] = float64(i % 7)
			}
			for _, width := range []int{1, 4, 8, 20} {
				if w := lipgloss.Width(spark(values, nil, -1, width)); w != width {
					t.Errorf("%s n=%d width=%d: got %d cells", m.name, n, width, w)
				}
			}
		}
	}
}

func TestSparkline_NonPositiveWidthGivesEmptyString(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	for _, w := range []int{0, -2} {
		if got := spark([]float64{1, 2, 3}, nil, -1, w); got != "" {
			t.Errorf("width %d: got %q, want empty", w, got)
		}
	}
}

func TestSparkline_EmptySeriesGivesSpaces(t *testing.T) {
	for _, m := range bothModes {
		useMode(t, m.mode)
		for _, series := range [][]float64{nil, {}} {
			if got, want := spark(series, nil, -1, 6), strings.Repeat(" ", 6); got != want {
				t.Errorf("%s: got %q, want %q", m.name, got, want)
			}
		}
	}
}

func TestSparkline_HighPointsDrawDifferentlyFromLowPoints(t *testing.T) {
	for _, m := range bothModes {
		useMode(t, m.mode)
		t.Run(m.name, func(t *testing.T) {
			got := []rune(spark([]float64{0, 0, 0, 0, 10, 10, 10, 10}, nil, -1, 8))

			if len(got) != 8 {
				t.Fatalf("got %d cells: %q", len(got), string(got))
			}
			for i := 1; i < 4; i++ {
				if got[i] != got[0] || got[4+i] != got[4] {
					t.Errorf("equal values must draw equally: %q", string(got))
				}
			}
			if got[0] == got[4] {
				t.Errorf("low and high points drew the same glyph: %q", string(got))
			}
		})
	}
}

func TestSparkline_PeakStandsOutFromTheRest(t *testing.T) {
	for _, m := range bothModes {
		useMode(t, m.mode)
		t.Run(m.name, func(t *testing.T) {
			got := []rune(spark([]float64{9, 1, 1, 1}, nil, -1, 4))

			if len(got) != 4 {
				t.Fatalf("got %d cells: %q", len(got), string(got))
			}
			if got[0] == got[1] || got[1] != got[2] || got[2] != got[3] {
				t.Errorf("want a distinct first glyph over three equal ones, got %q", string(got))
			}
		})
	}
}

func TestSparkline_ScalesTheSeriesToItsOwnRange(t *testing.T) {
	want := map[string]string{"unicode": "▃▅▆█", "ascii": "-==#"}
	for _, m := range bothModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)
			small := spark([]float64{1, 2, 3, 4}, nil, -1, 4)
			large := spark([]float64{100, 200, 300, 400}, nil, -1, 4)

			if small != want[m.name] || large != want[m.name] {
				t.Errorf("same shape at a different magnitude: small %q, large %q, want both %q", small, large, want[m.name])
			}
		})
	}
}

func TestSparkline_LongSeriesIsBucketedByMax(t *testing.T) {
	want := map[string]string{"unicode": "█▂", "ascii": "#_"}
	for _, m := range bothModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)
			got := spark([]float64{1, 9, 1, 1}, nil, -1, 2)

			if got != want[m.name] {
				t.Errorf("each column must show the max of its bucket: got %q, want %q", got, want[m.name])
			}
		})
	}
}

func TestSparkline_BucketTakesTheRoleOfItsMaxPoint(t *testing.T) {
	useColourOn(t)
	roles := []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalBad, tui.RoleSignalGood, tui.RoleSignalGood}

	styles := cellStyles(spark([]float64{1, 9, 1, 1}, roles, -1, 2))

	if len(styles) != 2 || styles[0] != rolePrefix(tui.RoleSignalBad) || styles[1] != rolePrefix(tui.RoleSignalGood) {
		t.Errorf("want [bad good] styles, got %q", styles)
	}
}

func TestSparkline_TiedMaxTakesTheRoleOfTheFirstHoldingIt(t *testing.T) {
	useColourOn(t)
	roles := []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalBad, tui.RoleSignalWarning, tui.RoleDataCost}

	styles := cellStyles(spark([]float64{9, 9, 1, 1}, roles, -1, 2))

	if len(styles) != 2 || styles[0] != rolePrefix(tui.RoleSignalGood) || styles[1] != rolePrefix(tui.RoleSignalWarning) {
		t.Errorf("want [good warning] styles, got %q", styles)
	}
}

func TestSparkline_WidthOneHoldsTheWholeSeries(t *testing.T) {
	useColourOn(t)
	roles := []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalBad, tui.RoleSignalWarning}

	out := spark([]float64{1, 9, 3}, roles, -1, 1)

	styles := cellStyles(out)
	if stripANSI(out) != "#" || len(styles) != 1 || styles[0] != rolePrefix(tui.RoleSignalBad) {
		t.Errorf("want one top-level cell in the role of the max point, got %q styles %q", stripANSI(out), styles)
	}
}

func TestSparkline_EachPointTakesItsOwnRole(t *testing.T) {
	useColourOn(t)
	roles := []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalBad, tui.RoleSignalWarning, tui.RoleDataCost}

	styles := cellStyles(spark([]float64{1, 2, 3, 4}, roles, -1, 4))

	if len(styles) != 4 {
		t.Fatalf("got %d cells", len(styles))
	}
	for i, role := range roles {
		if styles[i] != rolePrefix(role) {
			t.Errorf("cell %d style %q, want the %s style", i, styles[i], role)
		}
	}
}

func TestSparkline_PointsWithoutRoleUseBody(t *testing.T) {
	useColourOn(t)
	roles := []tui.StyleRole{tui.RoleSignalBad, ""}

	styles := cellStyles(spark([]float64{1, 2, 3}, roles, -1, 3))

	if len(styles) != 3 || styles[0] != rolePrefix(tui.RoleSignalBad) {
		t.Fatalf("unexpected styles %q", styles)
	}
	if styles[1] != rolePrefix(tui.RoleBody) || styles[2] != rolePrefix(tui.RoleBody) {
		t.Errorf("empty and missing roles must use the body style, got %q", styles)
	}
}

func TestSparkline_MarkerReplacesOnlyTheColumnHoldingItsIndex(t *testing.T) {
	cases := []struct {
		name    string
		n       int
		width   int
		marker  int
		wantCol int
	}{
		{"one point per column, middle", 6, 6, 4, 4},
		{"first point", 6, 6, 0, 0},
		{"last point", 6, 6, 5, 5},
		{"bucketed, second bucket", 8, 4, 3, 1},
		{"bucketed, third bucket", 8, 4, 5, 2},
		{"bucketed, last index", 8, 4, 7, 3},
	}
	for _, m := range bothModes {
		useMode(t, m.mode)
		for _, tc := range cases {
			t.Run(m.name+"/"+tc.name, func(t *testing.T) {
				values := constant(tc.n, 3)

				plain := spark(values, nil, -1, tc.width)
				marked := spark(values, nil, tc.marker, tc.width)

				got := differingColumns(plain, marked)
				if len(got) != 1 || got[0] != tc.wantCol {
					t.Errorf("marker %d changed columns %v, want [%d]\nplain  %q\nmarked %q", tc.marker, got, tc.wantCol, plain, marked)
				}
			})
		}
	}
}

func TestSparkline_NegativeMarkerDrawsNone(t *testing.T) {
	want := map[string]string{"unicode": "▂█▅▇", "ascii": "-#=="}
	for _, m := range bothModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)
			values := []float64{1, 5, 3, 4}

			a, b := spark(values, nil, -1, 4), spark(values, nil, -7, 4)

			if a != want[m.name] || b != want[m.name] {
				t.Errorf("unmarked %q, negative marker %q, want both %q", a, b, want[m.name])
			}
		})
	}
}

func TestSparkline_MarkerKeepsWidth(t *testing.T) {
	for _, m := range bothModes {
		useMode(t, m.mode)
		if w := lipgloss.Width(spark([]float64{1, 2, 3, 4, 5}, nil, 2, 5)); w != 5 {
			t.Errorf("%s: got %d cells, want 5", m.name, w)
		}
	}
}

func TestSparkline_AsciiOutputIsSevenBit(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, marker := range []int{-1, 0, 3} {
		out := spark([]float64{0, 1, 4, 9, 2, 7}, nil, marker, 6)
		if out == "" || !isASCII(out) {
			t.Errorf("marker %d: want non-empty ASCII output, got %q", marker, out)
		}
	}
}

func TestSparkline_ColourOffEmitsNoEscapeSequences(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	roles := []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalBad}

	out := spark([]float64{1, 2}, roles, 0, 2)

	if out == "" || out != stripANSI(out) {
		t.Errorf("want plain non-empty text, got %q", out)
	}
}
