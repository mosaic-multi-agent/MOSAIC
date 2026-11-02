package cli_test

// The rewrite figures of the report JSON.
//
// Rules under test:
//   - Every cache object gains "rewrites" with the count of each kind, tokens,
//     cost, extra_cost and complete; the existing cache keys stay.
//   - Actor context gains "rewrites", one entry per rewrite with the time of the
//     call, kind key, label, pause in seconds, tokens, cost and extra cost; the
//     previous model is present for model_changed only.
//   - The per-run total keeps its field set.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

func jrFigures() domain.RewriteFigures {
	return domain.RewriteFigures{
		ModelChanged: 1, ColdIdle: 2, Partial: 3,
		Tokens: domain.Tokens(123_456),
		Cost:   vwKnown(6_370_000_000), ExtraCost: vwKnown(5_910_000_000),
		Complete: true,
	}
}

func jrAssertFigures(t *testing.T, name string, cache map[string]any) {
	t.Helper()
	rw := objectAt(t, cache, "rewrites")
	for key, want := range map[string]float64{"model_changed": 1, "cold_idle": 2, "partial": 3, "tokens": 123_456} {
		if rw[key] != want {
			t.Errorf("%s rewrites.%s = %v, want %v", name, key, rw[key], want)
		}
	}
	for key, nanos := range map[string]int64{"cost": 6_370_000_000, "extra_cost": 5_910_000_000} {
		m := vwObj(t, rw[key])
		if m["state"] != "known" || m["amount"] != vwKnown(nanos).Amount.String() {
			t.Errorf("%s rewrites.%s = %v, want known %d nanos", name, key, m, nanos)
		}
	}
	if rw["complete"] != true {
		t.Errorf("%s rewrites.complete = %v, want true", name, rw["complete"])
	}
}

func jrRun() domain.RunReport {
	run := jaRun()
	run.Cache.Rewrites = jrFigures()
	run.ByAgentType[0].Cache.Rewrites = jrFigures()
	run.ByAgentType[0].Dispatches[0].Cache.Rewrites = jrFigures()
	return run
}

func TestEncodeReport_Cache_RunCarriesRewriteFigures(t *testing.T) {
	jrAssertFigures(t, "run cache", objectAt(t, jaRunObj(t, jrRun()), "cache"))
}

func TestEncodeReport_Cache_AgentTypeAndDispatchCarryRewriteFigures(t *testing.T) {
	group := vwObj(t, vwList(t, jaRunObj(t, jrRun()), "by_agent_type")[0])
	dispatch := vwObj(t, vwList(t, group, "dispatches")[0])

	jrAssertFigures(t, "group cache", objectAt(t, group, "cache"))
	jrAssertFigures(t, "dispatch cache", objectAt(t, dispatch, "cache"))
}

func TestEncodeReport_Cache_RewriteFiguresAreAdditiveAndTheExistingKeysStay(t *testing.T) {
	cache := objectAt(t, jaRunObj(t, jrRun()), "cache")

	for _, key := range []string{"hit_ratio", "write_tokens", "write_cost", "net_benefit",
		"unread_write_tokens", "unread_write_cost", "complete", "rewrites"} {
		if _, ok := cache[key]; !ok {
			t.Errorf("cache lost or lacks the key %q", key)
		}
	}
}

func TestEncodeReport_Cache_RunWithoutRewritesHasZeroCounts(t *testing.T) {
	cache := objectAt(t, jaRunObj(t, jaRun()), "cache")

	rw := objectAt(t, cache, "rewrites")

	for _, key := range []string{"model_changed", "cold_idle", "partial"} {
		if rw[key] != float64(0) {
			t.Errorf("rewrites.%s = %v, want 0", key, rw[key])
		}
	}
}

func TestEncodeReport_Cache_UnpricedRewriteMoneyKeepsItsStateAndTheFiguresAreIncomplete(t *testing.T) {
	run := jaRun()
	run.Cache.Rewrites = domain.RewriteFigures{
		ColdIdle: 1, Tokens: domain.Tokens(100_000), Cost: domain.UnpricedMoney(), ExtraCost: domain.UnpricedMoney(),
	}

	rw := objectAt(t, objectAt(t, jaRunObj(t, run), "cache"), "rewrites")

	if vwObj(t, rw["cost"])["state"] != "unpriced" || vwObj(t, rw["extra_cost"])["state"] != "unpriced" || rw["complete"] != false {
		t.Errorf("rewrites = %v, want unpriced cost and extra_cost and complete false", rw)
	}
}

