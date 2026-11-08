package engine_test

// Resume with HARNESS_ERROR rows. A trailing HARNESS_ERROR row is an
// unaccepted attempt: resume re-dispatches the same assignment with the inputs
// recorded on its row. A further failure after resume stays bounded because the
// budget is read from the whole log. Old synthetic BLOCKED/E501 rows (written by
// earlier Runner versions) resume as before and count against the E501 budget.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

func heRow(seq int, agent, phase, stage string, row int, inputs string) domain.ExecutionLogEntry {
	e := execLogEntry(seq, agent, phase, stage, domain.StatusHARNESS_ERROR, row)
	e.Summary = heFailureReason
	e.Inputs = inputs
	return e
}

func TestResume_TrailingHarnessErrorRow_IsAnUnacceptedAttempt(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	planner := execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusSUCCESS, 1)
	state := withTrailing(stateWithLog(1, planner),
		heRow(2, "plan-review#2", "PLANNING", "", 2, "Plan.md, Stage-1/Plan.md"))

	info, err := engine.ResumePointFromSchedules(aw, schedules, state, domain.NewInfraAgentSet(declaredCheckpointInfraAgent()))

	if err != nil {
		t.Fatalf("ResumePointFromSchedules: %v", err)
	}
	if !info.RerunLast || info.RowIndex != 1 {
		t.Errorf("want RerunLast on row index 1 (the failed plan-review), got RerunLast=%v row=%d", info.RerunLast, info.RowIndex)
	}
}

func TestResume_TrailingHarnessErrorRow_RedispatchesSameAssignmentWithRecordedInputs(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	schedules := singleSchedules("Implementation-Only")
	planner := execLogEntry(1, "planner-tdd-soft#1", "PLANNING", "", domain.StatusSUCCESS, 1)
	state := withTrailing(stateWithLog(1, planner),
		heRow(2, "plan-review#2", "PLANNING", "", 2, "Plan.md, Stage-1/Plan.md, extra-context.md"))

	step, ok, err := redispatchFor(aw, schedules, quickFixAgents(), state, 2)

	got := requireRedispatch(t, step, ok, err)
	if got.RowIndex != 1 || got.Agent.Identifier != "plan-review" {
		t.Errorf("want plan-review at row index 1, got %q at %d", got.Agent.Identifier, got.RowIndex)
	}
	if got.Request.AgentInstanceID != "plan-review#3" {
		t.Errorf("want instance plan-review#3 (Seq+1), got %q", got.Request.AgentInstanceID)
	}
	requireInputs(t, got.Request.InputArtifacts, "Plan.md", "Stage-1/Plan.md", "extra-context.md")
	if got.Retry != domain.RetryNone {
		t.Errorf("a resume re-dispatch is not a mechanical retry, want RetryNone, got %q", got.Retry)
	}
}

func TestResume_FirstDispatchFailed_RedispatchesFirstRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, quickFixV216Content, "quick-fix", "3.0")
	state := domain.ArtifactState{
		GlobalSequence: 1,
		ExecutionLog:   []domain.ExecutionLogEntry{heRow(1, "planner-tdd-soft#1", "PLANNING", "", 1, "")},
	}

	step, ok, err := redispatchFor(aw, singleSchedules("Implementation-Only"), quickFixAgents(), state, 1)

	got := requireRedispatch(t, step, ok, err)
	if got.RowIndex != 0 || got.Agent.Identifier != "planner-tdd-soft" {
		t.Errorf("want planner-tdd-soft at row index 0, got %q at %d", got.Agent.Identifier, got.RowIndex)
	}
	if got.Request.AgentInstanceID != "planner-tdd-soft#2" {
		t.Errorf("want instance planner-tdd-soft#2, got %q", got.Request.AgentInstanceID)
	}
}

