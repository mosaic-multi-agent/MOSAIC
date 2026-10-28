package screens_test

// The cache block of the visual Overview: the hit gauge, the net cache benefit
// (a negative benefit in the bad role) and the run's unread cache writes, shown
// as reported data with the existing absent, unpriced and partial markers.

import (
	"regexp"
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	cacheHitPattern    = `(?i)cache hit`
	netBenefitPattern  = `(?i)net (cache )?benefit`
	unreadWritePattern = `(?i)unread cache writes?`

	// zeroPercentPattern is a whole 0.0% figure, not the tail of 60.0% or 40.0%.
	zeroPercentPattern = `(^|[^0-9.])0\.0%`
	// negativeBenefitPattern is a well-formed -$5.67 in either sign placement.
	negativeBenefitPattern = `(-\$5\.67|\$-5\.67)`
)

// negativeBenefit is the run of the negative net benefit tests: -$5.67.
func negativeBenefit() domain.RunReport {
	run := analysesRun()
	run.Cache.NetBenefit = domain.KnownMoney(-usd(567))
	return run
}

// assertNegativeBenefitInRole fails unless the well-formed negative amount on the
// raw line sits right after the opening of role.
func assertNegativeBenefitInRole(t *testing.T, rawLine string, role tuicommon.StyleRole) {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(rolePrefix(role)) + `\[?\s*` + negativeBenefitPattern)
	if !re.MatchString(rawLine) {
		t.Errorf("a negative benefit of -$5.67 must be drawn in role %q: %q", role, rawLine)
	}
}

// gaugeWeight is the filled length of the lines around the cache hit label, in
// eighths of a cell. Everything else on those lines is the same whatever the
// ratio, so only the gauge makes it differ.
func gaugeWeight(view string) int { return barWeight(windowAround(view, cacheHitPattern, 1)) }

func TestOverviewCache_GaugeFillsInProportionToTheHitRatio(t *testing.T) {
	colourOff(t)
	high := overviewView(withCacheHit(analysesRun(), 3000, 4500), overviewTallH)
	low := overviewView(withCacheHit(analysesRun(), 900, 4500), overviewTallH)
	none := overviewView(withCacheHit(analysesRun(), 0, 4500), overviewTallH)

	if !(gaugeWeight(high) > gaugeWeight(low) && gaugeWeight(low) > gaugeWeight(none)) {
		t.Errorf("gauge fill must grow with the ratio: 66.7%% -> %d, 20.0%% -> %d, 0.0%% -> %d",
			gaugeWeight(high), gaugeWeight(low), gaugeWeight(none))
	}
	assertContains(t, windowAround(high, cacheHitPattern, 1), "66.7%")
	assertMatches(t, firstLineMatching(none, cacheHitPattern), zeroPercentPattern)
}

func TestOverviewCache_AbsentRatioDrawsNoGaugeAndShowsTheMarker(t *testing.T) {
	colourOff(t)
	absent := overviewView(withCacheHit(analysesRun(), 0, 0), overviewTallH)
	zero := overviewView(withCacheHit(analysesRun(), 0, 4500), overviewTallH)

	window := windowAround(absent, cacheHitPattern, 1)

	assertContains(t, window, screens.AbsentMarker)
	if regexp.MustCompile(zeroPercentPattern).MatchString(firstLineMatching(absent, cacheHitPattern)) {
		t.Errorf("an absent ratio is not shown as 0.0%%\n%s", window)
	}
	if gaugeWeight(absent) != gaugeWeight(zero) {
		t.Errorf("an absent ratio must fill nothing, like zero: %d vs %d", gaugeWeight(absent), gaugeWeight(zero))
	}
}

func TestOverviewCache_NetBenefitShowsItsAmount(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	window := windowAround(view, netBenefitPattern, 1)
	if window == "" {
		t.Fatalf("the cache block must name the net cache benefit\n%s", view)
	}
	assertContains(t, window, "$12.34")
}

func TestOverviewCache_NegativeNetBenefitIsDrawnInTheBadRoleAndPositiveIsNot(t *testing.T) {
	colourOn(t)
	view := overviewView(negativeBenefit(), overviewTallH)

	bad := rawLineMatching(t, view, negativeBenefitPattern)
	assertNegativeBenefitInRole(t, bad, tuicommon.RoleSignalBad)

	positive := rawLineWith(t, overviewView(analysesRun(), overviewTallH), "$12.34")
	if strings.Contains(positive, rolePrefix(tuicommon.RoleSignalBad)+"$12.34") {
		t.Errorf("a positive benefit must not be drawn in the bad role: %q", positive)
	}
}

