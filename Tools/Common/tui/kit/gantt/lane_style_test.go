package gantt_test

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"
)

// plotStyles returns the style prefix of each plot-area cell of a lane line.
func plotStyles(line string) []string {
	out := make([]string, lineW-lw-1)
	for i := range out {
		out[i] = styleAt(line, lw+1+i)
	}
	return out
}

func TestRenderLane_ColourPerIntervalRole(t *testing.T) {
	useColourOn(t)
	lane := intervals(
		roled(span(12, 34), tui.RoleSignalGood),
		roled(span(62, 84), tui.RoleSignalBad),
		span(42, 44),
	)

	styles := plotStyles(lineOf(axisOf(100), lane))

	for col, role := range map[int]tui.StyleRole{1: tui.RoleSignalGood, 3: tui.RoleSignalGood, 6: tui.RoleSignalBad, 8: tui.RoleSignalBad, 4: tui.RoleBody} {
		if styles[col] != rolePrefix(role) {
			t.Errorf("cell %d: want role %q, got style %q", col, role, styles[col])
		}
	}
}

func TestRenderLane_ShadeAndPointCarryTheirRoles(t *testing.T) {
	useColourOn(t)
	lane := gantt.Lane{Label: "ab", Points: []gantt.Point{{At: at(43), Kind: chart.MarkerPrimary, Role: tui.RoleSignalGood}}}
	shade := gantt.Shade{Start: at(31), End: at(54), Role: tui.RoleSignalWarning}

	styles := plotStyles(lineOf(axisOf(100), lane, shade))

	if styles[3] != rolePrefix(tui.RoleSignalWarning) || styles[5] != rolePrefix(tui.RoleSignalWarning) {
		t.Errorf("shaded cells must carry the shade role, got %q and %q", styles[3], styles[5])
	}
	if styles[4] != rolePrefix(tui.RoleSignalGood) {
		t.Errorf("the point cell must carry the point role, got %q", styles[4])
	}
}

func TestRenderLane_LabelRoleAndSelection(t *testing.T) {
	useColourOn(t)
	lane := gantt.Lane{Label: "ab", LabelRole: tui.RoleDataActor}

	plain := gantt.RenderLane(axisOf(100), lane, nil, lw, lineW, false, theme())
	picked := gantt.RenderLane(axisOf(100), lane, nil, lw, lineW, true, theme())

	if got := styleAt(plain, 0); got != rolePrefix(tui.RoleDataActor) {
		t.Errorf("unselected label: want the label role, got %q", got)
	}
	if got := styleAt(picked, 0); got != rolePrefix(tui.RoleSelected) {
		t.Errorf("selected label: want the selected role, got %q", got)
	}
	if stripANSI(plain) != stripANSI(picked) {
		t.Errorf("selection changes the label style only: %q vs %q", stripANSI(plain), stripANSI(picked))
	}
}

func TestRenderLane_LabelWithoutRoleUsesBodyStyle(t *testing.T) {
	useColourOn(t)

	out := gantt.RenderLane(axisOf(100), gantt.Lane{Label: "ab"}, nil, lw, lineW, false, theme())

	if got := styleAt(out, 0); got != rolePrefix(tui.RoleBody) {
		t.Errorf("want body style, got %q", got)
	}
}

func TestRenderLane_LabelIsPaddedOrTruncatedToTheLabelColumn(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	render := func(label string, labelWidth int) string {
		return gantt.RenderLane(axisOf(100), gantt.Lane{Label: label}, nil, labelWidth, 20, false, theme())
	}
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{"short label is padded", "ab", "ab     "},
		{"empty label is blank", "", "       "},
		{"exact fit keeps the label", "abcdef", "abcdef "},
		{"long label ends with the ellipsis", "long-label-name", "lon... "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if out := render(tc.label, 6); !strings.HasPrefix(out, tc.want) {
				t.Errorf("want prefix %q, got %q", tc.want, out)
			}
		})
	}
}

func TestRenderLane_EmptyLaneIsBlankButFullWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	out := lineOf(axisOf(100), gantt.Lane{Label: "ab"})

	if out != "ab"+strings.Repeat(" ", lineW-2) {
		t.Errorf("want the label then blanks to width %d, got %q", lineW, out)
	}
}

func TestRenderLane_NeverExceedsWidthAndFillsItWhenThePlotFits(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	rich := gantt.Lane{
		Label:     "worker-long-name",
		Intervals: []gantt.Interval{span(3, 50), {Start: at(60), Open: true}, span(20, 20)},
		Points:    []gantt.Point{{At: at(10), Kind: chart.MarkerSecondary}, {At: at(99), Kind: chart.MarkerTertiary}},
	}
	shades := []gantt.Shade{{Start: at(0), End: at(100)}}

	for _, labelWidth := range []int{0, 3, 10} {
		for width := -2; width <= 60; width++ {
			out := gantt.RenderLane(axisOf(100), rich, shades, labelWidth, width, false, theme())
			switch {
			case width <= 0 && out != "":
				t.Fatalf("labelWidth %d width %d: want empty output, got %q", labelWidth, width, out)
			case strings.Contains(out, "\n") || runeCount(out) > max(width, 0):
				t.Fatalf("labelWidth %d width %d: line is %d cells: %q", labelWidth, width, runeCount(out), out)
			case width >= labelWidth+1 && runeCount(out) != width:
				t.Fatalf("labelWidth %d width %d: want exactly %d cells, got %d: %q", labelWidth, width, width, runeCount(out), out)
			}
		}
	}
}

func TestRenderLane_AsciiModeDrawsOnlySevenBitGlyphs(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{
		Label: "lane",
		Intervals: []gantt.Interval{
			span(3, 30), {Start: at(60), Open: true}, span(45, 45),
		},
		Points: []gantt.Point{
			{At: at(10), Kind: chart.MarkerPrimary}, {At: at(20), Kind: chart.MarkerSecondary}, {At: at(30), Kind: chart.MarkerTertiary},
		},
	}

	out := lineOf(axisOf(100), lane, gantt.Shade{Start: at(30), End: at(60)})

	if runeCount(out) != lineW || len(occupied(plotOf(out, lw))) < 8 || !isASCII(out) || strings.Contains(out, "\x1b") {
		t.Errorf("want plain 7-bit text, got %q", out)
	}
}

func TestRenderLane_BarsAndShadesAreDistinctGlyphsInBothModes(t *testing.T) {
	for _, m := range bothModes {
		t.Run(m.name, func(t *testing.T) {
			useMode(t, m.mode)
			whole := gantt.Shade{Start: at(0), End: at(100)}

			bar := plotFor(intervals(span(0, 100)))[5]
			open := plotFor(intervals(gantt.Interval{Start: at(0), Open: true}))[9]
			shade := plotFor(gantt.Lane{Label: "ab"}, whole)[5]

			if bar == ' ' || open == ' ' || shade == ' ' || bar == open || bar == shade || open == shade {
				t.Errorf("want three distinct visible glyphs, got bar %q open %q shade %q", bar, open, shade)
			}
		})
	}
}
