package gantt_test

// Flat shades, per-lane shades and cell precedence. Plot cells are ten wide on a
// 100 s axis, so cell n covers seconds 10n to 10n+10.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/gantt"
)

// useColourUnicode forces true colour with Unicode glyphs and restores everything afterwards.
func useColourUnicode(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	restore := tui.SetRenderMode(tui.RenderMode{Color: true, Glyphs: tui.GlyphUnicode})
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		restore()
		lipgloss.SetColorProfile(prev)
	})
}

func waitShade(from, to float64) gantt.Shade {
	return gantt.Shade{Start: at(from), End: at(to), Role: tui.RoleShadeNeutral, Flat: true, ASCII: '-'}
}

func gapShade(from, to float64) gantt.Shade {
	return gantt.Shade{Start: at(from), End: at(to), Role: tui.RoleShadeCool, Flat: true, ASCII: '_'}
}

// plotString is the plot area of a lane line without styling.
func plotString(line string) string { return string(plotOf(stripANSI(line), lw)) }

func TestFlatShade_AsciiModeDrawsItsOwnPlainCharacter(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{Label: "ab"}

	wait := plotString(lineOf(axisOf(100), lane, waitShade(30, 60)))
	gap := plotString(lineOf(axisOf(100), lane, gapShade(30, 60)))
	dflt := plotString(lineOf(axisOf(100), lane, gantt.Shade{Start: at(30), End: at(60), Flat: true}))

	if wait != "   ---    " {
		t.Errorf("flat shade with ASCII '-': got %q", wait)
	}
	if gap != "   ___    " {
		t.Errorf("flat shade with ASCII '_': got %q", gap)
	}
	if dflt != "   ...    " {
		t.Errorf("flat shade without an ASCII character draws '.': got %q", dflt)
	}
}

func TestFlatShade_UnicodeModeDrawsBlankCellsInTheRoleBackground(t *testing.T) {
	useColourUnicode(t)
	lane := gantt.Lane{Label: "ab"}

	line := lineOf(axisOf(100), lane, waitShade(30, 60))

	if got := plotString(line); got != "          " {
		t.Errorf("a flat shade draws blank cells, got %q", got)
	}
	prefix := rolePrefix(tui.RoleShadeNeutral)
	if !strings.Contains(prefix, "48;") {
		t.Fatalf("the neutral shade role must set a background colour, its style prefix is %q", prefix)
	}
	for col := 3; col <= 5; col++ {
		if got := styleAt(line, lw+1+col); got != prefix {
			t.Errorf("shaded cell %d carries style %q, want the shade role %q", col, got, prefix)
		}
	}
	for _, col := range []int{2, 6} {
		if got := styleAt(line, lw+1+col); got == prefix {
			t.Errorf("cell %d lies outside the shade and must not carry its style", col)
		}
	}
}

func TestFlatShade_NeverUsesTheDottedGlyph(t *testing.T) {
	useColourUnicode(t)

	line := lineOf(axisOf(100), gantt.Lane{Label: "ab"}, waitShade(0, 100))

	if strings.ContainsRune(stripANSI(line), '░') {
		t.Errorf("a flat shade must not draw the dotted glyph: %q", stripANSI(line))
	}
}

func TestShade_WithoutFlatKeepsTheDottedGlyph(t *testing.T) {
	useMode(t, tui.GlyphUnicode)

	got := plotString(lineOf(axisOf(100), gantt.Lane{Label: "ab"}, gantt.Shade{Start: at(30), End: at(60)}))

	if got != "   ░░░    " {
		t.Errorf("shade without Flat: got %q", got)
	}
}

func TestFlatShadeRoles_AreBackgroundsDistinctFromEachOtherAndFromBars(t *testing.T) {
	useColourUnicode(t)
	neutral, cool := rolePrefix(tui.RoleShadeNeutral), rolePrefix(tui.RoleShadeCool)

	for name, p := range map[string]string{"neutral": neutral, "cool": cool} {
		if !strings.Contains(p, "48;") {
			t.Errorf("the %s shade role must set a background, style prefix %q", name, p)
		}
	}
	if neutral == cool {
		t.Errorf("the two shade roles must look different, both are %q", neutral)
	}
	for _, bar := range []tui.StyleRole{tui.RoleSignalGood, tui.RoleSignalWarning, tui.RoleSignalBad,
		tui.RoleSignalMuted, tui.RoleDataActor, tui.RoleDataTool, tui.RoleBody} {
		if p := rolePrefix(bar); p == neutral || p == cool {
			t.Errorf("shade role equals the bar role %q (%q)", bar, p)
		}
	}
}

