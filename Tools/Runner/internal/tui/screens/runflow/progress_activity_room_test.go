package runflow

// Room for the live activity panel on the progress screen when the step rows
// overflow the height: the panel keeps its header plus at least one item line,
// grows on tall terminals, the rows keep showing their latest entries, and a
// hidden panel gives the rows the full height back.
//
// Drawn at 120 columns so no help bar or item line wraps.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

const roomRowCount = 40

func roomRowName(i int) string { return fmt.Sprintf("step-%02d#1", i) }

// newRoomScreen is a progress screen of the given height with roomRowCount
// step rows (the last one running), an invocation begun and itemCount tool
// items received.
func newRoomScreen(t *testing.T, height, itemCount int) *ProgressScreen {
	t.Helper()
	s := NewProgressScreen(120, height, progressStyles())
	for i := 1; i < roomRowCount; i++ {
		s.AppendRow(ProgressRow{AgentInstance: roomRowName(i), Phase: "EXECUTION", Status: "SUCCESS"})
	}
	s.AppendRow(ProgressRow{AgentInstance: roomRowName(roomRowCount), Phase: "EXECUTION", Status: "running"})
	s.BeginActivity(1, dispatchInv)
	for i := 0; i < itemCount; i++ {
		s.AppendActivity(1, toolItem("Read", fmt.Sprintf("file%02d.txt", i)))
	}
	return s
}

func viewLineCount(view string) int { return strings.Count(view, "\n") + 1 }

func countLinesWith(view, needle string) int {
	n := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, needle) {
			n++
		}
	}
	return n
}

func TestActivityRoom_OverflowingRows_PanelKeepsHeaderItemAndLatestRow(t *testing.T) {
	s := newRoomScreen(t, 20, 30)

	view := s.View()

	if !strings.Contains(view, "Activity: agent-a#3") {
		t.Errorf("activity header missing from view:\n%s", view)
	}
	if countLinesWith(view, "Read") < 1 {
		t.Errorf("no activity item line in view:\n%s", view)
	}
	if !strings.Contains(view, roomRowName(roomRowCount)) {
		t.Errorf("latest step row %s missing from view:\n%s", roomRowName(roomRowCount), view)
	}
	if got := viewLineCount(view); got > 20 {
		t.Errorf("view has %d lines, want at most the screen height 20", got)
	}
}

func TestActivityRoom_OverflowingRows_PlaceholderStillGetsALine(t *testing.T) {
	s := newRoomScreen(t, 20, 0)

	view := s.View()

	if !strings.Contains(view, "Activity: agent-a#3") || !strings.Contains(view, placeholderText) {
		t.Errorf("header and placeholder expected in view:\n%s", view)
	}
	if !strings.Contains(view, roomRowName(roomRowCount)) {
		t.Errorf("latest step row missing from view:\n%s", view)
	}
	if got := viewLineCount(view); got > 20 {
		t.Errorf("view has %d lines, want at most 20", got)
	}
}

func TestActivityRoom_UsageOnlyInvocation_ShowsHeaderAloneAndLatestRow(t *testing.T) {
	s := newRoomScreen(t, 20, 0)
	s.AppendActivity(1, domain.ActivityItem{Kind: domain.ActivityKindUsage})

	view := s.View()

	if !strings.Contains(view, "Activity: agent-a#3") {
		t.Errorf("activity header missing from view:\n%s", view)
	}
	if strings.Contains(view, placeholderText) {
		t.Errorf("placeholder shown although events were received:\n%s", view)
	}
	if !strings.Contains(view, roomRowName(roomRowCount)) {
		t.Errorf("latest step row missing from view:\n%s", view)
	}
	if got := viewLineCount(view); got > 20 {
		t.Errorf("view has %d lines, want at most 20", got)
	}
}

func TestActivityRoom_HiddenPanel_RowsUseTheFullHeightAgain(t *testing.T) {
	s := newRoomScreen(t, 20, 30)
	shownRows := countLinesWith(s.View(), "step-")

	pressProgressKey(s, "l")
	view := s.View()
	hiddenRows := countLinesWith(view, "step-")

	if hiddenRows <= shownRows {
		t.Errorf("rows with panel hidden = %d, want more than %d with the panel shown", hiddenRows, shownRows)
	}
	if hiddenRows < 20-6 {
		t.Errorf("rows with panel hidden = %d, want at least %d", hiddenRows, 20-6)
	}
	if strings.Contains(view, "Activity:") {
		t.Errorf("panel still rendered while hidden:\n%s", view)
	}
	if !strings.Contains(view, roomRowName(roomRowCount)) {
		t.Errorf("latest step row missing from view:\n%s", view)
	}
	if got := viewLineCount(view); got > 20 {
		t.Errorf("view has %d lines, want at most 20", got)
	}
}

