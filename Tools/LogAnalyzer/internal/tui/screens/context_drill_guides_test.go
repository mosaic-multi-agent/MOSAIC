package screens_test

// The band guides of the drill-down curve: each chart row is coloured with the
// role of the band its value falls in, so the 100k, 250k and 350k boundaries
// show on the chart.

import (
	"regexp"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var lastOpenBeforeMarker = regexp.MustCompile(`(\x1b\[[0-9;]*m)[^\x1b]*` + ctxFirstWriteGlyph)

// ctxGuideRoles drills into an actor with colour on and returns, top to bottom,
// the opening escape sequence of the first-write marker on each chart row (the
// marker spans the full height of the chart, so every chart row has one).
func ctxGuideRoles(t *testing.T, id string, w, h int) []string {
	t.Helper()
	s := ctxDrill(t, contextRun(), id, tableW, tallH)
	colourOn(t)
	var out []string
	for _, l := range strings.Split(s.View(w, h), "\n") {
		if m := lastOpenBeforeMarker.FindStringSubmatch(l); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

// collapse merges consecutive equal entries.
func collapse(in []string) []string {
	var out []string
	for _, v := range in {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
}

func TestContextDrill_CurveRowsAreColouredByBandGuides(t *testing.T) {
	roles := ctxGuideRoles(t, "builder#1", tableW, tallH)

	// The curve of builder#1 reaches 380,000, so all three bands show, from the
	// degraded band at the top down to the optimal band at the bottom.
	colourOn(t)
	bad := rolePrefix(screens.BandRole(domain.BandDegraded))
	warn := rolePrefix(screens.BandRole(domain.BandAboveOptimal))
	good := rolePrefix(screens.BandRole(domain.BandOptimal))
	want := []string{bad, warn, good}
	got := collapse(roles)
	if len(got) != len(want) {
		t.Fatalf("the chart rows show %d band runs from %d rows, want degraded, above optimal, optimal", len(got), len(roles))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("band run %d of the chart (top to bottom) has the wrong role: %q, want %q", i+1, got[i], want[i])
		}
	}
}

func TestContextDrill_GuidesLeaveNoColourWhenColourIsOff(t *testing.T) {
	view := ctxDrillView(t, "builder#1")

	assertContains(t, view, ctxFirstWriteGlyph)
	if strings.Contains(view, "\x1b[") {
		t.Errorf("with colour off the drill-down must carry no escape sequences\n%q", view)
	}
}

func TestContextDrill_GuidedCurveStaysWithinNarrowAndShortTerminals(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {80, 12}, {40, 24}, {40, 8}, {120, 6}} {
		s := ctxDrill(t, contextRun(), "builder#1", tableW, tallH)
		colourOn(t)
		view := s.View(size[0], size[1])

		if strings.TrimSpace(stripStyling(view)) == "" {
			t.Errorf("%dx%d: the drill-down drew nothing", size[0], size[1])
		}
		assertWithinBounds(t, view, size[0], size[1])
	}
}
