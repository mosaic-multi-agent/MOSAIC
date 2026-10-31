package gantt_test

// Swatches drawn exactly like the shaded cells and markers of a lane, and the
// packing of legend entries into lines.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"
	"mosaic-common/tui/kit/textwidth"
)

// laneCell is plot cell col of a one-lane render, with its glyph and style.
func laneCell(lane gantt.Lane, shades []gantt.Shade, col int) (glyph, style string) {
	line := gantt.RenderLane(axisOf(100), lane, shades, lw, lineW, false, theme())
	runes := []rune(stripANSI(line))
	return string(runes[lw+1+col]), styleAt(line, lw+1+col)
}

func TestShadeSwatch_IsOneCellDrawnLikeTheShadeInBothModes(t *testing.T) {
	for _, m := range bothModes {
		for name, shade := range map[string]gantt.Shade{"wait": waitShade(0, 100), "gap": gapShade(0, 100),
			"dotted": {Start: at(0), End: at(100), Role: tui.RoleSignalMuted}} {
			t.Run(m.name+"/"+name, func(t *testing.T) {
				useColourFor(t, m.mode)
				glyph, style := laneCell(gantt.Lane{Label: "ab"}, []gantt.Shade{shade}, 4)

				swatch := gantt.ShadeSwatch(shade, theme())

				if got := stripANSI(swatch); got != glyph || textwidth.Of(got) != 1 {
					t.Errorf("swatch shows %q, want the single cell %q the shade draws", got, glyph)
				}
				if got := styleAt(swatch, 0); got != style {
					t.Errorf("swatch style %q differs from the shaded cell style %q", got, style)
				}
			})
		}
	}
}

func TestShadeSwatch_TwoShadesLookDifferent(t *testing.T) {
	for _, m := range bothModes {
		t.Run(m.name, func(t *testing.T) {
			useColourFor(t, m.mode)

			wait := gantt.ShadeSwatch(waitShade(0, 1), theme())
			gap := gantt.ShadeSwatch(gapShade(0, 1), theme())

			if wait == "" || gap == "" || wait == gap {
				t.Errorf("the swatches must be non-empty and different: %q and %q", wait, gap)
			}
		})
	}
}

func TestPointSwatch_IsOneCellDrawnLikeThePointInBothModes(t *testing.T) {
	for _, m := range bothModes {
		for _, kind := range []chart.MarkerKind{chart.MarkerPrimary, chart.MarkerSecondary, chart.MarkerTertiary} {
			t.Run(m.name, func(t *testing.T) {
				useColourFor(t, m.mode)
				p := gantt.Point{At: at(45), Kind: kind, Role: tui.RoleSignalWarning}
				glyph, style := laneCell(gantt.Lane{Label: "ab", Points: []gantt.Point{p}}, nil, 4)

				swatch := gantt.PointSwatch(p, theme())

				if got := stripANSI(swatch); got != glyph || textwidth.Of(got) != 1 {
					t.Errorf("point swatch shows %q, want the single cell %q the point draws", got, glyph)
				}
				if got := styleAt(swatch, 0); got != style {
					t.Errorf("point swatch style %q differs from the point style %q", got, style)
				}
			})
		}
	}
}

// useColourFor forces colour with the given glyph mode.
func useColourFor(t *testing.T, g tui.GlyphMode) {
	t.Helper()
	if g == tui.GlyphASCII {
		useColourOn(t)
		return
	}
	useColourUnicode(t)
}

func entries(labels ...string) []gantt.LegendEntry {
	out := make([]gantt.LegendEntry, len(labels))
	for i, l := range labels {
		out[i] = gantt.LegendEntry{Swatch: "#", Label: l}
	}
	return out
}

func TestRenderLegend_SeparatesEntriesByTwoSpacesOnOneLine(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := gantt.RenderLegend(entries("waiting", "between"), 40, theme())

	if len(got) != 1 || stripANSI(got[0]) != "# waiting  # between" {
		t.Errorf("got %q, want one line %q", got, "# waiting  # between")
	}
}

func TestRenderLegend_WrapsOnlyBetweenEntries(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	all := entries("waiting on the user", "between sessions", "compaction")

	cases := []struct {
		width int
		want  []string
	}{
		{width: 60, want: []string{"# waiting on the user  # between sessions  # compaction"}},
		{width: 45, want: []string{"# waiting on the user  # between sessions", "# compaction"}},
		{width: 32, want: []string{"# waiting on the user", "# between sessions  # compaction"}},
		{width: 21, want: []string{"# waiting on the user", "# between sessions", "# compaction"}},
	}
	for _, tc := range cases {
		got := gantt.RenderLegend(all, tc.width, theme())

		var plain []string
		for _, l := range got {
			plain = append(plain, stripANSI(l))
		}
		if strings.Join(plain, "|") != strings.Join(tc.want, "|") {
			t.Errorf("width %d: got %q, want %q", tc.width, plain, tc.want)
		}
	}
}

func TestRenderLegend_NoLineIsWiderThanTheWidthEvenForATooWideEntry(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := gantt.RenderLegend(entries("a very long label that cannot fit", "short"), 12, theme())

	if len(got) < 2 {
		t.Fatalf("a too wide entry takes a line of its own and the next entry another: %q", got)
	}
	for _, l := range got {
		if w := textwidth.Of(stripANSI(l)); w > 12 || w == 0 {
			t.Errorf("line %q is %d cells, want 1..12", stripANSI(l), w)
		}
	}
	if !strings.HasPrefix(stripANSI(got[0]), "# a very") {
		t.Errorf("the cut entry keeps its swatch and the start of its label: %q", stripANSI(got[0]))
	}
}

func TestRenderLegend_EmptyInputOrNoWidthGivesNoLines(t *testing.T) {
	if got := gantt.RenderLegend(nil, 40, theme()); len(got) != 0 {
		t.Errorf("no entries: got %q", got)
	}
	if got := gantt.RenderLegend(entries("a"), 0, theme()); len(got) != 0 {
		t.Errorf("zero width: got %q", got)
	}
}

func TestRenderLegend_KeepsTheStyledSwatchOfEachEntry(t *testing.T) {
	useColourUnicode(t)
	swatch := gantt.ShadeSwatch(waitShade(0, 1), theme())

	got := gantt.RenderLegend([]gantt.LegendEntry{{Swatch: swatch, Label: "waiting"}}, 40, theme())

	if len(got) != 1 || !strings.Contains(got[0], swatch) {
		t.Errorf("the legend line must contain the swatch exactly as given: %q", got)
	}
}
