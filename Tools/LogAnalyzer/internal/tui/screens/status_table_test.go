package screens_test

// The By status view as a table: groups coloured by the status mapping, the
// dispatch-share and cost-share bars, sort cycling with the active sort in the
// header, and the bounds and markers with colour off.

import (
	"regexp"
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// statusRoleRun has a most expensive BLOCKED group, so the row that can hold the
// cursor is not one of the rows whose role is checked.
func statusRoleRun() domain.RunReport {
	run := overviewRun()
	run.ByStatus = domain.StatusBreakdown{Run: []domain.StatusGroup{
		statusGroup(domain.StatusReported, "BLOCKED", 9, 5000),
		statusGroup(domain.StatusReported, "SUCCESS", 7, 1234),
		statusGroup(domain.StatusReported, "CAPABILITY_EXCEEDED", 1, 999),
		statusGroup(domain.StatusReported, "NEEDS_CLARIFICATION", 2, 321),
		statusGroup(domain.StatusReported, "WEIRD_CODE", 1, 77),
		statusGroup(domain.StatusNone, "", 3, 456),
		openStatusGroup(2, 111),
	}}
	return run
}

// statusSharesRun is a run whose groups differ in only one of dispatches and cost.
func statusSharesRun(groups ...domain.StatusGroup) domain.RunReport {
	run := overviewRun()
	run.ByStatus = domain.StatusBreakdown{Run: groups}
	return run
}

var barRunRE = regexp.MustCompile(`[█▉▊▋▌▍▎▏]+`)

func statusSortAscending() map[screens.SortKey][]string {
	return map[screens.SortKey][]string{
		screens.SortCost:        {"$0.77", "$1.11", "$2.50", "$4.56", "$12.34"},
		screens.SortInvocations: {"WEIRD_CODE", screens.NoStatusLabel, "SUCCESS"},
	}
}

func TestStatusTable_GroupNameIsDrawnInTheRoleOfTheStatusMapping(t *testing.T) {
	colourOn(t)
	run := statusRoleRun()
	view := statusView(run, wideW, tallH)

	rows := []struct {
		amount, text string
		group        domain.StatusGroup
	}{
		{"$12.34", "SUCCESS", run.ByStatus.Run[1]},
		{"$9.99", "CAPABILITY_EXCEEDED", run.ByStatus.Run[2]},
		{"$3.21", "NEEDS_CLARIFICATION", run.ByStatus.Run[3]},
		{"$0.77", "WEIRD_CODE", run.ByStatus.Run[4]},
		{"$4.56", screens.NoStatusLabel, run.ByStatus.Run[5]},
		{"$1.11", screens.OpenLabel, run.ByStatus.Run[6]},
	}
	for _, r := range rows {
		assertDrawnInRole(t, rawLineWith(t, view, r.amount), r.text, screens.StatusGroupRole(r.group))
	}
}

func TestStatusTable_SuccessIsGoodBlockedIsBadAndEverythingElseIsAWarning(t *testing.T) {
	run := statusRoleRun()
	want := []tuicommon.StyleRole{
		tuicommon.RoleSignalBad, tuicommon.RoleSignalGood, tuicommon.RoleSignalBad, tuicommon.RoleSignalWarning,
		tuicommon.RoleSignalWarning, tuicommon.RoleSignalWarning, tuicommon.RoleSignalWarning,
	}

	for i, g := range run.ByStatus.Run {
		if got := screens.StatusGroupRole(g); got != want[i] {
			t.Errorf("group %d (%q, kind %d): role %q, want %q", i, g.Code, g.Kind, got, want[i])
		}
	}
}

func TestStatusTable_ShareBarsGrowWithTheGroupsShareOfDispatchesAndCost(t *testing.T) {
	colourOff(t)
	view := statusView(statusRun(), wideW, tallH)

	success, blocked, weird := barWeight(rowFor(t, view, "$12.34")), barWeight(rowFor(t, view, "$2.50")),
		barWeight(rowFor(t, view, "$0.77"))
	none, open := barWeight(rowFor(t, view, "$4.56")), barWeight(rowFor(t, view, "$1.11"))

	if !(success > blocked && blocked > weird && weird > 0) {
		t.Errorf("bars must follow SUCCESS > BLOCKED > WEIRD_CODE: got %d %d %d\n%s", success, blocked, weird, view)
	}
	if !(none > open && open > 0) {
		t.Errorf("bars must follow no status > open: got %d %d\n%s", none, open, view)
	}
}

func TestStatusTable_EachGroupRowHasADispatchShareBarAndACostShareBar(t *testing.T) {
	colourOff(t)

	row := rowFor(t, statusView(statusRun(), wideW, tallH), "$12.34")

	if n := len(barRunRE.FindAllString(row, -1)); n < 2 {
		t.Errorf("the row needs two separate bars (dispatches and cost), found %d: %q", n, row)
	}
}

func TestStatusTable_DispatchShareBarFollowsTheDispatchCountAlone(t *testing.T) {
	colourOff(t)
	run := statusSharesRun(
		statusGroup(domain.StatusReported, "ALPHA", 8, 100),
		statusGroup(domain.StatusReported, "BETA", 2, 100))

	view := statusView(run, wideW, tallH)

	if a, b := barWeight(rowFor(t, view, "ALPHA")), barWeight(rowFor(t, view, "BETA")); a <= b {
		t.Errorf("8 dispatches must draw more than 2 at equal cost: %d vs %d\n%s", a, b, view)
	}
}

func TestStatusTable_CostShareBarFollowsTheCostAlone(t *testing.T) {
	colourOff(t)
	run := statusSharesRun(
		statusGroup(domain.StatusReported, "GAMMA", 3, 900),
		statusGroup(domain.StatusReported, "DELTA", 3, 100))

	view := statusView(run, wideW, tallH)

	if g, d := barWeight(rowFor(t, view, "GAMMA")), barWeight(rowFor(t, view, "DELTA")); g <= d {
		t.Errorf("$9.00 must draw more than $1.00 at equal dispatches: %d vs %d\n%s", g, d, view)
	}
}

func TestStatusTable_ValuesAreAlignedWhateverTheCodeLength(t *testing.T) {
	colourOff(t)
	for _, width := range []int{tableW, 100, 80} {
		view := statusView(statusRun(), width, tallH)

		a := displayColumn(rowFor(t, view, "$12.34"), "$12.34")
		b := displayColumn(rowFor(t, view, "$2.50"), "$2.50")
		c := displayColumn(rowFor(t, view, "$0.77"), "$0.77")

		// Costs are aligned when they start at one column or end at one.
		if a < 0 || b < 0 || c < 0 || (b != c) || (a != b && a+len("$12.34") != b+len("$2.50")) {
			t.Errorf("width %d: costs are not aligned (columns %d, %d, %d)\n%s", width, a, b, c, view)
		}
	}
}

func TestStatusTable_IsReadableFromEightyColumnsAndStaysWithinTheTerminal(t *testing.T) {
	colourOff(t)
	for _, width := range []int{120, 100, 80, 60, 40, 30} {
		for _, height := range []int{5, 8, 12, 24, 40} {
			view := statusView(statusRun(), width, height)

			assertWithinBounds(t, view, width, height)
		}
	}
	view := statusView(statusRun(), 80, 40)
	for _, code := range []string{"SUCCESS", "BLOCKED", "WEIRD_CODE", screens.NoStatusLabel, screens.OpenLabel} {
		assertContains(t, view, code)
	}
	assertContains(t, view, "$12.34", "$2.50", "$0.77", "$4.56", "$1.11")
}

func TestStatusTable_DefaultSortIsCostDescendingAndHeaderShowsIt(t *testing.T) {
	colourOff(t)

	view := statusView(statusRun(), wideW, tallH)

	key, desc := activeSort(t, view)
	if key != screens.SortCost || !desc {
		t.Errorf("default sort = %q desc=%v, want cost descending", key, desc)
	}
}

func TestStatusTable_SortKeyCyclesThroughEveryOrderAndWraps(t *testing.T) {
	colourOff(t)
	s := screens.NewStatusScreen(statusRun(), tableW, tallH, paletteStyles())

	seen := walkSorts(t, s, tableW, tallH, statusSortAscending(), screens.SortCost, screens.SortInvocations, screens.SortName)

	if len(seen) == 0 || seen[0] != screens.SortCost {
		t.Errorf("the cycle must start on cost, got %v", seen)
	}
}

func TestStatusTable_SortingNeverDropsAGroupOrAnAgentType(t *testing.T) {
	colourOff(t)
	s := screens.NewStatusScreen(statusRun(), tableW, tallH, paletteStyles())

	for i := 0; i < 5; i++ {
		view := s.View(tableW, tallH)
		for _, want := range []string{"$12.34", "$2.50", "$0.77", "$4.56", "$1.11", "$7.07", "$0.40", "$3.03", "$0.09", "$0.88"} {
			assertContains(t, view, want)
		}
		assertAppearInOrder(t, view, []string{"builder", "reviewer", screens.UnknownTypeLabel}, true)
		press(s, screens.KeySort)
	}
}

func TestStatusTable_SortKeyIsListedInTheHelpOverlay(t *testing.T) {
	colourOff(t)
	s := screens.NewStatusScreen(statusRun(), 120, tallH, paletteStyles())

	overlay := strings.Join(overlayLines(openOverlay(s, 120, tallH)), "\n")

	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(screens.KeySort) + `\s+.*(?i:sort)`).MatchString(overlay) {
		t.Errorf("the help overlay must list the sort key %q\n%s", screens.KeySort, overlay)
	}
}

