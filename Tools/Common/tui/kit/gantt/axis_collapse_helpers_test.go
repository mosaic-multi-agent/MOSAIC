package gantt_test

import (
	"strings"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/gantt"
)

// Collapse tests use a 40 cell plot area behind a 3 cell label (44 cell lines). The test axis
// spans 1000 seconds with 200..900 cut out: 300 seconds stay to scale on 30 cells (10 seconds a
// cell), so cells 0-19 show 0..200, cells 20-29 are the marker and cells 30-39 show 900..1000.
const (
	collLabel = 3
	collPlotW = 40
	collLineW = collLabel + 1 + collPlotW
)

func cut(from, to float64) gantt.Collapse { return gantt.Collapse{Start: at(from), End: at(to)} }

// collAxis is the 1000 second axis with 200..900 collapsed.
func collAxis() gantt.Axis {
	a := axisOf(1000)
	a.Collapsed = []gantt.Collapse{cut(200, 900)}
	return a
}

// touchingAxis has two touching stretches, 100..500 and 500..900: cells 0-9 show 0..100, cells
// 10-19 and 20-29 are the markers and cells 30-39 show 900..1000.
func touchingAxis() gantt.Axis {
	a := axisOf(1000)
	a.Collapsed = []gantt.Collapse{cut(100, 500), cut(500, 900)}
	return a
}

// collPlot renders a lane and returns its 40 plot cells; a render that is too short is padded
// with U+FFFD cells so it fails the comparison instead of panicking.
func collPlot(a gantt.Axis, lane gantt.Lane, shades ...gantt.Shade) string {
	r := []rune(gantt.RenderLane(a, lane, shades, collLabel, collLineW, false, theme()))
	plot := []rune{}
	if len(r) > collLabel+1 {
		plot = r[collLabel+1:]
	}
	for len(plot) < collPlotW {
		plot = append(plot, '�')
	}
	return string(plot)
}

func rep(s string, n int) string { return strings.Repeat(s, n) }

// modeGlyphs are the bar and break glyphs of a glyph mode.
type modeGlyphs struct {
	name       string
	mode       tui.GlyphMode
	bar, brk   string
	shadeDot   string
	openMarker string
}

var collModes = []modeGlyphs{
	{"unicode", tui.GlyphUnicode, "█", "┊", "░", ">"},
	{"ascii", tui.GlyphASCII, "=", ":", ".", ">"},
}

// collAxisRow renders the axis row and returns its plot cells.
func collAxisRow(a gantt.Axis, plotWidth int) []rune {
	line := []rune(gantt.RenderAxis(a, collLabel, collLabel+1+plotWidth, theme()))
	if len(line) < collLabel+1 {
		return nil
	}
	return line[collLabel+1:]
}
