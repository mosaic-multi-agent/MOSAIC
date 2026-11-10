package engine_test

// Iteration end and phase end in Mode 1 (orchestrated). The engine never
// routes there, yet the session evaluates ITERATION_END and PHASE_END in every
// mode, so both answers follow the table's declared On Success as if the run
// were engine-routed: the row's declared entry, block exit and iteration
// completion from the Execution Log. A join row is judged by its own On
// Success. The fork-list rule is in boundary_mode1_fork_test.go.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

const forkColumns = "| Phase | Subagent | HITL | On Success | On Findings | Waits For | Input | Output |\n" +
	"|-------|----------|:----:|------------|-------------|-----------|-------|--------|\n"

// forkJoinLeavesBeforeLaterSamePhaseContent: the join routes to REPORT while a
// later row (cleanup) carries the fork's phase name again.
//
//	0 scout (fork)  1 research-a  2 research-b  3 merge (join)  4 reporter  5 cleanup
const forkJoinLeavesBeforeLaterSamePhaseContent = forkColumns +
	"| RESEARCH | scout | FALSE | research-a, research-b | - | - | - | out-0.md |\n" +
	"| RESEARCH | research-a | FALSE | merge | - | - | - | out-1.md |\n" +
	"| RESEARCH | research-b | FALSE | merge | - | - | - | out-2.md |\n" +
	"| RESEARCH | merge | FALSE | reporter | - | research-a, research-b | - | out-3.md |\n" +
	"| REPORT | reporter | FALSE | cleanup | - | - | - | out-4.md |\n" +
	"| RESEARCH | cleanup | FALSE | COMPLETE | - | - | - | out-5.md |\n"

// The join is judged by its own On Success: it leaves RESEARCH, so it ends the
// phase although another RESEARCH row follows later in the table. The fork and
// the branch rows route inside RESEARCH, so they end nothing.
func TestPhaseEndMode1_JoinRoutedOutOfPhase_IsPhaseEndDespiteLaterRowOfSamePhase(t *testing.T) {
	aw := bndAdmitRaw(t, forkJoinLeavesBeforeLaterSamePhaseContent, domain.ExecutionModeOrchestrated)
	steps := bndSteps(t, "0:", "1:", "2:", "3:", "4:", "5:")

	got := bndPhaseEnds(aw, nil, domain.ExecutionModeOrchestrated, steps)

	requireEnds(t, "phase end", got, []string{"3:", "4:", "5:"})
}

// Regression coverage: in the brownfield-tdd 4.0 shape the declared routing and
// the table position give the same answers; this protects the grouped block
// under declared routing, it is not evidence for the routing rule.
func TestBoundariesMode1_Regression_GroupedBlock_FollowDeclaredRouting(t *testing.T) {
	aw := mustAdmitScope(t, bsTddShapeRows(), bsApproachTables(), domain.ExecutionModeOrchestrated)
	schedules := scheduleOf("Stage", idsOf("Tests-Only", "1", "2")...)
	steps := bndSteps(t, "0:", "1:1.Test", "2:1.Test", "1:2.Test", "2:2.Test", "5:")

	iterations := bndIterationEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)
	phases := bndPhaseEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)

	requireEnds(t, "iteration end", iterations, []string{"2:1.Test", "2:2.Test"})
	requireEnds(t, "phase end", phases, []string{"0:", "2:2.Test", "5:"})
}

func TestBoundariesMode1_DeclaredCompleteOnFirstBlockRow_EndsIteration(t *testing.T) {
	aw := mustAdmitScope(t, icUngroupedCompleteRows(), stageBlockTables(), domain.ExecutionModeOrchestrated)
	schedules := scheduleOf("Stage", idsOf("", "1", "2")...)
	steps := bndSteps(t, "0:", "1:1", "1:2", "3:")

	iterations := bndIterationEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)
	phases := bndPhaseEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)

	requireEnds(t, "iteration end", iterations, []string{"1:1", "1:2"})
	requireEnds(t, "phase end", phases, []string{"0:", "1:2", "3:"})
}

// The consultant may route a row somewhere else than its declared On Success;
// the answers for the completed row keep following the declared one.
func TestBoundariesMode1_ConsultedRouteDiffersFromDeclared_AnswersFollowDeclared(t *testing.T) {
	t.Run("iteration end", func(t *testing.T) {
		aw := mustAdmitScope(t, icUngroupedCompleteRows(), stageBlockTables(), domain.ExecutionModeOrchestrated)
		schedules := scheduleOf("Stage", idsOf("", "1", "2")...)
		// Declared: worker (row 1) is COMPLETE. Consulted: run the checker
		// (row 2, the last row of the scope, NEXT) anyway.
		steps := bndSteps(t, "0:", "1:1", "2:1", "1:2", "3:")

		got := bndIterationEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)

		requireEnds(t, "iteration end", got, []string{"1:1", "2:1", "1:2"})
	})
	t.Run("phase end", func(t *testing.T) {
		// Declared: scout leaves RESEARCH for the reporter. Consulted: the
		// analyst, a later row of RESEARCH, ran next. The answer for scout
		// stays the declared one: the phase ends.
		rows := []scopeRow{
			{phase: "RESEARCH", agent: "scout", onSuccess: "reporter"},
			{phase: "RESEARCH", agent: "analyst"},
			{phase: "REPORT", agent: "reporter"},
		}
		aw := mustAdmitScope(t, rows, "", domain.ExecutionModeOrchestrated)
		steps := bndSteps(t, "0:", "1:", "2:")
		afterScout := bndInputAt(aw, nil, domain.ExecutionModeOrchestrated, steps, 0)
		afterScout.Log = bndLog(aw, steps) // the log already holds the step that followed

		got := engine.IsLastRowOfPhaseForIteration(afterScout)

		if !got {
			t.Error("scout declares a row of another phase next, so it is the phase end whatever was consulted")
		}
	})
}
