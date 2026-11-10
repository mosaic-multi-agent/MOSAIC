package cli_test

// Tests for the compaction figures and context bands of the report JSON.
//
// Rules under test:
//   - actor context.compactions gains harness and detected_only counts next to
//     logged and detected; detected keeps counting every drop.
//   - every context.detected_compactions[] entry keeps label "detected" and gains
//     matched_logged, harness, compaction_calls and compaction_call_cost.
//   - actor context gains compaction_calls with the count and cost of the actor's
//     compaction calls and of its unapplied attempts.
//   - the band key takes optimal, above_optimal or degraded; below_optimal never
//     appears. Existing keys keep their names; the per-run total keeps its shape.

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

func jccMoney(nanos int64) domain.MoneyValue { return domain.KnownMoney(domain.Money(nanos)) }

// jccContext has a logged compaction and three drops: one matched to it, one
// harness compaction (two compaction calls) and one detected only.
func jccContext() domain.ActorContext {
	a := jcOrchestratorContext()
	a.Calls = jcCalls(6)
	a.Compactions = []domain.CompactionMark{
		{Kind: domain.CompactionLogged, At: jcStart.Add(90 * time.Second), Before: 0, After: 1},
		{Kind: domain.CompactionDetected, At: jcStart.Add(1 * time.Minute), Before: 0, After: 1,
			BeforeTokens: 300_000, AfterTokens: 100_000, MatchedLogged: true},
		{Kind: domain.CompactionDetected, At: jcStart.Add(3 * time.Minute), Before: 2, After: 3,
			BeforeTokens: 200_000, AfterTokens: 50_000, Harness: true, CallCount: 2, CallCost: jccMoney(5_000_000)},
		{Kind: domain.CompactionDetected, At: jcStart.Add(5 * time.Minute), Before: 4, After: 5,
			BeforeTokens: 150_000, AfterTokens: 40_000},
	}
	a.CompactionCalls = domain.CompactionCallFigures{
		Count: 3, Cost: jccMoney(8_000_000), UnappliedCount: 1, UnappliedCost: jccMoney(3_000_000),
	}
	return a
}

func jccOrchestrator(t *testing.T) map[string]any {
	t.Helper()
	return jcOrchestratorObj(t, jcRun(jccContext(), jcAgentContext()))
}

func TestEncodeReport_Context_CompactionCountsGainHarnessAndDetectedOnly(t *testing.T) {
	counts := objectAt(t, jccOrchestrator(t), "compactions")

	want := map[string]float64{"logged": 1, "detected": 3, "harness": 1, "detected_only": 1}
	for key, w := range want {
		if counts[key] != w {
			t.Errorf("compactions.%s = %v, want %v (counts %v)", key, counts[key], w, counts)
		}
	}
}

func TestEncodeReport_Context_DetectedCompactionsStateTheirKindWithTwoFlags(t *testing.T) {
	list := vwList(t, jccOrchestrator(t), "detected_compactions")

	if len(list) != 3 {
		t.Fatalf("detected_compactions has %d entries, want every drop (3)", len(list))
	}
	want := []struct{ matched, harness bool }{{true, false}, {false, true}, {false, false}}
	for i, w := range want {
		d := vwObj(t, list[i])
		if d["label"] != "detected" {
			t.Errorf("entry %d label = %v, want detected on every entry", i, d["label"])
		}
		if d["matched_logged"] != w.matched || d["harness"] != w.harness {
			t.Errorf("entry %d matched_logged/harness = %v/%v, want %v/%v", i, d["matched_logged"], d["harness"], w.matched, w.harness)
		}
	}
}