func TestStatusTable_WithColourOffEveryMarkerStaysAsTextAndNoEscapeSequenceRemains(t *testing.T) {
	colourOff(t)
	run := statusRun()
	run.ByStatus.Run[0].Figures.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	run.ByStatus.Run[0].Figures.Totals.Money = domain.CategoryMoney{Total: known(1234), Complete: false}
	run.ByStatus.Run[1].Figures.Totals.AssumedTierCacheWrite = 4321
	run.ByStatus.Run[3].Figures.MissingUsage = 1
	run.ByStatus.Run[2].Figures.Totals.Money = domain.CategoryMoney{Total: domain.UnpricedMoney()}

	view := statusView(run, tableW, tallH)

	if view != stripStyling(view) {
		t.Errorf("colour off must leave no escape sequences: %q", view)
	}
	assertContains(t, view, "(partial)", screens.UnpricedMarker, screens.AssumedTierLabel(nil), screens.MissingUsageLabel,
		screens.NoStatusLabel, screens.OpenLabel)
	assertNotContains(t, view, "$0.00")
}

func TestStatusTable_AgentTypeBreakdownIsKeptWithItsGroupsInTheSameTableStyle(t *testing.T) {
	colourOff(t)
	view := statusView(statusRun(), wideW, tallH)

	for _, row := range []struct{ amount, code string }{
		{"$7.07", "SUCCESS"}, {"$0.40", screens.NoStatusLabel}, {"$3.03", "SUCCESS"}, {"$0.09", screens.OpenLabel}, {"$0.88", "BLOCKED"},
	} {
		line := rowFor(t, view, row.amount)
		if !strings.Contains(line, row.code) {
			t.Errorf("agent-type row %q must name its group %q", line, row.code)
		}
	}
}
