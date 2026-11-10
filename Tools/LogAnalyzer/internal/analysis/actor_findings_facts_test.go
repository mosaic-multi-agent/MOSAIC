package analysis_test

// Tests for the rewrite, compaction and failed-tool findings of an actor.
//
// Rules under test:
//   - Rewrites: one finding when the actor has rewrites, with their count and
//     summed extra cost.
//   - Compactions: one finding when the actor has a compaction mark or a
//     compaction call, with the logged + harness + detected-only count, the
//     unapplied compaction calls and the compaction call cost.
//   - Failed tools: one finding per tool with errors, most errors first and
//     then by name, with the error count. Calls without an error are not failures.
//   - An actor without any of these has no such finding.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// afStatusTool is a paired tool call with the given end status.
func afStatusTool(callID, name string, startSec int, status domain.ToolCallStatus) domain.ToolCall {
	c := ctxTool(callID, name, startSec, startSec+1)
	c.Status = status
	return c
}

func TestActorFindings_Rewrites_CountAndSummedExtraCostOfTheActorsRewrites(t *testing.T) {
	run := afPrice(t, ctxRunSpec{orchUsage: []domain.InvocationUsage{
		crCall("r1", 0, "", 10, 0, 100_000),
		crCall("r2", 600, "", 10, 0, 100_000),
		crCall("r3", 1200, "", 10, 0, 100_000),
	}})
	ctx := run.Context.Orchestrator
	crRequireRewrites(t, ctx, 2)

	f := afOne(t, afSession(t, run, domain.Orchestrator()), domain.ActorFindingRewrites)

	if f.Count != 2 {
		t.Errorf("rewrites = %d, want 2", f.Count)
	}
	want, _ := domain.SumMoney(ctx.Rewrites[0].ExtraCost, ctx.Rewrites[1].ExtraCost)
	if f.Cost != want || f.Cost.State != domain.MoneyKnown || f.Cost.Amount <= 0 {
		t.Errorf("extra cost = %+v, want the summed extra cost %+v of both rewrites", f.Cost, want)
	}
}

func TestActorFindings_Rewrites_ActorWithoutRewritesHasNoRewriteFinding(t *testing.T) {
	run := afPrice(t, ctxRunSpec{orchUsage: []domain.InvocationUsage{
		afInput("o1", 10, 1000),
		afInput("o2", 20, 2000),
	}})

	afNone(t, afSession(t, run, domain.Orchestrator()), domain.ActorFindingRewrites)
}

func TestActorFindings_Compactions_CountsLoggedHarnessAndDetectedOnlyWithTheirCallCost(t *testing.T) {
	spec := ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			ctxCall("m1", 0, 100_000, 0, 0, 0),
			hcCompactionCall("c1", 30, 2_000),
			ctxCall("m2", 60, 40_000, 0, 0, 0), // harness compaction: the drop follows a compaction call
			ctxCall("m3", 110, 41_000, 0, 0, 0),
			ctxCall("m4", 130, 100_000, 0, 0, 0),
			ctxCall("m5", 140, 40_000, 0, 0, 0), // detected only: no compaction call, no log
		},
		compactions: []domain.Compaction{detLogged(domain.Orchestrator(), 100)},
	}
	run := afPrice(t, spec)
	ctx := run.Context.Orchestrator
	marks := ctx.LoggedCount() + ctx.HarnessCount() + ctx.DetectedOnlyCount()
	if marks < 2 {
		t.Fatalf("the scenario holds %d compaction marks, want at least a logged and a detected one", marks)
	}

	f := afOne(t, afSession(t, run, domain.Orchestrator()), domain.ActorFindingCompactions)

	if f.Count != marks {
		t.Errorf("compactions = %d, want logged + harness + detected-only = %d", f.Count, marks)
	}
	if f.Unapplied != ctx.CompactionCalls.UnappliedCount {
		t.Errorf("unapplied = %d, want %d", f.Unapplied, ctx.CompactionCalls.UnappliedCount)
	}
	afMoney(t, "compaction cost", f.Cost, 2_000*afPerToken)
}

func TestActorFindings_Compactions_CompactionCallsWithoutAnyCompactionAreUnappliedWithTheirCost(t *testing.T) {
	run := afPrice(t, hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		hcCompactionCall("c1", 30, 2_000),
		hcCompactionCall("c2", 40, 3_000),
		ctxCall("m2", 60, 95_000, 0, 0, 0),
	))

	f := afOne(t, afSession(t, run, domain.Orchestrator()), domain.ActorFindingCompactions)

	if f.Count != 0 || f.Unapplied != 2 {
		t.Errorf("compactions/unapplied = %d/%d, want 0/2", f.Count, f.Unapplied)
	}
	afMoney(t, "compaction cost", f.Cost, 5_000*afPerToken)
}

