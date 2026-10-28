package gantt_test

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"
	"mosaic-common/tui/kit/textwidth"
)

func TestRenderLane_ExactLinesForTheDocumentedExamples(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	mixed := gantt.Lane{
		Label:     "ab",
		Intervals: []gantt.Interval{span(22, 44)},
		Points:    []gantt.Point{{At: at(73), Kind: chart.MarkerSecondary}},
	}
	open := intervals(gantt.Interval{Start: at(62), Open: true})

	if got, want := lineOf(axisOf(100), mixed, gantt.Shade{Start: at(50), End: at(70)}), "ab    ===..:  "; got != want {
		t.Errorf("interval, shade and point: got %q, want %q", got, want)
	}
	if got, want := lineOf(axisOf(100), open), "ab"+strings.Repeat(" ", 8)+"===>"; got != want {
		t.Errorf("open interval: got %q, want %q", got, want)
	}
}

func TestRenderLane_GlyphsOfIntervalOpenEndAndShadeInBothModes(t *testing.T) {
	cases := []struct {
		mode        tui.GlyphMode
		bar, shade  rune
		whichString string
	}{
		{tui.GlyphUnicode, '█', '░', "unicode"},
		{tui.GlyphASCII, '=', '.', "ascii"},
	}
	for _, tc := range cases {
		t.Run(tc.whichString, func(t *testing.T) {
			useMode(t, tc.mode)
			bar, open := plotFor(intervals(span(30, 60))), plotFor(intervals(gantt.Interval{Start: at(70), Open: true}))
			shade := plotFor(gantt.Lane{Label: "ab"}, gantt.Shade{Start: at(30), End: at(60)})

			if string(bar) != strings.Repeat(" ", 3)+strings.Repeat(string(tc.bar), 3)+strings.Repeat(" ", 4) {
				t.Errorf("interval cells: got %q", string(bar))
			}
			if string(open) != strings.Repeat(" ", 7)+strings.Repeat(string(tc.bar), 2)+">" {
				t.Errorf("open interval cells: got %q", string(open))
			}
			if string(shade) != strings.Repeat(" ", 3)+strings.Repeat(string(tc.shade), 3)+strings.Repeat(" ", 4) {
				t.Errorf("shade cells: got %q", string(shade))
			}
		})
	}
}

