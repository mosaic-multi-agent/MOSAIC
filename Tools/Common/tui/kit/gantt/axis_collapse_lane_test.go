package gantt_test

import (
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"
)

// Every row below is 40 cells on collAxis: cells 0-19 show 0..200, cells 20-29 are the marker
// (break glyph in the first and last cell, blanks between) and cells 30-39 show 900..1000.

func TestRenderLane_BarSpanningACollapsedStretch_ContinuesOnBothSidesOfTheMarker(t *testing.T) {
	for _, m := range collModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)
			lane := intervals(span(100, 955))

			got := collPlot(collAxis(), lane)

			want := rep(" ", 10) + rep(m.bar, 10) + m.brk + rep(" ", 8) + m.brk + rep(m.bar, 6) + rep(" ", 4)
			if got != want {
				t.Errorf("plot = %q, want %q", got, want)
			}
		})
	}
}

func TestRenderLane_ShadeOverTheWaitingAgentsOwnBar_LeavesTheBarOnBothSidesOfTheMarker(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := intervals(span(100, 955))
	lane.Shades = []gantt.Shade{{Start: at(150), End: at(955), Flat: true, ASCII: '-'}}

	got := collPlot(collAxis(), lane)

	want := rep(" ", 10) + rep("=", 5) + rep("-", 5) + ":" + rep(" ", 8) + ":" + rep("-", 6) + rep(" ", 4)
	if got != want {
		t.Errorf("plot = %q, want %q", got, want)
	}
}

func TestRenderLane_RenderWideShadeAcrossAStretch_IsInterruptedByTheMarker(t *testing.T) {
	for _, m := range collModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)

			got := collPlot(collAxis(), gantt.Lane{Label: "ab"}, gantt.Shade{Start: at(100), End: at(955)})

			want := rep(" ", 10) + rep(m.shadeDot, 10) + m.brk + rep(" ", 8) + m.brk + rep(m.shadeDot, 6) + rep(" ", 4)
			if got != want {
				t.Errorf("plot = %q, want %q", got, want)
			}
		})
	}
}

func TestRenderLane_OpenBar_RunsToThePlotEndAcrossTheMarker(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name  string
		start float64
		want  string
	}{
		{"starts before the stretch", 100, rep(" ", 10) + rep("=", 10) + ":" + rep(" ", 8) + ":" + rep("=", 9) + ">"},
		{"starts inside the stretch", 500, rep(" ", 20) + ":" + rep(" ", 8) + ":" + rep("=", 9) + ">"},
		{"starts after the stretch", 950, rep(" ", 20) + ":" + rep(" ", 8) + ":" + rep(" ", 5) + rep("=", 4) + ">"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := collPlot(collAxis(), intervals(gantt.Interval{Start: at(tc.start), Open: true}))

			if got != tc.want {
				t.Errorf("plot = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderLane_PointsAreMovedByTheCollapse(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	glyph := []rune(chart.MarkerGlyph(chart.MarkerPrimary))[0]
	lane := gantt.Lane{Label: "ab", Points: []gantt.Point{
		{At: at(50), Kind: chart.MarkerPrimary}, {At: at(910), Kind: chart.MarkerPrimary}, {At: at(1000), Kind: chart.MarkerPrimary},
	}}

	got := collPlot(collAxis(), lane)

	want := []rune(rep(" ", collPlotW))
	want[5], want[20], want[29], want[31], want[39] = glyph, ':', ':', glyph, glyph
	if got != string(want) {
		t.Errorf("plot = %q, want %q", got, string(want))
	}
}

func TestRenderLane_PointInsideACollapsedStretch_IsCoveredByTheMarker(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{Label: "ab", Points: []gantt.Point{{At: at(500), Kind: chart.MarkerPrimary}}}

	got := collPlot(collAxis(), lane)

	want := rep(" ", 20) + ":" + rep(" ", 8) + ":" + rep(" ", 10)
	if got != want {
		t.Errorf("plot = %q, want %q", got, want)
	}
}

func TestRenderLane_TouchingStretches_DrawTwoMarkersSideBySide(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := collPlot(touchingAxis(), intervals(span(50, 955)))

	want := rep(" ", 5) + rep("=", 5) + ":" + rep(" ", 8) + ":" + ":" + rep(" ", 8) + ":" + rep("=", 6) + rep(" ", 4)
	if got != want {
		t.Errorf("plot = %q, want %q", got, want)
	}
}

func TestRenderLane_PlotTooNarrowForTheMarkers_RendersLikeAnAxisWithoutCollapse(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	collapsed := axisOf(1000)
	collapsed.Collapsed = []gantt.Collapse{cut(150, 850)}
	lane := intervals(span(100, 955))

	for _, plotWidth := range []int{5, 11} {
		width := collLabel + 1 + plotWidth
		got := gantt.RenderLane(collapsed, lane, nil, collLabel, width, false, theme())
		want := gantt.RenderLane(axisOf(1000), lane, nil, collLabel, width, false, theme())
		if got != want {
			t.Errorf("plot of %d cells: got %q, want the linear %q", plotWidth, got, want)
		}
	}
}

func TestRenderLaneAndAxis_WithoutCollapsedStretches_RenderAsBefore(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := intervals(span(100, 955))
	plain := axisOf(1000)
	empty := axisOf(1000)
	empty.Collapsed = []gantt.Collapse{}

	if got, want := gantt.RenderLane(empty, lane, nil, collLabel, collLineW, false, theme()),
		gantt.RenderLane(plain, lane, nil, collLabel, collLineW, false, theme()); got != want {
		t.Errorf("empty Collapsed changes the lane: got %q, want %q", got, want)
	}
	if got, want := gantt.RenderAxis(empty, collLabel, collLineW, theme()), gantt.RenderAxis(plain, collLabel, collLineW, theme()); got != want {
		t.Errorf("empty Collapsed changes the axis: got %q, want %q", got, want)
	}
	if want := rep(" ", 4) + rep("=", 35) + rep(" ", 1); collPlot(plain, lane) != want {
		t.Errorf("linear lane = %q, want %q", collPlot(plain, lane), want)
	}
}