func TestLaneShade_IsDrawnInItsOwnLaneOnly(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	waiting := gantt.Lane{Label: "ab", Shades: []gantt.Shade{waitShade(30, 60)}}
	other := gantt.Lane{Label: "cd"}

	if got := plotString(lineOf(axisOf(100), waiting)); got != "   ---    " {
		t.Errorf("the lane with the shade: got %q", got)
	}
	if got := plotString(lineOf(axisOf(100), other)); got != strings.Repeat(" ", 10) {
		t.Errorf("a lane without shades must stay blank, got %q", got)
	}
}

func TestLaneShade_OverTheLanesOwnBar_ReplacesTheBarAndTheBarContinuesOnBothSides(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{Label: "ab", Intervals: []gantt.Interval{span(20, 80)}, Shades: []gantt.Shade{waitShade(40, 60)}}

	got := plotString(lineOf(axisOf(100), lane))

	if got != "  ==--==  " {
		t.Errorf("bar 20..80 with a lane shade 40..60: got %q", got)
	}
}

func TestLaneShade_OverTheBar_InUnicodeColourShowsTheShadeStyleInPlaceOfTheBar(t *testing.T) {
	useColourUnicode(t)
	lane := gantt.Lane{Label: "ab", Intervals: []gantt.Interval{roled(span(20, 80), tui.RoleSignalGood)},
		Shades: []gantt.Shade{waitShade(40, 60)}}

	line := lineOf(axisOf(100), lane)

	if got := plotString(line); got != "  ██  ██  " {
		t.Errorf("cells 4 and 5 are the shade (blank), cells 2, 3, 6, 7 stay bars: got %q", got)
	}
	shade, bar := rolePrefix(tui.RoleShadeNeutral), rolePrefix(tui.RoleSignalGood)
	for _, col := range []int{4, 5} {
		if got := styleAt(line, lw+1+col); got != shade {
			t.Errorf("cell %d must carry the shade style %q, got %q", col, shade, got)
		}
	}
	for _, col := range []int{2, 3, 6, 7} {
		if got := styleAt(line, lw+1+col); got != bar {
			t.Errorf("cell %d must keep the bar style %q, got %q", col, bar, got)
		}
	}
}

func TestLaneShade_IsDrawnUnderPoints(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{Label: "ab", Shades: []gantt.Shade{waitShade(40, 60)},
		Points: []gantt.Point{{At: at(50), Kind: chart.MarkerSecondary}}}

	got := plotString(lineOf(axisOf(100), lane))

	if got != "    -:    " {
		t.Errorf("point over a lane shade: got %q", got)
	}
}

func TestLaneShade_WinsOverRenderWideShade(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{Label: "ab", Shades: []gantt.Shade{waitShade(40, 60)}}

	got := plotString(lineOf(axisOf(100), lane, gantt.Shade{Start: at(30), End: at(70)}))

	if got != "   .--.   " {
		t.Errorf("lane shade inside a render-wide shade: got %q", got)
	}
}

func TestRenderWideFlatShade_IsDrawnInEveryLaneWhereNothingCoversTheCell(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	bar := gantt.Lane{Label: "ab", Intervals: []gantt.Interval{span(0, 40)}}
	empty := gantt.Lane{Label: "cd"}
	shade := gapShade(30, 70)

	if got := plotString(lineOf(axisOf(100), bar, shade)); got != "====___   " {
		t.Errorf("render-wide flat shade next to a bar: got %q", got)
	}
	if got := plotString(lineOf(axisOf(100), empty, shade)); got != "   ____   " {
		t.Errorf("render-wide flat shade in an empty lane: got %q", got)
	}
}

func TestLaneShade_OutsideTheAxisIsNotDrawn(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{Label: "ab", Shades: []gantt.Shade{waitShade(150, 200)}}

	if got := plotString(lineOf(axisOf(100), lane)); got != strings.Repeat(" ", 10) {
		t.Errorf("a lane shade after the axis end draws nothing, got %q", got)
	}
}

func TestLaneShade_OverAnOpenBarKeepsTheOpenEnd(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lane := gantt.Lane{Label: "ab", Intervals: []gantt.Interval{{Start: at(20), Open: true}},
		Shades: []gantt.Shade{waitShade(40, 60)}}

	got := plotString(lineOf(axisOf(100), lane))

	if got != "  ==--===>" {
		t.Errorf("open bar with a lane shade: got %q", got)
	}
}
