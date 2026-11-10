package engine_test

// The engine continues from the workflow row recorded in the Execution Log
// for every kind of row: a non-EXECUTION agent that fills two rows of one
// phase, a single-row EXECUTION agent, and a multi-row EXECUTION agent. A
// recorded row that is not in the table or names another agent is refused.
// An entry that records no row ("-") still resolves by agent and phase.

import (
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// twoPlanningRowsV216Content has planner-tdd-soft in two PLANNING rows: it
// drafts the plan (row 1) and, after the reviewer's findings, finalises it
// (row 3, told apart by a mode qualifier).
const twoPlanningRowsV216Content = v216Columns +
	`| PLANNING | planner-tdd-soft | TRUE | plan-review | - | - | Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md |
| PLANNING | plan-review | FALSE | planner-tdd-soft(final) | planner-tdd-soft | Plan.md | plan-review.md |
| PLANNING | planner-tdd-soft(final) | TRUE | [Stage] | - | Plan.md, plan-review.md | Plan.md |
| EXECUTION.[Stage] | implementation-tdd | FALSE | COMPLETE | - | Stage-{Stage}/Plan.md | Stage-{Stage}/PlanProgress.md |
| REVIEW | test-runner | FALSE | COMPLETE | - | - | TestResults.md |
` + v216StageBlock

// 1-based rows of twoPlanningRowsV216Content.
const (
	tpRowDraft     = 1
	tpRowReview    = 2
	tpRowFinalise  = 3
	tpRowImplement = 4
)

func twoPlanningRowsAgents() map[string]domain.AgentReference {
	return newTestAgents("planner-tdd-soft", "plan-review", "implementation-tdd", "test-runner")
}

func TestNext_NonExecutionAgentAtSecondRow_ContinuesFromRecordedRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, twoPlanningRowsV216Content, "two-planning-rows", "1.0")
	log := &runLog{}
	log.
		workflowStep("planner-tdd-soft", "", domain.StatusSUCCESS, tpRowDraft).
		workflowStep("plan-review", "", domain.StatusCOMPLETED_NEEDS_ACTION, tpRowReview).
		workflowStep("planner-tdd-soft", "", domain.StatusSUCCESS, tpRowFinalise).
		inPhase("PLANNING")

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), twoPlanningRowsAgents(), log)

	if got := agentName(step.Request.AgentInstanceID); got != "implementation-tdd" {
		t.Errorf("after the planner's second row, want implementation-tdd, got %s", got)
	}
	if step.RowIndex != tpRowImplement-1 {
		t.Errorf("dispatched row index: want %d, got %d", tpRowImplement-1, step.RowIndex)
	}
}

func TestNext_NonExecutionAgentAtFirstRow_ContinuesFromRecordedRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, twoPlanningRowsV216Content, "two-planning-rows", "1.0")
	log := &runLog{}
	log.workflowStep("planner-tdd-soft", "", domain.StatusSUCCESS, tpRowDraft).inPhase("PLANNING")

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), twoPlanningRowsAgents(), log)

	if step.RowIndex != tpRowReview-1 {
		t.Errorf("dispatched row index: want %d, got %d", tpRowReview-1, step.RowIndex)
	}
}

func TestNext_SingleRowExecutionAgent_ContinuesFromRecordedRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter)

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), buildVerifiedAgents(), log)

	requireStepAt(t, step, bvRowTestBuild-1, "1.Test")
}

func TestNext_MultiRowExecutionAgentAtImplementationRow_AdvancesToNextRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := newTestSchedules(
		domain.ScheduleIteration{ID: "1", Approach: "TDD"}, domain.ScheduleIteration{ID: "2", Approach: "TDD"}, domain.ScheduleIteration{ID: "3", Approach: "TDD"},
	)
	log := &runLog{}
	log.workflowStep("build-review", "3.Implementation", domain.StatusSUCCESS, bvRowImplBuild)

	step := requireNextStepIDs(t, aw, schedules, buildVerifiedAgents(), log)

	if got := agentName(step.Request.AgentInstanceID); got != "implementation-review" {
		t.Errorf("after build-review at row %d, want implementation-review, got %s", bvRowImplBuild, got)
	}
	requireStepAt(t, step, bvRowImplReview-1, "3.Implementation")
}

func TestNext_NoRecordedRow_FallsBackToAgentAndPhaseResolution(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.workflowStep("codebase-research", "", domain.StatusSUCCESS, int(domain.NoWorkflowRow)).inPhase("RESEARCH")

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), buildVerifiedAgents(), log)

	if got := agentName(step.Request.AgentInstanceID); got != "requirements-refinement" {
		t.Errorf("a log row without a recorded row must resolve by agent and phase, got %s", got)
	}
}

// nextAfterRecordedRow drives Next after one step of agent, recorded in the
// given phase and stage with the given 1-based row.
func nextAfterRecordedRow(t *testing.T, agent, phase, stage string, row int) domain.EngineDecision {
	t.Helper()
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.workflowStep(agent, stage, domain.StatusSUCCESS, row).inPhase(phase)
	return nextAfterLogIDs(aw, singleSchedules("TDD"), buildVerifiedAgents(), log)
}

func TestNext_NonExecutionAgentRecordedRowHoldsDifferentAgent_Refuses(t *testing.T) {
	// Row 3 is requirements-review, not codebase-research.
	dec := nextAfterRecordedRow(t, "codebase-research", "RESEARCH", "", 3)

	requirePositionCause(t, dec, domain.CauseRecordedRowInvalid)
}

func TestNext_SingleRowExecutionAgentRecordedRowHoldsDifferentAgent_Refuses(t *testing.T) {
	dec := nextAfterRecordedRow(t, "test-writer-tdd", "EXECUTION", "1.Test", bvRowTestsReview)

	perr := requirePositionCause(t, dec, domain.CauseRecordedRowInvalid)
	if perr.RecordedRow != domain.WorkflowRow(bvRowTestsReview) {
		t.Errorf("error must carry the recorded row %d, got %d", bvRowTestsReview, perr.RecordedRow)
	}
}

func TestNext_RecordedRowOutsideTable_RefusesNamingRow(t *testing.T) {
	// The build-verified workflow has 13 rows.
	for _, tc := range []struct{ agent, phase, stage string }{
		{"codebase-research", "RESEARCH", ""},
		{"test-writer-tdd", "EXECUTION", "1.Test"},
	} {
		for _, row := range []int{14, 99} {
			t.Run(tc.agent+"/row="+strconv.Itoa(row), func(t *testing.T) {
				dec := nextAfterRecordedRow(t, tc.agent, tc.phase, tc.stage, row)

				requirePositionCause(t, dec, domain.CauseRecordedRowInvalid)
				if reason := requireStop(t, dec).Reason; !strings.Contains(reason, strconv.Itoa(row)) {
					t.Errorf("stop reason must name recorded row %d, got %q", row, reason)
				}
			})
		}
	}
}