func TestOverviewCache_NegativeNetBenefitKeepsItsSignAsTextWithColourOff(t *testing.T) {
	colourOff(t)

	view := overviewView(negativeBenefit(), overviewTallH)

	assertMatches(t, windowAround(view, netBenefitPattern, 1), negativeBenefitPattern)
}

func TestOverviewCache_UnpricedOrMissingNetBenefitIsNeverZeroDollars(t *testing.T) {
	colourOff(t)
	for name, value := range map[string]domain.MoneyValue{"unpriced": domain.UnpricedMoney(), "no data": domain.NoMoneyData()} {
		t.Run(name, func(t *testing.T) {
			run := analysesRun()
			run.Cache.NetBenefit = value

			view := overviewView(run, overviewTallH)

			assertNotContains(t, view, "$0.00", "$12.34")
			if window := windowAround(view, netBenefitPattern, 1); window != "" &&
				!strings.Contains(window, screens.UnpricedMarker) && !strings.Contains(window, screens.AbsentMarker) {
				t.Errorf("a shown net benefit without a value needs a marker\n%s", window)
			}
		})
	}
}

func TestOverviewCache_UnreadWritesShowTokensAndCostAsReportedDataNextToTheCacheBlock(t *testing.T) {
	colourOff(t)

	view := overviewView(analysesRun(), overviewTallH)

	window := windowAround(view, unreadWritePattern, 2)
	if window == "" {
		t.Fatalf("the cache block must show the unread cache writes\n%s", view)
	}
	assertContains(t, window, "777", "$4.56")
	assertContains(t, strings.ToLower(window), "reported data")
	if d := patternLine(view, unreadWritePattern) - patternLine(view, cacheHitPattern); d < -8 || d > 8 {
		t.Errorf("unread cache writes sit %d lines from the cache hit gauge, want them in the same block", d)
	}
}

func TestOverviewCache_UnreadWritesAreNotPresentedAsAnEstimate(t *testing.T) {
	colourOff(t)

	window := strings.ToLower(windowAround(overviewView(analysesRun(), overviewTallH), unreadWritePattern, 2))

	if window == "" {
		t.Fatal("the cache block must show the unread cache writes")
	}
	assertNotContains(t, window, "estimate", "approx", "~")
}

func TestOverviewCache_UnavailableUnreadWritesShowTheMarkerNotZero(t *testing.T) {
	colourOff(t)
	run := analysesRun()
	run.Cache.UnreadWriteTokens = domain.AbsentTokens()
	run.Cache.UnreadWriteCost = domain.NoMoneyData()

	window := windowAround(overviewView(run, overviewTallH), unreadWritePattern, 2)

	if window == "" {
		t.Fatal("the unread cache writes keep their place when they are unavailable")
	}
	assertContains(t, window, screens.AbsentMarker)
	assertNotContains(t, window, "$0.00", "777", "$4.56")
}

func TestOverviewCache_UnreadWriteCostIsMarkedUnpricedOrPartial(t *testing.T) {
	colourOff(t)
	unpriced := analysesRun()
	unpriced.Cache.UnreadWriteCost = domain.UnpricedMoney()
	partial := analysesRun()
	partial.Cache.Complete = false

	unpricedWindow := windowAround(overviewView(unpriced, overviewTallH), unreadWritePattern, 2)
	partialWindow := windowAround(overviewView(partial, overviewTallH), unreadWritePattern, 2)

	assertContains(t, unpricedWindow, screens.UnpricedMarker)
	assertNotContains(t, unpricedWindow, "$0.00")
	assertContains(t, partialWindow, "$4.56 (partial)")
}

func TestOverviewCache_OnlyTheRunLevelFiguresAreShownNotTheOrchestratorsOwn(t *testing.T) {
	colourOff(t)
	run := analysesRun()
	run.OrchestratorCache = domain.CacheFigures{
		UnreadWriteTokens: domain.Tokens(31337), UnreadWriteCost: known(8765), NetBenefit: known(4321),
	}

	view := overviewView(run, overviewTallH)

	assertNotContains(t, view, "31,337", "$87.65", "$43.21")
}
