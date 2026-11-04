package screens_test

// The ? overlay of the Session screen describes the columns of both its tables:
// the per-call table and the tools table each have a group of their own, with one
// sentence for every column title.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// sessLegendGroup is the legend group of the overlay with the title, or fails.
func sessLegendGroup(t *testing.T, overlay, title string) columnLegendGroup {
	t.Helper()
	var titles []string
	for _, g := range columnGroups(overlayGroupsOf(overlay)) {
		if g.title == title {
			return g
		}
		titles = append(titles, g.title)
	}
	t.Fatalf("the overlay has no legend group %q; its column groups are %q\n%s", title, titles, stripStyling(overlay))
	return columnLegendGroup{}
}

func TestSessionFindingsLegend_EachTableHasItsOwnColumnsGroup(t *testing.T) {
	colourOff(t)
	s := sessOpenFrom(t, sessionFindingsRun(), "builder#1", screens.ScreenContext, headerW, legendOverlayH)
	overlay := openOverlay(s, headerW, legendOverlayH)

	sessLegendGroup(t, overlay, "Columns: per-call table")
	sessLegendGroup(t, overlay, "Columns: tools")
}

func TestSessionFindingsLegend_EveryPerCallColumnHasExactlyOneEntryWithASentence(t *testing.T) {
	colourOff(t)
	s := sessOpenFrom(t, sessionFindingsRun(), "builder#1", screens.ScreenContext, headerW, legendOverlayH)
	titles := readHeader(t, s.View(headerW, legendOverlayH), "time").titles()
	group := sessLegendGroup(t, openOverlay(s, headerW, legendOverlayH), "Columns: per-call table")

	for _, title := range titles {
		var found []columnLegendEntry
		for _, e := range group.entries {
			if e.key == title {
				found = append(found, e)
			}
		}
		if len(found) != 1 {
			t.Errorf("the per-call legend has %d entries for the column %q, want exactly 1", len(found), title)
		} else if len(strings.Fields(found[0].desc)) < 2 {
			t.Errorf("the legend entry %q has no sentence as description: %q", title, found[0].desc)
		}
	}
}

func TestSessionFindingsLegend_EveryToolsColumnHasExactlyOneEntryWithASentence(t *testing.T) {
	colourOff(t)
	s := sessOpenFrom(t, sessionFindingsRun(), "builder#1", screens.ScreenContext, headerW, legendOverlayH)
	titles := sessToolsBlock(t, s.View(headerW, legendOverlayH)).titles()
	group := sessLegendGroup(t, openOverlay(s, headerW, legendOverlayH), "Columns: tools")

	for _, title := range titles {
		var found []columnLegendEntry
		for _, e := range group.entries {
			if e.key == title {
				found = append(found, e)
			}
		}
		if len(found) != 1 {
			t.Errorf("the tools legend has %d entries for the column %q, want exactly 1", len(found), title)
		} else if len(strings.Fields(found[0].desc)) < 2 {
			t.Errorf("the legend entry %q has no sentence as description: %q", title, found[0].desc)
		}
	}
}

func TestSessionFindingsLegend_ActorWithoutToolsStillDescribesTheToolsColumns(t *testing.T) {
	colourOff(t)
	s := sessOpenFrom(t, sessionFindingsRun(), "reviewer#1", screens.ScreenContext, headerW, legendOverlayH)

	group := sessLegendGroup(t, openOverlay(s, headerW, legendOverlayH), "Columns: tools")

	if len(group.entries) == 0 {
		t.Error("the tools legend is empty for an actor without tools")
	}
}
