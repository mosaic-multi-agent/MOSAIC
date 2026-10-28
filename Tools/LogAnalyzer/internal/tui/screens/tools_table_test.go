package screens_test

// The tools view as a table: the error count in the bad role when non-zero, the
// tool names in the tool role, the share bar that follows the active sort, long
// tool names truncated with an ellipsis, aligned cells, sort cycling, tool
// selection and bounds.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestToolsTable_NonZeroErrorCountIsDrawnInTheBadRole(t *testing.T) {
	colourOn(t)
	view := screens.NewToolsScreen(errorsRun(), tableW, tallH, paletteStyles()).View(tableW, tallH)

	// The first row can hold the cursor, so the other rows are checked.
	assertDrawnInRole(t, rawLineWith(t, view, "Beta"), "4", screens.ToolErrorsRole(4))
	if screens.ToolErrorsRole(4) != tuicommon.RoleSignalBad {
		t.Errorf("a non-zero error count must map to the bad role, got %q", screens.ToolErrorsRole(4))
	}
}

func TestToolsTable_ZeroErrorCountIsNotDrawnInTheBadRole(t *testing.T) {
	colourOn(t)
	view := screens.NewToolsScreen(errorsRun(), tableW, tallH, paletteStyles()).View(tableW, tallH)

	re := regexp.MustCompile(regexp.QuoteMeta(rolePrefix(tuicommon.RoleSignalBad)) + `\s*0\b`)
	if line := rawLineWith(t, view, "Gamma"); re.MatchString(line) {
		t.Errorf("a zero error count must not be drawn in the bad role: %q", line)
	}
}

func TestToolsTable_ToolNamesAreDrawnInTheToolRole(t *testing.T) {
	colourOn(t)
	view := screens.NewToolsScreen(errorsRun(), tableW, tallH, paletteStyles()).View(tableW, tallH)

	assertDrawnInRole(t, rawLineWith(t, view, "Beta"), "Beta", tuicommon.RoleDataTool)
	assertDrawnInRole(t, rawLineWith(t, view, "Gamma"), "Gamma", tuicommon.RoleDataTool)
}

func TestToolsTable_ShareBarFollowsTheCallsSortThenTheDurationSort(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(shareRun(), tableW, tallH, paletteStyles())

	view := s.View(tableW, tallH)
	if key, _ := activeSort(t, view); key != screens.SortCalls {
		t.Fatalf("the default sort must be calls, got %q", key)
	}
	a, b := barWeight(rowFor(t, view, "ToolA")), barWeight(rowFor(t, view, "ToolB"))
	if !(a > b && a > 0) {
		t.Errorf("under the calls sort the bar follows the share of calls (ToolA 10, ToolB 2): got %d and %d\n%s", a, b, view)
	}

	press(s, screens.KeySort)
	view = s.View(tableW, tallH)
	if key, _ := activeSort(t, view); key != screens.SortDuration {
		t.Fatalf("the second sort must be duration, got %q", key)
	}
	a, b = barWeight(rowFor(t, view, "ToolA")), barWeight(rowFor(t, view, "ToolB"))
	if !(b > a && b > 0) {
		t.Errorf("under the duration sort the bar follows the share of total duration (ToolA 1s, ToolB 20s): got %d and %d\n%s", a, b, view)
	}
}

func TestToolsTable_ShareBarGrowsWithTheShareOfCalls(t *testing.T) {
	colourOff(t)
	view := screens.NewToolsScreen(errorsRun(), tableW, tallH, paletteStyles()).View(tableW, tallH)

	alpha, beta, gamma := barWeight(rowFor(t, view, "Alpha")), barWeight(rowFor(t, view, "Beta")), barWeight(rowFor(t, view, "Gamma"))

	if !(alpha > beta && beta > gamma && gamma > 0) {
		t.Errorf("bars must follow the calls 20, 5 and 3: got %d %d %d\n%s", alpha, beta, gamma, view)
	}
}

func TestToolsTable_LongMCPNameIsEllipsisTruncatedAtNarrowWidthsAndKeepsItsFigures(t *testing.T) {
	run := runWithTools(
		domain.ToolFigures{Tool: longMCPTool, Calls: 12, Errors: 1, Durations: durationStats(12, 12*time.Second, time.Second, time.Second, time.Second)},
		domain.ToolFigures{Tool: "Bash", Calls: 3},
	)
	for _, width := range []int{tableW, 120, 100, 80, 60, 50} {
		view := toolsView(t, run, width, tallH)

		assertWithinBounds(t, view, width, tallH)
		assertNameShown(t, view, longMCPTool, 8)
		assertFields(t, rowFor(t, view, longMCPTool[:8]), "12")
	}
	view := toolsView(t, run, 50, tallH)
	assertNotContains(t, view, longMCPTool)
	assertContains(t, rowFor(t, view, longMCPTool[:8]), "…")
}

