package screens_test

// Every share bar also shows its value as a whole percent. A non-zero share under
// 1% reads "<1%", a known zero reads "0%", and an unpriced cost reads the absent
// marker. Shares are told from rounded ones by fixtures whose rows hold 50%, 30%,
// 19%, 0.6% and 0.4% of the whole.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/tui/screens"
)

// assertRowPercents fails unless the row holding key shows exactly the percent
// texts in order.
func assertRowPercents(t *testing.T, view, key string, want ...string) {
	t.Helper()
	row := rowFor(t, view, key)
	if got := sharePercents(row); !reflect.DeepEqual(got, want) {
		t.Errorf("the row of %q shows the percents %q, want %q\n%s", key, got, want, row)
	}
}

// assertUnpricedShowsAbsentMarker fails unless the row holding key shows the absent
// marker in more places when its cost is unpriced than when the same cost is a known
// zero, and that known zero reads "0%".
func assertUnpricedShowsAbsentMarker(t *testing.T, key string, render func(last lastShare) string) {
	t.Helper()
	unpriced := rowFor(t, render(unpricedLast()), key)
	zero := rowFor(t, render(zeroCostLast()), key)

	if strings.Count(unpriced, screens.AbsentMarker) <= strings.Count(zero, screens.AbsentMarker) {
		t.Errorf("the unpriced row shows no absent marker for its share\nunpriced: %s\nknown zero: %s", unpriced, zero)
	}
	if got := sharePercents(zero); len(got) == 0 || got[len(got)-1] != "0%" {
		t.Errorf("a known zero cost must read 0%%, got %q\n%s", got, zero)
	}
}

func statusShareView(last lastShare) string {
	return stripStyling(screens.NewStatusScreen(statusShareRun(last), tableW, tallH, paletteStyles()).View(tableW, tallH))
}

func TestStatus_EveryGroupShowsItsDispatchShareAndItsCostShareAsWholePercents(t *testing.T) {
	colourOff(t)
	view := statusShareView(defaultLast())

	assertRowPercents(t, view, "$50.00", "40%", "50%")
	assertRowPercents(t, view, "$30.00", "25%", "30%")
	assertRowPercents(t, view, "$19.00", "34%", "19%")
}

func TestStatus_ShareUnderOnePercentReadsLessThanOne(t *testing.T) {
	colourOff(t)
	view := statusShareView(defaultLast())

	assertRowPercents(t, view, "$0.60", "<1%", "<1%")
	assertRowPercents(t, view, "$0.40", "<1%", "<1%")
}

func TestStatus_UnpricedCostShowsTheAbsentMarkerAsItsShare(t *testing.T) {
	colourOff(t)

	assertUnpricedShowsAbsentMarker(t, screens.OpenLabel, statusShareView)
}

func dispatchShareView(last lastShare) string {
	return stripStyling(dispatchesView(dispatchShareRun(last), tableW, tallH))
}

func TestDispatches_EveryRowShowsItsShareOfTheRunCostAsAWholePercent(t *testing.T) {
	colourOff(t)
	view := dispatchShareView(defaultLast())

	assertRowPercents(t, view, "orchestrator", "50%")
	assertRowPercents(t, view, "alpha#1", "30%")
	assertRowPercents(t, view, "beta#2", "19%")
	assertRowPercents(t, view, "gamma#3", "<1%")
	assertRowPercents(t, view, "delta#4", "<1%")
}

func TestDispatches_UnpricedCostShowsTheAbsentMarkerAsItsShare(t *testing.T) {
	colourOff(t)

	assertUnpricedShowsAbsentMarker(t, "delta#4", dispatchShareView)
}

func typeShareView(last lastShare) string {
	return stripStyling(screens.NewAgentTypesScreen(dispatchShareRun(last), tableW, tallH, paletteStyles()).View(tableW, tallH))
}

func TestAgentTypes_EveryTypeShowsItsShareOfTheRunCostAsAWholePercent(t *testing.T) {
	colourOff(t)
	view := typeShareView(defaultLast())

	assertRowPercents(t, view, "alpha", "30%")
	assertRowPercents(t, view, "beta", "19%")
	assertRowPercents(t, view, "gamma", "<1%")
	assertRowPercents(t, view, "delta", "<1%")
}

func TestAgentTypes_UnpricedCostShowsTheAbsentMarkerAsItsShare(t *testing.T) {
	colourOff(t)

	assertUnpricedShowsAbsentMarker(t, "delta", typeShareView)
}