func TestRenderLane_PointsOnlyDrawnWithinTheAxis(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name string
		at   float64
		want []int
	}{
		{"at the axis start", 0, []int{0}},
		{"at the axis end", 100, []int{9}},
		{"just before the axis", -0.5, nil},
		{"just after the axis", 100.5, nil},
		{"far outside", 500, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lane := gantt.Lane{Label: "ab", Points: []gantt.Point{{At: at(tc.at), Kind: chart.MarkerPrimary}}}

			if got := occupied(plotFor(lane)); !equalInts(got, tc.want) {
				t.Errorf("occupied cells %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderLane_LaterIntervalWinsInASharedCell(t *testing.T) {
	useColourOn(t)
	lane := intervals(roled(span(12, 54), tui.RoleSignalGood), roled(span(32, 74), tui.RoleSignalBad))

	styles := plotStyles(lineOf(axisOf(100), lane))

	for col, role := range map[int]tui.StyleRole{1: tui.RoleSignalGood, 2: tui.RoleSignalGood, 3: tui.RoleSignalBad, 5: tui.RoleSignalBad, 7: tui.RoleSignalBad} {
		if styles[col] != rolePrefix(role) {
			t.Errorf("cell %d: want role %q, got style %q", col, role, styles[col])
		}
	}
}

func TestRenderLane_LaterIntervalCoversAnEarlierOpenEnd(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	open := gantt.Interval{Start: at(62), Open: true}

	covered := plotFor(intervals(open, span(88, 98)))
	opening := plotFor(intervals(span(88, 98), open))

	if string(covered) != "      ====" {
		t.Errorf("a later closed interval covers the open end: got %q", string(covered))
	}
	if string(opening) != "      ===>" {
		t.Errorf("a later open interval keeps its open end: got %q", string(opening))
	}
}

func TestRenderLane_HighestPointKindWinsAndEqualKindsKeepTheEarlierPoint(t *testing.T) {
	useColourOn(t)
	lane := gantt.Lane{Label: "ab", Points: []gantt.Point{
		{At: at(51), Kind: chart.MarkerPrimary, Role: tui.RoleSignalGood},
		{At: at(52), Kind: chart.MarkerTertiary, Role: tui.RoleSignalBad},
		{At: at(23), Kind: chart.MarkerSecondary, Role: tui.RoleSignalGood},
		{At: at(24), Kind: chart.MarkerSecondary, Role: tui.RoleSignalBad},
	}}

	line := lineOf(axisOf(100), lane)

	if got := plotOf(stripANSI(line), lw); got[5] != '*' || got[2] != ':' {
		t.Errorf("want the tertiary glyph in cell 5 and a secondary glyph in cell 2, got %q", string(got))
	}
	if got := plotStyles(line); got[5] != rolePrefix(tui.RoleSignalBad) || got[2] != rolePrefix(tui.RoleSignalGood) {
		t.Errorf("want the winning point's role: cell 5 bad, cell 2 good, got %q and %q", got[5], got[2])
	}
}

func TestRenderLane_LaterShadeWinsInASharedCell(t *testing.T) {
	useColourOn(t)
	first := gantt.Shade{Start: at(31), End: at(54), Role: tui.RoleSignalGood}
	second := gantt.Shade{Start: at(41), End: at(64), Role: tui.RoleSignalBad}

	styles := plotStyles(lineOf(axisOf(100), gantt.Lane{Label: "ab"}, first, second))

	for col, role := range map[int]tui.StyleRole{3: tui.RoleSignalGood, 4: tui.RoleSignalBad, 6: tui.RoleSignalBad} {
		if styles[col] != rolePrefix(role) {
			t.Errorf("cell %d: want role %q, got style %q", col, role, styles[col])
		}
	}
}

func TestRenderLane_OpenEndGlyphCarriesItsIntervalsRole(t *testing.T) {
	useColourOn(t)
	lane := intervals(roled(gantt.Interval{Start: at(62), Open: true}, tui.RoleSignalWarning))

	styles := plotStyles(lineOf(axisOf(100), lane))

	for col := 6; col <= 9; col++ {
		if styles[col] != rolePrefix(tui.RoleSignalWarning) {
			t.Errorf("cell %d: want the interval's role, got %q", col, styles[col])
		}
	}
}

func TestRenderLane_BlankCellsAndTheSeparatorCarryNoStyle(t *testing.T) {
	useColourOn(t)
	lane := intervals(roled(span(32, 54), tui.RoleSignalGood))

	styles := plotStyles(lineOf(axisOf(100), lane))
	separator := styleAt(lineOf(axisOf(100), lane), lw)

	for _, col := range []int{0, 1, 2, 6, 9} {
		if styles[col] != "" {
			t.Errorf("blank cell %d must carry no style, got %q", col, styles[col])
		}
	}
	if separator != "" {
		t.Errorf("the separator space must carry no style, got %q", separator)
	}
}

func TestRenderLane_EmptyAxisDrawsNothing(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{
		Label:     "ab",
		Intervals: []gantt.Interval{span(0, 50), {Start: at(10), Open: true}},
		Points:    []gantt.Point{{At: at(5)}},
	}

	for name, a := range map[string]gantt.Axis{"zero": {}, "start equals end": {Start: base, End: base}, "end before start": {Start: at(10), End: at(5)}} {
		out := gantt.RenderLane(a, lane, []gantt.Shade{{Start: at(0), End: at(50)}}, lw, lineW, false, theme())

		if want := "ab" + strings.Repeat(" ", lineW-2); out != want {
			t.Errorf("%s: want the label then blanks %q, got %q", name, want, out)
		}
	}
}

func TestRenderLane_NegativeLabelWidthCountsAsZero(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := intervals(span(22, 44))

	negative := gantt.RenderLane(axisOf(100), lane, nil, -3, 11, false, theme())
	zero := gantt.RenderLane(axisOf(100), lane, nil, 0, 11, false, theme())

	if negative != zero || negative != "   ===     " {
		t.Errorf("want %q for both, got %q and %q", "   ===     ", negative, zero)
	}
}

func TestRenderLane_NarrowLinesHoldTheLabelWithoutAPlotArea(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{Label: "worker-long", Intervals: []gantt.Interval{span(0, 100)}}
	const labelWidth = 6

	for width := 1; width <= labelWidth; width++ {
		want := textwidth.Pad("worker-long", width, false)
		if got := gantt.RenderLane(axisOf(100), lane, nil, labelWidth, width, false, theme()); got != want {
			t.Errorf("width %d: want only the cut label %q, got %q", width, want, got)
		}
	}
	want := textwidth.Pad("worker-long", labelWidth, false) + " "
	if got := gantt.RenderLane(axisOf(100), lane, nil, labelWidth, labelWidth+1, false, theme()); got != want {
		t.Errorf("width labelWidth+1: want the label and the separator %q, got %q", want, got)
	}
}
