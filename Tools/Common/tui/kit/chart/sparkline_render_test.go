package chart_test

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-common/tui"
)

func ramp(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i)
	}
	return out
}

func TestSparkline_ExactGlyphsPerLevel(t *testing.T) {
	cases := []struct {
		name   string
		mode   tui.GlyphMode
		values []float64
		marker int
		width  int
		want   string
	}{
		{"unicode eight levels", tui.GlyphUnicode, ramp(8), -1, 8, "▁▂▃▄▅▆▇█"},
		{"ascii four levels, left aligned", tui.GlyphASCII, []float64{0, 3, 6, 9}, -1, 6, "_-=#  "},
		{"ascii marker replaces glyph", tui.GlyphASCII, []float64{0, 3, 6, 9}, 2, 6, "_-|#  "},
		{"unicode marker replaces glyph", tui.GlyphUnicode, ramp(8), 3, 8, "▁▂▃┃▅▆▇█"},
		{"all zero uses lowest glyph", tui.GlyphUnicode, []float64{0, 0, 0}, -1, 5, "▁▁▁  "},
		{"all zero ascii", tui.GlyphASCII, []float64{0, 0}, -1, 3, "__ "},
		{"negative treated as zero", tui.GlyphASCII, []float64{-5, 4}, -1, 2, "_#"},
		{"single point is the top level", tui.GlyphUnicode, []float64{3}, -1, 3, "█  "},
		{"uneven buckets", tui.GlyphUnicode, []float64{5, 1, 1, 9, 1}, -1, 2, "▅█"},
		{"uneven buckets ascii", tui.GlyphASCII, []float64{5, 1, 1, 9, 1}, -1, 2, "=#"},
		{"marker beyond the series draws nothing", tui.GlyphASCII, []float64{0, 3}, 2, 3, "_# "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useMode(t, tc.mode)
			if got := spark(tc.values, nil, tc.marker, tc.width); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSparkline_ShortSeriesStaysLeftAlignedWithBlankTail(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	short := spark([]float64{0, 9}, nil, -1, 5)
	long := spark([]float64{0, 9, 0, 9, 0}, nil, -1, 5)

	if len(short) != 5 || len(long) != 5 {
		t.Fatalf("both lines must be exactly 5 ASCII cells: %q and %q", short, long)
	}
	if short[:2] != long[:2] || short[2:] != "   " {
		t.Errorf("column i must always be point i: %q vs %q", short, long)
	}
}

func TestSparkline_MarkerKeepsItsColumnRole(t *testing.T) {
	useColourOn(t)
	roles := []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalBad, tui.RoleSignalGood}

	out := spark([]float64{1, 2, 3}, roles, 1, 3)

	styles := cellStyles(out)
	if stripANSI(out) != "-|#" || len(styles) != 3 || styles[1] != rolePrefix(tui.RoleSignalBad) {
		t.Errorf("want marker drawn in the bad role of its column, got %q styles %q", stripANSI(out), styles)
	}
}

func TestSparkline_BlankColumnsCarryNoStyle(t *testing.T) {
	useColourOn(t)

	styles := cellStyles(spark([]float64{1, 2}, []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalGood}, -1, 4))

	if len(styles) != 4 || styles[2] != "" || styles[3] != "" {
		t.Errorf("blank columns must be unstyled, got %q", styles)
	}
}

func TestSparkline_Golden(t *testing.T) {
	for _, m := range bothModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)
			var b strings.Builder
			for _, w := range []int{4, 8, 12} {
				fmt.Fprintf(&b, "width %d\n|%s|\n", w, spark(ramp(8), nil, -1, w))
			}
			fmt.Fprintf(&b, "marker 3 at width 12\n|%s|\n", spark(ramp(8), nil, 3, 12))
			assertGolden(t, "sparkline_"+m.name, strings.TrimSuffix(b.String(), "\n"))
		})
	}
}
