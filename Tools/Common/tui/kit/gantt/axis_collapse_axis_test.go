package gantt_test

import (
	"regexp"
	"testing"
	"time"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/gantt"
)

// threeDayAxis spans three days and five minutes with a 3 day stretch cut out after 200 seconds:
// cells 0-19 show the first 200 seconds, cells 20-29 are the marker and cells 30-39 show the
// last 100 seconds.
func threeDayAxis() gantt.Axis {
	a := axisOf(259_500)
	a.Collapsed = []gantt.Collapse{cut(200, 259_400)}
	return a
}

// weekAxis cuts a week out of an axis that otherwise shows two hours, on a 60 cell plot area.
func weekAxis() gantt.Axis {
	a := axisOf(612_000)
	a.Collapsed = []gantt.Collapse{cut(3600, 608_400)}
	return a
}

var dateLabelRE = regexp.MustCompile(`\d\d-\d\d \d\d:\d\d`)

func TestRenderAxis_CollapsedStretch_ShowsBreakGlyphsAroundTheCentredLength(t *testing.T) {
	for _, m := range collModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)

			plot := collAxisRow(threeDayAxis(), collPlotW)

			if len(plot) != collPlotW {
				t.Fatalf("axis plot has %d cells, want %d", len(plot), collPlotW)
			}
			if got, want := string(plot[20:30]), m.brk+"   3d   "+m.brk; got != want {
				t.Errorf("marker cells = %q, want %q", got, want)
			}
		})
	}
}

func TestRenderAxis_MarkerLengthLabelUsesTheTwoLargestUnits(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	a := axisOf(259_500 + 7200)
	a.Collapsed = []gantt.Collapse{cut(200, 259_400+7200)} // 3d 2h

	plot := collAxisRow(a, collPlotW)

	if len(plot) != collPlotW {
		t.Fatalf("axis plot has %d cells, want %d", len(plot), collPlotW)
	}
	inner := string(plot[21:29])
	if plot[20] != ':' || plot[29] != ':' || !regexp.MustCompile(`^ {1,2}3d 2h {1,2}$`).MatchString(inner) {
		t.Errorf("marker cells = %q, want : and : around a centred \"3d 2h\"", string(plot[20:30]))
	}
}

// assertTickLabels checks the date labels of an axis row against the collapsed stretches: a label
// is at the column its time maps to, names no time inside a stretch and never overlaps a marker.
func assertTickLabels(t *testing.T, a gantt.Axis, plotWidth int) {
	t.Helper()
	plot := string(collAxisRow(a, plotWidth))
	labels := dateLabelRE.FindAllStringIndex(plot, -1)
	if len(labels) == 0 {
		t.Fatalf("axis row %q has no date labels", plot)
	}
	if labels[0][0] != 0 {
		t.Errorf("first label starts at cell %d, want the axis start at cell 0 (row %q)", labels[0][0], plot)
	}
	for _, loc := range labels {
		tick, err := time.Parse("2006-01-02 15:04", "2026-"+plot[loc[0]:loc[1]])
		if err != nil {
			t.Fatalf("label %q is not a time: %v", plot[loc[0]:loc[1]], err)
		}
		if col, _ := a.Column(tick, plotWidth); col != loc[0] {
			t.Errorf("label %q sits at cell %d, its time maps to cell %d", plot[loc[0]:loc[1]], loc[0], col)
		}
		for _, c := range a.Collapsed {
			if tick.After(c.Start) && tick.Before(c.End) {
				t.Errorf("label %q names a time inside the collapsed stretch %v..%v", plot[loc[0]:loc[1]], c.Start, c.End)
			}
		}
		assertClearOfMarkers(t, a, plotWidth, loc)
	}
}

// assertClearOfMarkers fails when the label cells [loc[0], loc[1]) touch a marker's cells.
func assertClearOfMarkers(t *testing.T, a gantt.Axis, plotWidth int, loc []int) {
	t.Helper()
	for _, c := range a.Collapsed {
		first, _ := a.Column(c.Start, plotWidth)
		if loc[0] < first+gantt.CollapseMarkerWidth && loc[1] > first {
			t.Errorf("label cells %d..%d overlap the marker at cells %d..%d", loc[0], loc[1]-1, first, first+gantt.CollapseMarkerWidth-1)
		}
	}
}

func TestRenderAxis_TicksNeverLabelTimeInsideACollapsedStretch(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	t.Run("three days cut out", func(t *testing.T) { assertTickLabels(t, threeDayAxis(), collPlotW) })
	t.Run("a week cut out, many midnights inside", func(t *testing.T) { assertTickLabels(t, weekAxis(), 60) })
}