func TestEncodeReport_Context_HarnessEntryCarriesItsCompactionCallCountAndCost(t *testing.T) {
	list := vwList(t, jccOrchestrator(t), "detected_compactions")

	d := vwObj(t, list[1])

	if d["compaction_calls"] != float64(2) {
		t.Errorf("compaction_calls = %v, want 2", d["compaction_calls"])
	}
	cost := vwObj(t, d["compaction_call_cost"])
	if cost["state"] != "known" || cost["amount"] != jccMoney(5_000_000).Amount.String() {
		t.Errorf("compaction_call_cost = %v, want the known cost of the two calls", cost)
	}
	if other := vwObj(t, list[2]); other["compaction_calls"] != float64(0) {
		t.Errorf("detected-only entry compaction_calls = %v, want 0", other["compaction_calls"])
	}
}

func TestEncodeReport_Context_CompactionCallsObjectCarriesCountsAndCosts(t *testing.T) {
	calls := objectAt(t, jccOrchestrator(t), "compaction_calls")

	if calls["count"] != float64(3) || calls["unapplied_count"] != float64(1) {
		t.Errorf("compaction_calls = %v, want count 3 and unapplied_count 1", calls)
	}
	for key, nanos := range map[string]int64{"cost": 8_000_000, "unapplied_cost": 3_000_000} {
		m := vwObj(t, calls[key])
		if m["state"] != "known" || m["amount"] != jccMoney(nanos).Amount.String() {
			t.Errorf("compaction_calls.%s = %v, want known %d nanos", key, m, nanos)
		}
	}
}

func TestEncodeReport_Context_ActorWithoutCompactionCallsStillHasTheObject(t *testing.T) {
	ctx := jcAgentObj(t, jcRun(jccContext(), jcAgentContext()))

	calls := objectAt(t, ctx, "compaction_calls")

	if calls["count"] != float64(0) || calls["unapplied_count"] != float64(0) {
		t.Errorf("compaction_calls = %v, want zero counts", calls)
	}
}

func TestEncodeReport_Context_ExistingKeysKeepTheirNames(t *testing.T) {
	ctx := jccOrchestrator(t)

	for _, key := range []string{"calls", "first", "peak", "final", "final_definition", "first_write",
		"compactions", "detected_compactions", "calls_by_kind"} {
		if _, ok := ctx[key]; !ok {
			t.Errorf("context lost the key %q", key)
		}
	}
	d := vwObj(t, vwList(t, ctx, "detected_compactions")[0])
	for _, key := range []string{"label", "from_at", "to_at", "before_tokens", "after_tokens"} {
		if _, ok := d[key]; !ok {
			t.Errorf("detected compaction lost the key %q", key)
		}
	}
}

func TestEncodeReport_ContextBands_NeverUseBelowOptimalAndKeepTheOtherKeys(t *testing.T) {
	var buf bytes.Buffer
	run := jcRun(jccContext(), jcAgentContext())
	run.Context.Orchestrator.First = jcFig(0, false)

	if err := cli.EncodeReport(&buf, vwReport(run)); err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "below_optimal") || strings.Contains(out, "below optimal") {
		t.Errorf("the report JSON names the removed below-optimal band")
	}
	ctx := jcOrchestratorObj(t, run)
	jcAssertFig(t, "context", ctx, "first", 0, "optimal", false)
	jcAssertFig(t, "context", ctx, "peak", 360_000, "degraded", true)
}

func TestEncodeRunTotal_RunWithCompactionFigures_KeepsTheSameFieldSet(t *testing.T) {
	total := totalWithAssumedTier(t, true)
	report := domain.Report{Runs: []domain.RunReport{{
		Run:     domain.NamedRun(cliTestRunID),
		Totals:  domain.Totals{Money: domain.CategoryMoney{Total: jccMoney(1_500_000_000), Complete: true}},
		Context: domain.RunContext{Orchestrator: jccContext()},
	}}}
	withFigures, ok := app.RunTotalFor(report, cliTestRunID)
	if !ok {
		t.Fatalf("RunTotalFor: run %s not found", cliTestRunID)
	}

	_, want := encodedKeys(t, total)
	_, got := encodedKeys(t, withFigures)

	assertKeySet(t, "top level", got, want)
}
