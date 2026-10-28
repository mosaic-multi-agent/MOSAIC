package screens_test

// The cache columns of the Dispatches view: cache-hit ratio, cache-write tokens
// and cost, net benefit and the unread cache writes (tokens and cost of the cache
// writes of the dispatch's last call). Absent figures show the absent marker and
// never 0.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// withDispatch replaces the dispatch with the instance id in its agent type group.
func withDispatch(run domain.RunReport, id string, change func(*domain.Dispatch)) domain.RunReport {
	for gi := range run.ByAgentType {
		for di := range run.ByAgentType[gi].Dispatches {
			if string(run.ByAgentType[gi].Dispatches[di].Invocation.Actor.Instance) == id {
				change(&run.ByAgentType[gi].Dispatches[di])
			}
		}
	}
	return run
}

func TestDispatches_CacheColumnsShowEachDispatchsOwnFigures(t *testing.T) {
	colourOff(t)
	view := dispatchesView(dispatchRun(), tableW, tallH)

	for _, c := range []struct {
		id    string
		wants []string
	}{
		{"builder#1", []string{"75.0%", "12,345", "$0.77", "$1.11", "4,567", "$0.29"}},
		{longInstance, []string{"12.5%", "98,765", "$0.66", "$0.46", "55,555", "$0.33"}},
		{"reviewer#1", []string{"66.7%", "3,210", "$0.19", "$0.12", "1,234", "$0.08"}},
		{"builder#2", []string{"50.0%", "7,777", "$0.11", "$0.05"}},
	} {
		assertContains(t, rowFor(t, view, c.id), c.wants...)
	}
}

func TestDispatches_CacheFiguresOfOneDispatchNeverAppearOnAnotherRow(t *testing.T) {
	colourOff(t)
	view := dispatchesView(dispatchRun(), tableW, tallH)

	assertNotContains(t, rowFor(t, view, "builder#1"), "98,765", "55,555", "$0.33", "12.5%")
	assertNotContains(t, rowFor(t, view, longInstance), "12,345", "4,567", "$0.29", "75.0%")
}

func TestDispatches_UnreadCacheWritesAreLabelledAsSuch(t *testing.T) {
	colourOff(t)

	view := strings.ToLower(dispatchesView(dispatchRun(), tableW, tallH))

	assertContains(t, view, "unread cache write")
}

func TestDispatches_AnAbsentUnreadFigureShowsTheMarkerNotZero(t *testing.T) {
	colourOff(t)
	absent := dispatchesView(dispatchRun(), tableW, tallH)
	zero := dispatchesView(withDispatch(dispatchRun(), "builder#2", func(d *domain.Dispatch) {
		d.Cache.UnreadWriteTokens = domain.Tokens(0)
		d.Cache.UnreadWriteCost = known(0)
	}), tableW, tallH)

	absentRow, zeroRow := rowFor(t, absent, "builder#2"), rowFor(t, zero, "builder#2")

	if got := countOf(absentRow, "—") - countOf(zeroRow, "—"); got != 2 {
		t.Errorf("an absent unread figure must show the absent marker for tokens and cost, %d more markers than for a present zero: %q vs %q",
			got, absentRow, zeroRow)
	}
	assertNotContains(t, absentRow, "$0.00")
	assertContains(t, zeroRow, "$0.00")
}

func TestDispatches_DispatchWithoutCacheFiguresShowsMarkersAndNoZeroPercent(t *testing.T) {
	colourOff(t)
	absent := dispatchesView(dispatchRun(), tableW, tallH)
	zero := dispatchesView(withDispatch(dispatchRun(), "builder#3", func(d *domain.Dispatch) {
		d.Cache = domain.CacheFigures{
			HitRatio: domain.Ratio{Numerator: 0, Denominator: 100}, WriteTokens: domain.Tokens(0), WriteCost: known(0),
			NetBenefit: known(0), UnreadWriteTokens: domain.Tokens(0), UnreadWriteCost: known(0), Complete: true,
		}
	}), tableW, tallH)

	absentRow, zeroRow := rowFor(t, absent, "builder#3"), rowFor(t, zero, "builder#3")

	if got := countOf(absentRow, "—") - countOf(zeroRow, "—"); got < 4 {
		t.Errorf("absent cache figures must show the marker where present zeros show 0 (%d more markers): %q vs %q", got, absentRow, zeroRow)
	}
	assertNotContains(t, absentRow, "0.0%", "$0.00")
	assertContains(t, zeroRow, "0.0%", "$0.00")
}

func TestDispatches_OrchestratorRowShowsItsOwnCacheFiguresNotADispatchsOrTheRuns(t *testing.T) {
	colourOff(t)

	row := rowFor(t, dispatchesView(dispatchRun(), tableW, tallH), "orchestrator")

	assertContains(t, row, "70.0%", "31,313", "2,222")
	assertNotContains(t, row, "1,111", "6,543", "99,999", "4,567", "55,555")
}

func TestDispatches_OrchestratorRowTakesItsCacheFiguresFromTheOrchestratorCacheField(t *testing.T) {
	colourOff(t)
	run := dispatchRun()
	run.OrchestratorCache = cacheOf(1, 3, 45_454, 41, 17, 8_642, 14)

	row := rowFor(t, dispatchesView(run, tableW, tallH), "orchestrator")

	assertContains(t, row, "33.3%", "45,454", "8,642")
	assertNotContains(t, row, "2,222", "31,313", "70.0%")
}

func TestDispatches_OrchestratorWithoutAnUnreadCacheWriteShowsNoNumberForIt(t *testing.T) {
	colourOff(t)
	absentRun := dispatchRun()
	absentRun.OrchestratorCache.UnreadWriteTokens = domain.AbsentTokens()
	absentRun.OrchestratorCache.UnreadWriteCost = domain.NoMoneyData()
	zeroRun := dispatchRun()
	zeroRun.OrchestratorCache.UnreadWriteTokens = domain.Tokens(0)
	zeroRun.OrchestratorCache.UnreadWriteCost = known(0)

	absentRow := rowFor(t, dispatchesView(absentRun, tableW, tallH), "orchestrator")
	zeroRow := rowFor(t, dispatchesView(zeroRun, tableW, tallH), "orchestrator")

	assertNotContains(t, absentRow, "2,222", "1,111", "6,543", "$0.00")
	if got := countOf(absentRow, "—") - countOf(zeroRow, "—"); got != 2 {
		t.Errorf("an absent unread figure must show the absent marker for tokens and cost, %d more markers than for a present zero: %q vs %q",
			got, absentRow, zeroRow)
	}
	assertContains(t, zeroRow, "$0.00")
}

func TestDispatches_CacheColumnsDropBeforeCostPeakAndHitRatioAsTheWidthShrinks(t *testing.T) {
	colourOff(t)
	// Drop order of the design: cost (kept) > peak context > cache hit > cache write > unread writes.
	figures := []string{"$3.17", "120,000", "75.0%", "12,345", "4,567"}
	for _, width := range []int{240, 200, 160, 140, 120, 100, 90, 80, 70, 60} {
		view := dispatchesView(dispatchRun(), width, tallH)

		assertWithinBounds(t, view, width, tallH)
		row := rowFor(t, view, "$3.17") // the first dispatch's row, whatever is left of its name
		for i := 1; i < len(figures); i++ {
			if strings.Contains(row, figures[i]) && !strings.Contains(row, figures[i-1]) {
				t.Errorf("width %d: %q is shown but %q, which must outlast it, is not: %q", width, figures[i], figures[i-1], row)
			}
		}
	}
}
