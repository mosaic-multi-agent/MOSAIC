package engine_test

// The row that Next identifies must not change when the log holds extra
// steps: infrastructure entries interleaved with a route-back, or extra
// invocations (route-backs, infrastructure steps) in an earlier stage.

import (
	"testing"

	"mosaic-run/internal/domain"
)

const checkpointInfra = "checkpoint-manager-git"

// requireSameNextStep asserts that two runs dispatch the same agent at the same
// row and stage.
func requireSameNextStep(t *testing.T, got, want domain.DispatchStep) {
	t.Helper()
	if agentName(got.Request.AgentInstanceID) != agentName(want.Request.AgentInstanceID) {
		t.Errorf("next agent: want %s (run without extra steps), got %s",
			agentName(want.Request.AgentInstanceID), agentName(got.Request.AgentInstanceID))
	}
	requireStepAt(t, got, want.RowIndex, want.Stage)
}

// stageOneStraight appends stage 1 of a TDD run with no route-back and no
// infrastructure steps.
func stageOneStraight(l *runLog) *runLog {
	return l.
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild).
		workflowStep("tests-review-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestsReview).
		workflowStep("implementation-tdd", "1.Implementation", domain.StatusSUCCESS, bvRowImplementation).
		workflowStep("build-review", "1.Implementation", domain.StatusSUCCESS, bvRowImplBuild).
		workflowStep("implementation-review", "1.Implementation", domain.StatusSUCCESS, bvRowImplReview)
}

// stageOneWithRouteBack appends stage 1 in which tests-review-tdd sent the run
// back to test-writer-tdd once, so the stage holds three extra invocations.
func stageOneWithRouteBack(l *runLog) *runLog {
	return l.
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild).
		workflowStep("tests-review-tdd", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, bvRowTestsReview).
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild).
		workflowStep("tests-review-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestsReview).
		workflowStep("implementation-tdd", "1.Implementation", domain.StatusSUCCESS, bvRowImplementation).
		workflowStep("build-review", "1.Implementation", domain.StatusSUCCESS, bvRowImplBuild).
		workflowStep("implementation-review", "1.Implementation", domain.StatusSUCCESS, bvRowImplReview)
}

// stageOneWithInfra appends stage 1 with an infrastructure step after every
// workflow step.
func stageOneWithInfra(l *runLog) *runLog {
	return l.
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		infraStep(checkpointInfra, "1.Test").
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild).
		infraStep(checkpointInfra, "1.Test").
		workflowStep("tests-review-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestsReview).
		infraStep(checkpointInfra, "1.Test").
		workflowStep("implementation-tdd", "1.Implementation", domain.StatusSUCCESS, bvRowImplementation).
		infraStep(checkpointInfra, "1.Implementation").
		workflowStep("build-review", "1.Implementation", domain.StatusSUCCESS, bvRowImplBuild).
		infraStep(checkpointInfra, "1.Implementation").
		workflowStep("implementation-review", "1.Implementation", domain.StatusSUCCESS, bvRowImplReview).
		infraStep(checkpointInfra, "1.Implementation")
}

// ===== Infrastructure entries interleaved with a route-back inside a stage =====

func TestNext_InfraInterleavedWithRouteBack_SuccessMatchesRunWithoutInfra(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := singleSchedules("TDD")
	agents := buildVerifiedAgents()
	want := requireNextStepIDs(t, aw, schedules, agents, buildVerifiedRouteBackLog())

	log := &runLog{}
	log.
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		infraStep(checkpointInfra, "1.Test").
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild).
		workflowStep("tests-review-tdd", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, bvRowTestsReview).
		infraStep(checkpointInfra, "1.Test").
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		infraStep(checkpointInfra, "1.Test").
		infraStep(checkpointInfra, "1.Test").
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild)
	got := requireNextStepIDs(t, aw, schedules, agents, log)

	requireSameNextStep(t, got, want)
	requireStepAt(t, got, bvRowTestsReview-1, "1.Test")
}

