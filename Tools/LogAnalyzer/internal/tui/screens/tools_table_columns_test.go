package screens_test

// The tools table's columns, long names at 80 columns, and the share bar in
// every section.

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestToolsTable_LongMCPNameIsEllipsisTruncatedAtEightyColumns(t *testing.T) {
	run := runWithTools(
		domain.ToolFigures{Tool: longMCPTool, Calls: 12, Errors: 1, Durations: durationStats(12, 12*time.Second, time.Second, time.Second, time.Second)},
		domain.ToolFigures{Tool: "Bash", Calls: 3},
	)

	view := toolsView(t, run, 80, tallH)

	row := rowFor(t, view, longMCPTool[:8])
	assertNotContains(t, view, longMCPTool)
	assertContains(t, row, "…")
	assertFields(t, row, "12")
}

func TestToolsTable_HeaderNamesEveryColumnAndAToolRowShowsItsDurations(t *testing.T) {
	colourOff(t)
	view := toolsView(t, toolsRun(), tableW, tallH)

	header := ""
	for _, line := range strings.Split(stripStyling(view), "\n") {
		l := strings.ToLower(line)
		if strings.Contains(l, "tool") && strings.Contains(l, "calls") && strings.Contains(l, "errors") &&
			strings.Contains(l, "avg") && strings.Contains(l, "min") && strings.Contains(l, "max") && strings.Contains(l, "unpaired") {
			header = line
			break
		}
	}
	if header == "" {
		t.Errorf("no header line names tool, calls, errors, avg, min, max and unpaired\n%s", view)
	}
	read := rowFor(t, view, "Read")
	for _, d := range []time.Duration{4 * time.Second, ms(1250), ms(7500)} {
		if !hasDuration(read, wantDuration(t, d)) {
			t.Errorf("the Read row must show %v as %q: %q", d, wantDuration(t, d), read)
		}
	}
}

func TestToolsTable_ShareBarFollowsTheCallsInOrchestratorAndAgentTypeSections(t *testing.T) {
	colourOff(t)
	run := overviewRun()
	run.Tools = domain.ToolsBreakdown{
		Orchestrator: []domain.ToolFigures{
			{Tool: "OrchBig", Calls: 20},
			{Tool: "OrchSmall", Calls: 2},
		},
		ByAgentType: []domain.AgentTypeTools{{AgentType: "builder", Tools: []domain.ToolFigures{
			{Tool: "TypeBig", Calls: 30},
			{Tool: "TypeSmall", Calls: 3},
		}}},
	}

	view := toolsView(t, run, tableW, tallH)

	if big, small := barWeight(rowFor(t, view, "OrchBig")), barWeight(rowFor(t, view, "OrchSmall")); !(big > small && small >= 0 && big > 0) {
		t.Errorf("orchestrator bars must follow calls 20 and 2: got %d and %d\n%s", big, small, view)
	}
	if big, small := barWeight(rowFor(t, view, "TypeBig")), barWeight(rowFor(t, view, "TypeSmall")); !(big > small && big > 0) {
		t.Errorf("agent type bars must follow calls 30 and 3: got %d and %d\n%s", big, small, view)
	}
}

func TestToolsTable_ToolWithNoPairedDurationHasAnEmptyBarUnderTheDurationSort(t *testing.T) {
	colourOff(t)
	run := runWithTools(
		domain.ToolFigures{Tool: "Timed", Calls: 2, Durations: durationStats(2, 10*time.Second, 5*time.Second, 5*time.Second, 5*time.Second)},
		domain.ToolFigures{Tool: "Unpaired", Calls: 4, Unpaired: 4},
	)
	s := screens.NewToolsScreen(run, tableW, tallH, paletteStyles())
	press(s, screens.KeySort)

	view := s.View(tableW, tallH)

	assertWithinBounds(t, view, tableW, tallH)
	if key, _ := activeSort(t, view); key != screens.SortDuration {
		t.Fatalf("the second sort must be duration, got %q", key)
	}
	if timed, none := barWeight(rowFor(t, view, "Timed")), barWeight(rowFor(t, view, "Unpaired")); timed <= 0 || none != 0 {
		t.Errorf("the timed tool needs a bar and the all-unpaired tool an empty one: got %d and %d\n%s", timed, none, view)
	}
}
