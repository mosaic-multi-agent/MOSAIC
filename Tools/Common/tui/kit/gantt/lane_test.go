package gantt_test

import (
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"
)

// Lane tests use a 100 second axis and a 10 cell plot area: label width 3, one space, 10 cells.
const (
	lw    = 3
	lineW = 14
)

func lineOf(a gantt.Axis, lane gantt.Lane, shades ...gantt.Shade) string {
	return gantt.RenderLane(a, lane, shades, lw, lineW, false, theme())
}

func plotFor(lane gantt.Lane, shades ...gantt.Shade) []rune {
	return plotOf(lineOf(axisOf(100), lane, shades...), lw)
}

func intervals(ivs ...gantt.Interval) gantt.Lane { return gantt.Lane{Label: "ab", Intervals: ivs} }

func TestRenderLane_PlacesIntervalsProportionallyOnTheAxis(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name string
		ivs  []gantt.Interval
		want []int
	}{
		{"inside the axis", []gantt.Interval{span(22, 44)}, []int{2, 3, 4}},
		{"nearly the whole axis", []gantt.Interval{span(4, 94)}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{"two intervals of one lane", []gantt.Interval{span(12, 34), span(62, 84)}, []int{1, 2, 3, 6, 7, 8}},
		{"non-zero interval narrower than a cell still shows", []gantt.Interval{span(41, 41.5)}, []int{4}},
		{"zero-length interval shows one cell", []gantt.Interval{span(73, 73)}, []int{7}},
		{"clipped at the axis start", []gantt.Interval{span(-30, 24)}, []int{0, 1, 2}},
		{"clipped at the axis end", []gantt.Interval{span(82, 170)}, []int{8, 9}},
		{"entirely before the axis is not drawn", []gantt.Interval{span(-20, -10)}, nil},
		{"entirely after the axis is not drawn", []gantt.Interval{span(110, 120)}, nil},
		{"zero-length before the axis is not drawn", []gantt.Interval{span(-5, -5)}, nil},
		{"zero-length after the axis is not drawn", []gantt.Interval{span(150, 150)}, nil},
		{"no intervals", nil, nil},
		{"ending on a cell boundary does not cover the next cell", []gantt.Interval{span(20, 40)}, []int{2, 3}},
		{"ending exactly at the axis start shows in the first cell", []gantt.Interval{span(-20, 0)}, []int{0}},
		{"starting exactly at the axis end shows in the last cell", []gantt.Interval{span(100, 130)}, []int{9}},
		{"end before start counts as zero length", []gantt.Interval{span(60, 40)}, []int{6}},
		{"end before start after the axis is not drawn", []gantt.Interval{span(120, 110)}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := occupied(plotFor(intervals(tc.ivs...))); !equalInts(got, tc.want) {
				t.Errorf("occupied cells %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderLane_LanesOnOneAxisLineUp(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	a := axisOf(100)

	first := plotOf(lineOf(a, intervals(span(12, 44))), lw)
	second := plotOf(lineOf(a, intervals(span(22, 84))), lw)

	if !equalInts(occupied(first), []int{1, 2, 3, 4}) || !equalInts(occupied(second), []int{2, 3, 4, 5, 6, 7, 8}) {
		t.Errorf("overlapping intervals of different lanes must share columns: %v and %v", occupied(first), occupied(second))
	}
}

func TestRenderLane_AllCellsOfOneIntervalShareAGlyph(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	plot := plotFor(intervals(span(22, 44)))

	if !equalInts(occupied(plot), []int{2, 3, 4}) || plot[2] != plot[3] || plot[3] != plot[4] {
		t.Errorf("one closed interval is drawn with one glyph, got %q", string(plot))
	}
}

func TestRenderLane_OpenIntervalRunsToTheAxisEndWithAnOpenEndGlyph(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name string
		iv   gantt.Interval
		want []int
	}{
		{"zero end", gantt.Interval{Start: at(62), Open: true}, []int{6, 7, 8, 9}},
		{"end earlier than the axis end is ignored", gantt.Interval{Start: at(62), End: at(70), Open: true}, []int{6, 7, 8, 9}},
		{"starting before the axis", gantt.Interval{Start: at(-10), Open: true}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{"starting after the axis end is not drawn", gantt.Interval{Start: at(120), Open: true}, nil},
		{"starting exactly at the axis end shows the open end only", gantt.Interval{Start: at(100), Open: true}, []int{9}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := occupied(plotFor(intervals(tc.iv))); !equalInts(got, tc.want) {
				t.Errorf("occupied cells %v, want %v", got, tc.want)
			}
		})
	}
	plot := plotFor(intervals(gantt.Interval{Start: at(62), Open: true}))
	if plot[9] == plot[8] || plot[7] != plot[8] {
		t.Errorf("the last cell must carry the open-end glyph, the rest the interval glyph: %q", string(plot))
	}
}

func TestRenderLane_OpenIntervalDiffersFromAClosedOneToTheAxisEnd(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	open := plotFor(intervals(gantt.Interval{Start: at(62), Open: true}))
	closed := plotFor(intervals(span(62, 100)))

	if string(open) == string(closed) {
		t.Errorf("an open interval must be told apart from a closed one: %q", string(open))
	}
}

func TestRenderLane_PointsUseTheMarkerGlyphOfTheirKind(t *testing.T) {
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
		t.Run(string(tc.glyph), func(t *testing.T) {
			useMode(t, tc.mode)
			lane := gantt.Lane{Label: "ab", Points: []gantt.Point{{At: at(53), Kind: tc.kind}}}

			plot := plotFor(lane)

			if !equalInts(occupied(plot), []int{5}) || plot[5] != tc.glyph {
				t.Errorf("want only %q in cell 5, got %q", tc.glyph, string(plot))
			}
		})
	}
}

func TestRenderLane_PointIsDrawnAboveAnInterval(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{
		Label:     "ab",
		Intervals: []gantt.Interval{span(4, 94)},
		Points:    []gantt.Point{{At: at(43), Kind: chart.MarkerPrimary}},
	}

	plot := plotFor(lane)

	if plot[4] != '|' || plot[3] == '|' || plot[5] == '|' || len(occupied(plot)) != 10 {
		t.Errorf("want the marker in cell 4 and the interval everywhere else, got %q", string(plot))
	}
}

func TestRenderLane_ShadeFillsItsCellsAndDiffersFromIntervalsWithColourOff(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	shade := gantt.Shade{Start: at(31), End: at(54)}

	shaded := plotFor(gantt.Lane{Label: "ab"}, shade)
	barred := plotFor(intervals(span(31, 54)))

	if !equalInts(occupied(shaded), []int{3, 4, 5}) {
		t.Errorf("shade must be visible in cells 3..5 with colour off, got %q", string(shaded))
	}
	if shaded[4] == barred[4] {
		t.Errorf("a shade must look different from an interval: both %q", string(shaded[4]))
	}
}

func TestRenderLane_ShadeIsOnlyTheBackgroundOfIntervalsAndPoints(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	shade := gantt.Shade{Start: at(31), End: at(54)}
	intervalGlyph := plotFor(intervals(span(42, 44)))[4]
	shadeGlyph := plotFor(gantt.Lane{Label: "ab"}, shade)[4]

	withInterval := plotFor(intervals(span(42, 44)), shade)
	withPoint := plotFor(gantt.Lane{Label: "ab", Points: []gantt.Point{{At: at(43), Kind: chart.MarkerTertiary}}}, shade)

	if withInterval[3] != shadeGlyph || withInterval[4] != intervalGlyph || withInterval[5] != shadeGlyph {
		t.Errorf("interval must cover the shade in cell 4 only: %q", string(withInterval))
	}
	if withPoint[3] != shadeGlyph || withPoint[4] != '*' || withPoint[5] != shadeGlyph {
		t.Errorf("point must cover the shade in cell 4 only: %q", string(withPoint))
	}
}

func TestRenderLane_ShadeIsClippedToTheAxis(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name  string
		shade gantt.Shade
		want  []int
	}{
		{"past the end", gantt.Shade{Start: at(82), End: at(200)}, []int{8, 9}},
		{"before the start", gantt.Shade{Start: at(-50), End: at(24)}, []int{0, 1, 2}},
		{"outside the axis", gantt.Shade{Start: at(120), End: at(150)}, nil},
		{"zero length covers one cell", gantt.Shade{Start: at(60), End: at(60)}, []int{6}},
		{"end before start counts as zero length", gantt.Shade{Start: at(60), End: at(40)}, []int{6}},
		{"end before start outside the axis", gantt.Shade{Start: at(160), End: at(140)}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := occupied(plotFor(gantt.Lane{Label: "ab"}, tc.shade)); !equalInts(got, tc.want) {
				t.Errorf("occupied cells %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderLane_NarrowingTheAxisZoomsTheSameLane(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	narrowed := axisOf(100).Narrow(at(20), at(60)) // 4 seconds per cell
	cases := []struct {
		name string
		lane gantt.Lane
		want []int
	}{
		{"interval inside the range", intervals(span(21, 45)), []int{0, 1, 2, 3, 4, 5, 6}},
		{"interval running past the range end", intervals(span(49, 90)), []int{7, 8, 9}},
		{"interval outside the range", intervals(span(70, 90)), nil},
		{"point inside the range", gantt.Lane{Label: "ab", Points: []gantt.Point{{At: at(53)}}}, []int{8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := occupied(plotOf(lineOf(narrowed, tc.lane), lw))
			if !equalInts(got, tc.want) {
				t.Errorf("occupied cells %v, want %v", got, tc.want)
			}
		})
	}
	lane := intervals(span(21, 45))
	if lineOf(axisOf(100), lane) == lineOf(narrowed, lane) {
		t.Error("the same lane must render differently on a narrowed axis")
	}
}
