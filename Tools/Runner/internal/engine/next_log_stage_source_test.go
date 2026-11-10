package engine_test

// Next takes the stage of the step that just ran from the Execution Log entry
// of the last agent, not from the frontmatter current_state.stage, for every
// decision it makes: dispatch, review-loop counting, findings route-back and
// deviation reports. When the last agent has no log entry, the entry is built
// from the current state and carries no workflow row.

import (
	"errors"
	"testing"

	"mosaic-run/internal/domain"
)

// laggingFrontmatterState returns the state after log with the frontmatter
// stage left at Test.1 while the log says otherwise.
func laggingFrontmatterState(log *runLog, limit int) domain.ArtifactState {
	state := log.state()
	state.CurrentState.Stage = "1.Test"
	state.ReviewLoopLimit = limit
	return state
}

// stageTwoTestsReviewLog is a stage-2 Test group in which the tests review
// has just asked for changes.
func stageTwoTestsReviewLog() *runLog {
	log := &runLog{}
	return log.
		workflowStep("test-writer-tdd", "2.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "2.Test", domain.StatusSUCCESS, bvRowTestBuild).
		workflowStep("tests-review-tdd", "2.Test", domain.StatusCOMPLETED_NEEDS_ACTION, bvRowTestsReview)
}

func TestNext_FindingsRouteBack_StageComesFromLogEntryNotFrontmatter(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := stageTwoTestsReviewLog()

	dec := engineNextAutoIDs(aw, twoSchedules("TDD"), laggingFrontmatterState(log, 0), cnaResponse(log.last().Agent))

	requireStepAt(t, requireDispatch(t, dec), bvRowTestWriter-1, "2.Test")
}

func TestNext_ReviewLoopLimit_CountsFindingsAtTheLogEntryStage(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := stageTwoTestsReviewLog()

	// One findings round at Test.2 reaches the limit of 1. Counted at the
	// frontmatter stage Test.1 there are none, and the run would route back.
	dec := engineNextAutoIDs(aw, twoSchedules("TDD"), laggingFrontmatterState(log, 1), cnaResponse(log.last().Agent))

	dev := requireLoopLimitDeviation(t, dec)
	if dev.Info.CurrentStage != "2.Test" {
		t.Errorf("deviation stage: want the log entry's 2.Test, got %q", dev.Info.CurrentStage)
	}
}

func TestNext_NonSuccessDeviation_ReportsTheLogEntryStage(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.workflowStep("test-writer-tdd", "2.Test", domain.StatusBLOCKED, bvRowTestWriter)

	dec := engineNextAutoIDs(aw, twoSchedules("TDD"), laggingFrontmatterState(log, 0), responseFor(log.last()))

	dev := requireDeviation(t, dec)
	if dev.Info.CurrentStage != "2.Test" {
		t.Errorf("deviation stage: want the log entry's 2.Test, got %q", dev.Info.CurrentStage)
	}
	if dev.Info.CurrentRow != bvRowTestWriter-1 {
		t.Errorf("deviation row: want %d, got %d", bvRowTestWriter-1, dev.Info.CurrentRow)
	}
}

func TestNext_LastAgentEntryIsNotTheFinalLogEntry_UsesTheLastAgentsEntry(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.workflowStep("test-writer-tdd", "2.Test", domain.StatusSUCCESS, bvRowTestWriter).
		infraStep(checkpointAgent, "2.Test")
	state := log.state()
	state.CurrentState = domain.CurrentState{
		Phase: "EXECUTION", Stage: "1.Test", LastStatus: domain.StatusSUCCESS,
		LastAgent: log.entries[0].Agent,
	}

	dec := engineNextAutoIDs(aw, twoSchedules("TDD"), state, successResponse(log.entries[0].Agent))

	requireStepAt(t, requireDispatch(t, dec), bvRowTestBuild-1, "2.Test")
}

func TestNext_NoLogEntryForLastAgent_SingleRowExecutionAgent_FallsBackToCurrentState(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	state := domain.ArtifactState{
		GlobalSequence: 1,
		CurrentState: domain.CurrentState{
			Phase: "EXECUTION", Stage: "1.Test", LastStatus: domain.StatusSUCCESS,
			LastAgent: "test-writer-tdd#1",
		},
	}

	dec := engineNextAutoIDs(aw, singleSchedules("TDD"), state, successResponse("test-writer-tdd#1"))

	requireStepAt(t, requireDispatch(t, dec), bvRowTestBuild-1, "1.Test")
}

func TestNext_NoLogEntryForLastAgent_NonExecutionAgent_FallsBackToCurrentState(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	state := domain.ArtifactState{
		GlobalSequence: 1,
		CurrentState: domain.CurrentState{
			Phase: "RESEARCH", LastStatus: domain.StatusSUCCESS, LastAgent: "codebase-research#1",
		},
	}

	dec := engineNextAutoIDs(aw, singleSchedules("TDD"), state, successResponse("codebase-research#1"))

	step := requireDispatch(t, dec)
	if got := agentName(step.Request.AgentInstanceID); got != "requirements-refinement" {
		t.Errorf("want requirements-refinement, got %s", got)
	}
}

// A log row that records "-" for the row and no stage keeps resolving as it
// did before the row was recorded: it is not the new missing-stage refusal.
func TestNext_StagedRowWithNoRecordedRowAndNoStage_IsNotTheMissingStageRefusal(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.workflowStep("test-writer-tdd", "", domain.StatusSUCCESS, int(domain.NoWorkflowRow))
	state := log.state()
	state.CurrentState.Stage = "1.Test"

	dec := engineNextAutoIDs(aw, singleSchedules("TDD"), state, successResponse(log.last().Agent))

	var perr *domain.PositionUnresolvedError
	if dec.Stop != nil && errors.As(dec.Stop.Err, &perr) && perr.Cause == domain.CauseStagedRowWithoutStage {
		t.Errorf("a log row without a recorded row must not get the missing-stage refusal: %v", perr)
	}
}

func TestResumePoint_StagedRowWithNoRecordedRowAndNoStage_IsNotTheMissingStageRefusal(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := newBuildVerifiedLog()
	log.workflowStep("test-writer-tdd", "", domain.StatusSUCCESS, int(domain.NoWorkflowRow))

	_, err := resumeLogIDs(aw, singleSchedules("TDD"), log, false)

	var perr *domain.PositionUnresolvedError
	if errors.As(err, &perr) && perr.Cause == domain.CauseStagedRowWithoutStage {
		t.Errorf("a log row without a recorded row must not get the missing-stage refusal: %v", err)
	}
}
