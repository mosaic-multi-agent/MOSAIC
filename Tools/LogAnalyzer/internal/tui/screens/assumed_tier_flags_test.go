package screens_test

// Every view that flags incomplete figures through the shared incompleteness
// labels names the tier that was assumed, not a fixed 5m tier.
//
// Rules under test:
//   - The overview (both versions), the status table, the by-type table and the
//     across-runs run and type rows carry "assumed 1h tier" for a 1h figure and
//     never "assumed 5m".

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

func TestOverview_AssumedTierFlagNamesTheTierInBothVersions(t *testing.T) {
	colourOff(t)
	run := overviewRun()
	run.Overview.Totals.AssumedTierCacheWrite = 12345
	run.Overview.Totals.AssumedTiers = oneHour
	run.Totals = run.Overview.Totals

	for _, v := range overviewVersions() {
		t.Run(v.name, func(t *testing.T) {
			view := v.view(run, tallH)

			assertContains(t, view, screens.FormatAssumedTier(12345, oneHour))
			assertNotContains(t, view, "assumed 5m")
		})
	}
}

func TestStatus_AssumedTierFlagNamesTheTier(t *testing.T) {
	colourOff(t)
	run := statusRun()
	run.ByStatus.Run[1].Figures.Totals.AssumedTierCacheWrite = 12345
	run.ByStatus.Run[1].Figures.Totals.AssumedTiers = oneHour

	line := rowFor(t, statusView(run, wideW, tallH), "$2.50")

	assertContains(t, line, screens.FormatAssumedTier(12345, oneHour))
	assertNotContains(t, line, "assumed 5m")
}

func TestAgentTypesTable_AssumedTierFlagNamesTheTier(t *testing.T) {
	colourOff(t)
	run := typesFiguresRun()
	run.ByAgentType[0].Figures.Totals.AssumedTierCacheWrite = 4321
	run.ByAgentType[0].Figures.Totals.AssumedTiers = oneHour

	row := rowFor(t, typesFiguresView(run, tableW, tallH), "builder")

	assertContains(t, row, screens.FormatAssumedTier(4321, oneHour))
	assertNotContains(t, row, "assumed 5m")
}

func TestAcrossRuns_RunRowFlagNamesTheAssumedTier(t *testing.T) {
	report := acrossReport()
	report.AcrossRuns.Runs[2].Totals.AssumedTiers = oneHour
	report.AcrossRuns.ByAgentType[1].Figures.Totals.AssumedTiers = oneHour

	view := acrossView(t, report, tableW, tallH)

	assertContains(t, view, screens.FormatAssumedTier(1234, oneHour), screens.FormatAssumedTier(99, oneHour))
	assertNotContains(t, view, "assumed 5m")
}

func TestAcrossRuns_RowsNameDifferentAssumedTiersOfTheirOwnFigures(t *testing.T) {
	report := acrossReport()
	report.AcrossRuns.Runs[2].Totals.AssumedTiers = []domain.CacheWriteTier{domain.CacheWriteTier5m}
	report.AcrossRuns.ByAgentType[1].Figures.Totals.AssumedTiers = oneHour

	view := acrossView(t, report, tableW, tallH)

	assertContains(t, view, "assumed 5m tier", "assumed 1h tier", "1,234", "99")
}
