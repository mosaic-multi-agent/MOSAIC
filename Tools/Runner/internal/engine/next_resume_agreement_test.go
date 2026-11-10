package engine_test

// Live routing (Next after a step) and resume (ResumePoint on the state that
// step wrote) derive the same row and iteration from the same recorded log entry.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// iterationOfRecordedStage returns the iteration ID of a recorded iteration
// value in the 3.0 form ("2.Test" and "2" both give "2"; "" gives "").
func iterationOfRecordedStage(recorded string) domain.IterationID {
	id, _, _ := strings.Cut(recorded, ".")
	return domain.IterationID(id)
}

// requireNextAndResumeAgree runs Next for the last log entry and ResumePoint
// on the same state, and requires both to name wantRow (zero-based) and the
// stage of wantStage. Next reports the recorded stage form ("2.Test"), while
// ResumePoint reports its own stage text plus the stage number, so the schedules
// are compared by number.
func requireNextAndResumeAgree(
	t *testing.T,
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	agents map[string]domain.AgentReference,
	state domain.ArtifactState,
	wantRow int,
	wantStage string,
) {
	t.Helper()
	requireAgreement(t, aw, schedules, agents, state, wantRow, wantStage, false)
}

// requireNextAndResumeAgreeOrFinished is requireNextAndResumeAgree for a
// position whose row already finished: ResumePoint has no deviation concept and
// names the row, so Next agrees when it either dispatches that row or deviates
// with the finished-work kind and the row as PlannedContinuation.
func requireNextAndResumeAgreeOrFinished(
	t *testing.T,
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	agents map[string]domain.AgentReference,
	state domain.ArtifactState,
	wantRow int,
	wantStage string,
) {
	t.Helper()
	requireAgreement(t, aw, schedules, agents, state, wantRow, wantStage, true)
}

func requireAgreement(
	t *testing.T,
	aw domain.AdmittedWorkflow,
	schedules domain.BlockSchedules,
	agents map[string]domain.AgentReference,
	state domain.ArtifactState,
	wantRow int,
	wantStage string,
	finishedAllowed bool,
) {
	t.Helper()
	last := state.ExecutionLog[len(state.ExecutionLog)-1]
	dec := engine.Next(engine.NextInput{
		Workflow:     aw,
		Schedules:    schedules,
		State:        state,
		LastResponse: responseFor(last),
		Agents:       agents,
		Seq:          last.Seq,
		Now:          fixedNow,
		Mode:         domain.ExecutionModeAutoReview,
	})
	var step domain.DispatchStep
	if finishedAllowed && dec.Deviation != nil {
		step = *requireFinishedWork(t, dec, wantRow, wantStage).PlannedContinuation
	} else {
		step = requireDispatch(t, dec)
	}

	info, err := engine.ResumePointFromSchedules(aw, schedules, state, domain.NewInfraAgentSet(declaredCheckpointInfraAgent()))
	if err != nil {
		t.Fatalf("ResumePoint: %v", err)
	}

	requireStepAt(t, step, wantRow, wantStage)
	wantIteration := iterationOfRecordedStage(wantStage)
	if info.RowIndex != wantRow {
		t.Errorf("ResumePoint row: want %d, got %d", wantRow, info.RowIndex)
	}
	if info.Iteration != wantIteration {
		t.Errorf("ResumePoint iteration: want %q, got %q", wantIteration, info.Iteration)
	}
	stepIteration := iterationOfRecordedStage(step.Stage)
	if info.RowIndex != step.RowIndex || info.Iteration != stepIteration {
		t.Errorf("Next (row %d, iteration %q) and ResumePoint (row %d, iteration %q) disagree",
			step.RowIndex, stepIteration, info.RowIndex, info.Iteration)
	}
}

func TestNextAndResumePoint_FrontmatterStageDiffersFromLogStage_BothFollowTheLogEntry(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.workflowStep("test-writer-tdd", "2.Test", domain.StatusSUCCESS, bvRowTestWriter)
	state := log.state()
	state.CurrentState.Stage = "1.Test" // frontmatter lags behind the logged step

	requireNextAndResumeAgree(t, aw, twoSchedules("TDD"), buildVerifiedAgents(), state,
		bvRowTestBuild-1, "2.Test")
}

func TestNextAndResumePoint_FrontmatterStageEmptyButLogStageSet_BothFollowTheLogEntry(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	log := &runLog{}
	log.workflowStep("build-review", "2.Implementation", domain.StatusSUCCESS, bvRowImplBuild)
	state := log.state()
	state.CurrentState.Stage = ""

	requireNextAndResumeAgree(t, aw, twoSchedules("TDD"), buildVerifiedAgents(), state,
		bvRowImplReview-1, "2.Implementation")
}

func TestNextAndResumePoint_AgentFillingTwoRowsOfOnePhase_BothFollowTheRecordedRow(t *testing.T) {
	aw := mustParseAndAdmitV216(t, twoPlanningRowsV216Content, "two-planning-rows", "1.0")
	log := &runLog{}
	log.
		workflowStep("planner-tdd-soft", "", domain.StatusSUCCESS, tpRowDraft).
		workflowStep("plan-review", "", domain.StatusCOMPLETED_NEEDS_ACTION, tpRowReview).
		workflowStep("planner-tdd-soft", "", domain.StatusSUCCESS, tpRowFinalise).
		inPhase("PLANNING")

	requireNextAndResumeAgree(t, aw, singleSchedules("TDD"), twoPlanningRowsAgents(), log.state(),
		tpRowImplement-1, "1")
}

func TestNextAndResumePoint_RouteBackRunWithRecordedRows_AgreeAfterEveryStep(t *testing.T) {
	aw := mustParseAndAdmitV216(t, brownfieldBuildVerifiedV216Content, "brownfield-tdd-build-verified", "2.0")
	// steps is the number of buildVerifiedRun steps logged. The third step is
	// the tests review asking for changes, which is not a clean continue and
	// is covered by the route-back dispatch test.
	wantAfter := []struct {
		steps int
		row   int
		stage string
	}{
		{1, bvRowTestBuild - 1, "1.Test"},   // after the writer
		{2, bvRowTestsReview - 1, "1.Test"}, // after the Test-group build-review
		{4, bvRowTestBuild - 1, "1.Test"},   // writer again
		{5, bvRowTestsReview - 1, "1.Test"}, // build-review again
		{6, bvRowImplementation - 1, "1.Implementation"},
		{7, bvRowImplBuild - 1, "1.Implementation"},
		{8, bvRowImplReview - 1, "1.Implementation"},
	}
	for _, want := range wantAfter {
		step := buildVerifiedRun[want.steps-1]
		t.Run(step.agent+"@"+step.stage, func(t *testing.T) {
			log := newBuildVerifiedLog().play(buildVerifiedRun, want.steps)

			// The row the resume point names may already have finished (the
			// writer again after the review asked for changes): Next then
			// names it in a finished-work deviation instead of dispatching.
			requireNextAndResumeAgreeOrFinished(t, aw, singleSchedules("TDD"), buildVerifiedAgents(),
				resumeState(log, false), want.row, want.stage)
		})
	}
}
