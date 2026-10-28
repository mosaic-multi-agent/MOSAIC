package screens_test

// The wall-time block of the visual Overview: the stacked split with each
// category's duration, the tool names that counted as waits, the open-ended
// waits, the stated pairing limit and the hint about missing MCP contact tools.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const wallTimePattern = `(?i)wall.?time`

func TestOverviewWallTime_LegendNamesTheFourCategoriesWithTheirShares(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	assertMatches(t, view, legendEntry("█", "waiting", "21%"), legendEntry("▓", "subagents", "55%"),
		legendEntry("▒", "orchestrator only", "16%"), legendEntry("░", "between sessions", "8%"))
}

func TestOverviewWallTime_ShowsTheDurationOfEachCategory(t *testing.T) {
	colourOff(t)
	view := overviewView(analysesRun(), overviewTallH)

	for _, d := range []time.Duration{25 * time.Minute, 66 * time.Minute, 19 * time.Minute, 10 * time.Minute} {
		assertContains(t, view, screens.FormatDuration(d, true))
	}
}

func TestOverviewWallTime_ListsTheToolNamesThatCountedAsWaitsWithTheirCalls(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	askLine := firstLineMatching(view, `AskUserQuestion`)
	mcpLine := firstLineMatching(view, `mcp__contact-user__ask_user_questions`)
	if askLine == "" || mcpLine == "" {
		t.Fatalf("both counted tool names must be listed\n%s", view)
	}
	assertFields(t, askLine, "5")
	assertFields(t, mcpLine, "2")
}

func TestOverviewWallTime_NamesNoToolThatDidNotCountAsAWait(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	assertNotContains(t, view, "mcp__other__ask")
}

// hasOpenEndedCount reports whether some line other than the pairing note speaks
// of open waits and shows count as a figure.
func hasOpenEndedCount(view, count string) bool {
	for _, l := range strings.Split(stripStyling(view), "\n") {
		low := strings.ToLower(l)
		if strings.Contains(low, "open") && !strings.Contains(low, "cannot be paired") && hasField(l, count) {
			return true
		}
	}
	return false
}

func TestOverviewWallTime_MarksTheCountOfOpenEndedWaits(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	if !hasOpenEndedCount(view, "3") {
		t.Errorf("a line must count the 3 open-ended waits\n%s", view)
	}
	run := analysesRun()
	run.WallTime.OpenWaits = run.WallTime.OpenWaits[:1]
	if !hasOpenEndedCount(overviewView(run, overviewTallH), "1") {
		t.Error("the open-ended count must follow the open waits of the run (1)")
	}
}

func TestOverviewWallTime_StatesThatPermissionNotificationsCannotBePaired(t *testing.T) {
	colourOff(t)

	view := strings.ToLower(overviewView(analysesRun(), overviewTallH))

	assertContains(t, view, "permission notifications cannot be paired", "open-ended")
}

func TestOverviewWallTime_HintPointsToTheToolsViewWhenContactToolsMayBeMissing(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	line := firstLineMatching(view, `(?i)mcp contact tools may be missing`)
	if line == "" {
		t.Fatalf("the hint must say MCP contact tools may be missing\n%s", view)
	}
	assertContains(t, line, "Tools", screens.KeyViewTools)
}

func TestOverviewWallTime_NoHintWhenTheConditionDoesNotHold(t *testing.T) {
	colourOff(t)
	run := analysesRun()
	run.WallTime.MissingContactToolsHint = false
	run.WallTime.UnconfiguredTools = nil

	view := strings.ToLower(overviewView(run, overviewTallH))

	assertNotContains(t, view, "may be missing", "mcp contact tools")
}

func TestOverviewWallTime_RunWithoutALoggedSpanShowsTheMarkerNotZeroDurations(t *testing.T) {
	colourOff(t)
	run := analysesRun()
	run.WallTime = domain.WallTimeSplit{}

	view := overviewView(run, overviewTallH)

	window := windowAround(view, wallTimePattern, 2)
	if window == "" {
		t.Fatalf("the wall-time block keeps its place without a span\n%s", view)
	}
	assertContains(t, window, screens.AbsentMarker)
	if regexp.MustCompile(`(^|[^0-9])0s`).MatchString(stripStyling(view)) {
		t.Errorf("no span must not read as zero durations\n%s", view)
	}
}
