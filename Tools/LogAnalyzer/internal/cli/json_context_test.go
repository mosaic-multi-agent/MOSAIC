package cli_test

// Tests for the per-actor context and compaction figures in the report JSON.
//
// Rules under test:
//   - The orchestrator object and every agents[] object carry a "context"
//     object with the call count, first/peak/final figures, the definition of
//     final context, the first-write figure (labelled heuristic), logged and
//     detected compaction counts and the detected compactions themselves.
//   - A context figure is {tokens, band, partial}; an absent figure is JSON
//     null, never 0.
//   - Existing actor fields keep their names.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

var jcStart = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)

func jcFig(tokens int64, partial bool) domain.ContextFigure {
	return domain.ContextFigure{Tokens: domain.Tokens(tokens), Band: domain.BandOf(tokens), Partial: partial}
}

func jcCalls(n int) []domain.ContextCall {
	calls := make([]domain.ContextCall, n)
	for i := range calls {
		calls[i] = domain.ContextCall{At: jcStart.Add(time.Duration(i) * time.Minute), RecordID: "r"}
	}
	return calls
}

// jcOrchestratorContext has 3 calls, one logged and one detected compaction.
func jcOrchestratorContext() domain.ActorContext {
	return domain.ActorContext{
		Actor: domain.Orchestrator(),
		Calls: jcCalls(3),
		First: jcFig(50_000, false),
		Peak:  jcFig(360_000, true),
		Final: jcFig(120_000, false),
		Compactions: []domain.CompactionMark{
			{Kind: domain.CompactionLogged, At: jcStart.Add(90 * time.Second), Before: 1, After: 2},
			{Kind: domain.CompactionDetected, At: jcStart.Add(2 * time.Minute), Before: 1, After: 2,
				BeforeTokens: 360_000, AfterTokens: 120_000},
		},
	}
}

func jcAgentContext() domain.ActorContext {
	return domain.ActorContext{
		Actor:        domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"},
		Calls:        jcCalls(2),
		First:        jcFig(10_000, false),
		Peak:         jcFig(260_000, false),
		Final:        jcFig(260_000, false),
		FirstWrite:   jcFig(10_000, false),
		FirstWriteAt: jcStart.Add(30 * time.Second),
	}
}

// jcRun is a run with one agent (coder#1) and context for both actors.
func jcRun(orch, agent domain.ActorContext) domain.RunReport {
	run := vwRun()
	run.Agents = []domain.ActorReport{{
		Actor: domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"},
	}}
	run.Context = domain.RunContext{Orchestrator: orch, Invocations: []domain.ActorContext{agent}}
	return run
}

func jcOrchestratorObj(t *testing.T, run domain.RunReport) map[string]any {
	t.Helper()
	return objectAt(t, objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(run))), "orchestrator"), "context")
}

func jcAgentObj(t *testing.T, run domain.RunReport) map[string]any {
	t.Helper()
	agent := vwObj(t, vwList(t, firstRun(t, decodeReportJSON(t, vwReport(run))), "agents")[0])
	return objectAt(t, agent, "context")
}

func jcAssertFig(t *testing.T, name string, m map[string]any, key string, tokens float64, band string, partial bool) {
	t.Helper()
	fig := objectAt(t, m, key)
	if fig["tokens"] != tokens || fig["band"] != band || fig["partial"] != partial {
		t.Errorf("%s.%s = %v, want tokens %v band %q partial %v", name, key, fig, tokens, band, partial)
	}
}

func TestEncodeReport_OrchestratorContext_CarriesFirstPeakFinalWithBands(t *testing.T) {
	ctx := jcOrchestratorObj(t, jcRun(jcOrchestratorContext(), jcAgentContext()))

	if ctx["calls"] != float64(3) {
		t.Errorf("context.calls = %v, want 3", ctx["calls"])
	}
	jcAssertFig(t, "context", ctx, "first", 50_000, "below_optimal", false)
	jcAssertFig(t, "context", ctx, "peak", 360_000, "degraded", true)
	jcAssertFig(t, "context", ctx, "final", 120_000, "optimal", false)
}

func TestEncodeReport_Context_StatesTheFinalContextDefinition(t *testing.T) {
	ctx := jcOrchestratorObj(t, jcRun(jcOrchestratorContext(), jcAgentContext()))

	if ctx["final_definition"] != domain.FinalContextDefinition {
		t.Errorf("context.final_definition = %v, want %q", ctx["final_definition"], domain.FinalContextDefinition)
	}
}

func TestEncodeReport_OrchestratorContext_FirstWriteIsAlwaysNull(t *testing.T) {
	ctx := jcOrchestratorObj(t, jcRun(jcOrchestratorContext(), jcAgentContext()))

	if v, present := ctx["first_write"]; !present || v != nil {
		t.Errorf("orchestrator context.first_write = %v (present %v), want null", v, present)
	}
}

