package gantt_test

import (
	"testing"

	"mosaic-common/tui/kit/gantt"
)

func TestAxisColumn_WithACollapsedStretch_MapsTimeOnBothSidesOfTheMarker(t *testing.T) {
	a := collAxis()
	cases := []struct {
		name string
		sec  float64
		want int
	}{
		{"axis start", 0, 0},
		{"early in the first segment", 95, 9},
		{"just before the stretch", 199, 19},
		{"just after the stretch", 910, 31},
		{"late in the last segment", 999, 39},
		{"axis end", 1000, 39},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			col, ok := a.Column(at(tc.sec), collPlotW)

			if !ok || col != tc.want {
				t.Errorf("Column(%gs) = %d, %v; want %d", tc.sec, col, ok, tc.want)
			}
		})
	}
}

func TestAxisColumn_TimeInsideACollapsedStretch_MapsToTheFirstMarkerColumn(t *testing.T) {
	a := collAxis()

	for _, sec := range []float64{201, 500, 899} {
		if col, ok := a.Column(at(sec), collPlotW); !ok || col != 20 {
			t.Errorf("Column(%gs) = %d, %v; want the first marker column 20", sec, col, ok)
		}
	}
}

func TestAxisColumn_MarkerKeepsItsWidthWhateverTheStretchLength(t *testing.T) {
	for _, length := range []float64{700, 90_000, 5_000_000} {
		a := axisOf(300 + length)
		a.Collapsed = []gantt.Collapse{cut(200, 200+length)}

		col, ok := a.Column(at(200+length+50), collPlotW)

		if !ok || col != 35 {
			t.Errorf("stretch of %gs: Column of 50s after it = %d, %v; want 35 (marker is %d columns)",
				length, col, ok, gantt.CollapseMarkerWidth)
		}
	}
}

func TestAxisColumn_TouchingStretchesAreSeparateMarkersSideBySide(t *testing.T) {
	a := touchingAxis()
	cases := []struct {
		sec  float64
		want int
	}{{50, 5}, {450, 10}, {550, 20}, {950, 35}}

	for _, tc := range cases {
		if col, ok := a.Column(at(tc.sec), collPlotW); !ok || col != tc.want {
			t.Errorf("Column(%gs) = %d, %v; want %d", tc.sec, col, ok, tc.want)
		}
	}
}

func TestAxisColumn_PlotTooNarrowForTheMarkers_ActsAsIfNothingWereCollapsed(t *testing.T) {
	collapsed := axisOf(1000)
	collapsed.Collapsed = []gantt.Collapse{cut(150, 850)}
	linear := axisOf(1000)

	// 11 cells cannot hold a 10 cell marker plus one cell for each of the two segments.
	for _, sec := range []float64{0, 100, 500, 860, 1000} {
		got, _ := collapsed.Column(at(sec), 11)
		want, _ := linear.Column(at(sec), 11)
		if got != want {
			t.Errorf("11 cells, %gs: Column = %d, want the linear %d", sec, got, want)
		}
	}
	for _, sec := range []float64{100, 500, 860} {
		got, _ := collapsed.Column(at(sec), 5)
		want, _ := linear.Column(at(sec), 5)
		if got != want {
			t.Errorf("5 cells, %gs: Column = %d, want the linear %d", sec, got, want)
		}
	}
}

func TestAxisColumn_PlotJustWideEnoughForTheMarkers_StaysCollapsed(t *testing.T) {
	a := axisOf(1000)
	a.Collapsed = []gantt.Collapse{cut(150, 850)}
	want := map[float64]int{100: 0, 500: 1, 860: 11, 999: 11}

	for sec, col := range want {
		if got, ok := a.Column(at(sec), 12); !ok || got != col {
			t.Errorf("12 cells, %gs: Column = %d, %v; want %d", sec, got, ok, col)
		}
	}
}

func TestAxisNarrow_KeepsOnlyTheStretchesEntirelyInsideTheRange(t *testing.T) {
	a := axisOf(1000)
	a.Collapsed = []gantt.Collapse{cut(100, 500), cut(600, 900)}
	cases := []struct {
		name     string
		from, to float64
		want     []gantt.Collapse
	}{
		{"first stretch only", 50, 700, []gantt.Collapse{cut(100, 500)}},
		{"first stretch cut by the range start", 300, 1000, []gantt.Collapse{cut(600, 900)}},
		{"second stretch only", 550, 950, []gantt.Collapse{cut(600, 900)}},
		{"no stretch inside", 0, 200, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := a.Narrow(at(tc.from), at(tc.to)).Collapsed

			if len(got) != len(tc.want) {
				t.Fatalf("Narrow(%g, %g) keeps %v, want %v", tc.from, tc.to, got, tc.want)
			}
			for i := range got {
				if !got[i].Start.Equal(tc.want[i].Start) || !got[i].End.Equal(tc.want[i].End) {
					t.Errorf("kept stretch %d = %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