func TestAgentTypeDrillDown_DispatchRowsShowTheirShareAsAWholePercent(t *testing.T) {
	colourOff(t)
	s := screens.NewAgentTypesScreen(dispatchShareRun(defaultLast()), tableW, tallH, paletteStyles())
	press(s, "enter") // alpha, the most expensive type

	assertRowPercents(t, stripStyling(s.View(tableW, tallH)), "alpha#1", "30%")
}

func modelShareView(last lastShare) string {
	return stripStyling(screens.NewOverviewScreen(modelShareRun(last), tableW, overviewTallH, paletteStyles()).View(tableW, overviewTallH))
}

func TestOverviewModels_EveryModelShowsItsShareOfTheRunCostAsAWholePercent(t *testing.T) {
	colourOff(t)
	view := modelShareView(defaultLast())

	assertRowPercents(t, view, "model-a", "50%")
	assertRowPercents(t, view, "model-b", "30%")
	assertRowPercents(t, view, "model-c", "19%")
	assertRowPercents(t, view, "model-d", "<1%")
	assertRowPercents(t, view, "model-e", "<1%")
}

func TestOverviewModels_UnpricedCostShowsTheAbsentMarkerAsItsShare(t *testing.T) {
	colourOff(t)

	assertUnpricedShowsAbsentMarker(t, "model-e", modelShareView)
}

func acrossShareView(last lastShare) string {
	return stripStyling(newAcross(acrossShareReport(last), tableW, tallH).View(tableW, tallH))
}

func TestAcrossRuns_EveryRunAndEveryTypeShowsItsShareAsAWholePercent(t *testing.T) {
	colourOff(t)
	view := acrossShareView(defaultLast())

	assertRowPercents(t, view, "20260101T120000Z-aaaa", "50%")
	assertRowPercents(t, view, "20260102T120000Z-bbbb", "30%")
	assertRowPercents(t, view, "20260103T120000Z-cccc", "19%")
	assertRowPercents(t, view, "20260104T120000Z-dddd", "<1%")
	assertRowPercents(t, view, "20260105T120000Z-eeee", "<1%")
	assertRowPercents(t, view, "coder", "70%")
	assertRowPercents(t, view, "tester", "29%")
	assertRowPercents(t, view, "scribe", "<1%")
	assertRowPercents(t, view, "oiler", "<1%")
}

func TestAcrossRuns_UnpricedCostShowsTheAbsentMarkerAsItsShare(t *testing.T) {
	colourOff(t)

	assertUnpricedShowsAbsentMarker(t, "20260105T120000Z-eeee", acrossShareView)
	assertUnpricedShowsAbsentMarker(t, "oiler", acrossShareView)
}

func TestTools_EveryToolShowsItsShareOfTheCallsAsAWholePercent(t *testing.T) {
	colourOff(t)
	view := stripStyling(screens.NewToolsScreen(toolShareRun(), tableW, tallH, paletteStyles()).View(tableW, tallH))

	assertRowPercents(t, view, "Alpha", "50%")
	assertRowPercents(t, view, "Beta", "30%")
	assertRowPercents(t, view, "Gamma", "19%")
	assertRowPercents(t, view, "Delta", "<1%")
	assertRowPercents(t, view, "Epsilon", "<1%")
}

func TestShareColumns_ViewsStayWithinTheTerminalAtEveryWidth(t *testing.T) {
	colourOff(t)
	for _, w := range []int{80, 120, 180} {
		views := map[string]string{
			"status":     screens.NewStatusScreen(statusShareRun(defaultLast()), w, tallH, paletteStyles()).View(w, tallH),
			"dispatches": dispatchesView(dispatchShareRun(defaultLast()), w, tallH),
			"types":      screens.NewAgentTypesScreen(dispatchShareRun(defaultLast()), w, tallH, paletteStyles()).View(w, tallH),
			"across":     newAcross(acrossShareReport(defaultLast()), w, tallH).View(w, tallH),
			"tools":      screens.NewToolsScreen(toolShareRun(), w, tallH, paletteStyles()).View(w, tallH),
		}
		for _, view := range views {
			assertWithinBounds(t, view, w, tallH)
		}
		assertWithinBounds(t, screens.NewOverviewScreen(modelShareRun(defaultLast()), w, overviewTallH, paletteStyles()).View(w, overviewTallH), w, overviewTallH)
	}
}
