package gantt_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/gantt"
)

// hourAxis spans one hour starting at base (14:00 to 15:00 UTC on 2026-01-02).
func hourAxis() gantt.Axis { return axisOf(3600) }

func renderAxis(a gantt.Axis, labelWidth, width int) string {
	return gantt.RenderAxis(a, labelWidth, width, theme())
}

// axisLine builds the expected line: labelWidth+1 blanks, then a plot area of plotWidth cells
// with each label starting at its plot column.
func axisLine(labelWidth, plotWidth int, labels map[int]string) string {
	plot := []rune(strings.Repeat(" ", plotWidth))
	for col, l := range labels {
		copy(plot[col:], []rune(l))
	}
	return strings.Repeat(" ", labelWidth+1) + string(plot)
}

func TestRenderAxis_PlacesLabelsAtTheirTimes(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	zone := time.FixedZone("plus7", 7*3600)
	oddZone := time.FixedZone("plus545", 5*3600+45*60)
	cases := []struct {
		name       string
		axis       gantt.Axis
		labelWidth int
		width      int
		labels     map[int]string
	}{
		{"30 minute step, last label would not fit", hourAxis(), 8, 26, map[int]string{0: "14:00", 8: "14:30"}},
		{"10 minute step at 80 columns", hourAxis(), 12, 80, map[int]string{
			0: "14:00", 11: "14:10", 22: "14:20", 33: "14:30", 44: "14:40", 55: "14:50",
		}},
		{"steps under a minute show seconds", axisOf(60), 0, 61, map[int]string{
			0: "14:00:00", 10: "14:00:10", 20: "14:00:20", 30: "14:00:30", 40: "14:00:40", 50: "14:00:50",
		}},
		{"labels gain the date when the axis crosses midnight", gantt.Axis{
			Start: time.Date(2026, 1, 2, 23, 0, 0, 0, time.UTC), End: time.Date(2026, 1, 3, 1, 0, 0, 0, time.UTC),
		}, 0, 61, map[int]string{0: "01-02 23:00", 15: "01-02 23:30", 30: "01-03 00:00", 45: "01-03 00:30"}},
		{"labels use the start's own location", gantt.Axis{
			Start: time.Date(2026, 1, 2, 20, 0, 0, 0, zone), End: time.Date(2026, 1, 2, 21, 0, 0, 0, zone),
		}, 4, 80, map[int]string{0: "20:00", 12: "20:10", 25: "20:20", 37: "20:30", 50: "20:40", 62: "20:50"}},
		{"ticks are multiples of the step after local midnight", gantt.Axis{
			Start: time.Date(2026, 1, 2, 14, 0, 0, 0, oddZone), End: time.Date(2026, 1, 2, 15, 0, 0, 0, oddZone),
		}, 8, 26, map[int]string{0: "14:00", 8: "14:30"}},
		{"a narrowed axis is labelled for its own range", hourAxis().Narrow(at(1200), at(1800)), 4, 80, map[int]string{
			0: "14:20", 7: "14:21", 15: "14:22", 22: "14:23", 30: "14:24", 37: "14:25", 45: "14:26", 52: "14:27", 60: "14:28", 67: "14:29",
		}},
		{"no tick in range: the start label sits in column 0", gantt.Axis{
			Start: time.Date(2026, 1, 2, 14, 1, 0, 0, time.UTC), End: time.Date(2026, 1, 2, 14, 4, 0, 0, time.UTC),
		}, 0, 6, map[int]string{0: "14:01"}},
		{"no tick in range and the label does not fit", gantt.Axis{
			Start: time.Date(2026, 1, 2, 14, 1, 0, 0, time.UTC), End: time.Date(2026, 1, 2, 14, 4, 0, 0, time.UTC),
		}, 0, 5, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := axisLine(tc.labelWidth, tc.width-tc.labelWidth-1, tc.labels)

			if got := renderAxis(tc.axis, tc.labelWidth, tc.width); got != want {
				t.Errorf("\n got %q\nwant %q", got, want)
			}
		})
	}
}

