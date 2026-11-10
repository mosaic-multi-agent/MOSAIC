package screens_test

// Colour of the changes in the comparison view: increase bad, decrease good,
// no change muted. The direction also reads as a sign with colour off (see the
// table tests).

import (
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// The first row can hold the cursor, so the rows checked here are not the first
// of the default order (reviewer).

func TestCompareRoles_IncreaseOfCostAndTokensIsDrawnInTheBadRole(t *testing.T) {
	colourOn(t)
	row := rawLineWith(t, compareView(tableW, tallH), "builder")

	assertDrawnInRole(t, row, "+$5.00", tuicommon.RoleSignalBad)
	assertDrawnInRole(t, row, "+2,000", tuicommon.RoleSignalBad)
	if screens.ChangeRole(domain.ChangeIncrease) != tuicommon.RoleSignalBad {
		t.Errorf("an increase must map to the bad role")
	}
}

func TestCompareRoles_DecreaseOfCostAndTokensIsDrawnInTheGoodRole(t *testing.T) {
	colourOn(t)
	row := rawLineWith(t, compareView(tableW, tallH), "linter")

	assertDrawnInRole(t, row, "-$1.00", tuicommon.RoleSignalGood)
	assertDrawnInRole(t, row, "-100", tuicommon.RoleSignalGood)
}

func TestCompareRoles_NoChangeIsDrawnInTheMutedRole(t *testing.T) {
	colourOn(t)
	row := rawLineWith(t, compareView(tableW, tallH), "tester")

	assertDrawnInRole(t, row, "0", tuicommon.RoleSignalMuted)
}

func TestCompareRoles_TotalsRowChangeUsesTheSameRoles(t *testing.T) {
	colourOn(t)
	line := rawLineWith(t, compareView(tableW, tallH), "$25.00")

	assertDrawnInRole(t, line, "-$5.00", tuicommon.RoleSignalGood)
	assertDrawnInRole(t, line, "+1,450", tuicommon.RoleSignalBad)
}