func TestEncodeReport_Context_CompactionCountsSeparateLoggedFromDetected(t *testing.T) {
	ctx := jcOrchestratorObj(t, jcRun(jcOrchestratorContext(), jcAgentContext()))

	counts := objectAt(t, ctx, "compactions")

	if counts["logged"] != float64(1) || counts["detected"] != float64(1) {
		t.Errorf("context.compactions = %v, want logged 1 and detected 1", counts)
	}
}

func TestEncodeReport_Context_DetectedCompactionsAreLabelledAndListedWithTokensAndTimes(t *testing.T) {
	orch := jcOrchestratorContext()
	ctx := jcOrchestratorObj(t, jcRun(orch, jcAgentContext()))

	detected := vwList(t, ctx, "detected_compactions")

	if len(detected) != 1 {
		t.Fatalf("detected_compactions has %d entries, want 1 (the logged mark is not listed)", len(detected))
	}
	d := vwObj(t, detected[0])
	if d["label"] != "detected" || d["before_tokens"] != float64(360_000) || d["after_tokens"] != float64(120_000) {
		t.Errorf("detected compaction = %v, want label detected, 360000 before, 120000 after", d)
	}
	wantFrom, wantTo := orch.Calls[1].At, orch.Calls[2].At
	for key, want := range map[string]time.Time{"from_at": wantFrom, "to_at": wantTo} {
		s, _ := d[key].(string)
		if got, err := time.Parse(time.RFC3339Nano, s); err != nil || !got.Equal(want) {
			t.Errorf("detected compaction %s = %v, want %s as RFC3339", key, d[key], want.Format(time.RFC3339Nano))
		}
	}
}

func TestEncodeReport_AgentContext_IsMatchedByInstanceAndFirstWriteIsLabelledHeuristic(t *testing.T) {
	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), jcAgentContext()))

	if ctx["calls"] != float64(2) {
		t.Errorf("agent context.calls = %v, want 2", ctx["calls"])
	}
	jcAssertFig(t, "agent context", ctx, "peak", 260_000, "above_optimal", false)
	fw := objectAt(t, ctx, "first_write")
	if fw["tokens"] != float64(10_000) || fw["band"] != "below_optimal" || fw["heuristic"] != true {
		t.Errorf("agent context.first_write = %v, want 10000 tokens, below_optimal, heuristic true", fw)
	}
	if counts := objectAt(t, ctx, "compactions"); counts["logged"] != float64(0) || counts["detected"] != float64(0) {
		t.Errorf("agent context.compactions = %v, want 0 logged and 0 detected", counts)
	}
}

func TestEncodeReport_Context_AbsentFiguresAreNullNeverZero(t *testing.T) {
	orch := domain.ActorContext{Actor: domain.Orchestrator()}
	agent := domain.ActorContext{Actor: jcAgentContext().Actor}

	for name, ctx := range map[string]map[string]any{
		"orchestrator": jcOrchestratorObj(t, jcRun(orch, agent)),
		"agent":        jcAgentObj(t, jcRun(orch, agent)),
	} {
		if ctx["calls"] != float64(0) {
			t.Errorf("%s context.calls = %v, want 0", name, ctx["calls"])
		}
		for _, key := range []string{"first", "peak", "final", "first_write"} {
			if v, present := ctx[key]; !present || v != nil {
				t.Errorf("%s context.%s = %v (present %v), want null", name, key, v, present)
			}
		}
		if list := vwList(t, ctx, "detected_compactions"); len(list) != 0 {
			t.Errorf("%s context.detected_compactions = %v, want an empty array", name, list)
		}
		if ctx["final_definition"] != domain.FinalContextDefinition {
			t.Errorf("%s context.final_definition = %v, want the definition even without calls", name, ctx["final_definition"])
		}
	}
}

func TestEncodeReport_Context_ExistingActorFieldsKeepTheirNames(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(jcRun(jcOrchestratorContext(), jcAgentContext()))))

	orch := objectAt(t, run, "orchestrator")
	agent := vwObj(t, vwList(t, run, "agents")[0])

	for name, obj := range map[string]map[string]any{"orchestrator": orch, "agent": agent} {
		for _, key := range []string{"actor", "models", "model_keys", "requests_without_model", "unpriced_models", "totals", "effort"} {
			if _, ok := obj[key]; !ok {
				t.Errorf("%s object lost %q; keys = %v", name, key, sortedKeys(obj))
			}
		}
	}
	if agent["actor"] != "coder#1" {
		t.Errorf("agent actor = %v, want coder#1", agent["actor"])
	}
}