func TestRenderAxis_LineHasExactlyTheRequestedWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, labelWidth := range []int{-2, 0, 4, 12} {
		for width := 1; width <= 130; width++ {
			out := renderAxis(hourAxis(), labelWidth, width)
			if strings.Contains(out, "\n") || runeCount(out) != width {
				t.Fatalf("labelWidth %d width %d: want one line of %d cells, got %d: %q", labelWidth, width, width, runeCount(out), out)
			}
		}
	}
}

func TestRenderAxis_NoWidthGivesNothing(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	if out := renderAxis(hourAxis(), 4, 40); out == "" {
		t.Fatal("control: a positive width must render a line")
	}
	for _, width := range []int{0, -3} {
		if out := renderAxis(hourAxis(), 4, width); out != "" {
			t.Errorf("width %d: want empty output, got %q", width, out)
		}
	}
}

func TestRenderAxis_NoPlotAreaOrEmptyAxisGivesBlanks(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for width := 1; width <= 9; width++ {
		if out := renderAxis(hourAxis(), 8, width); out != strings.Repeat(" ", width) {
			t.Errorf("width %d leaves no plot area: want %d spaces, got %q", width, width, out)
		}
	}
	for _, a := range []gantt.Axis{{}, {Start: base, End: base}, {Start: at(10), End: at(5)}} {
		if out := renderAxis(a, 4, 60); out != strings.Repeat(" ", 60) {
			t.Errorf("an empty axis must render 60 spaces, got %q", out)
		}
	}
}

var timeLabelRE = regexp.MustCompile(`\d\d:\d\d(:\d\d)?`)

func TestRenderAxis_LabelsAreWholeEquallyLongAndTwoBlanksApart(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	if n := len(timeLabelRE.FindAllString(renderAxis(hourAxis(), 8, 200), -1)); n < 2 {
		t.Fatalf("setup: want several labels at 200 columns, got %d", n)
	}

	for width := 20; width <= 200; width++ {
		out := renderAxis(hourAxis(), 8, width)
		spans := timeLabelRE.FindAllStringIndex(out, -1)
		for i, s := range spans {
			length := s[1] - s[0]
			if length != 5 && length != 8 {
				t.Fatalf("width %d: %q holds a cut or merged label in %q", width, out[s[0]:s[1]], out)
			}
			if i > 0 && (length != spans[0][1]-spans[0][0] || s[0]-spans[i-1][1] < 2) {
				t.Fatalf("width %d: labels must be equally long and two blanks apart: %q", width, out)
			}
		}
	}
}

func TestRenderAxis_ColourOffAndAsciiAreEscapeFreeAndSevenBit(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	out := renderAxis(hourAxis(), 8, 80)

	if len(timeLabelRE.FindAllString(out, -1)) == 0 || strings.Contains(out, "\x1b") || !isASCII(out) {
		t.Errorf("want plain 7-bit text, got %q", out)
	}
}

func TestRenderAxis_LabelsUseTheMutedRoleAndBlanksCarryNoStyle(t *testing.T) {
	useColourOn(t)

	out := renderAxis(hourAxis(), 8, 26)

	styles := cellStyles(out)
	if len(styles) != 26 {
		t.Fatalf("want 26 styled cells, got %d: %q", len(styles), out)
	}
	if styles[9] != rolePrefix(tui.RoleMuted) || styles[17] != rolePrefix(tui.RoleMuted) {
		t.Fatalf("want label cells 9 and 17 in the muted role, got %q and %q", styles[9], styles[17])
	}
	for i, st := range styles {
		inLabel := (i >= 9 && i < 14) || (i >= 17 && i < 22)
		switch {
		case inLabel && st != rolePrefix(tui.RoleMuted):
			t.Errorf("cell %d: want the muted role, got %q", i, st)
		case !inLabel && st != "":
			t.Errorf("cell %d is blank and must carry no style, got %q", i, st)
		}
	}
}
