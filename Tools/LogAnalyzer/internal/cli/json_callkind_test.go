package cli_test

// Tests for the call kind in the report JSON.
//
// Rules under test:
//   - Every actor "context" object gains "calls_by_kind" with the three keys
//     main, compaction and auxiliary, always present, counting every call; a call
//     without a kind is a main call.
//   - Nothing else changes: "calls" still counts every call, and the other
//     context keys keep their names.
//   - The per-run total (total --format json) keeps its exact field set, which
//     external tools decode.

import (
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

func jkCalls(kinds ...domain.CallKind) []domain.ContextCall {
	calls := jcCalls(len(kinds))
	for i, k := range kinds {
		calls[i].Kind = k
	}
	return calls
}

func jkAssertByKind(t *testing.T, ctx map[string]any, main, compaction, auxiliary float64) {
	t.Helper()
	by := objectAt(t, ctx, "calls_by_kind")
	assertKeySet(t, "calls_by_kind", sortedKeys(by), []string{"main", "compaction", "auxiliary"})
	if by["main"] != main || by["compaction"] != compaction || by["auxiliary"] != auxiliary {
		t.Errorf("calls_by_kind = %v, want main %v, compaction %v, auxiliary %v", by, main, compaction, auxiliary)
	}
}

func TestEncodeReport_OrchestratorContext_CountsCallsByKind(t *testing.T) {
	orch := jcOrchestratorContext()
	orch.Calls = jkCalls(domain.CallMain, domain.CallAuxiliary, domain.CallCompaction, domain.CallAuxiliary, domain.CallMain)

	ctx := jcOrchestratorObj(t, jcRun(orch, jcAgentContext()))

	jkAssertByKind(t, ctx, 2, 1, 2)
	if ctx["calls"] != float64(5) {
		t.Errorf("context.calls = %v, want 5 (every call, whatever its kind)", ctx["calls"])
	}
}

func TestEncodeReport_AgentContext_CountsCallsByKind(t *testing.T) {
	agent := jcAgentContext()
	agent.Calls = jkCalls(domain.CallMain, domain.CallCompaction)

	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), agent))

	jkAssertByKind(t, ctx, 1, 1, 0)
}

func TestEncodeReport_CallsWithoutAKind_CountAsMain(t *testing.T) {
	ctx := jcOrchestratorObj(t, jcRun(jcOrchestratorContext(), jcAgentContext()))

	jkAssertByKind(t, ctx, 3, 0, 0)
}

func TestEncodeReport_ActorWithoutCalls_StillCarriesAllThreeKindKeys(t *testing.T) {
	agent := jcAgentContext()
	agent.Calls = nil

	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), agent))

	jkAssertByKind(t, ctx, 0, 0, 0)
}

func TestEncodeReport_Context_KeepsItsExistingKeysAndAddsOnlyTheAdditiveFields(t *testing.T) {
	ctx := jcOrchestratorObj(t, jcRun(jcOrchestratorContext(), jcAgentContext()))

	assertKeySet(t, "context", sortedKeys(ctx), []string{
		"calls", "first", "peak", "final", "final_definition", "first_write",
		"compactions", "detected_compactions", "calls_by_kind", "compaction_calls",
		"rewrites",
	})
}

func TestEncodeRunTotal_RunWithHelperCalls_KeepsExactTopLevelFieldSet(t *testing.T) {
	orch := jcOrchestratorContext()
	orch.Calls = jkCalls(domain.CallMain, domain.CallAuxiliary, domain.CallCompaction)
	report := vwReport(jcRun(orch, jcAgentContext()))
	report.Runs[0].Run = domain.NamedRun(cliTestRunID)
	report.Runs[0].Totals.Money.Complete = true
	total, ok := app.RunTotalFor(report, cliTestRunID)
	if !ok {
		t.Fatalf("RunTotalFor: run %s not found", cliTestRunID)
	}

	_, keys := encodedKeys(t, total)

	assertKeySet(t, "top level", keys, []string{
		"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
	})
}
