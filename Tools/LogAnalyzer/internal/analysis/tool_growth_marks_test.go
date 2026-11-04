package analysis_test

// Tests for singular-call detection around compaction marks (detected and
// logged, including their boundaries), recorded order and pooling.
//
// Rules under test:
//   - A detected compaction (a context drop past the configured percentage
//     between two main calls) between the two calls makes the call not singular.
//   - A mark counts only when it lies between the two calls: an unapplied mark
//     (no call after it) and marks exactly at either call's start do not.
//   - Pairing follows the per-call time order, not the input order.
//   - Singular is decided from recorded figures, whatever the time gap.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestToolGrowth_DetectedCompactionBetween_IsNotSingular(t *testing.T) {
	// 100000 -> 60000 is a 40% fall, past the default 30%: a detected compaction.
	run := tgAgentRun(t,
		tgCall("m1", 10, 100_000, 100, tgOne("read_file")),
		tgCall("m2", 20, 60_000, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertNotSingular(t, "call across a detected compaction", got)
}

func TestToolGrowth_ContextFallBelowTheDropThreshold_IsANegativeSampleNotACompaction(t *testing.T) {
	// 100000 -> 90000 is a 10% fall: no compaction, so the call is singular and negative.
	run := tgAgentRun(t,
		tgCall("m1", 10, 100_000, 100, tgOne("read_file")),
		tgCall("m2", 20, 90_000, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	if got.State != domain.GrowthNegative || got.Tool != "read_file" {
		t.Errorf("growth = %+v, want negative for read_file", got)
	}
}

func TestToolGrowth_ContextRiseBetween_IsMeasuredNotACompaction(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("m1", 10, 100_000, 100, tgOne("read_file")),
		tgCall("m2", 20, 120_000, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 19_900)
}

func TestToolGrowth_UnappliedCompactionAfterAllCalls_DoesNotMatter(t *testing.T) {
	// The mark at 50s has no call after it; it lies after the pair m2 -> m3.
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("m1", 10, 1000, 100, domain.NewRequestedTools(nil)),
			tgCall("m2", 20, 1500, 100, tgOne("read_file")),
			tgCall("m3", 40, 2000, 50, domain.NewRequestedTools(nil)),
		}}},
		compactions: []domain.Compaction{tgLoggedCompaction(tgAgentActor(), 50)},
	}})

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m2")

	tgAssertGrowth(t, "m2", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_CompactionExactlyAtTheSecondCall_DoesNotBreakThePair(t *testing.T) {
	// The mark sits between m2 and m3, not between m1 and m2.
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("m1", 10, 1000, 100, tgOne("read_file")),
			tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
			tgCall("m3", 40, 2000, 50, domain.NewRequestedTools(nil)),
		}}},
		compactions: []domain.Compaction{tgLoggedCompaction(tgAgentActor(), 20)},
	}})

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_CompactionExactlyAtTheFirstCall_DoesNotBreakThePair(t *testing.T) {
	// The mark sits before m1 (no call precedes it), not between m1 and m2.
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("m1", 10, 1000, 100, tgOne("read_file")),
			tgCall("m2", 20, 1500, 50, domain.NewRequestedTools(nil)),
		}}},
		compactions: []domain.Compaction{tgLoggedCompaction(tgAgentActor(), 10)},
	}})

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "m1")

	tgAssertGrowth(t, "m1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowth_CallsGivenOutOfTimeOrder_PairByTime(t *testing.T) {
	// Recorded order c3, c1, c2; time order c1 (10s), c2 (20s), c3 (30s).
	run := tgAgentRun(t,
		tgCall("c3", 30, 1800, 50, domain.NewRequestedTools(nil)),
		tgCall("c1", 10, 1000, 100, tgOne("read_file")),
		tgCall("c2", 20, 1500, 50, tgOne("glob")),
	)

	a := ctxRequireAgent(t, run, ctxAgentID)

	tgAssertGrowth(t, "c1", tgCallGrowth(t, a, "c1"), domain.GrowthMeasured, "read_file", 400)
	tgAssertGrowth(t, "c2", tgCallGrowth(t, a, "c2"), domain.GrowthMeasured, "glob", 250)
	tgAssertNotSingular(t, "c3", tgCallGrowth(t, a, "c3"))
}

func TestToolGrowth_HoursApart_IsStillSingular(t *testing.T) {
	run := tgAgentRun(t,
		tgCall("c1", 10, 1000, 100, tgOne("read_file")),
		tgCall("c2", 10+6*3600, 1500, 50, domain.NewRequestedTools(nil)),
	)

	got := tgCallGrowth(t, ctxRequireAgent(t, run, ctxAgentID), "c1")

	tgAssertGrowth(t, "c1", got, domain.GrowthMeasured, "read_file", 400)
}

func TestToolGrowthFigures_AgentTypeWithGrowthButNoToolCalls_GetsARow(t *testing.T) {
	// The agent has no tool call entries at all, only growth samples.
	run := tgAgentRun(t,
		tgCall("a1", 10, 2000, 100, tgOne("read_file")),
		tgCall("a2", 20, 2300, 10, domain.NewRequestedTools(nil)),
	)

	tools := tgAgentTypeTools(t, run, tgAgentType)

	tgAssertGrowthFigures(t, "read_file", requireTool(t, tools, "read_file").Growth, 1, 200, 200, 200)
}

func TestToolGrowthFigures_TwoInvocationsOfOneAgentType_ArePooled(t *testing.T) {
	none := domain.NewRequestedTools(nil)
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{agents: []ctxAgent{
		{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("a1", 10, 2000, 100, tgOne("read_file")), tgCall("a2", 20, 2300, 10, none), // 200
		}},
		{id: "coder#2", typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("b1", 10, 3000, 100, tgOne("read_file")), tgCall("b2", 20, 3700, 10, none), // 600
		}},
	}}})

	read := requireTool(t, tgAgentTypeTools(t, run, tgAgentType), "read_file")

	tgAssertGrowthFigures(t, "read_file", read.Growth, 2, 400, 200, 600)
}
