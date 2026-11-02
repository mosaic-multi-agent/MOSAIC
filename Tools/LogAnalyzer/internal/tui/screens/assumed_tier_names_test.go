package screens_test

// The assumed-tier markings name the tier that was assumed.
//
// Rules under test:
//   - The label reads "assumed <tier> tier" for one tier and
//     "assumed <a>/<b> tiers" for several, in the order given.
//   - The labelled count, the notice and the flag cells of the run views carry
//     the tiers of the totals they describe; a 1h figure never reads "5m".

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var (
	oneHour   = []domain.CacheWriteTier{domain.CacheWriteTier1h}
	fiveMin   = []domain.CacheWriteTier{domain.CacheWriteTier5m}
	bothTiers = []domain.CacheWriteTier{domain.CacheWriteTier5m, domain.CacheWriteTier1h}
)

func TestAssumedTierLabel_NamesTheAssumedTiers(t *testing.T) {
	cases := []struct {
		name  string
		tiers []domain.CacheWriteTier
		want  string
	}{
		{"one hour", oneHour, "assumed 1h tier"},
		{"five minutes", fiveMin, "assumed 5m tier"},
		{"both tiers", bothTiers, "assumed 5m/1h tiers"},
	}
	for _, c := range cases {
		if got := screens.AssumedTierLabel(c.tiers); got != c.want {
			t.Errorf("%s: AssumedTierLabel = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFormatAssumedTier_NamesTheTierBesideTheGroupedCount(t *testing.T) {
	got := screens.FormatAssumedTier(12345, oneHour)

	assertContains(t, got, "assumed 1h tier", "12,345")
	assertNotContains(t, got, "5m")
}

func TestFormatAssumedTier_ZeroTokensIsEmptyWhateverTheTiers(t *testing.T) {
	if got := screens.FormatAssumedTier(0, oneHour); got != "" {
		t.Errorf("FormatAssumedTier(0, 1h) = %q, want empty", got)
	}
}

func TestAssumedTierNotice_NamesTheTier(t *testing.T) {
	got := screens.AssumedTierNotice(7_777, oneHour)

	assertContains(t, got, "assumed 1h tier", "7,777")
	assertNotContains(t, got, "5m")
}

func TestDispatches_RunNoticeNamesTheAssumedTier(t *testing.T) {
	colourOff(t)
	run := dispatchRun()
	run.Totals.AssumedTierCacheWrite = 7_777
	run.Totals.AssumedTiers = oneHour

	view := dispatchesView(run, tableW, tallH)

	assertContains(t, view, screens.AssumedTierNotice(7_777, oneHour))
	assertNotContains(t, view, "assumed 5m")
}

func TestDispatches_DispatchFlagNamesTheAssumedTier(t *testing.T) {
	colourOff(t)
	run := withDispatch(dispatchRun(), "builder#4", func(d *domain.Dispatch) {
		d.Totals.AssumedTierCacheWrite = 4_321
		d.Totals.AssumedTiers = oneHour
	})

	row := rowFor(t, dispatchesView(run, tableW, tallH), "builder#4")

	assertContains(t, row, screens.FormatAssumedTier(4_321, oneHour))
	assertNotContains(t, row, "assumed 5m")
}

func TestDispatches_OrchestratorFlagNamesTheAssumedTier(t *testing.T) {
	colourOff(t)
	run := dispatchRun()
	run.Orchestrator.Totals.AssumedTierCacheWrite = 555
	run.Orchestrator.Totals.AssumedTiers = oneHour

	row := rowForName(t, dispatchesView(run, tableW, tallH), "orchestrator", 6)

	assertContains(t, row, screens.FormatAssumedTier(555, oneHour))
	assertNotContains(t, row, "assumed 5m")
}

func TestAgentTypes_AssumedTierFlagNamesTheTier(t *testing.T) {
	run := agentTypesRun()
	run.ByAgentType[0].Figures.Totals.AssumedTierCacheWrite = 4321
	run.ByAgentType[0].Figures.Totals.AssumedTiers = oneHour

	view := typesView(run, tallH)

	line := lineWith(view, "4,321")
	if !strings.Contains(line, "assumed 1h tier") || strings.Contains(line, "5m") {
		t.Errorf("assumed-tier line %q must name the 1h tier only", line)
	}
}

func TestRuns_AssumedTierNoticeNamesTheTiersOfAllRuns(t *testing.T) {
	colourOff(t)
	report := runsReport()
	report.AllRuns.AssumedTierCacheWrite = 4_321
	report.AllRuns.AssumedTiers = oneHour

	view := runsView(report, tableW, tallH)

	assertContains(t, view, screens.AssumedTierNotice(4_321, oneHour))
	assertNotContains(t, view, "assumed 5m")
}

func TestRuns_AssumedTierNoticeNamesBothTiersWhenRunsAndUnattributableDiffer(t *testing.T) {
	colourOff(t)
	report := runsReport()
	report.AllRuns.AssumedTierCacheWrite = 4_000
	report.AllRuns.AssumedTiers = fiveMin
	report.Unattributable = &domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: domain.Totals{AssumedTierCacheWrite: 321, AssumedTiers: oneHour},
	}

	view := runsView(report, tableW, tallH)

	assertContains(t, view, "assumed 5m/1h tiers", "4,321")
}
