package engine_test

// Exact-row identity of mode-qualified rows. The repeated bare agent
// (build-review) sits in the same phase and, when grouped, in the same group,
// so nothing but the recorded workflow row or the typed On Findings target can
// select the row.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// samePhaseRows: planner, then the ungrouped block Stage on BUILD with a
// builder, two build-review rows under different qualifiers, and a verifier
// whose On Findings names findings; then the reporter.
func samePhaseRows(findings string) []scopeRow {
	return []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "build-review(tests)"},
		{phase: "BUILD.[Stage]", agent: "build-review(impl)"},
		{phase: "BUILD.[Stage]", agent: "verifier", onFindings: findings},
		{phase: "REPORT", agent: "reporter"},
	}
}

// samePhaseBlockRows: like samePhaseRows with a builder first; used for
// position tests, where the reviewers need a row before them.
func samePhaseBlockRows() []scopeRow {
	return []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "BUILD.[Stage]", agent: "builder"},
		{phase: "BUILD.[Stage]", agent: "build-review(tests)", onFindings: "builder"},
		{phase: "BUILD.[Stage]", agent: "build-review(impl)", onFindings: "builder"},
		{phase: "REPORT", agent: "reporter"},
	}
}

func samePhaseSchedules() domain.BlockSchedules {
	return scheduleOf("Stage", idsOf("", "1", "2")...)
}

func TestQualifiedRows_FindingsRouteBack_SamePhaseRows_LandsOnRowNamed(t *testing.T) {
	cases := []struct {
		name      string
		findings  string
		wantRows  []int
		wantFocus string
	}{
		{"first qualified row", "build-review(tests)", []int{0, 1, 2, 3, 1, 2, 3, 4}, "tests"},
		{"second qualified row", "build-review(impl)", []int{0, 1, 2, 3, 2, 3, 4}, "impl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aw := mustAdmitScope(t, samePhaseRows(tc.findings), stageBlockTables(), domain.ExecutionModeAutoReview)
			given := false
			run := scopeRun{
				aw: aw, schedules: scheduleOf("Stage", idsOf("", "1")...), mode: domain.ExecutionModeAutoReview,
				statusOf: func(_ int, step domain.DispatchStep) domain.StatusCode {
					if step.RowIndex == 3 && !given {
						given = true
						return domain.StatusCOMPLETED_NEEDS_ACTION
					}
					return domain.StatusSUCCESS
				},
			}

			trace := run.play(t)

			requireRowsThenComplete(t, trace, tc.wantRows)
			if len(trace.steps) < 5 {
				t.Fatalf("want a route-back dispatch after the findings, got rows %v", trace.rows())
			}
			routed := trace.steps[4]
			if got := agentName(routed.Request.AgentInstanceID); got != "build-review" {
				t.Errorf("route-back dispatches bare agent build-review, got %s", routed.Request.AgentInstanceID)
			}
			if routed.Request.TaskDescription != focusTask(tc.wantFocus) {
				t.Errorf("route-back task description: want %q, got %q", focusTask(tc.wantFocus), routed.Request.TaskDescription)
			}
		})
	}
}

// reviewerLog records planner, builder and the build-review row at the given
// 1-based row of iteration 1 of the block on BUILD.
func reviewerLog(reviewerRow int) *runLog {
	return (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("builder", "1", domain.StatusSUCCESS, 2).lastInPhase("BUILD").
		workflowStep("build-review", "1", domain.StatusSUCCESS, reviewerRow).lastInPhase("BUILD")
}

func TestQualifiedRows_SamePhase_ResumeInterrupted_ReturnsRecordedRowNotFirstOfAgent(t *testing.T) {
	aw := mustAdmitScope(t, samePhaseBlockRows(), stageBlockTables(), domain.ExecutionModeAutoReview)

	info, err := resumeLogIDs(aw, samePhaseSchedules(), reviewerLog(4), true)

	requireResumeBlock(t, info, err, 3, true, "Stage", "1")
}

func TestQualifiedRows_SamePhase_ResumeAdvanced_ContinuesAfterRecordedRowNotFirstOfAgent(t *testing.T) {
	aw := mustAdmitScope(t, samePhaseBlockRows(), stageBlockTables(), domain.ExecutionModeAutoReview)

	// After the last row of the block the next iteration starts at its first
	// row; continuing after the first build-review row would stay on row 3.
	info, err := resumeLogIDs(aw, samePhaseSchedules(), reviewerLog(4), false)

	requireResumeBlock(t, info, err, 1, false, "Stage", "2")
}

func TestQualifiedRows_SamePhase_NextAfterRecordedRow_ContinuesAfterRecordedRowNotFirstOfAgent(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := mustAdmitScope(t, samePhaseBlockRows(), stageBlockTables(), mode)

		step := requireDispatch(t, nextAfterLogMode(aw, samePhaseSchedules(), reviewerLog(4), mode, domain.StageSource{}))

		if step.RowIndex != 1 || step.Stage != "2" {
			t.Errorf("want row 1 of iteration 2, got row %d stage %q", step.RowIndex, step.Stage)
		}
	})
}

// groupedSamePhaseRows: both build-review rows share the group Test.
func groupedSamePhaseRows() []scopeRow {
	return []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Test", agent: "build-review(tests)", onFindings: "test-writer"},
		{phase: "EXECUTION.[Stage].Test", agent: "build-review(impl)", onFindings: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
}

func groupedSamePhaseLog(reviewerRow int) *runLog {
	return (&runLog{}).
		workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, reviewerRow)
}

func TestQualifiedRows_SameGroup_ResumeInterrupted_ReturnsRecordedRowNotFirstOfAgent(t *testing.T) {
	aw := mustAdmitScope(t, groupedSamePhaseRows(), groupedStageTables(), domain.ExecutionModeAutoReview)

	info, err := resumeLogIDs(aw, qualifiedSchedules(), groupedSamePhaseLog(4), true)

	requireResumeBlock(t, info, err, 3, true, "Stage", "1")
	requireResumeGroup(t, info, 0)
}

func TestQualifiedRows_SameGroup_ResumeAdvanced_ContinuesAfterRecordedRowNotFirstOfAgent(t *testing.T) {
	aw := mustAdmitScope(t, groupedSamePhaseRows(), groupedStageTables(), domain.ExecutionModeAutoReview)

	// After the last row of the Test group the Implementation group starts;
	// continuing after the first build-review row would stay on row 3.
	info, err := resumeLogIDs(aw, qualifiedSchedules(), groupedSamePhaseLog(4), false)

	requireResumeBlock(t, info, err, 4, false, "Stage", "1")
	requireResumeGroup(t, info, 1)
}
