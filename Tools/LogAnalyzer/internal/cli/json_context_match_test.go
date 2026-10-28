package cli_test

// Tests that per-actor context figures in the report JSON are matched to the
// right actor by instance, and that compaction figures are per actor.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func jcAgentRef(instance, typ string) domain.ActorRef {
	return domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(instance), Type: typ}
}

func jcAgentCtx(instance string, calls int, peak int64) domain.ActorContext {
	return domain.ActorContext{
		Actor: jcAgentRef(instance, "coder"),
		Calls: jcCalls(calls),
		First: jcFig(peak/2, false),
		Peak:  jcFig(peak, false),
		Final: jcFig(peak, false),
	}
}

// jcMultiAgentRun lists agents coder#1, coder#2, coder#3 while the context
// invocations are in a different order and coder#3 has no context entry.
func jcMultiAgentRun(orch domain.ActorContext, invocations ...domain.ActorContext) domain.RunReport {
	run := vwRun()
	run.Agents = []domain.ActorReport{
		{Actor: jcAgentRef("coder#1", "coder")},
		{Actor: jcAgentRef("coder#2", "coder")},
		{Actor: jcAgentRef("coder#3", "coder")},
	}
	run.Context = domain.RunContext{Orchestrator: orch, Invocations: invocations}
	return run
}

func jcAgentContextByIndex(t *testing.T, run domain.RunReport, idx int) map[string]any {
	t.Helper()
	agent := vwObj(t, vwList(t, firstRun(t, decodeReportJSON(t, vwReport(run))), "agents")[idx])
	return objectAt(t, agent, "context")
}

func TestEncodeReport_AgentContext_EachAgentGetsItsOwnFiguresWhateverTheInvocationOrder(t *testing.T) {
	run := jcMultiAgentRun(jcOrchestratorContext(),
		jcAgentCtx("coder#2", 4, 300_000),
		jcAgentCtx("coder#1", 2, 100_000),
	)

	first := jcAgentContextByIndex(t, run, 0)
	second := jcAgentContextByIndex(t, run, 1)

	if first["calls"] != float64(2) {
		t.Errorf("coder#1 context.calls = %v, want 2", first["calls"])
	}
	jcAssertFig(t, "coder#1 context", first, "peak", 100_000, "optimal", false)
	if second["calls"] != float64(4) {
		t.Errorf("coder#2 context.calls = %v, want 4", second["calls"])
	}
	jcAssertFig(t, "coder#2 context", second, "peak", 300_000, "above_optimal", false)
}

func TestEncodeReport_AgentContext_AgentWithoutContextEntryHasZeroCallsAndNullFigures(t *testing.T) {
	run := jcMultiAgentRun(jcOrchestratorContext(),
		jcAgentCtx("coder#2", 4, 300_000),
		jcAgentCtx("coder#1", 2, 100_000),
	)

	ctx := jcAgentContextByIndex(t, run, 2)

	if ctx["calls"] != float64(0) {
		t.Errorf("coder#3 context.calls = %v, want 0", ctx["calls"])
	}
	for _, key := range []string{"first", "peak", "final", "first_write"} {
		if v, present := ctx[key]; !present || v != nil {
			t.Errorf("coder#3 context.%s = %v (present %v), want null", key, v, present)
		}
	}
}

func TestEncodeReport_Context_CompactionCountsArePerActor(t *testing.T) {
	agent := jcAgentCtx("coder#1", 3, 200_000)
	agent.Compactions = []domain.CompactionMark{
		{Kind: domain.CompactionDetected, At: jcStart, Before: 0, After: 1, BeforeTokens: 200_000, AfterTokens: 50_000},
		{Kind: domain.CompactionDetected, At: jcStart.Add(time.Minute), Before: 1, After: 2, BeforeTokens: 190_000, AfterTokens: 40_000},
	}
	orch := jcOrchestratorContext()
	orch.Compactions = orch.Compactions[:1] // logged only
	run := jcMultiAgentRun(orch, agent)

	orchCounts := objectAt(t, jcOrchestratorObj(t, run), "compactions")
	agentCounts := objectAt(t, jcAgentContextByIndex(t, run, 0), "compactions")

	if orchCounts["logged"] != float64(1) || orchCounts["detected"] != float64(0) {
		t.Errorf("orchestrator compactions = %v, want logged 1 detected 0", orchCounts)
	}
	if agentCounts["logged"] != float64(0) || agentCounts["detected"] != float64(2) {
		t.Errorf("coder#1 compactions = %v, want logged 0 detected 2", agentCounts)
	}
}

func TestEncodeReport_Context_DetectedCompactionWithOutOfRangeCallIndexHasNullTimes(t *testing.T) {
	orch := jcOrchestratorContext()
	orch.Compactions = []domain.CompactionMark{
		{Kind: domain.CompactionDetected, Before: 40, After: 41, BeforeTokens: 360_000, AfterTokens: 120_000},
	}

	ctx := jcOrchestratorObj(t, jcRun(orch, jcAgentContext()))

	d := vwObj(t, vwList(t, ctx, "detected_compactions")[0])
	for _, key := range []string{"from_at", "to_at"} {
		if v, present := d[key]; !present || v != nil {
			t.Errorf("detected compaction %s = %v (present %v), want null", key, v, present)
		}
	}
}

func TestEncodeReport_AgentContext_PartialFirstWriteKeepsItsPartialFlag(t *testing.T) {
	agent := jcAgentContext()
	agent.FirstWrite = jcFig(10_000, true)

	ctx := jcAgentObj(t, jcRun(jcOrchestratorContext(), agent))

	fw := objectAt(t, ctx, "first_write")
	if fw["partial"] != true || fw["heuristic"] != true {
		t.Errorf("agent context.first_write = %v, want partial true and heuristic true", fw)
	}
}
