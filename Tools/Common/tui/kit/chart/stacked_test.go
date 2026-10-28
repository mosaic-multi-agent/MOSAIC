package chart_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
)

var segRoles = []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalWarning, tui.RoleSignalBad, tui.RoleDataCost}

// segs builds segments with distinct roles and the given values.
func segs(values ...float64) []chart.Segment {
	out := make([]chart.Segment, len(values))
	for i, v := range values {
		out[i] = chart.Segment{Label: string(rune('A' + i)), Value: v, Role: segRoles[i]}
	}
	return out
}

// cellsPerSegment renders segs in ASCII mode with colour on and counts the cells of each role.
func cellsPerSegment(t *testing.T, s []chart.Segment, width int) []int {
	t.Helper()
	useColourOn(t)
	out := chart.StackedBar(s, width, theme())
	counts := make([]int, len(s))
	for i, seg := range s {
		counts[i] = countStyled(out, seg.Role)
	}
	return counts
}

func sum(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}

func TestStackedBar_SegmentCellsFollowShares(t *testing.T) {
	cases := []struct {
		name   string
		values []float64
		width  int
		want   []int
	}{
		{"fifty thirty twenty at 10", []float64{50, 30, 20}, 10, []int{5, 3, 2}},
		{"fifty thirty twenty at 20", []float64{50, 30, 20}, 20, []int{10, 6, 4}},
		{"shares not totals matter", []float64{5, 3, 2}, 10, []int{5, 3, 2}},
		{"single segment fills", []float64{7}, 9, []int{9}},
		{"zero segment gets no cell", []float64{1, 0, 1}, 8, []int{4, 0, 4}},
		{"negative treated as zero", []float64{-5, 10}, 6, []int{0, 6}},
		{"equal thirds, leftover to earliest", []float64{1, 1, 1}, 10, []int{4, 3, 3}},
		{"leftover goes to largest remainder", []float64{1000, 1, 1}, 10, []int{8, 1, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cellsPerSegment(t, segs(tc.values...), tc.width)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("cells per segment %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStackedBar_SegmentsAlwaysFillTheWidthExactly(t *testing.T) {
	sets := [][]float64{{1, 1, 1}, {1, 1}, {3, 3, 3, 3}, {1, 2, 3, 4}, {97, 1, 1, 1}, {0.1, 0.2, 0.7}}
	for _, values := range sets {
		for width := 1; width <= 40; width++ {
			got := cellsPerSegment(t, segs(values...), width)
			if sum(got) != width {
				t.Fatalf("values %v width %d: cells %v sum to %d", values, width, got, sum(got))
			}
		}
	}
}

func TestStackedBar_TinyPositiveShareStaysVisible(t *testing.T) {
	cases := []struct {
		name   string
		values []float64
		width  int
	}{
		{"two tiny next to a huge one", []float64{1000, 1, 1}, 10},
		{"width equal to segment count", []float64{1000, 1, 1}, 3},
		{"tiny first", []float64{1, 1000}, 10},
		{"tiny in a wide bar", []float64{100000, 1}, 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cellsPerSegment(t, segs(tc.values...), tc.width)
			for i, c := range got {
				if c < 1 {
					t.Errorf("segment %d (value %v) got no cell: %v", i, tc.values[i], got)
				}
			}
			if sum(got) != tc.width {
				t.Errorf("cells %v sum to %d, want %d", got, sum(got), tc.width)
			}
		})
	}
}

func TestStackedBar_NarrowerThanSegmentCountStillFillsWidth(t *testing.T) {
	got := cellsPerSegment(t, segs(1, 1, 1), 2)

	if fmt.Sprint(got) != fmt.Sprint([]int{1, 1, 0}) {
		t.Errorf("cells per segment %v, want [1 1 0] (equal remainders go to the earlier segments)", got)
	}
}

func TestStackedBar_VisibilityDonorIsTheLargestSegmentTiesToEarliest(t *testing.T) {
	cases := []struct {
		name   string
		values []float64
		want   []int
	}{
		{"tied donors, earliest gives", []float64{1, 50, 50}, []int{1, 4, 5}},
		{"two empty segments, donor changes after first transfer", []float64{1, 1, 60, 60}, []int{1, 1, 4, 4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cellsPerSegment(t, segs(tc.values...), 10)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("cells per segment %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStackedBar_LeftoverCellsGoToLargestRemainders(t *testing.T) {
	got := cellsPerSegment(t, segs(5, 3, 3), 7)

	if fmt.Sprint(got) != fmt.Sprint([]int{3, 2, 2}) {
		t.Errorf("cells per segment %v, want [3 2 2]", got)
	}
}

func TestStackedBar_NothingToShowGivesSpaces(t *testing.T) {
	for _, mode := range []tui.GlyphMode{tui.GlyphUnicode, tui.GlyphASCII} {
		useMode(t, mode)
		cases := []struct {
			name string
			segs []chart.Segment
		}{
			{"no segments", nil},
			{"all zero", segs(0, 0)},
			{"all negative", segs(-1, -2)},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("mode%d/%s", mode, tc.name), func(t *testing.T) {
				if got, want := chart.StackedBar(tc.segs, 6, theme()), strings.Repeat(" ", 6); got != want {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}

func TestStackedBar_NonPositiveWidthGivesEmptyString(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	for _, w := range []int{0, -3} {
		if got := chart.StackedBar(segs(1, 2), w, theme()); got != "" {
			t.Errorf("width %d: got %q, want empty", w, got)
		}
	}
}

func TestStackedBar_IsExactlyWidthCellsInBothGlyphModes(t *testing.T) {
	for _, mode := range []tui.GlyphMode{tui.GlyphUnicode, tui.GlyphASCII} {
		useMode(t, mode)
		for width := 1; width <= 30; width++ {
			out := chart.StackedBar(segs(5, 1, 3, 1), width, theme())
			if w := lipgloss.Width(out); w != width {
				t.Fatalf("mode %d width %d: %d cells: %q", mode, width, w, out)
			}
			if mode == tui.GlyphASCII && !isASCII(out) {
				t.Fatalf("width %d: non-ASCII output %q", width, out)
			}
		}
	}
}

func TestStackedBar_SegmentsUseTheirOwnGlyphs(t *testing.T) {
	cases := []struct {
		name   string
		mode   tui.GlyphMode
		values []float64
		width  int
		want   string
	}{
		{"ascii quarter at 4", tui.GlyphASCII, []float64{50, 25, 25}, 4, "##=+"},
		{"ascii quarter at 8", tui.GlyphASCII, []float64{50, 25, 25}, 8, "####==++"},
		{"ascii fifty thirty twenty", tui.GlyphASCII, []float64{50, 30, 20}, 10, "#####===++"},
		{"ascii equal thirds", tui.GlyphASCII, []float64{1, 1, 1}, 10, "####===+++"},
		{"ascii tiny shares", tui.GlyphASCII, []float64{1000, 1, 1}, 10, "########=+"},
		{"ascii tiny shares at segment count", tui.GlyphASCII, []float64{1000, 1, 1}, 3, "#=+"},
		{"ascii fifth segment repeats the first glyph", tui.GlyphASCII, []float64{1, 1, 1, 1, 1}, 5, "#=+:#"},
		{"unicode quarter at 4", tui.GlyphUnicode, []float64{50, 25, 25}, 4, "██▓▒"},
		{"unicode four glyphs then repeat", tui.GlyphUnicode, []float64{1, 1, 1, 1, 1}, 5, "█▓▒░█"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useMode(t, tc.mode)
			s := make([]chart.Segment, len(tc.values))
			for i, v := range tc.values {
				s[i] = chart.Segment{Value: v}
			}
			if got := chart.StackedBar(s, tc.width, theme()); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStackedBar_SameRoleSegmentsStillDifferInGlyphWithColourOff(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	s := []chart.Segment{{Value: 1, Role: tui.RoleSignalWarning}, {Value: 1, Role: tui.RoleSignalWarning}}

	if got := chart.StackedBar(s, 4, theme()); got != "##==" {
		t.Errorf("got %q, want %q", got, "##==")
	}
}

func TestStackedBar_FillCarriesRoleAndBlankBarCarriesNoStyle(t *testing.T) {
	useColourOn(t)

	blank := chart.StackedBar(nil, 6, theme())
	if blank != strings.Repeat(" ", 6) {
		t.Errorf("blank bar %q carries styling or wrong width", blank)
	}
	styles := cellStyles(chart.StackedBar([]chart.Segment{{Value: 1}}, 3, theme()))
	if len(styles) != 3 {
		t.Fatalf("want 3 filled cells, got %d: %q", len(styles), styles)
	}
	for i, s := range styles {
		if s != rolePrefix(tui.RoleBody) {
			t.Errorf("cell %d style %q, want the body style for an empty role", i, s)
		}
	}
}

func TestStackedBar_FullBarGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode tui.GlyphMode
	}{{"stacked_unicode", tui.GlyphUnicode}, {"stacked_ascii", tui.GlyphASCII}} {
		t.Run(tc.name, func(t *testing.T) {
			useMode(t, tc.mode)
			var b strings.Builder
			for _, w := range []int{4, 8, 16} {
				fmt.Fprintf(&b, "width %d\n|%s|\n", w, chart.StackedBar(quarterSegs(), w, theme()))
			}
			assertGolden(t, tc.name, strings.TrimSuffix(b.String(), "\n"))
		})
	}
}
