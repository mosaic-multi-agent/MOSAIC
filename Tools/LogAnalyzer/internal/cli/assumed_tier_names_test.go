package cli_test

// The text and JSON reports name the cache-write tier that was assumed.
//
// Rules under test:
//   - Text: the totals line and the report note carry "assumed <tier> tier" for
//     the tiers of the totals they describe ("assumed 5m/1h tiers" for both);
//     a 1h figure never reads "5m".
//   - JSON: totals gain the additive key assumed_cache_write_tiers, a list of
//     tier names in log-format order, omitted when no tier was assumed; the
//     existing token-count key keeps its meaning.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

const tiersKey = "assumed_cache_write_tiers"

var (
	hourTier  = []domain.CacheWriteTier{domain.CacheWriteTier1h}
	minTier   = []domain.CacheWriteTier{domain.CacheWriteTier5m}
	bothTier  = []domain.CacheWriteTier{domain.CacheWriteTier5m, domain.CacheWriteTier1h}
	unpricedK = []domain.PricingKey{{Model: "mystery-model"}}
)

func lineWithAll(out string, needles ...string) string {
	for _, line := range strings.Split(out, "\n") {
		ok := true
		for _, n := range needles {
			ok = ok && strings.Contains(line, n)
		}
		if ok {
			return line
		}
	}
	return ""
}

func TestEncodeText_RunTotalsLineNamesTheAssumedTier(t *testing.T) {
	run := runWithAssumedTier(12_345)
	run.Totals.AssumedTiers = hourTier
	out := encodeTextString(t, domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{run}})

	line := lineWithAll(out, "12,345", "assumed 1h tier")

	if line == "" {
		t.Fatalf("no line carries 12,345 and \"assumed 1h tier\"; output:\n%s", out)
	}
	if strings.Contains(out, "assumed 5m") {
		t.Errorf("a 1h-only report must not mention the 5m tier; output:\n%s", out)
	}
}

func TestEncodeText_ActorTotalsLineNamesItsOwnTier(t *testing.T) {
	run := runWithAssumedTier(0)
	run.Agents = []domain.ActorReport{{
		Actor:  domain.AgentInstance("Implementer#3"),
		Totals: domain.Totals{AssumedTierCacheWrite: 4_567, AssumedTiers: hourTier},
	}}
	out := encodeTextString(t, domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{run}})

	actorAt := strings.Index(out, "Agent: Implementer#3")
	if actorAt < 0 {
		t.Fatalf("actor block not found; output:\n%s", out)
	}
	if lineWithAll(out[actorAt:], "4,567", "assumed 1h tier") == "" {
		t.Errorf("actor line must carry 4,567 and \"assumed 1h tier\"; output:\n%s", out)
	}
}

func TestEncodeText_FiveMinuteTier_StillNamedAsFiveMinutes(t *testing.T) {
	run := runWithAssumedTier(900)
	run.Totals.AssumedTiers = minTier
	out := encodeTextString(t, domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{run}})

	if lineWithAll(out, "900", "assumed 5m tier") == "" {
		t.Errorf("no line carries 900 and \"assumed 5m tier\"; output:\n%s", out)
	}
}

func TestEncodeText_ReportNoteNamesTheAssumedTierAfterTheUnpricedSection(t *testing.T) {
	report := domain.Report{
		Currency:       domain.Currency,
		AllRuns:        domain.Totals{AssumedTierCacheWrite: 4_321, AssumedTiers: hourTier},
		UnpricedModels: unpricedK,
	}

	out := encodeTextString(t, report)

	unpricedAt := strings.Index(out, "mystery-model")
	noteAt := strings.LastIndex(out, "assumed 1h tier")
	if unpricedAt < 0 || noteAt < unpricedAt || !strings.Contains(out[noteAt:], "4,321") {
		t.Errorf("no note naming \"assumed 1h tier\" with 4,321 after the unpriced section; output:\n%s", out)
	}
	if strings.Contains(out, "assumed 5m") {
		t.Errorf("a 1h-only report must not mention the 5m tier; output:\n%s", out)
	}
}

