package chart_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
)

func TestBar_UnicodeUsesFullBlocksAndEighthBlockRemainder(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	cases := []struct {
		name       string
		value, max float64
		width      int
		want       string
	}{
		{"half cell remainder", 1, 4, 10, "██▌       "},
		{"one and a quarter cells", 1, 8, 10, "█▎        "},
		{"whole cells only", 3, 8, 8, "███     "},
		{"seven of eight", 7, 8, 8, "███████ "},
		{"half of a single cell", 1, 2, 1, "▌"},
		{"under one cell", 1, 16, 8, "▌       "},
		{"one eighth", 1, 64, 8, "▏       "},
		{"seven eighths", 7, 64, 8, "▉       "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := chart.Bar(tc.value, tc.max, tc.width, tui.RoleSignalGood, theme()); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBar_AsciiUsesHashForWholeCells(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name       string
		value, max float64
		width      int
		want       string
	}{
		{"half", 1, 2, 10, "#####     "},
		{"three quarters", 3, 4, 8, "######  "},
		{"full", 2, 2, 6, "######"},
		{"empty", 0, 2, 6, "      "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := chart.Bar(tc.value, tc.max, tc.width, tui.RoleSignalGood, theme()); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBar_NoRenderableValueGivesSpaces(t *testing.T) {
	for _, mode := range []tui.GlyphMode{tui.GlyphUnicode, tui.GlyphASCII} {
		useMode(t, mode)
		cases := []struct {
			name       string
			value, max float64
		}{
			{"zero value", 0, 10},
			{"negative value", -3, 10},
			{"zero maximum", 5, 0},
			{"negative maximum", 5, -10},
			{"zero of zero", 0, 0},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("mode%d/%s", mode, tc.name), func(t *testing.T) {
				if got, want := chart.Bar(tc.value, tc.max, 8, tui.RoleSignalGood, theme()), strings.Repeat(" ", 8); got != want {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}

func TestBar_ValueAtOrAboveMaximumFillsWidth(t *testing.T) {
	cases := []struct {
		mode tui.GlyphMode
		cell string
	}{{tui.GlyphUnicode, "█"}, {tui.GlyphASCII, "#"}}
	for _, c := range cases {
		useMode(t, c.mode)
		for _, v := range []float64{10, 10.5, 1000} {
			t.Run(fmt.Sprintf("mode%d/value%v", c.mode, v), func(t *testing.T) {
				if got, want := chart.Bar(v, 10, 7, tui.RoleSignalGood, theme()), strings.Repeat(c.cell, 7); got != want {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}

func TestBar_NonPositiveWidthGivesEmptyString(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	for _, w := range []int{0, -1, -40} {
		if got := chart.Bar(5, 10, w, tui.RoleSignalGood, theme()); got != "" {
			t.Errorf("width %d: got %q, want empty", w, got)
		}
	}
}

func TestBar_IsAlwaysExactlyWidthCellsAndNeverShrinksWithValue(t *testing.T) {
	for _, mode := range []tui.GlyphMode{tui.GlyphUnicode, tui.GlyphASCII} {
		useMode(t, mode)
		for width := 1; width <= 12; width++ {
			prevFilled := 0
			for step := 0; step <= 40; step++ {
				out := chart.Bar(float64(step), 40, width, tui.RoleSignalGood, theme())
				if w := lipgloss.Width(out); w != width {
					t.Fatalf("mode %d width %d step %d: %d cells: %q", mode, width, step, w, out)
				}
				filled := width - strings.Count(out, " ")
				if filled < prevFilled {
					t.Fatalf("mode %d width %d step %d: filled cells dropped from %d to %d", mode, width, step, prevFilled, filled)
				}
				prevFilled = filled
			}
		}
	}
}

func TestBar_AsciiOutputIsSevenBit(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, v := range []float64{0, 0.3, 1, 2.7, 5} {
		if out := chart.Bar(v, 5, 12, tui.RoleSignalGood, theme()); lipgloss.Width(out) != 12 || !isASCII(out) {
			t.Errorf("value %v: want 12 ASCII cells, got %q", v, out)
		}
	}
}

func TestBar_FilledCellsCarryTheRoleStyle(t *testing.T) {
	useColourOn(t)
	if rolePrefix(tui.RoleSignalBad) == rolePrefix(tui.RoleSignalGood) {
		t.Fatal("test roles must render differently")
	}

	out := chart.Bar(1, 2, 10, tui.RoleSignalBad, theme())

	styles := cellStyles(out)
	if len(styles) != 10 {
		t.Fatalf("got %d cells, want 10", len(styles))
	}
	for i := 0; i < 5; i++ {
		if styles[i] != rolePrefix(tui.RoleSignalBad) {
			t.Errorf("cell %d style %q, want the bad role style", i, styles[i])
		}
	}
	if got := stripANSI(out); got != "#####     " {
		t.Errorf("plain text %q", got)
	}
}

func TestBar_Golden(t *testing.T) {
	fractions := []float64{0, 0.125, 0.25, 0.5, 0.75, 1}
	t.Run("unicode", func(t *testing.T) {
		useMode(t, tui.GlyphUnicode)
		var b strings.Builder
		for _, w := range []int{4, 10, 20} {
			fmt.Fprintf(&b, "width %d\n", w)
			for _, f := range fractions {
				fmt.Fprintf(&b, "|%s|\n", chart.Bar(f, 1, w, tui.RoleSignalGood, theme()))
			}
		}
		assertGolden(t, "bar_unicode", strings.TrimSuffix(b.String(), "\n"))
	})
	t.Run("ascii", func(t *testing.T) {
		useMode(t, tui.GlyphASCII)
		var b strings.Builder
		for _, w := range []int{8, 16, 24} {
			fmt.Fprintf(&b, "width %d\n", w)
			for _, v := range []float64{0, 2, 4, 6, 8} {
				fmt.Fprintf(&b, "|%s|\n", chart.Bar(v, 8, w, tui.RoleSignalGood, theme()))
			}
		}
		assertGolden(t, "bar_ascii", strings.TrimSuffix(b.String(), "\n"))
	})
}

func TestBar_EmptyRoleRendersAsBodyAndPaddingIsUnstyled(t *testing.T) {
	useColourOn(t)

	styles := cellStyles(chart.Bar(1, 2, 6, "", theme()))

	if len(styles) != 6 {
		t.Fatalf("got %d cells, want 6", len(styles))
	}
	for i, s := range styles {
		want := ""
		if i < 3 {
			want = rolePrefix(tui.RoleBody)
		}
		if s != want {
			t.Errorf("cell %d style %q, want %q", i, s, want)
		}
	}
}

func TestBar_PositiveValueBelowHalfAStepRendersAsSpaces(t *testing.T) {
	cases := []struct {
		mode  tui.GlyphMode
		value float64
		max   float64
		width int
	}{
		{tui.GlyphUnicode, 1, 1000, 8},
		{tui.GlyphASCII, 1, 100, 8},
	}
	for _, c := range cases {
		useMode(t, c.mode)
		if got, want := chart.Bar(c.value, c.max, c.width, tui.RoleSignalGood, theme()), strings.Repeat(" ", c.width); got != want {
			t.Errorf("mode %d: got %q, want %q", c.mode, got, want)
		}
	}
}

func TestBar_UnicodeRoundsHalfEighthUp(t *testing.T) {
	useMode(t, tui.GlyphUnicode)

	if got, want := chart.Bar(1, 128, 8, tui.RoleSignalGood, theme()), "▏       "; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBar_AsciiRoundsHalfCellUp(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	if got, want := chart.Bar(1, 8, 4, tui.RoleSignalGood, theme()), "#   "; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
