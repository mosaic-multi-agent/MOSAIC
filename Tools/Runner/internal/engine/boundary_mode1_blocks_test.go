package engine_test

// Mode 1 answers for sibling blocks and for a Depends On-reordered schedule.
// The engine never routes in Mode 1, so the steps are written by hand in the
// order the script orchestrator ran them; the answers still follow the declared
// routing and the iterations the Execution Log shows completed.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func mode1SiblingBlockRows() ([]scopeRow, string) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage]", agent: "worker"},
		{phase: "EXECUTION.[Stage]", agent: "checker"},
		{phase: "EXECUTION.[Audit]", agent: "fixer"},
		{phase: "EXECUTION.[Audit]", agent: "verifier"},
		bsReporter(),
	}
	return rows, scopeBlocks(stageBlockRow, "| Audit | Audit.md | Items |")
}

// Two blocks on one phase: the first block's exit continues in the same phase,
// so only the second block's last row ends it.
func TestPhaseEndMode1_SiblingBlocksOnOnePhase_FirstBlockExitIsNotPhaseEnd(t *testing.T) {
	rows, tables := mode1SiblingBlockRows()
	aw := mustAdmitScope(t, rows, tables, domain.ExecutionModeOrchestrated)
	schedules := joinSchedules(
		scheduleOf("Stage", idsOf("", "1", "2")...),
		scheduleOf("Audit", idsOf("", "x1", "x2")...))
	steps := bndSteps(t, "0:", "1:1", "2:1", "1:2", "2:2", "3:x1", "4:x1", "3:x2", "4:x2", "5:")

	phases := bndPhaseEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)
	iterations := bndIterationEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)

	requireEnds(t, "phase end", phases, []string{"0:", "4:x2", "5:"})
	requireEnds(t, "iteration end", iterations, []string{"2:1", "2:2", "4:x1", "4:x2"})
}

// A (depends on C), B, C ran as B, C, A. The phase ends after A, the last to
// run, not after C, the last entry of the schedule; every iteration ends at its
// own last row.
func TestBoundariesMode1_ScheduleReorderedByDependsOn_EndsAfterLastIterationToRun(t *testing.T) {
	rows := []scopeRow{bsPlanner(), bsBuild("worker", ""), bsBuild("checker", ""), bsReporter()}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeOrchestrated)
	schedules := scheduleOf("Stage", itIteration("A", "C"), itIteration("B"), itIteration("C"))
	steps := bndSteps(t, "0:", "1:B", "2:B", "1:C", "2:C", "1:A", "2:A", "3:")

	phases := bndPhaseEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)
	iterations := bndIterationEnds(aw, schedules, domain.ExecutionModeOrchestrated, steps)

	requireEnds(t, "phase end", phases, []string{"0:", "2:A", "3:"})
	requireEnds(t, "iteration end", iterations, []string{"2:B", "2:C", "2:A"})
}
