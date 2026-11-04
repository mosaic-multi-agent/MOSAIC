package screens_test

// Grouping of the Gantt lanes by agent type, and the keys of the timeline: they
// must not collide with existing keys and must appear in the help line and the
// ? overlay.

import (
	"regexp"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

var groupTypes = map[string]string{"b-1": "builder", "b-2": "builder", "r-1": "reviewer", "r-2": "reviewer", "u-1": screens.UnknownTypeLabel}

func TestGanttGroup_KeyGroupsLanesByAgentTypeInByTypeOrderAndBackAgain(t *testing.T) {
	s := openTimeline(t, groupRun(), wideW, tallH)

	dispatchOrder := s.View(wideW, tallH)
	assertLaneOrder(t, dispatchOrder, groupTypes, "b-1", "r-1", "u-1", "b-2", "r-2")

	press(s, screens.KeyGroup)
	grouped := s.View(wideW, tallH)
	// reviewer first (as in By agent type), dispatch order within a type, unknown type last
	assertLaneOrder(t, grouped, groupTypes, "r-1", "r-2", "b-1", "b-2", "u-1")

	press(s, screens.KeyGroup)
	if back := s.View(wideW, tallH); back != dispatchOrder {
		t.Errorf("a second g must return to dispatch order\n%s\nvs\n%s", back, dispatchOrder)
	}
}

func TestGanttGroup_KeepsEveryLaneAndTheLineCount(t *testing.T) {
	s := openTimeline(t, groupRun(), wideW, tallH)
	before := s.View(wideW, tallH)

	press(s, screens.KeyGroup)
	after := s.View(wideW, tallH)

	for id, typ := range groupTypes {
		if laneIndex(after, id, typ) < 0 {
			t.Errorf("grouping must keep the lane of %s\n%s", id, stripStyling(after))
		}
	}
	if lineCount(before) != lineCount(after) {
		t.Errorf("grouping reorders lanes only: %d lines vs %d", lineCount(after), lineCount(before))
	}
	assertWithinBounds(t, after, wideW, tallH)
}

func TestGanttGroup_StripFollowsTheCursorInTheGroupedOrder(t *testing.T) {
	s := openTimeline(t, groupRun(), wideW, tallH)
	press(s, screens.KeyGroup)
	press(s, repeat("up", 10)...) // the first lane, wherever the cursor was
	press(s, "down")

	// id and type on strip line 1, cost on strip line 3 (line 2 is not constrained here)
	stripBlock(t, s.View(wideW, tallH), "r-2", "reviewer", "", "$4.00")
}

func TestTimelineKeys_DoNotCollideWithExistingKeys(t *testing.T) {
	if screens.KeyGroup != "g" || screens.KeyToggleView != "v" {
		t.Errorf("KeyGroup = %q, KeyToggleView = %q, want g and v", screens.KeyGroup, screens.KeyToggleView)
	}
	taken := []string{
		screens.KeyViewDispatches, screens.KeyViewOverview, screens.KeyViewAgentTypes, screens.KeyViewStatus,
		screens.KeyViewTools, screens.KeyViewTimeline, screens.KeyAcrossRuns, screens.KeyHelp, screens.KeySort,
		screens.KeyMarkUserTool, screens.KeyExportRun, screens.KeyExportAll,
		"up", "down", "j", "k", "enter", "esc", "q", "s", "p",
	}
	for _, k := range taken {
		if k == screens.KeyGroup || k == screens.KeyToggleView {
			t.Errorf("the timeline key %q collides with an existing key", k)
		}
	}
}

func TestTimelineKeys_AreListedInTheHelpLineAndTheOverlay(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)

	for _, k := range []string{screens.KeyGroup, screens.KeyToggleView} {
		if !regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(k) + `\s`).MatchString(s.Help()) {
			t.Errorf("help line %q must list the key %q as its own entry", s.Help(), k)
		}
	}
	overlay := stripStyling(openOverlay(s, wideW, tallH))
	if !regexp.MustCompile(`(?mi)^\s+` + screens.KeyGroup + `\s+.*group`).MatchString(overlay) {
		t.Errorf("the overlay must list %q with its purpose (grouping)\n%s", screens.KeyGroup, overlay)
	}
	if !regexp.MustCompile(`(?mi)^\s+` + screens.KeyToggleView + `\s+.*list`).MatchString(overlay) {
		t.Errorf("the overlay must list %q with its purpose (the list version)\n%s", screens.KeyToggleView, overlay)
	}
}

func TestTimelineKeys_ListVersionHelpListsTheSortKey(t *testing.T) {
	s := openTimeline(t, timelineRun(), wideW, tallH)
	press(s, screens.KeyToggleView)

	overlay := stripStyling(openOverlay(s, wideW, tallH))

	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeySort) + `\s+\S`).MatchString(overlay) {
		t.Errorf("the list version's overlay must list the sort key %q\n%s", screens.KeySort, overlay)
	}
}
