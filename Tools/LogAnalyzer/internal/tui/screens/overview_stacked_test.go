package screens_test

// The stacked bars of the visual Overview: cost share, token split, outcome
// split. Each carries a legend naming every segment with its whole-percent share;
// the outcome split is drawn in the status roles.

import (
	"regexp"
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// legendEntry matches a legend entry: swatch glyph, label, then the share. A share
// that starts with a digit is not the tail of a longer number (4% is not 64%).
func legendEntry(glyph, label, share string) string {
	boundary := ""
	if share != "" && share[0] >= '0' && share[0] <= '9' {
		boundary = `[^0-9%.]`
	}
	return `(?i)` + regexp.QuoteMeta(glyph) + ` ` + regexp.QuoteMeta(label) + `[^%]*` + boundary + regexp.QuoteMeta(share)
}

var unicodeSwatches = []string{"█", "▓", "▒", "░"}

func TestOverviewVisual_CostShareLegendNamesOrchestratorAndSubagentsWithShares(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	assertMatches(t, view, legendEntry("█", "orchestrator", "40%"), legendEntry("▓", "subagent", "60%"))
}

func TestOverviewVisual_CostShareLegendAddsUnattributedAsThirdSegment(t *testing.T) {
	colourOff(t)

	view := overviewView(withUnattributed(analysesRun()), overviewTallH)

	assertMatches(t, view, legendEntry("█", "orchestrator", "%"), legendEntry("▓", "subagent", "%"),
		legendEntry("▒", "unattributed", "%"))
}

func TestOverviewVisual_TokenSplitLegendNamesEveryCategoryWithItsShare(t *testing.T) {
	colourOff(t)
	view := overviewView(analysesRun(), overviewTallH)

	shares := []string{"21%", "64%", "11%", "4%"}
	for i, cat := range domain.BillableCategories() {
		assertMatches(t, view, legendEntry(unicodeSwatches[i], cat.Label(), shares[i]))
	}
}

func TestOverviewVisual_TokenSplitShowsReasoningInsideOutput(t *testing.T) {
	colourOff(t)
	view := overviewView(analysesRun(), overviewTallH)

	assertContains(t, strings.ToLower(view), domain.CategoryReasoning.Label())
	// Reasoning is part of output, so it is not a fifth share.
	assertNotContains(t, strings.ToLower(view), "▓ reasoning", "▒ reasoning", "░ reasoning", "█ reasoning")
}

func TestOverviewVisual_OutcomeSplitLegendNamesTheThreeGroupsWithShares(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	assertMatches(t, view, legendEntry("█", "success", "63%"), legendEntry("▓", "other", "22%"),
		legendEntry("▒", "open", "15%"))
}

func TestOverviewVisual_OutcomeSplitIsDrawnInTheStatusRoles(t *testing.T) {
	colourOn(t)
	view := overviewView(analysesRun(), overviewTallH)

	success := rawLineMatching(t, view, legendEntry("█", "success", "63%"))
	other := rawLineMatching(t, view, legendEntry("▓", "other", "22%"))
	open := rawLineMatching(t, view, legendEntry("▒", "open", "15%"))

	assertDrawnInRole(t, success, "█", tuicommon.RoleSignalGood)
	assertDrawnInRole(t, other, "▓", tuicommon.RoleSignalWarning)
	assertDrawnInRole(t, open, "▒", tuicommon.RoleSignalWarning)
}

func TestOverviewVisual_TheTwoWarningSegmentsStayDistinguishableWithColourOff(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	other := firstLineMatching(view, legendEntry("▓", "other", "22%"))
	open := firstLineMatching(view, legendEntry("▒", "open", "15%"))
	if other == "" || open == "" {
		t.Errorf("segments that share a role need different glyphs in the legend\n%s", view)
	}
	if view != stripStyling(view) {
		t.Errorf("colour off must leave no escape sequences")
	}
}

func TestOverviewVisual_UsesAsciiSwatchesInAsciiGlyphMode(t *testing.T) {
	asciiOn(t)

	view := overviewView(analysesRun(), overviewTallH)

	assertMatches(t, view, legendEntry("#", "orchestrator", "40%"), legendEntry("=", "subagent", "60%"),
		legendEntry("#", "success", "63%"), legendEntry("=", "other", "22%"), legendEntry("+", "open", "15%"))
	assertNotContains(t, view, "█", "▓", "▒", "░")
}

func TestOverviewVisual_EveryShareIsALegendEntryNotAnEmptyBar(t *testing.T) {
	colourOff(t)
	run := analysesRun()
	run.Overview.Outcomes.OpenOrNone = domain.GroupFigures{}

	view := overviewView(run, overviewTallH)

	assertMatches(t, view, legendEntry("█", "success", "%"), legendEntry("▓", "other", "%"))
	// The open segment with no dispatches reads 0%, never a hidden entry.
	assertMatches(t, view, legendEntry("▒", "open", "0%"))
}

func TestOverviewFiguresOnly_HasNoStackedBars(t *testing.T) {
	colourOff(t)

	view := overviewFiguresView(analysesRun(), overviewTallH)

	assertNotContains(t, view, "▓", "▒", "░")
}
