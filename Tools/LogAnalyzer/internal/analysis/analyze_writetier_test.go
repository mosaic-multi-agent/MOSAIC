package analysis_test

// Tests for pricing unsplit cache writes at the default cache-write tier of the
// request's harness.
//
// Fixtures are the tier fixtures of analyze_pricing_test.go (harness
// claude-code, model "tier-model" with flat rates: 5m 3.75 / 1h 6.00 per 1M
// tokens). Nanodollars for n tokens at rate r are n * r * 1000.
//
// Rules under test:
//   - A harness set to 1h prices its unsplit cache writes at the 1h rate; the
//     other harnesses stay at 5m.
//   - Requests with a reported split are priced as reported.
//   - Assumed-tier tokens are still counted, and every totals figure names the
//     distinct assumed tiers in the order of domain.CacheWriteTiers().

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func settingsWithTier(h domain.Harness, tier domain.CacheWriteTier) domain.AnalysisSettings {
	s := domain.DefaultAnalysisSettings()
	s.CacheWriteTiers = map[domain.Harness]domain.CacheWriteTier{h: tier}
	return s
}

func analyzeTierFixturesWith(t *testing.T, settings domain.AnalysisSettings) domain.Report {
	t.Helper()
	table := domain.NewPricingTable([]domain.ModelPricing{flatModelPricing(t, tierModel)})
	return analysis.Analyze(tierFixtureInput(t), table, settings, testSource(), nil)
}

func assertAssumedTiers(t *testing.T, name string, got domain.Totals, want ...domain.CacheWriteTier) {
	t.Helper()
	if len(got.AssumedTiers) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got.AssumedTiers, want) {
		t.Errorf("%s: AssumedTiers = %v, want %v", name, got.AssumedTiers, want)
	}
}

func TestAnalyze_HarnessSetToOneHour_PricesUnsplitCacheWritesAtOneHourRate(t *testing.T) {
	report := analyzeTierFixturesWith(t, settingsWithTier(domain.HarnessClaudeCode, domain.CacheWriteTier1h))

	run := requireRunReport(t, report, tierRunA)
	worker := requireActorReport(t, run, "worker#1")

	// r1 30_000 and r3 50_000 unsplit at 6.00, r2 as reported: 15_000 x 3.75 + 5_000 x 6.00.
	assertMoneyKnown(t, "worker#1 CacheCreation", worker.Totals.Money.CacheCreation, 566_250_000)
	assertMoneyKnown(t, "orchestrator CacheCreation", run.Orchestrator.Totals.Money.CacheCreation, 60_000_000) // 10_000 x 6.00
	assertMoneyKnown(t, "unattributable CacheCreation", report.Unattributable.Totals.Money.CacheCreation, 12_000_000)
}

func TestAnalyze_HarnessSetToOneHour_StillCountsAssumedTokensAndNamesTheTier(t *testing.T) {
	report := analyzeTierFixturesWith(t, settingsWithTier(domain.HarnessClaudeCode, domain.CacheWriteTier1h))

	run := requireRunReport(t, report, tierRunA)
	worker := requireActorReport(t, run, "worker#1")

	assertAssumedTokens(t, "worker#1", worker.Totals, 80_000)
	assertAssumedTiers(t, "worker#1", worker.Totals, domain.CacheWriteTier1h)
	assertAssumedTiers(t, "orchestrator", run.Orchestrator.Totals, domain.CacheWriteTier1h)
	assertAssumedTiers(t, "run t401", run.Totals, domain.CacheWriteTier1h)
	assertAssumedTiers(t, "AllRuns", report.AllRuns, domain.CacheWriteTier1h)
	assertAssumedTiers(t, "unattributable", report.Unattributable.Totals, domain.CacheWriteTier1h)
}

func TestAnalyze_SettingForAnotherHarness_LeavesClaudeCodeAtFiveMinutes(t *testing.T) {
	report := analyzeTierFixturesWith(t, settingsWithTier(domain.HarnessVSCodeGHCP, domain.CacheWriteTier1h))

	run := requireRunReport(t, report, tierRunA)
	worker := requireActorReport(t, run, "worker#1")

	assertMoneyKnown(t, "worker#1 CacheCreation", worker.Totals.Money.CacheCreation, 386_250_000)
	assertAssumedTiers(t, "worker#1", worker.Totals, domain.CacheWriteTier5m)
}

func TestAnalyze_NoTierSetting_PricesAtFiveMinutesAndNamesFiveMinutes(t *testing.T) {
	report := analyzeTierFixturesWith(t, domain.DefaultAnalysisSettings())

	run := requireRunReport(t, report, tierRunA)

	assertMoneyKnown(t, "orchestrator CacheCreation", run.Orchestrator.Totals.Money.CacheCreation, 37_500_000) // 10_000 x 3.75
	assertAssumedTiers(t, "run t401", run.Totals, domain.CacheWriteTier5m)
	assertAssumedTiers(t, "AllRuns", report.AllRuns, domain.CacheWriteTier5m)
}

