package gantt_test

import (
	"testing"

	"mosaic-common/tui/kit/gantt"
)

func TestAxisColumn_MapsTimeProportionallyAndClamps(t *testing.T) {
	cases := []struct {
		name  string
		sec   float64
		width int
		want  int
	}{
		{"start", 0, 10, 0},
		{"one tenth", 10, 10, 1},
		{"middle", 50, 10, 5},
		{"inside a cell", 52, 10, 5},
		{"last cell", 90, 10, 9},
		{"just before the end", 99, 10, 9},
		{"exactly the end clamps to the last cell", 100, 10, 9},
		{"before the start clamps to zero", -10, 10, 0},
		{"after the end clamps to the last cell", 250, 10, 9},
		{"narrow plot", 30, 4, 1},
		{"narrow plot, later", 80, 4, 3},
		{"single cell", 77, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			col, ok := axisOf(100).Column(at(tc.sec), tc.width)
			if !ok || col != tc.want {
				t.Errorf("Column(%vs, %d) = %d, %v; want %d, true", tc.sec, tc.width, col, ok, tc.want)
			}
		})
	}
}

func TestAxisColumn_EmptyAxisReportsNotOK(t *testing.T) {
	cases := map[string]gantt.Axis{
		"zero value":       {},
		"start equals end": {Start: base, End: base},
		"end before start": {Start: at(10), End: at(5)},
	}
	if _, ok := axisOf(100).Column(at(3), 10); !ok {
		t.Fatal("control: a non-empty axis must report ok == true")
	}
	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := a.Column(at(3), 10); ok {
				t.Error("an empty axis must report ok == false")
			}
		})
	}
}

func TestAxisColumn_NoWidthReportsNotOK(t *testing.T) {
	if _, ok := axisOf(100).Column(at(30), 10); !ok {
		t.Fatal("control: a positive width must report ok == true")
	}
	for _, width := range []int{0, -4} {
		if _, ok := axisOf(100).Column(at(30), width); ok {
			t.Errorf("width %d: want ok == false", width)
		}
	}
}

func TestAxisNarrow_IntersectsWithTheAxis(t *testing.T) {
	cases := []struct {
		name             string
		from, to         float64
		wantFrom, wantTo float64
	}{
		{"inside", 20, 60, 20, 60},
		{"starts before the axis", -50, 40, 0, 40},
		{"ends after the axis", 70, 500, 70, 100},
		{"wider than the axis", -5, 500, 0, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := axisOf(100).Narrow(at(tc.from), at(tc.to))
			if !got.Start.Equal(at(tc.wantFrom)) || !got.End.Equal(at(tc.wantTo)) {
				t.Errorf("got %v..%v, want %vs..%vs", got.Start, got.End, tc.wantFrom, tc.wantTo)
			}
		})
	}
}

func TestAxisNarrow_DisjointRangeIsEmpty(t *testing.T) {
	got := axisOf(100).Narrow(at(200), at(300))
	overlapping := axisOf(100).Narrow(at(50), at(300))

	if _, ok := got.Column(at(250), 10); ok {
		t.Errorf("a range outside the axis must narrow to an empty axis, got %v..%v", got.Start, got.End)
	}
	if _, ok := overlapping.Column(at(75), 10); !ok {
		t.Error("a range that overlaps the axis must not narrow to an empty axis")
	}
}

func TestAxisNarrow_ColumnsFollowTheNarrowedRange(t *testing.T) {
	narrowed := axisOf(100).Narrow(at(50), at(100))

	mid, okMid := narrowed.Column(at(75), 10)
	before, okBefore := narrowed.Column(at(40), 10)

	if !okMid || mid != 5 || !okBefore || before != 0 {
		t.Errorf("want 75s in column 5 and 40s clamped to 0, got %d (%v) and %d (%v)", mid, okMid, before, okBefore)
	}
}
