package analysis_test

// Tests that the cache-write tier split survives aggregation.
//
// Rules under test:
//   - Each aggregate entry (one per usage_record, or per legacy turn /
//     invocation_end) carries the usage exactly as decoded, including the split
//     when present and no split when absent.
//   - Summing an actor's or a run's usage adds the tier fields absent-preserving:
//     a total-only entry never receives a tier, and the unsplit remainder of the
//     sum is the total of the entries that carried no split.
//   - The cache_creation total keeps meaning "all cache writes".

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const tierModel = domain.ModelID("tier-model")

func splitCacheUsage(total, fiveMin, oneHour int64) domain.TokenUsage {
	return domain.TokenUsage{
		Input:           domain.Tokens(1),
		CacheCreation:   domain.Tokens(total),
		CacheCreation5m: domain.Tokens(fiveMin),
		CacheCreation1h: domain.Tokens(oneHour),
	}
}

func totalOnlyCacheUsage(total int64) domain.TokenUsage {
	return domain.TokenUsage{Input: domain.Tokens(1), CacheCreation: domain.Tokens(total)}
}

func requireSplit(t *testing.T, name string, u domain.TokenUsage, fiveMin, oneHour int64) {
	t.Helper()
	assertTokenPresent(t, name+" CacheCreation5m", u.CacheCreation5m, fiveMin)
	assertTokenPresent(t, name+" CacheCreation1h", u.CacheCreation1h, oneHour)
	if !u.HasCacheWriteSplit() {
		t.Errorf("%s: HasCacheWriteSplit() = false, want true", name)
	}
}

func TestAggregate_UsageRecordWithSplit_EntryCarriesSplit(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		usageRecordEvent("a#1", "r1", tierModel, splitCacheUsage(150, 100, 50)))

	agg, _ := analysis.Aggregate(in)

	agent := requireOneAgent(t, requireOneRun(t, agg))
	if len(agent.Invocations) != 1 {
		t.Fatalf("got %d entries, want 1", len(agent.Invocations))
	}
	u := agent.Invocations[0].Usage
	requireSplit(t, "usage_record entry", u, 100, 50)
	assertTokenPresent(t, "CacheCreation total", u.CacheCreation, 150)
}

func TestAggregate_LegacyInvocationEndWithSplit_EntryCarriesSplit(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		invEndEvent("a#1", tierModel, splitCacheUsage(150, 100, 50)))

	agg, _ := analysis.Aggregate(in)

	agent := requireOneAgent(t, requireOneRun(t, agg))
	if len(agent.Invocations) != 1 {
		t.Fatalf("got %d entries, want 1", len(agent.Invocations))
	}
	requireSplit(t, "invocation_end entry", agent.Invocations[0].Usage, 100, 50)
}

func TestAggregate_OrchestratorTurnWithSplit_EntryCarriesSplit(t *testing.T) {
	in := orchestratorOnlyInput(turnEvent(tierModel, splitCacheUsage(150, 100, 50)))

	agg, _ := analysis.Aggregate(in)

	run := requireOneRun(t, agg)
	if len(run.Orchestrator.Invocations) != 1 {
		t.Fatalf("got %d entries, want 1", len(run.Orchestrator.Invocations))
	}
	requireSplit(t, "turn entry", run.Orchestrator.Invocations[0].Usage, 100, 50)
}

func TestAggregate_UsageWithoutSplit_EntryHasNoSplitAndNoInventedTier(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		usageRecordEvent("a#1", "r1", tierModel, totalOnlyCacheUsage(150)))

	agg, _ := analysis.Aggregate(in)

	u := requireOneAgent(t, requireOneRun(t, agg)).Invocations[0].Usage
	assertTokenAbsent(t, "CacheCreation5m", u.CacheCreation5m)
	assertTokenAbsent(t, "CacheCreation1h", u.CacheCreation1h)
	assertTokenPresent(t, "CacheCreation total", u.CacheCreation, 150)
	assertTokenPresent(t, "unsplit", u.UnsplitCacheCreation(), 150)
}

func TestAggregate_ActorTotals_SumTierFieldsAcrossRecords(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		usageRecordEvent("a#1", "r1", tierModel, splitCacheUsage(150, 100, 50)),
		usageRecordEvent("a#1", "r2", tierModel, splitCacheUsage(30, 10, 20)))

	agg, _ := analysis.Aggregate(in)

	totals := requireOneAgent(t, requireOneRun(t, agg)).Totals()
	requireSplit(t, "actor totals", totals, 110, 70)
	assertTokenPresent(t, "CacheCreation total", totals.CacheCreation, 180)
	assertTokenPresent(t, "unsplit", totals.UnsplitCacheCreation(), 0)
}

func TestAggregate_ActorTotals_MixedSplitAndTotalOnly_RemainderIsUnsplit(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		usageRecordEvent("a#1", "r1", tierModel, splitCacheUsage(150, 100, 50)),
		usageRecordEvent("a#1", "r2", tierModel, totalOnlyCacheUsage(40)))

	agg, _ := analysis.Aggregate(in)

	totals := requireOneAgent(t, requireOneRun(t, agg)).Totals()
	assertTokenPresent(t, "CacheCreation total", totals.CacheCreation, 190)
	assertTokenPresent(t, "CacheCreation5m", totals.CacheCreation5m, 100)
	assertTokenPresent(t, "CacheCreation1h", totals.CacheCreation1h, 50)
	assertTokenPresent(t, "unsplit", totals.UnsplitCacheCreation(), 40)
}

func TestAggregate_ActorTotals_AllTotalOnly_TiersStayAbsent(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		usageRecordEvent("a#1", "r1", tierModel, totalOnlyCacheUsage(40)),
		usageRecordEvent("a#1", "r2", tierModel, totalOnlyCacheUsage(60)))

	agg, _ := analysis.Aggregate(in)

	totals := requireOneAgent(t, requireOneRun(t, agg)).Totals()
	assertTokenPresent(t, "CacheCreation total", totals.CacheCreation, 100)
	assertTokenAbsent(t, "CacheCreation5m", totals.CacheCreation5m)
	assertTokenAbsent(t, "CacheCreation1h", totals.CacheCreation1h)
}

func TestAggregate_RunTotals_SumTierFieldsAcrossOrchestratorAndAgents(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{
			turnEvent(tierModel, splitCacheUsage(150, 100, 50)),
		}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{
			invEndEvent("a#1", tierModel, splitCacheUsage(30, 10, 20)),
		}},
	}}

	agg, _ := analysis.Aggregate(in)

	totals := requireOneRun(t, agg).Totals()
	requireSplit(t, "run totals", totals, 110, 70)
	assertTokenPresent(t, "CacheCreation total", totals.CacheCreation, 180)
}

func TestAggregate_TierSplit_DoesNotChangeFindings(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		usageRecordEvent("a#1", "r1", tierModel, splitCacheUsage(150, 100, 50)))

	_, findings := analysis.Aggregate(in)

	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}