func TestAnalyze_RunWithOnlySplitRequests_NamesNoAssumedTier(t *testing.T) {
	report := analyzeTierFixturesWith(t, settingsWithTier(domain.HarnessClaudeCode, domain.CacheWriteTier1h))

	runB := requireRunReport(t, report, tierRunB)

	assertAssumedTokens(t, "run t402", runB.Totals, 0)
	assertAssumedTiers(t, "run t402", runB.Totals)
}

func TestAnalyze_ViewsCarryTheAssumedTier(t *testing.T) {
	report := analyzeTierFixturesWith(t, settingsWithTier(domain.HarnessClaudeCode, domain.CacheWriteTier1h))

	run := requireRunReport(t, report, tierRunA)

	if len(run.ByAgentType) == 0 {
		t.Fatal("run has no agent-type view")
	}
	groupsChecked, dispatchesChecked := 0, 0
	for _, group := range run.ByAgentType {
		if group.Figures.Totals.AssumedTierCacheWrite == 0 {
			continue
		}
		groupsChecked++
		assertAssumedTiers(t, "agent type "+group.AgentType, group.Figures.Totals, domain.CacheWriteTier1h)
		for _, d := range group.Dispatches {
			if d.Totals.AssumedTierCacheWrite > 0 {
				dispatchesChecked++
				assertAssumedTiers(t, "dispatch "+d.Invocation.Actor.Label(), d.Totals, domain.CacheWriteTier1h)
			}
		}
	}
	if groupsChecked == 0 || dispatchesChecked == 0 {
		t.Fatalf("checked %d agent-type groups and %d dispatches with assumed tokens, want at least one of each",
			groupsChecked, dispatchesChecked)
	}
}

// mixedHarnessAggregate is one agent whose two requests come from different
// harnesses; both carry 100_000 cache-write tokens without a split.
func mixedHarnessAggregate() domain.Aggregate {
	unsplit := domain.TokenUsage{CacheCreation: domain.Tokens(100_000)}
	return singleAgentAggregate(domain.NamedRun(tierRunA), "worker#1",
		domain.InvocationUsage{Model: tierModel, Harness: domain.HarnessClaudeCode, Usage: unsplit},
		domain.InvocationUsage{Model: tierModel, Harness: domain.HarnessVSCodeGHCP, Usage: unsplit},
	)
}

func priceMixedHarnesses(t *testing.T, settings domain.AnalysisSettings) domain.Report {
	t.Helper()
	table := domain.NewPricingTable([]domain.ModelPricing{flatModelPricing(t, tierModel)})
	report, _ := analysis.Price(mixedHarnessAggregate(), table, settings, testSource())
	return report
}

func TestPrice_TwoHarnessesInOneActor_EachRequestUsesItsOwnHarnessTier(t *testing.T) {
	report := priceMixedHarnesses(t, settingsWithTier(domain.HarnessClaudeCode, domain.CacheWriteTier1h))

	worker := requireActorReport(t, requireRunReport(t, report, tierRunA), "worker#1")

	// 100_000 x 6.00 (claude-code) + 100_000 x 3.75 (vscode-ghcp)
	assertMoneyKnown(t, "worker#1 CacheCreation", worker.Totals.Money.CacheCreation, 975_000_000)
	assertAssumedTokens(t, "worker#1", worker.Totals, 200_000)
}

func TestPrice_TwoHarnessesWithDifferentTiers_NamesBothTiersInLogFormatOrder(t *testing.T) {
	report := priceMixedHarnesses(t, settingsWithTier(domain.HarnessVSCodeGHCP, domain.CacheWriteTier1h))

	run := requireRunReport(t, report, tierRunA)
	worker := requireActorReport(t, run, "worker#1")

	assertAssumedTiers(t, "worker#1", worker.Totals, domain.CacheWriteTier5m, domain.CacheWriteTier1h)
	assertAssumedTiers(t, "run", run.Totals, domain.CacheWriteTier5m, domain.CacheWriteTier1h)
	assertAssumedTiers(t, "AllRuns", report.AllRuns, domain.CacheWriteTier5m, domain.CacheWriteTier1h)
}

func TestPrice_TwoHarnessesWithTheSameTier_NamesThatTierOnce(t *testing.T) {
	settings := settingsWithTier(domain.HarnessClaudeCode, domain.CacheWriteTier1h)
	settings.CacheWriteTiers[domain.HarnessVSCodeGHCP] = domain.CacheWriteTier1h

	report := priceMixedHarnesses(t, settings)

	worker := requireActorReport(t, requireRunReport(t, report, tierRunA), "worker#1")
	assertMoneyKnown(t, "worker#1 CacheCreation", worker.Totals.Money.CacheCreation, 1_200_000_000) // 200_000 x 6.00
	assertAssumedTiers(t, "worker#1", worker.Totals, domain.CacheWriteTier1h)
}