func TestToolsTable_ShortAndLongNamesKeepTheirCellsAligned(t *testing.T) {
	run := runWithTools(
		domain.ToolFigures{Tool: "LongerToolName", Calls: 123, Errors: 45},
		domain.ToolFigures{Tool: "Short", Calls: 7},
	)
	for _, width := range []int{tableW, 100, 80} {
		view := toolsView(t, run, width, tallH)

		long := displayColumn(rowFor(t, view, "LongerToolName"), "123")
		short := displayColumn(rowFor(t, view, "Short"), "7")
		// Cells are aligned when they start at one column (left) or end at one (right).
		if long < 0 || short < 0 || (long != short && long+len("123") != short+len("7")) {
			t.Errorf("width %d: call counts are not aligned (columns %d and %d)\n%s", width, long, short, view)
		}
	}
}

func TestToolsTable_ReadableAtEightyColumns(t *testing.T) {
	view := toolsView(t, toolsRun(), 80, tallH)

	assertWithinBounds(t, view, 80, tallH)
	assertFields(t, rowFor(t, view, "Read"), "11", "3")
	assertFields(t, rowFor(t, view, "Bash"), "6")
	assertContains(t, view, "Grep", "Edit", "Write")
}

func TestToolsTable_NothingIsDrawnPastTheTerminalAtAnySize(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(toolsRun(), 100, tallH, paletteStyles())

	for height := 0; height <= 25; height++ {
		for _, width := range []int{100, 80, 60, 30} {
			assertWithinBounds(t, s.View(width, height), width, height)
		}
	}
}

func TestToolsTable_WithColourOffEveryMarkerRemainsAsText(t *testing.T) {
	view := toolsView(t, toolsRun(), tableW, tallH)

	if view != stripStyling(view) {
		t.Errorf("colour off must leave no escape sequences: %q", view)
	}
	assertContains(t, view, screens.UnpairedLabel, screens.AbsentMarker, "sort: calls")
}

func toolsSortAscending() map[screens.SortKey][]string {
	return map[screens.SortKey][]string{
		screens.SortCalls:    {screens.UnnamedToolLabel, "Bash", "Read"},
		screens.SortDuration: {"Bash", "Read"},
		screens.SortErrors:   {"Bash", "Read"},
		screens.SortName:     {"Bash", "Read"},
	}
}

func TestToolsTable_DefaultSortIsCallsDescendingAndTheHeaderSaysSo(t *testing.T) {
	colourOff(t)

	view := screens.NewToolsScreen(toolsRun(), tableW, tallH, paletteStyles()).View(tableW, tallH)

	if key, desc := activeSort(t, view); key != screens.SortCalls || !desc {
		t.Errorf("default sort = %q desc=%v, want calls descending", key, desc)
	}
	assertAppearInOrder(t, view, []string{"Read", "Bash", screens.UnnamedToolLabel}, true)
}

func TestToolsTable_SortKeyCyclesThroughEveryOrderAndWrapsAround(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(toolsRun(), tableW, tallH, paletteStyles())

	seen := walkSorts(t, s, tableW, tallH, toolsSortAscending(),
		screens.SortCalls, screens.SortDuration, screens.SortErrors, screens.SortName)

	if len(seen) == 0 || seen[0] != screens.SortCalls {
		t.Errorf("the cycle must start on calls, got %v", seen)
	}
}

func TestToolsTable_SortingNeverDropsAToolOrASection(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(toolsRun(), tableW, tallH, paletteStyles())

	for i := 0; i < 5; i++ {
		view := s.View(tableW, tallH)
		assertContains(t, view, "Read", "Bash", screens.UnnamedToolLabel, "Grep", "Edit", "Write", "builder", screens.UnknownTypeLabel)
		assertContains(t, strings.ToLower(view), "orchestrator")
		press(s, screens.KeySort)
	}
}

func TestToolsTable_SortKeyIsListedInTheHelpOverlay(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(toolsRun(), 120, tallH, paletteStyles())

	overlay := strings.Join(overlayLines(openOverlay(s, 120, tallH)), "\n")

	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeySort) + `\s+.*(?i:sort)`).MatchString(overlay) {
		t.Errorf("the help overlay must list the sort key %q\n%s", screens.KeySort, overlay)
	}
}

func TestToolsTable_SelectedToolIsTheRowUnderTheCursorOfTheSortedTable(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(toolsRun(), tableW, tallH, paletteStyles())

	first, ok := s.SelectedTool()
	if !ok || first.Tool != "Read" || first.Calls != 11 {
		t.Errorf("the first calls-sorted row is Read with 11 calls, got (%+v, %v)", first, ok)
	}
	press(s, "down")
	second, ok := s.SelectedTool()

	if !ok || second.Tool != "Bash" {
		t.Errorf("down must select the second row, Bash; got (%+v, %v)", second, ok)
	}
}

func TestToolsTable_NoSelectedToolWhenTheRunHasNoToolCalls(t *testing.T) {
	colourOff(t)
	s := screens.NewToolsScreen(overviewRun(), tableW, tallH, paletteStyles())

	if got, ok := s.SelectedTool(); ok {
		t.Errorf("a run without tool calls has no selected tool, got %+v", got)
	}
}
