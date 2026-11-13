package screens_test

// The findings overlay of the runs list: f opens it in place of the table, f and
// esc close it, it takes every key while open, and it shows one row per finding
// kind over every finding of the report, with the number of runs the kind is in.

import (
	"reflect"
	"regexp"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const runsW, runsH = 220, 40

func openFindings(s screens.Screen, width, height int) string {
	return press(s, screens.KeyFindings).View(width, height)
}

func TestRunsFindings_KeyIsFAndDoesNotCollideWithRunsListKeys(t *testing.T) {
	if screens.KeyFindings != "f" {
		t.Errorf("KeyFindings = %q, want f", screens.KeyFindings)
	}
	for _, k := range []string{
		screens.KeySort, screens.KeyMarkRun, screens.KeyCompare, screens.KeyAcrossRuns, screens.KeyHelp,
		screens.KeyExportRun, screens.KeyExportAll, screens.KeyViewDispatches, screens.KeyViewOverview,
		screens.KeyViewAgentTypes, screens.KeyViewStatus, screens.KeyViewTools, screens.KeyViewTimeline,
		screens.KeyViewContext, screens.KeyToggleView, screens.KeyGroup, screens.KeyMarkUserTool,
		"s", "p", "q", "up", "down", "j", "k", "enter", "esc",
	} {
		if screens.KeyFindings == k {
			t.Errorf("the findings key collides with %q", k)
		}
	}
}

func TestRunsFindings_FOpensTheOverlayInPlaceOfTheTable(t *testing.T) {
	colourOff(t)
	s := newFindingsRuns(runsW, runsH)
	before := s.View(runsW, runsH)

	overlay := openFindings(s, runsW, runsH)

	if !s.FindingsOpen() {
		t.Fatal("f must open the findings overlay")
	}
	if overlay == before {
		t.Fatal("f must change the view")
	}
	lines := viewLines(overlay)
	assertContains(t, headOf(lines), "Log Analyzer")
	assertNotContains(t, stripStyling(overlay), runIDOne)
	assertWithinBounds(t, overlay, runsW, runsH)
}

func TestRunsFindings_OneRowPerKindWithSeverityCountAndExplanation(t *testing.T) {
	colourOff(t)

	overlay := openFindings(newFindingsRuns(runsW, runsH), runsW, runsH)

	assertRow(t, overlay, domain.FindingMalformedLine, "warning", "4", "in 2 runs")
	assertRow(t, overlay, domain.FindingCrossStreamRecord, "warning", "3", "in 2 runs")
	assertRow(t, overlay, domain.FindingInvalidPricingEntry, "warning", "1", "")
	assertRow(t, overlay, domain.FindingMissingModel, "info", "1", "in 1 run")
}

func TestRunsFindings_RowsAreSortedWarningBeforeInfoThenCountThenKind(t *testing.T) {
	colourOff(t)

	overlay := openFindings(newFindingsRuns(runsW, runsH), runsW, runsH)

	assertRowOrder(t, overlay,
		domain.FindingMalformedLine, domain.FindingCrossStreamRecord, domain.FindingInvalidPricingEntry,
		futureKind, domain.FindingMissingModel)
}

func TestRunsFindings_UnknownKindShowsItsRawName(t *testing.T) {
	colourOff(t)

	overlay := openFindings(newFindingsRuns(runsW, runsH), runsW, runsH)

	line := rowLine(t, overlay, futureKind)
	assertContains(t, line, "info", "in 1 run")
}

func TestRunsFindings_CountsRunsWithoutTheRunlessAndUnattributableFindings(t *testing.T) {
	colourOff(t)

	overlay := openFindings(newFindingsRuns(runsW, runsH), runsW, runsH)

	// malformed_line has four findings: two in a, one in b, one unattributable.
	line := rowLine(t, overlay, domain.FindingMalformedLine)
	if !regexp.MustCompile(`in 2 runs`).MatchString(line) {
		t.Errorf("the unattributable finding must not count as a run: %q", line)
	}
	// invalid_pricing_entry is tied to no run: no run count at all.
	if line := rowLine(t, overlay, domain.FindingInvalidPricingEntry); regexp.MustCompile(`in \d+ runs?`).MatchString(line) {
		t.Errorf("a kind that occurs in no run must not say in how many: %q", line)
	}
}

func TestRunsFindings_NeverListsIndividualFindings(t *testing.T) {
	colourOff(t)

	s := newFindingsRuns(runsW, runsH)
	overlay := openFindings(s, runsW, runsH)

	if !s.FindingsOpen() {
		t.Fatal("f must open the findings overlay")
	}
	assertNoFindingDetails(t, overlay)
}

func TestRunsFindings_FindingsCarriedByDigestsAppearLikeAnyOther(t *testing.T) {
	colourOff(t)
	r := runsReport()
	// A digest keeps only kind, severity, run and actor.
	r.Quality = domain.NewQualitySummary([]domain.Finding{
		{Kind: domain.FindingCrossStreamRecord, Severity: domain.SeverityWarning, Run: domain.NamedRun(runIDOne), Actor: domain.Orchestrator()},
		{Kind: domain.FindingCrossStreamRecord, Severity: domain.SeverityWarning, Run: domain.NamedRun(runIDTwo), Actor: domain.AgentInstance("coder#1")},
	})
	s := screens.NewRunsScreen(r, runsW, runsH, paletteStyles())

	overlay := openFindings(s, runsW, runsH)

	assertRow(t, overlay, domain.FindingCrossStreamRecord, "warning", "2", "in 2 runs")
}

func TestRunsFindings_NoFindingsShowsASingleNoFindingsLine(t *testing.T) {
	colourOff(t)
	s := newRuns(runsW, runsH)

	overlay := stripStyling(openFindings(s, runsW, runsH))

	assertContains(t, overlay, "no findings")
}

func TestRunsFindings_ScopeIsEveryFindingOfTheReport(t *testing.T) {
	s := newFindingsRuns(runsW, runsH)

	scope := s.FindingsScope()

	if scope.Label != "" {
		t.Errorf("Label = %q, want empty on the runs list", scope.Label)
	}
	want := listedFindingsReport().Quality.Findings
	if len(want) != 10 {
		t.Fatalf("setup: fixture has %d findings, want 10", len(want))
	}
	if !reflect.DeepEqual(scope.Findings, want) {
		t.Errorf("scope covers %d findings, want all %d of the report (runs, unattributable and run-less)", len(scope.Findings), len(want))
	}
}

func TestRunsFindings_FAndEscCloseItAndRestoreTheList(t *testing.T) {
	colourOff(t)
	for _, closer := range []string{screens.KeyFindings, "esc"} {
		s := newFindingsRuns(runsW, runsH)
		before := s.View(runsW, runsH)

		open := openFindings(s, runsW, runsH)
		if !s.FindingsOpen() || open == before {
			t.Fatalf("f must open the overlay and change the list before %q can close it", closer)
		}
		after := press(s, closer).View(runsW, runsH)

		if s.FindingsOpen() {
			t.Errorf("%q must close the overlay", closer)
		}
		if after != before {
			t.Errorf("%q must restore the list\n%s\nvs\n%s", closer, after, before)
		}
		if s.Back() {
			t.Errorf("%q closing the overlay must not leave the runs list", closer)
		}
	}
}

func TestRunsFindings_EscLeavesTheListOnlyAfterTheOverlayIsClosed(t *testing.T) {
	s := newFindingsRuns(runsW, runsH)

	openFindings(s, runsW, runsH)
	press(s, "esc")
	if s.Back() || s.FindingsOpen() {
		t.Fatalf("the first esc must close the overlay without leaving the list (back=%v open=%v)", s.Back(), s.FindingsOpen())
	}
	press(s, "esc")

	if !s.Back() {
		t.Error("esc after closing the overlay must leave the list")
	}
}

func TestRunsFindings_EveryOtherKeyIsSwallowedWhileOpen(t *testing.T) {
	colourOff(t)
	keys := []string{
		"enter", "q", screens.KeySort, screens.KeyMarkRun, screens.KeyCompare, screens.KeyAcrossRuns, "s", "p",
		screens.KeyExportRun, screens.KeyExportAll, screens.KeyHelp, screens.KeyViewDispatches, screens.KeyViewOverview,
	}
	for _, k := range keys {
		s := newFindingsRuns(runsW, runsH)
		before := s.View(runsW, runsH)
		open := openFindings(s, runsW, runsH)

		press(s, k)

		if !s.FindingsOpen() {
			t.Errorf("%q must not close the overlay", k)
		}
		if s.Done() || s.Back() {
			t.Errorf("%q must not leave the runs list while the overlay is open", k)
		}
		if got := s.View(runsW, runsH); got != open {
			t.Errorf("%q must change nothing while the overlay is open\n%s\nvs\n%s", k, got, open)
		}
		if after := press(s, screens.KeyFindings).View(runsW, runsH); after != before {
			t.Errorf("%q leaked through the overlay into the list\n%s\nvs\n%s", k, after, before)
		}
	}
}

func TestRunsFindings_ScrollKeysMoveTheOverlayNotTheListSelection(t *testing.T) {
	colourOff(t)
	s := newFindingsRuns(runsW, runsH)
	before := s.View(runsW, runsH)

	openFindings(s, runsW, runsH)
	press(s, "down", "j", "j", "up", "k")
	after := press(s, screens.KeyFindings).View(runsW, runsH)

	if after != before {
		t.Errorf("scrolling the overlay must not move the list selection\n%s\nvs\n%s", after, before)
	}
}

func TestRunsFindings_ScrollsAMoreRowsThanFitOnAShortTerminalWithinBounds(t *testing.T) {
	colourOff(t)
	r := runsReport()
	r.Quality = domain.NewQualitySummary(manyKindFindings(30, domain.NamedRun(runIDOne)))
	s := screens.NewRunsScreen(r, 120, 12, paletteStyles())

	first := openFindings(s, 120, 12)
	assertWithinBounds(t, first, 120, 12)
	assertContains(t, stripStyling(first), "kind_00")
	assertNotContains(t, stripStyling(first), "kind_29")

	bottom := press(s, repeat("down", 100)...).View(120, 12)
	assertWithinBounds(t, bottom, 120, 12)
	assertContains(t, stripStyling(bottom), "kind_29")
	assertNotContains(t, stripStyling(bottom), "kind_00")

	top := press(s, repeat("up", 100)...).View(120, 12)
	assertContains(t, stripStyling(top), "kind_00")
}

func TestRunsFindings_HelpOverlayWinsWhileOpenAndFIsConsumedByIt(t *testing.T) {
	colourOff(t)
	s := newFindingsRuns(runsW, runsH)
	before := s.View(runsW, runsH)
	help := openOverlay(s, runsW, runsH)

	press(s, screens.KeyFindings)

	if s.FindingsOpen() {
		t.Error("f while the help overlay is open must not open the findings overlay")
	}
	if got := s.View(runsW, runsH); got != help {
		t.Errorf("f must change nothing while the help overlay is open\n%s\nvs\n%s", got, help)
	}
	if after := press(s, screens.KeyHelp).View(runsW, runsH); after != before {
		t.Errorf("closing the help overlay must show the list again\n%s\nvs\n%s", after, before)
	}
}

func TestRunsFindings_QuestionMarkWhileOpenDoesNotOpenTheHelpOverlay(t *testing.T) {
	colourOff(t)
	s := newFindingsRuns(runsW, runsH)
	open := openFindings(s, runsW, runsH)

	if got := press(s, screens.KeyHelp).View(runsW, runsH); got != open {
		t.Errorf("? must do nothing inside the findings overlay\n%s\nvs\n%s", got, open)
	}
}

func TestRunsFindings_ShortHelpListsFWhileClosedAndTheClosingKeysWhileOpen(t *testing.T) {
	colourOff(t)
	s := newFindingsRuns(runsW, runsH)

	if line := footerLine(s.View(runsW, runsH)); !hasEntry(line, "f findings") {
		t.Errorf("footer %q must list the entry %q", line, "f findings")
	}
	if help := s.Help(); !hasEntry(help, "f findings") {
		t.Errorf("Help() = %q must list %q", help, "f findings")
	}

	line := footerLine(openFindings(s, runsW, runsH))
	assertContains(t, line, "f/esc close", "scroll")
	assertNotContains(t, line, "f findings")
}

func TestRunsFindings_HelpOverlayListsTheFindingsKey(t *testing.T) {
	colourOff(t)

	overlay := runsOverlay(newFindingsRuns(runsW, 60), runsW, 60)

	if !regexp.MustCompile(`(?m)^\s+f\s+.*(?i:finding)`).MatchString(overlay) {
		t.Errorf("the runs list help overlay must list the findings key\n%s", overlay)
	}
}