func TestActorFindings_Compactions_LoggedCompactionWithoutCallsIsCounted(t *testing.T) {
	spec := hcOrch(
		ctxCall("m1", 0, 100_000, 0, 0, 0),
		ctxCall("m2", 60, 30_000, 0, 0, 0),
	)
	spec.compactions = []domain.Compaction{detLogged(domain.Orchestrator(), 30)}

	f := afOne(t, afSession(t, afPrice(t, spec), domain.Orchestrator()), domain.ActorFindingCompactions)

	if f.Count != 1 || f.Unapplied != 0 {
		t.Errorf("compactions/unapplied = %d/%d, want 1/0", f.Count, f.Unapplied)
	}
}

func TestActorFindings_Compactions_ActorWithoutCompactionsHasNoCompactionFinding(t *testing.T) {
	run := afPrice(t, hcOrch(afInput("o1", 10, 1000), afInput("o2", 20, 2000)))

	afNone(t, afSession(t, run, domain.Orchestrator()), domain.ActorFindingCompactions)
}

func TestActorFindings_FailedTools_OneFindingPerToolMostErrorsFirstThenByName(t *testing.T) {
	run := afPrice(t, ctxRunSpec{orchTools: []domain.ToolCall{
		afStatusTool("t1", "Read", 10, domain.ToolCallError),
		afStatusTool("t2", "Bash", 20, domain.ToolCallError),
		afStatusTool("t3", "Bash", 30, domain.ToolCallError),
		afStatusTool("t4", "Edit", 40, domain.ToolCallError),
		afStatusTool("t5", "Glob", 50, ""),
		afStatusTool("t6", "Bash", 60, ""),
	}})

	got := afKind(afSession(t, run, domain.Orchestrator()), domain.ActorFindingFailedTool)

	want := []struct {
		tool   string
		errors int
	}{{"Bash", 2}, {"Edit", 1}, {"Read", 1}}
	if len(got) != len(want) {
		t.Fatalf("failed tool findings = %+v, want %d (one per tool with errors, none for Glob)", got, len(want))
	}
	for i, w := range want {
		if got[i].Tool != w.tool || got[i].Count != w.errors {
			t.Errorf("finding %d = %s x%d, want %s x%d", i, got[i].Tool, got[i].Count, w.tool, w.errors)
		}
	}
}

func TestActorFindings_FailedTools_UnpairedCallWithoutAnErrorStatusIsNotAFailure(t *testing.T) {
	start := domain.ToolCall{CallID: "t1", ToolName: "Bash", StartedAt: ctxAt(10), Pairing: domain.ToolCallStartOnly}
	run := afPrice(t, ctxRunSpec{orchTools: []domain.ToolCall{start}})

	afNone(t, afSession(t, run, domain.Orchestrator()), domain.ActorFindingFailedTool)
}

func TestActorFindings_FailedTools_CountOnlyTheActorsOwnCalls(t *testing.T) {
	run := afPrice(t, ctxRunSpec{
		orchTools: []domain.ToolCall{afStatusTool("o1", "Bash", 10, domain.ToolCallError)},
		agents: []ctxAgent{afAgent(ctxAgentID, "coder", nil,
			afStatusTool("a1", "Bash", 10, domain.ToolCallError),
			afStatusTool("a2", "Bash", 20, domain.ToolCallError))},
	})

	orch := afOne(t, afSession(t, run, domain.Orchestrator()), domain.ActorFindingFailedTool)
	agent := afOne(t, afSession(t, run, afRef(ctxAgentID, "coder")), domain.ActorFindingFailedTool)

	if orch.Count != 1 || agent.Count != 2 {
		t.Errorf("errors orchestrator/agent = %d/%d, want 1/2", orch.Count, agent.Count)
	}
}

func TestActorFindings_FindingsComeInTheOrderOfTheirKind(t *testing.T) {
	run := afPrice(t, ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			crCall("r1", 0, "", 10, 0, 100_000),
			crCall("r2", 600, "", 10, 0, 100_000),
			hcCompactionCall("c1", 700, 2_000),
		},
		orchTools: []domain.ToolCall{afStatusTool("t1", "Bash", 10, domain.ToolCallError)},
	})

	s := afSession(t, run, domain.Orchestrator())

	for i := 1; i < len(s.Findings); i++ {
		if s.Findings[i].Kind < s.Findings[i-1].Kind {
			t.Fatalf("findings are not in kind order: %+v", s.Findings)
		}
	}
	for _, k := range []domain.ActorFindingKind{
		domain.ActorFindingCostliestCall, domain.ActorFindingRewrites, domain.ActorFindingCompactions, domain.ActorFindingFailedTool,
	} {
		if len(afKind(s, k)) == 0 {
			t.Errorf("the scenario produced no finding of kind %d: %+v", k, s.Findings)
		}
	}
}
