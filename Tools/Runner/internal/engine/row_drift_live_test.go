package engine_test

// Live routing identifies the row that ran from the row recorded in the
// Execution Log, not from the invocation count. These tests drive engine.Next
// through findings route-backs for agents that fill several EXECUTION rows.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// requireStepAt asserts the dispatched step's zero-based row index and stage.
func requireStepAt(t *testing.T, step domain.DispatchStep, wantRowIndex int, wantStage string) {
	t.Helper()
	if step.RowIndex != wantRowIndex {
		t.Errorf("dispatched row index: want %d, got %d (agent %s)",
			wantRowIndex, step.RowIndex, step.Request.AgentInstanceID)
	}
	if step.Stage != wantStage {
		t.Errorf("dispatched stage: want %q, got %q", wantStage, step.Stage)
	}
}

// buildVerifiedRouteBackLog is the hand-built run: test-writer-tdd, build-review,
// tests-review-tdd (needs action), test-writer-tdd again, build-review again,
// all in the Test group of stage 1.
func buildVerifiedRouteBackLog() *runLog {
	log := &runLog{}
	return log.
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild).
		workflowStep("tests-review-tdd", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, bvRowTestsReview).
		workflowStep("test-writer-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestWriter).
		workflowStep("build-review", "1.Test", domain.StatusSUCCESS, bvRowTestBuild)
}

func TestNext_BuildVerified_TestGroupBuildReviewAfterRouteBack_SuccessContinuesToTestsReview(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), buildVerifiedAgents(), buildVerifiedRouteBackLog())

	if got := agentName(step.Request.AgentInstanceID); got != "tests-review-tdd" {
		t.Errorf("after the Test-group build-review that follows a route-back, want tests-review-tdd, got %s", got)
	}
	requireStepAt(t, step, bvRowTestsReview-1, "1.Test")
}

func TestNext_BuildVerified_TestGroupBuildReviewAfterRouteBack_NeedsActionRoutesToTestWriter(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := buildVerifiedRouteBackLog()
	log.entries[len(log.entries)-1].Status = domain.StatusCOMPLETED_NEEDS_ACTION

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), buildVerifiedAgents(), log)

	if got := agentName(step.Request.AgentInstanceID); got != "test-writer-tdd" {
		t.Errorf("findings from the Test-group build-review must route to test-writer-tdd, got %s", got)
	}
	requireStepAt(t, step, bvRowTestWriter-1, "1.Test")
}

func TestNext_BuildVerified_ContinuedRunAfterRouteBack_DispatchesImplementation(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := buildVerifiedRouteBackLog().
		workflowStep("tests-review-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestsReview)

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), buildVerifiedAgents(), log)

	if got := agentName(step.Request.AgentInstanceID); got != "implementation-tdd" {
		t.Errorf("after tests-review-tdd succeeds, want implementation-tdd, got %s", got)
	}
	requireStepAt(t, step, bvRowTestsReview, "1.Implementation")
}

func TestNext_BuildVerified_ImplementationBuildReviewAfterRouteBack_ContinuesWithinImplementationGroup(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := buildVerifiedRouteBackLog().
		workflowStep("tests-review-tdd", "1.Test", domain.StatusSUCCESS, bvRowTestsReview).
		workflowStep("implementation-tdd", "1.Implementation", domain.StatusSUCCESS, bvRowImplementation).
		workflowStep("build-review", "1.Implementation", domain.StatusSUCCESS, bvRowImplBuild)

	step := requireNextStepIDs(t, aw, singleSchedules("TDD"), buildVerifiedAgents(), log)

	if got := agentName(step.Request.AgentInstanceID); got != "implementation-review" {
		t.Errorf("after the Implementation-group build-review, want implementation-review, got %s", got)
	}
	requireStepAt(t, step, bvRowImplReview-1, "1.Implementation")
}

// ===== Grouped findings loop: one agent fills every block row =====
//
// The fixture is findingsLoopContent: a single agent (told apart by mode
// qualifiers) fills three Test-group rows and one Implementation-group row, and
// the gate row names that same agent's writer row as its On Findings target.

const (
	loopRowWriter = 1
	loopRowGate   = 2
	loopRowReview = 3
	loopRowImpl   = 4
)

func TestNext_StagedFindingsLoop_RepeatedRouteBackCycles_IdentifiesRowAfterEveryStep(t *testing.T) {
	aw := mustParseAndAdmitV216(t, findingsLoopContent, "staged-findings-loop", "1.0")
	schedules := singleSchedules("TDD")
	agents := newTestAgents("loop-agent")

	// Each step is followed by the zero-based row and stage the engine must
	// dispatch next.
	steps := []struct {
		name      string
		stage     string
		status    domain.StatusCode
		row       int
		wantIndex int
		wantStage string
	}{
		{"writer", "1.Test", domain.StatusSUCCESS, loopRowWriter, loopRowGate - 1, "1.Test"},
		{"gate needs action", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, loopRowGate, loopRowWriter - 1, "1.Test"},
		{"writer again", "1.Test", domain.StatusSUCCESS, loopRowWriter, loopRowGate - 1, "1.Test"},
		{"gate needs action again", "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, loopRowGate, loopRowWriter - 1, "1.Test"},
		{"writer a third time", "1.Test", domain.StatusSUCCESS, loopRowWriter, loopRowGate - 1, "1.Test"},
		{"gate succeeds", "1.Test", domain.StatusSUCCESS, loopRowGate, loopRowReview - 1, "1.Test"},
		{"review", "1.Test", domain.StatusSUCCESS, loopRowReview, loopRowImpl - 1, "1.Implementation"},
	}

	log := &runLog{}
	for _, s := range steps {
		log.workflowStep("loop-agent", s.stage, s.status, s.row)
		snapshot := &runLog{entries: append([]domain.ExecutionLogEntry(nil), log.entries...)}

		t.Run(s.name, func(t *testing.T) {
			step := requireNextStepIDs(t, aw, schedules, agents, snapshot)
			requireStepAt(t, step, s.wantIndex, s.wantStage)
		})
	}
}