func TestResume_TrailingHarnessErrorRow_StagedRowKeepsRecordedIteration(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldTDDV216Content, "brownfield-tdd", "3.4")
	writer := execLogEntry(8, "test-writer-tdd#8", "EXECUTION", "2.Test", domain.StatusSUCCESS, rtRowTestWriter)
	state := withTrailing(stateWithLog(8, writer),
		heRow(9, "tests-review-tdd#9", "EXECUTION", "2.Test", rtRowTestsReview, "Stage-2/Plan.md, ContractsDesign.md"))

	step, ok, err := redispatchFor(aw, twoSchedules("TDD"), brownfieldAgents(), state, 9)

	got := requireRedispatch(t, step, ok, err)
	if got.RowIndex != rtRowTestsReview-1 || got.Stage != "2.Test" {
		t.Errorf("want row index %d at iteration 2.Test, got row %d stage %q", rtRowTestsReview-1, got.RowIndex, got.Stage)
	}
	requireInputs(t, got.Request.InputArtifacts, "Stage-2/Plan.md", "ContractsDesign.md")
}

// The attempts before the resume count: with the budget spent by the rows
// already in the log, a failure of the re-dispatched attempt deviates.
func TestResume_FurtherFailureAfterResume_IsBoundedByTheLoggedBudget(t *testing.T) {
	f := newRetryFixture(t)
	for _, mode := range autoModes {
		t.Run(modeName(mode), func(t *testing.T) {
			build := func(failures int) *runLog {
				return (&runLog{}).ok(rtPlanner, "", rtRowPlanner).
					repeat(failures, func(l *runLog) { l.harnessError(rtPlanReview, "", rtRowPlanReview) }).inPhase(rtPlanPhase)
			}

			requireRetryStep(t, f.nextAfterHE(mode, build(engine.HarnessErrorAttemptLimit-1), 0),
				rtRowPlanReview, "", domain.RetryHarnessError)
			requireNonSuccessDeviation(t, f.nextAfterHE(mode, build(engine.HarnessErrorAttemptLimit), 0))
		})
	}
}

// Old synthetic rows: BLOCKED, a Summary starting "harness:", error code E501.
func syntheticHarnessLog(n int) *runLog {
	return (&runLog{}).repeat(n, func(l *runLog) {
		l.stepWith(rtPlanner, "", domain.StatusBLOCKED, e501, rtRowPlanner, "harness: subprocess exited with status 1")
	}).inPhase(rtPlanPhase)
}

func TestResume_OldSyntheticBlockedE501Rows_ResumeAsBefore(t *testing.T) {
	f := newRetryFixture(t)
	aw := f.aw
	state := syntheticHarnessLog(1).state()

	info, err := engine.ResumePointFromSchedules(aw, f.schedules, state, nil)

	if err != nil {
		t.Fatalf("ResumePointFromSchedules: %v", err)
	}
	if info.RerunLast {
		t.Error("an old synthetic row is an accepted BLOCKED step, not an unaccepted attempt: want RerunLast=false")
	}
}

func TestResume_OldSyntheticBlockedE501Rows_CountAgainstTheE501Budget(t *testing.T) {
	f := newRetryFixture(t)
	for _, mode := range autoModes {
		t.Run(modeName(mode), func(t *testing.T) {
			requireRetryStep(t, f.next(mode, syntheticHarnessLog(engine.E501AttemptLimit-1), true),
				rtRowPlanner, "", domain.RetryToolUnavailable)
			requireNonSuccessDeviation(t, f.next(mode, syntheticHarnessLog(engine.E501AttemptLimit), true))
		})
	}
}

func TestResume_OldSyntheticRows_DoNotSpendTheHarnessErrorBudget(t *testing.T) {
	log := syntheticHarnessLog(engine.E501AttemptLimit)
	row := domain.RoutingRow{Index: rtRowPlanner - 1}

	if got := engine.HarnessErrorBudgetRemaining(log.entries, row, ""); got != engine.HarnessErrorAttemptLimit {
		t.Errorf("want the full harness error budget %d, got %d", engine.HarnessErrorAttemptLimit, got)
	}
}
