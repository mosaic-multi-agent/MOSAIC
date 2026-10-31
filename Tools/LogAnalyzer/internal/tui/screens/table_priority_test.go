package screens_test

// The order in which the other tables lose columns as the terminal narrows:
// decorative columns (share bars) first, figure columns next, and the columns
// that name the row last.
//
// Not covered here by design: the option list of a select-one overlay has a
// single droppable column (description) beside the never-dropped option name,
// and the shared share-bar column is exercised through the dispatch, type,
// tools, status and across-runs tests.

import (
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

func dispatchTableAt(t *testing.T) func(width int) string {
	return func(width int) string {
		colourOff(t)
		return dispatchesView(dispatchRun(), width, tallH)
	}
}

var dispatchFigures = []string{"tokens", "duration", "peak context", "cache hit", "rewrites", "cache write", "write cost", "net cache", "unread cache writes", "effort"}

func TestDispatchTable_ShareBarIsDroppedBeforeEveryFigureColumn(t *testing.T) {
	assertDropOrder(t, dispatchTableAt(t), "instance", []string{"share"}, dispatchFigures)
}

func TestDispatchTable_StatusOutlastsEveryFigureColumn(t *testing.T) {
	assertDropOrder(t, dispatchTableAt(t), "instance", dispatchFigures, []string{"status"})
}

func agentTypeTableAt(t *testing.T) func(width int) string {
	return func(width int) string {
		colourOff(t)
		return screens.NewAgentTypesScreen(typesFiguresRun(), width, tallH, paletteStyles()).View(width, tallH)
	}
}

var typeFigures = []string{"average", "average peak context", "maximum peak context", "cache hit", "rewrites", "cache write", "write cost"}

func TestAgentTypeTable_ShareBarIsDroppedBeforeEveryFigureColumn(t *testing.T) {
	assertDropOrder(t, agentTypeTableAt(t), "type", []string{"share"}, typeFigures)
}

func TestAgentTypeTable_TypeOutlastsEveryFigureColumn(t *testing.T) {
	assertDropOrder(t, agentTypeTableAt(t), "type", typeFigures, []string{"type"})
}

func TestToolsTable_ShareBarIsDroppedBeforeEveryFigureColumn(t *testing.T) {
	render := func(width int) string {
		colourOff(t)
		return screens.NewToolsScreen(toolsRun(), width, tallH, paletteStyles()).View(width, tallH)
	}

	// The last tier is the anchor: the header is found by it at every width, so
	// asserting it keeps the identifying column from being dropped for a figure.
	assertDropOrder(t, render, "tool", []string{"share"}, []string{"average duration", "minimum duration", "maximum duration", "unpaired"}, []string{"tool"})
}

func TestStatusTable_ShareBarsAreDroppedBeforeTheFlags(t *testing.T) {
	render := func(width int) string {
		colourOff(t)
		return screens.NewStatusScreen(statusRun(), width, tallH, paletteStyles()).View(width, tallH)
	}

	assertDropOrder(t, render, "status", []string{"dispatch share", "cost share"}, []string{"flags"}, []string{"status"})
}

func TestTimelineInvocationTable_StatusOutlastsTheTimesAndTheOverlap(t *testing.T) {
	render := func(width int) string { return listView(t, timelineRun(), width, tallH) }

	assertDropOrder(t, render, "instance", []string{"overlap", "start", "end"}, []string{"status"})
}

func TestTimelineCompactionTable_TriggerOutlastsTheContextFigures(t *testing.T) {
	render := func(width int) string { return listView(t, timelineRun(), width, tallH) }

	assertDropOrder(t, render, "time", []string{"shape", "before", "after"}, []string{"trigger"})
}

func TestAcrossRunsRunTable_ShareBarIsDroppedBeforeTheFigures(t *testing.T) {
	render := func(width int) string {
		return newAcross(acrossReport(), width, tallH).View(width, tallH)
	}

	assertDropOrder(t, render, "run", []string{"share"}, []string{"subagents", "flags"}, []string{"run"})
}

func TestAcrossRunsTypeTable_ShareBarIsDroppedBeforeTheFigures(t *testing.T) {
	render := func(width int) string {
		return newAcross(acrossReport(), width, tallH).View(width, tallH)
	}

	assertDropOrder(t, render, "type",
		[]string{"share"},
		[]string{"invocations", "runs", "average per run", "invocations per run", "tokens per run", "flags"},
		[]string{"type"},
	)
}

func TestCompareTable_TypeOutlastsEveryFigureColumn(t *testing.T) {
	render := func(width int) string { return compareView(width, tallH) }

	assertDropOrder(t, render, "type",
		[]string{"invocations A", "invocations B", "tokens A", "tokens B", "invocations change", "tokens change"},
		[]string{"cost A", "cost B", "cost change"},
		[]string{"type"},
	)
}

func TestRunsTable_RunOutlastsTheInvocationsAndTheDuration(t *testing.T) {
	render := func(width int) string { return runsView(runsReport(), width, tallH) }

	assertDropOrder(t, render, "compare|cmp", []string{"duration", "invocations"}, []string{"run", "cost"})
}

func TestOverviewModelTable_ShareBarIsDroppedBeforeTheFigures(t *testing.T) {
	render := func(width int) string {
		colourOff(t)
		return screens.NewOverviewScreen(analysesRun(), width, overviewTallH, paletteStyles()).View(width, overviewTallH)
	}

	assertDropOrder(t, render, "model", []string{"share"}, []string{"tokens", "API calls", "cost"}, []string{"model"})
}

// The wall-time table of the Overview has one droppable column, harness; the
// tool name and the call count never drop before it.
func TestOverviewWallTimeToolTable_HarnessIsDroppedBeforeTheCallsAndTheToolName(t *testing.T) {
	// The long flex title is clipped below this width, so the sweep stops here.
	const narrowest = 60
	render := func(width int) string {
		colourOff(t)
		width = max(width, narrowest)
		return screens.NewOverviewScreen(analysesRun(), width, overviewTallH, paletteStyles()).View(width, overviewTallH)
	}

	assertDropOrder(t, render, "tools counted as waits", []string{"harness"}, []string{"calls"})
}

func TestPricingTable_ModelOutlastsTheOtherColumns(t *testing.T) {
	render := func(width int) string {
		colourOff(t)
		return screens.NewPricingScreen(sharedModelStatuses(), "", width, tallH, paletteStyles()).View(width, tallH)
	}

	assertDropOrder(t, render, "model", []string{"source", "status"}, []string{"model"})
}
