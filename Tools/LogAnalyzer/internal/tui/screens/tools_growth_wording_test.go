package screens_test

// The growth figures measure the tool result only: the titles say so, and the note
// and the legends state that the tool input is counted in the calling call's output
// tokens. The growth values and the JSON field names are unchanged.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	resultOnlyPhrase  = "tool result only"
	inputInOutputText = "calling call's output tokens"
)

func TestToolsGrowthWording_ColumnTitlesAndGroupNameTheToolResult(t *testing.T) {
	view := growthView(t, growthRun(), tableW)

	titles := growthTitles(view)
	for _, want := range []string{"average result growth", "minimum result growth", "maximum result growth", "result growth samples", "negative result growth"} {
		if !hasTitle(titles, want) {
			t.Errorf("no column titled %q; titles are %q", want, titles)
		}
	}
	for _, old := range []string{"average growth", "minimum growth", "maximum growth", "growth samples", "negative growth"} {
		if hasTitle(titles, old) {
			t.Errorf("the old title %q is still shown; titles are %q", old, titles)
		}
	}
	assertContains(t, stripStyling(view), "tool result growth")
	assertNotContains(t, stripStyling(view), "context growth")
}

func TestToolsGrowthWording_NoteStatesToolResultOnlyAndInputInOutputTokens(t *testing.T) {
	for _, w := range growthWidths {
		view := growthView(t, growthRun(), w)

		assertContains(t, flatText(view), resultOnlyPhrase, inputInOutputText)
	}
}

func TestToolsGrowthWording_ToolsLegendStatesBothFacts(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(growthRun(), headerW, legendOverlayH, paletteStyles())
	overlay := openOverlay(s, headerW, legendOverlayH)

	desc := legendEntryText(t, overlay, "average result growth", "minimum result growth")

	assertContains(t, desc, resultOnlyPhrase, inputInOutputText)
	assertContains(t, flatText(overlay), "Columns: tool result growth")
}

func TestToolsGrowthWording_SessionToolsLegendStatesBothFacts(t *testing.T) {
	colourOff(t)
	s := sessOpenFrom(t, sessionRun(), "builder#1", screens.ScreenContext, headerW, legendOverlayH)
	overlay := openOverlay(s, headerW, legendOverlayH)

	desc := legendEntryText(t, overlay, "average result growth", "minimum result growth")

	assertContains(t, desc, resultOnlyPhrase, inputInOutputText)
}

func TestToolsGrowthWording_SessionPerCallColumnAndLegendNameTheToolResult(t *testing.T) {
	colourOff(t)
	s := sessOpenFrom(t, sessionRun(), "builder#1", screens.ScreenContext, headerW, legendOverlayH)
	body := s.View(headerW, legendOverlayH)
	titles := readHeader(t, body, "time").titles()
	overlay := openOverlay(s, headerW, legendOverlayH)

	flat := flatText(overlay)
	start := strings.Index(flat, " requested tools ")
	if start < 0 {
		t.Fatalf("the legend has no entry \"requested tools\"\n%s", flat)
	}
	desc := legendEntryText(t, flat[start:], "result growth", "Columns: tools")

	if !hasTitle(titles, "result growth") || hasTitle(titles, "context growth") {
		t.Errorf("the per-call table must have the column \"result growth\" and not \"context growth\", got %q", titles)
	}
	assertContains(t, desc, resultOnlyPhrase, inputInOutputText)
}