func TestActivityRoom_ShowingThePanelAgain_TakesRoomBackFromTheRows(t *testing.T) {
	s := newRoomScreen(t, 20, 30)
	pressProgressKey(s, "l")
	pressProgressKey(s, "L")

	view := s.View()

	if !strings.Contains(view, "Activity: agent-a#3") || countLinesWith(view, "Read") < 1 {
		t.Errorf("panel header and an item expected after showing it again:\n%s", view)
	}
	if !strings.Contains(view, roomRowName(roomRowCount)) {
		t.Errorf("latest step row missing from view:\n%s", view)
	}
}

func TestActivityRoom_WithStopConfirmBanner_PanelStillKeepsHeaderAndItem(t *testing.T) {
	s := newRoomScreen(t, 24, 30)
	pressProgressKey(s, "s")
	if !s.ConfirmPending() {
		t.Fatal("stop confirmation not pending")
	}

	view := s.View()

	if !strings.Contains(view, "Activity: agent-a#3") {
		t.Errorf("activity header missing from view:\n%s", view)
	}
	if countLinesWith(view, "Read") < 1 {
		t.Errorf("no activity item line in view:\n%s", view)
	}
	if !strings.Contains(view, roomRowName(roomRowCount)) {
		t.Errorf("latest step row missing from view:\n%s", view)
	}
	if got := viewLineCount(view); got > 24 {
		t.Errorf("view has %d lines, want at most the screen height 24", got)
	}
}

func TestActivityRoom_TallTerminal_GivesThePanelMoreItemLinesThanAShortOne(t *testing.T) {
	short := newRoomScreen(t, 20, 60).View()
	tall := newRoomScreen(t, 40, 60).View()

	shortItems := countLinesWith(short, "Read")
	tallItems := countLinesWith(tall, "Read")

	if shortItems < 1 {
		t.Errorf("short terminal shows %d item lines, want at least 1", shortItems)
	}
	if tallItems <= shortItems {
		t.Errorf("item lines: tall terminal %d, short terminal %d; want more on the tall one", tallItems, shortItems)
	}
	if !strings.Contains(tall, roomRowName(roomRowCount)) {
		t.Errorf("latest step row missing on the tall terminal:\n%s", tall)
	}
	if got := viewLineCount(tall); got > 40 {
		t.Errorf("tall view has %d lines, want at most 40", got)
	}
}

func TestActivityRoom_ShowsNewestItemsOfTheInvocation(t *testing.T) {
	s := newRoomScreen(t, 20, 30)

	view := s.View()

	if !strings.Contains(view, "file29.txt") {
		t.Errorf("newest item missing from view:\n%s", view)
	}
	if strings.Contains(view, "file00.txt") {
		t.Errorf("oldest item shown although only the newest fit:\n%s", view)
	}
}

func TestActivityRoom_SmallPanelOnTallScreen_RowsTakeAllLeftoverRoom(t *testing.T) {
	s := newRoomScreen(t, 40, 1)

	view := s.View()

	if got := viewLineCount(view); got != 40 {
		t.Errorf("view has %d lines, want the full height 40 (panel needs 2, rows take the rest)", got)
	}
	if got := countLinesWith(view, "step-"); got != 40-6-2 {
		t.Errorf("step rows shown = %d, want %d", got, 40-6-2)
	}
	if countLinesWith(view, "Read") < 1 {
		t.Errorf("the single item is missing from view:\n%s", view)
	}
}

func TestActivityRoom_ThreeLinesOfRoom_PanelKeepsHeaderAndItemAndOneRow(t *testing.T) {
	s := newRoomScreen(t, 9, 5)

	view := s.View()

	if !strings.Contains(view, "Activity: agent-a#3") || countLinesWith(view, "Read") < 1 {
		t.Errorf("header and one item expected at the smallest panel room:\n%s", view)
	}
	if !strings.Contains(view, roomRowName(roomRowCount)) {
		t.Errorf("latest step row missing from view:\n%s", view)
	}
	if got := viewLineCount(view); got > 9 {
		t.Errorf("view has %d lines, want at most 9", got)
	}
}

func TestActivityRoom_PanelLinesNeverShrinkAsTheScreenGrows(t *testing.T) {
	prev := 0
	for _, h := range []int{8, 12, 16, 20, 30, 40} {
		view := newRoomScreen(t, h, 60).View()
		got := countLinesWith(view, "Read") + countLinesWith(view, "Activity:")
		if got < prev {
			t.Errorf("panel lines at H=%d = %d, fewer than %d at the smaller height", h, got, prev)
		}
		prev = got
	}
}
