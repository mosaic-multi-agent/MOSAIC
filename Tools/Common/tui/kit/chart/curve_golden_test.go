package chart_test

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-common/tui/kit/chart"
)

// The golden files hold the real rendered text of the curve at three widths, each line wrapped in
// '|' so its width is visible. Colour is off.

func goldenCurve() chart.Curve {
	values := make([]float64, 24)
	for i := range values {
		values[i] = float64((i*7)%11 + (i/6)*2)
	}
	return chart.Curve{
		Values: values,
		Markers: []chart.Marker{
			{Index: 0, Kind: chart.MarkerPrimary},
			{Index: 8, Kind: chart.MarkerSecondary},
			{Index: 15, Kind: chart.MarkerTertiary},
			{Index: 40, Kind: chart.MarkerTertiary},
		},
	}
}

func TestRenderCurve_GoldenShapes(t *testing.T) {
	for _, m := range bothModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)
			var b strings.Builder
			for _, width := range []int{12, 30, 60} {
				fmt.Fprintf(&b, "width %d height 6\n", width)
				out := chart.RenderCurve(goldenCurve(), width, 6, theme())
				for _, line := range strings.Split(out, "\n") {
					fmt.Fprintf(&b, "|%s|\n", fmt.Sprintf("%-*s", width, line))
				}
			}
			assertGolden(t, "curve_"+m.name, strings.TrimSuffix(b.String(), "\n"))
		})
	}
}