// jrContext has three calls (a minute apart) and a rewrite of each kind.
func jrContext() domain.ActorContext {
	a := jcOrchestratorContext()
	a.Rewrites = []domain.ContextRewrite{
		{Call: 1, Kind: domain.RewriteColdIdle, Pause: 10 * time.Minute, Tokens: 100_000,
			Cost: vwKnown(375_000_000), ExtraCost: vwKnown(345_000_000)},
		{Call: 2, Kind: domain.RewriteModelChanged, Pause: 90 * time.Second, PreviousModel: "model-a", Tokens: 60_000,
			Cost: vwKnown(225_000_000), ExtraCost: vwKnown(207_000_000)},
		{Call: 2, Kind: domain.RewritePartial, Pause: 30 * time.Second, Tokens: 20_000,
			Cost: vwKnown(75_000_000), ExtraCost: vwKnown(69_000_000)},
	}
	return a
}

func TestEncodeReport_Context_RewritesListEachRewriteWithItsFigures(t *testing.T) {
	list := vwList(t, jcOrchestratorObj(t, jcRun(jrContext(), jcAgentContext())), "rewrites")

	if len(list) != 3 {
		t.Fatalf("rewrites has %d entries, want 3", len(list))
	}
	cold := vwObj(t, list[0])
	if cold["kind"] != "cold_idle" || cold["label"] != "cold after 10 min idle" || cold["pause_seconds"] != float64(600) || cold["tokens"] != float64(100_000) {
		t.Errorf("cold entry = %v", cold)
	}
	if want := jcStart.Add(time.Minute).Format(time.RFC3339Nano); cold["at"] != want {
		t.Errorf("cold entry at = %v, want %v (the time of its call)", cold["at"], want)
	}
	for key, nanos := range map[string]int64{"cost": 375_000_000, "extra_cost": 345_000_000} {
		m := vwObj(t, cold[key])
		if m["state"] != "known" || m["amount"] != vwKnown(nanos).Amount.String() {
			t.Errorf("cold entry %s = %v, want known %d nanos", key, m, nanos)
		}
	}
	if _, has := cold["previous_model"]; has {
		t.Errorf("cold entry carries previous_model %v, want it for model_changed only", cold["previous_model"])
	}
}

func TestEncodeReport_Context_ModelChangedRewriteCarriesThePreviousModelAlone(t *testing.T) {
	list := vwList(t, jcOrchestratorObj(t, jcRun(jrContext(), jcAgentContext())), "rewrites")

	model := vwObj(t, list[1])
	partial := vwObj(t, list[2])

	if model["kind"] != "model_changed" || model["label"] != "model changed" || model["previous_model"] != "model-a" || model["pause_seconds"] != float64(90) {
		t.Errorf("model changed entry = %v", model)
	}
	if partial["kind"] != "partial" || partial["label"] != "partial rewrite" {
		t.Errorf("partial entry = %v", partial)
	}
	if _, has := partial["previous_model"]; has {
		t.Errorf("partial entry carries previous_model %v", partial["previous_model"])
	}
}

func TestEncodeReport_Context_ActorWithoutRewritesHasAnEmptyList(t *testing.T) {
	ctx := jcAgentObj(t, jcRun(jrContext(), jcAgentContext()))

	list := vwList(t, ctx, "rewrites")

	if len(list) != 0 {
		t.Errorf("rewrites = %v, want an empty array", list)
	}
}

func TestEncodeRunTotal_RunWithRewriteFigures_KeepsTheSameFieldSet(t *testing.T) {
	plain := totalWithAssumedTier(t, true)
	run := domain.RunReport{
		Run:     domain.NamedRun(cliTestRunID),
		Totals:  domain.Totals{Money: domain.CategoryMoney{Total: vwKnown(1_500_000_000), Complete: true}},
		Cache:   domain.CacheFigures{Rewrites: jrFigures()},
		Context: domain.RunContext{Orchestrator: jrContext()},
	}
	withRewrites, ok := app.RunTotalFor(domain.Report{Runs: []domain.RunReport{run}}, cliTestRunID)
	if !ok {
		t.Fatalf("RunTotalFor: run %s not found", cliTestRunID)
	}

	_, want := encodedKeys(t, plain)
	_, got := encodedKeys(t, withRewrites)

	assertKeySet(t, "top level", got, want)
}
