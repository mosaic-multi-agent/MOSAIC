package engine_test

// Auto-review route-back by the typed On Findings entry, with On Success
// jumps in the same run: rows 1-5 where row 1 names row 4, row 4 returns
// findings that name row 2, and rows 2 and 3 move on.

import (
	"testing"

	"mosaic-run/internal/domain"
)

const rowReviewerOfFive = 3 // zero-based row 4

// fiveRowRows is the forward-jump and route-back table: 1 -> 4, 4 findings ->
// 2, 2 -> 3 -> 4 -> 5. Rows 2 and 4 carry mode qualifiers, so their cells are
// the targets as written.
func fiveRowRows() []scopeRow {
	return []scopeRow{
		{phase: "RESEARCH", agent: "agent-1", onSuccess: "reviewer(final)"},
		{phase: "PLANNING", agent: "fixer(plan)"},
		{phase: "PLANNING", agent: "agent-3"},
		{phase: "REVIEW", agent: "reviewer(final)", onFindings: "fixer(plan)"},
		{phase: "REPORT", agent: "agent-5"},
	}
}

// findingsFromReviewer scripts COMPLETED_NEEDS_ACTION for the first n visits
// of row 4 and SUCCESS for every other step.
func findingsFromReviewer(n int) func(int, domain.DispatchStep) domain.StatusCode {
	given := 0
	return func(_ int, step domain.DispatchStep) domain.StatusCode {
		if step.RowIndex == rowReviewerOfFive && given < n {
			given++
			return domain.StatusCOMPLETED_NEEDS_ACTION
		}
		return domain.StatusSUCCESS
	}
}

func TestFindingsRouteBack_AutoReview_RowsJumpForwardAndRouteBackInOrder(t *testing.T) {
	aw := mustAdmitScope(t, fiveRowRows(), "", domain.ExecutionModeAutoReview)
	run := scopeRun{aw: aw, mode: domain.ExecutionModeAutoReview, statusOf: findingsFromReviewer(1)}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 3, 1, 2, 3, 4})
}

func TestFindingsRouteBack_AutoMode_DeviatesOnFindings(t *testing.T) {
	aw := mustAdmitScope(t, fiveRowRows(), "", domain.ExecutionModeAuto)
	run := scopeRun{aw: aw, mode: domain.ExecutionModeAuto, statusOf: findingsFromReviewer(1)}

	trace := run.play(t)

	if got := trace.rows(); len(got) != 2 || got[0] != 0 || got[1] != rowReviewerOfFive {
		t.Errorf("dispatched rows: want [0 3] before the deviation, got %v", got)
	}
	if trace.final.Deviation == nil {
		t.Fatalf("auto mode must deviate to the consultant on findings, got %s", trace.describe())
	}
	if row := trace.final.Deviation.Info.CurrentRow; row != rowReviewerOfFive {
		t.Errorf("deviation row: want %d, got %d", rowReviewerOfFive, row)
	}
	if kind := trace.final.Deviation.Info.Kind; kind != domain.DeviationNonSuccess {
		t.Errorf("deviation kind: want %q, got %q", domain.DeviationNonSuccess, kind)
	}
}

func TestFindingsRouteBack_NoFindingsTarget_DeviatesInAutoReview(t *testing.T) {
	rows := fiveRowRows()
	rows[rowReviewerOfFive].onFindings = ""
	aw := mustAdmitScope(t, rows, "", domain.ExecutionModeAutoReview)
	run := scopeRun{aw: aw, mode: domain.ExecutionModeAutoReview, statusOf: findingsFromReviewer(1)}

	trace := run.play(t)

	if trace.final.Deviation == nil {
		t.Fatalf("a '-' On Findings must deviate, got %s", trace.describe())
	}
	info := trace.final.Deviation.Info
	if info.CurrentRow != rowReviewerOfFive {
		t.Errorf("deviation row: want %d, got %d", rowReviewerOfFive, info.CurrentRow)
	}
	if info.Kind != domain.DeviationNonSuccess {
		t.Errorf("a '-' On Findings is a findings deviation: want kind %q, got %q", domain.DeviationNonSuccess, info.Kind)
	}
	if got := trace.rows(); len(got) != 2 {
		t.Errorf("run must stop at the findings, got rows %v", got)
	}
}

func TestFindingsRouteBack_ReviewLoopLimit_StopsRouteBackAtLimit(t *testing.T) {
	aw := mustAdmitScope(t, fiveRowRows(), "", domain.ExecutionModeAutoReview)
	run := scopeRun{
		aw: aw, mode: domain.ExecutionModeAutoReview,
		statusOf: findingsFromReviewer(10), loopLimit: 2,
	}

	trace := run.play(t)

	if got := trace.rows(); len(got) != 5 || got[4] != rowReviewerOfFive {
		t.Errorf("dispatched rows: want [0 3 1 2 3] before the limit, got %v", got)
	}
	if trace.final.Deviation == nil || trace.final.Deviation.Info.Kind != domain.DeviationReviewLoopLimit {
		t.Fatalf("want a review-loop-limit deviation, got %s", trace.describe())
	}
}

// Regression coverage on an EXECUTION block with a unique target; the BUILD
// case at the end of this file discriminates the per-block model.
func TestFindingsRouteBack_WithinBlock_KeepsCurrentIteration(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "impl-reviewer", onFindings: "implementer"},
		{phase: "REPORT", agent: "reporter"},
	}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	given := false
	run := scopeRun{
		aw: aw, schedules: scheduleOf("Stage", idsOf("", "1", "2")...), mode: domain.ExecutionModeAutoReview,
		statusOf: func(_ int, step domain.DispatchStep) domain.StatusCode {
			// Findings from the reviewer of the second iteration, once.
			if step.RowIndex == 2 && step.Stage == "2" && !given {
				given = true
				return domain.StatusCOMPLETED_NEEDS_ACTION
			}
			return domain.StatusSUCCESS
		},
	}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 1, 2, 1, 2, 1, 2, 3})
	requireStages(t, trace, []string{"", "1", "1", "2", "2", "2", "2", ""})
}

// The route-back target is the first of two rows of one bare agent, on a block
// that is not named EXECUTION: the nearest row of that agent above the
// reviewer is the other one.
func TestFindingsRouteBack_WithinNonExecutionBlock_ToQualifiedRowKeepsIteration(t *testing.T) {
	rows := []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "worker(first)"},
		{phase: "BUILD.[Stage]", agent: "worker(second)"},
		{phase: "BUILD.[Stage]", agent: "build-reviewer", onFindings: "worker(first)"},
		{phase: "REPORT", agent: "reporter"},
	}
	aw := mustAdmitScope(t, rows, stageBlockTables(), domain.ExecutionModeAutoReview)
	given := false
	run := scopeRun{
		aw: aw, schedules: scheduleOf("Stage", idsOf("", "1", "2")...), mode: domain.ExecutionModeAutoReview,
		statusOf: func(_ int, step domain.DispatchStep) domain.StatusCode {
			if step.RowIndex == 3 && step.Stage == "2" && !given {
				given = true
				return domain.StatusCOMPLETED_NEEDS_ACTION
			}
			return domain.StatusSUCCESS
		},
	}

	trace := run.play(t)

	requireRowsThenComplete(t, trace, []int{0, 1, 2, 3, 1, 2, 3, 1, 2, 3, 4})
	requireStages(t, trace, []string{"", "1", "1", "1", "2", "2", "2", "2", "2", "2", ""})
}