func TestNext_InfraInterleavedWithRouteBack_NeedsActionMatchesRunWithoutInfra(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	schedules := singleSchedules("TDD")
	agents := buildVerifiedAgents()
	baseline := buildVerifiedRouteBackLog()
	baseline.entries[len(baseline.entries)-1].Status = domain.StatusCOMPLETED_NEEDS_ACTION
	want := requireNextStepIDs(t, aw, schedules, agents, baseline)

	log := &runLog{}
	log.
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild).
		infraStep(checkpointInfra, "1.Test").
		workflowStep("tests-review-tdd", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, bvRowTestsReview).
		infraStep(checkpointInfra, "1.Test").
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		infraStep(checkpointInfra, "1.Test").
		workflowStep("build-review", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, bvRowTestBuild)
	got := requireNextStepIDs(t, aw, schedules, agents, log)

	requireSameNextStep(t, got, want)
	requireStepAt(t, got, bvRowTestWriter-1, "1.Test")
}

// ===== Stage 2 after extra steps in stage 1 =====

// stageTwoTestBuildReviewNext appends the first two steps of stage 2's Test group to
// a stage-1 log built by stageOne, and returns the step Next dispatches.
func stageTwoTestBuildReviewNext(t *testing.T, stageOne func(*runLog) *runLog) domain.DispatchStep {
	t.Helper()
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := stageOne(&runLog{}).
		workflowStep("test-writer-tdd", "2.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "2.Test", domain.StatusSUCCESS, bvRowTestBuild)
	return requireNextStepIDs(t, aw, twoSchedules("TDD"), buildVerifiedAgents(), log)
}

func TestNext_StageTwoTestGroup_AfterRouteBackInStageOne_MatchesRunWithoutRouteBack(t *testing.T) {
	want := stageTwoTestBuildReviewNext(t, stageOneStraight)

	got := stageTwoTestBuildReviewNext(t, stageOneWithRouteBack)

	requireSameNextStep(t, got, want)
	if agentName(got.Request.AgentInstanceID) != "tests-review-tdd" {
		t.Errorf("stage 2 Test-group build-review must continue to tests-review-tdd, got %s",
			got.Request.AgentInstanceID)
	}
	requireStepAt(t, got, bvRowTestsReview-1, "2.Test")
}

func TestNext_StageTwoTestGroup_AfterInfraInStageOne_MatchesRunWithoutInfra(t *testing.T) {
	want := stageTwoTestBuildReviewNext(t, stageOneStraight)

	got := stageTwoTestBuildReviewNext(t, stageOneWithInfra)

	requireSameNextStep(t, got, want)
	requireStepAt(t, got, bvRowTestsReview-1, "2.Test")
}

func TestNext_StageTwoImplementationGroup_AfterRouteBackInStageOne_ContinuesToImplementationReview(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := stageOneWithRouteBack(&runLog{}).
		workflowStep("test-writer-tdd", "2.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "2.Test", domain.StatusSUCCESS, bvRowTestBuild).
		workflowStep("tests-review-tdd", "2.Test", domain.StatusSUCCESS, bvRowTestsReview).
		workflowStep("implementation-tdd", "2.Implementation", domain.StatusSUCCESS, bvRowImplementation).
		workflowStep("build-review", "2.Implementation", domain.StatusSUCCESS, bvRowImplBuild)

	step := requireNextStepIDs(t, aw, twoSchedules("TDD"), buildVerifiedAgents(), log)

	if got := agentName(step.Request.AgentInstanceID); got != "implementation-review" {
		t.Errorf("stage 2 Implementation-group build-review must continue to implementation-review, got %s", got)
	}
	requireStepAt(t, step, bvRowImplReview-1, "2.Implementation")
}

func TestNext_StageTwoRouteBack_AfterRouteBackInStageOne_RoutesToStageTwoTestWriter(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := stageOneWithRouteBack(&runLog{}).
		workflowStep("test-writer-tdd", "2.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "2.Test", domain.StatusCOMPLETED_NEEDS_ACTION, bvRowTestBuild)

	step := requireNextStepIDs(t, aw, twoSchedules("TDD"), buildVerifiedAgents(), log)

	if got := agentName(step.Request.AgentInstanceID); got != "test-writer-tdd" {
		t.Errorf("findings from stage 2 build-review must route to test-writer-tdd, got %s", got)
	}
	requireStepAt(t, step, bvRowTestWriter-1, "2.Test")
}
