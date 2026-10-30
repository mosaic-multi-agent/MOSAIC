package screens_test

// Behaviour of the drill-down of one actor in the Context view: the curve with
// its markers and the band names as text, the way back to the table, and the
// bounds at narrow and short terminals.

import (
	"strings"
	"testing"
)

// Marker glyphs the curve draws in Unicode mode with colour off: first write
// (primary), logged compaction (secondary) and detected compaction (tertiary).
const (
	ctxFirstWriteGlyph = "┃"
	ctxLoggedGlyph     = "│"
	ctxDetectedGlyph   = "┊"
	ctxPointGlyph      = "•"
)

// ctxDrillView drills into an actor of the standard run and renders the result.
func ctxDrillView(t *testing.T, id string) string {
	t.Helper()
	return ctxDrill(t, contextRun(), id, tableW, tallH).View(tableW, tallH)
}

func TestContextDrill_ShowsOnlyTheSelectedActor(t *testing.T) {
	view := ctxDrillView(t, "builder#1")

	assertContains(t, stripStyling(view), "builder#1")
	assertNotContains(t, stripStyling(view), "reviewer#1", "coder#1", "tester#1", "ghost#1")
}

func TestContextDrill_CurveMarksTheFirstWriteOfAnActorWithoutCompactions(t *testing.T) {
	view := stripStyling(ctxDrillView(t, "builder#1"))

	assertContains(t, view, ctxFirstWriteGlyph)
	assertNotContains(t, view, ctxLoggedGlyph, ctxDetectedGlyph)
}

func TestContextDrill_CurveMarksDetectedCompactionsWithoutAFirstWrite(t *testing.T) {
	view := stripStyling(ctxDrillView(t, "coder#1"))

	assertContains(t, view, ctxDetectedGlyph)
	assertNotContains(t, view, ctxFirstWriteGlyph, ctxLoggedGlyph)
}

func TestContextDrill_LoggedAndDetectedCompactionsHaveDistinctMarkers(t *testing.T) {
	view := stripStyling(ctxDrillView(t, "orchestrator"))

	assertContains(t, view, ctxLoggedGlyph, ctxDetectedGlyph)
	assertNotContains(t, view, ctxFirstWriteGlyph)
}

func TestContextDrill_CurveStaysUsefulWhenNoFirstWriteWasFound(t *testing.T) {
	view := stripStyling(ctxDrillView(t, "coder#1"))

	// five calls, one of them under the compaction marker.
	if n := strings.Count(view, ctxPointGlyph); n < 3 {
		t.Errorf("the curve draws %d points, want the context of the calls drawn as a curve\n%s", n, view)
	}
}

func TestContextDrill_NamesTheBandOfPeakAndFinalAsText(t *testing.T) {
	view := strings.ToLower(stripStyling(ctxDrillView(t, "coder#1")))

	if lineWithAll(view, "peak", "above optimal") == "" {
		t.Errorf("the peak (300,000) must be named with its band, above optimal\n%s", view)
	}
	if lineWithAll(view, "final", "optimal") == "" || strings.Contains(view, "below optimal") {
		t.Errorf("the final context (80,000) must be named with its band, optimal\n%s", view)
	}
}

func TestContextDrill_NamesTheDegradedBand(t *testing.T) {
	view := strings.ToLower(stripStyling(ctxDrillView(t, "builder#1")))

	if lineWithAll(view, "peak", "degraded") == "" || lineWithAll(view, "final", "degraded") == "" {
		t.Errorf("peak and final (380,000 and 360,000) must be named with the degraded band\n%s", view)
	}
}

func TestContextDrill_EscReturnsToTheTableThenLeavesTheView(t *testing.T) {
	s := ctxDrill(t, contextRun(), "builder#1", tableW, tallH)

	press(s, "esc")
	if s.Back() {
		t.Error("the first esc must close the drill-down, not leave the per-run views")
	}
	table := stripStyling(s.View(tableW, tallH))
	assertContains(t, table, "builder#1", "coder#1", "reviewer#1")
	assertNotContains(t, table, ctxCallLabels(70)[0])

	press(s, "esc")
	if !s.Back() {
		t.Error("the second esc must leave the per-run views")
	}
}

func TestContextDrill_AnInvocationWithoutCallsStillRenders(t *testing.T) {
	view := ctxDrillView(t, "ghost#1")

	assertContains(t, stripStyling(view), "ghost#1")
	assertWithinBounds(t, view, tableW, tallH)
}

func TestContextDrill_StaysWithinNarrowAndShortTerminals(t *testing.T) {
	for _, id := range []string{"orchestrator", "builder#1", "reviewer#1", "ghost#1"} {
		s := ctxDrill(t, contextRun(), id, tableW, tallH)
		for _, size := range [][2]int{{80, 24}, {80, 12}, {40, 24}, {40, 8}, {120, 6}} {
			view := s.View(size[0], size[1])

			if strings.TrimSpace(view) == "" {
				t.Errorf("%s at %dx%d: the drill-down drew nothing", id, size[0], size[1])
			}
			assertWithinBounds(t, view, size[0], size[1])
		}
	}
}

func TestContextDrill_NamesTheLowestBandOptimalNeverBelowOptimal(t *testing.T) {
	view := strings.ToLower(stripStyling(ctxDrillView(t, "tester#1")))

	if lineWithAll(view, "peak", "optimal") == "" || strings.Contains(view, "below optimal") {
		t.Errorf("the peak of tester#1 (95,000) must be named optimal and no figure below optimal\n%s", view)
	}
}