func TestEncodeText_ReportNoteNamesBothTiersWhenRunsAndUnattributableDiffer(t *testing.T) {
	unattributable := domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: domain.Totals{AssumedTierCacheWrite: 8_765, AssumedTiers: hourTier},
	}
	report := domain.Report{
		Currency:       domain.Currency,
		AllRuns:        domain.Totals{AssumedTierCacheWrite: 4_321, AssumedTiers: minTier},
		Unattributable: &unattributable,
		UnpricedModels: unpricedK,
	}

	out := encodeTextString(t, report)

	noteAt := strings.LastIndex(out, "assumed 5m/1h tiers")
	if noteAt < strings.Index(out, "mystery-model") || !strings.Contains(out[noteAt:], "13,086") {
		t.Errorf("no note naming \"assumed 5m/1h tiers\" with 13,086 after the unpriced section; output:\n%s", out)
	}
}

func decodeTiers(t *testing.T, where string, totals map[string]any) []string {
	t.Helper()
	raw, ok := totals[tiersKey].([]any)
	if !ok {
		t.Fatalf("%s.%s = %v, want a list of tier names", where, tiersKey, totals[tiersKey])
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

func TestEncodeReport_AssumedTiers_AppearOnRunActorAllRunsAndUnattributableTotals(t *testing.T) {
	run := domain.RunReport{
		Run:          domain.NamedRun(cliTestRunID),
		Totals:       domain.Totals{AssumedTierCacheWrite: 300, AssumedTiers: bothTier},
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: domain.Totals{AssumedTierCacheWrite: 100, AssumedTiers: minTier}},
		Agents: []domain.ActorReport{{
			Actor:  domain.AgentInstance("Implementer#3"),
			Totals: domain.Totals{AssumedTierCacheWrite: 200, AssumedTiers: hourTier},
		}},
	}
	unattributable := domain.RunReport{
		Run:    domain.UnattributableRun(),
		Totals: domain.Totals{AssumedTierCacheWrite: 77, AssumedTiers: hourTier},
	}
	report := domain.Report{
		Currency:       domain.Currency,
		AllRuns:        domain.Totals{AssumedTierCacheWrite: 300, AssumedTiers: bothTier},
		Runs:           []domain.RunReport{run},
		Unattributable: &unattributable,
	}

	m := decodeReportJSON(t, report)

	want := map[string][]string{
		"all_runs":                    {"5m", "1h"},
		"runs[0].totals":              {"5m", "1h"},
		"runs[0].orchestrator.totals": {"5m"},
		"runs[0].agents[0].totals":    {"1h"},
		"unattributable.totals":       {"1h"},
	}
	jr := firstRun(t, m)
	agents, _ := jr["agents"].([]any)
	agent, _ := agents[0].(map[string]any)
	got := map[string][]string{
		"all_runs":                    decodeTiers(t, "all_runs", objectAt(t, m, "all_runs")),
		"runs[0].totals":              decodeTiers(t, "runs[0].totals", objectAt(t, jr, "totals")),
		"runs[0].orchestrator.totals": decodeTiers(t, "orchestrator", objectAt(t, objectAt(t, jr, "orchestrator"), "totals")),
		"runs[0].agents[0].totals":    decodeTiers(t, "agent", objectAt(t, agent, "totals")),
		"unattributable.totals":       decodeTiers(t, "unattributable", objectAt(t, objectAt(t, m, "unattributable"), "totals")),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("assumed tiers = %v, want %v", got, want)
	}
}

func TestEncodeReport_AssumedTiers_KeepTheExistingTokenCountKey(t *testing.T) {
	report := domain.Report{
		Currency: domain.Currency,
		AllRuns:  domain.Totals{AssumedTierCacheWrite: 300, AssumedTiers: hourTier},
	}

	m := decodeReportJSON(t, report)

	assertAssumedCount(t, "all_runs", objectAt(t, m, "all_runs"), 300)
}

func TestEncodeReport_NoAssumedTiers_OmitsTheTiersKeyEverywhere(t *testing.T) {
	run := domain.RunReport{
		Run:          domain.NamedRun(cliTestRunID),
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator()},
		Agents:       []domain.ActorReport{{Actor: domain.AgentInstance("Implementer#3")}},
	}
	report := domain.Report{Currency: domain.Currency, Runs: []domain.RunReport{run}}

	m := decodeReportJSON(t, report)

	jr := firstRun(t, m)
	for where, totals := range map[string]map[string]any{
		"all_runs":                    objectAt(t, m, "all_runs"),
		"runs[0].totals":              objectAt(t, jr, "totals"),
		"runs[0].orchestrator.totals": objectAt(t, objectAt(t, jr, "orchestrator"), "totals"),
	} {
		if v, present := totals[tiersKey]; present {
			t.Errorf("%s carries %s = %v although no tier was assumed", where, tiersKey, v)
		}
	}
}
