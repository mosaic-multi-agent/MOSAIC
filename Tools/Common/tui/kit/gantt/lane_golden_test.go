package gantt_test

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"
)

// The golden files hold the real rendered text of an axis line and four lanes at three widths,
// each line wrapped in '|' so its width is visible. Colour is off.

const goldenLabelWidth = 8

var goldenAxisSeconds = 600.0

func goldenLanes() []gantt.Lane {
	return []gantt.Lane{
		{Label: "lead", Intervals: []gantt.Interval{span(67, 547)}, Points: []gantt.Point{
			{At: at(187), Kind: chart.MarkerPrimary}, {At: at(367), Kind: chart.MarkerSecondary},
		}},
		{Label: "worker-1", Intervals: []gantt.Interval{span(67, 187), span(307, 307.4)}},
		{Label: "worker-2-long-name", Intervals: []gantt.Interval{{Start: at(427), Open: true}}, Points: []gantt.Point{
			{At: at(487), Kind: chart.MarkerTertiary},
		}},
		{Label: "idle", Intervals: []gantt.Interval{span(-120, 37), span(247, 247), span(667, 727)}},
	}
}

func goldenShades() []gantt.Shade { return []gantt.Shade{{Start: at(187), End: at(247)}} }

func TestRenderLane_GoldenRenders(t *testing.T) {
	for _, m := range bothModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)
			var b strings.Builder
			for _, width := range []int{19, 29, 49} {
				a := axisOf(goldenAxisSeconds)
				fmt.Fprintf(&b, "width %d\n", width)
				fmt.Fprintf(&b, "|%s|\n", gantt.RenderAxis(a, goldenLabelWidth, width, theme()))
				for _, lane := range goldenLanes() {
					fmt.Fprintf(&b, "|%s|\n", gantt.RenderLane(a, lane, goldenShades(), goldenLabelWidth, width, false, theme()))
				}
			}
			assertGolden(t, "lanes_"+m.name, strings.TrimSuffix(b.String(), "\n"))
		})
	}
}
