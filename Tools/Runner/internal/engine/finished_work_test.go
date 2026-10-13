package engine_test

// Finished-work check on the brownfield TDD workflow: after an engine-routed
// SUCCESS in Modes 2 and 3, a next step that already has a SUCCESS at the same
// recorded row and iteration value is not dispatched again; the engine returns
// a finished-work deviation naming that step.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

func TestFinishedWork_PlannerSucceedsAfterDetour_PlanReviewFinished(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 4)
		log := fwStage4TestGroup(fwBase(&runLog{}, 3)).ok(rtPlanner, "", fwRowPlanner)

		dec := f.next(mode, log, false)

		info := requireFinishedWork(t, dec, fwRowPlanReview-1, "")
		if got := info.PlannedContinuation.Agent.Identifier; got != rtPlanReview {
			t.Errorf("planned continuation agent: want %q, got %q", rtPlanReview, got)
		}
		if got := info.PlannedContinuation.Request.AgentInstanceID; !strings.HasPrefix(got, rtPlanReview+"#") {
			t.Errorf("planned continuation must be a fully built step with an instance ID, got %q", got)
		}
	})
}

func TestFinishedWork_DeviationDescribesTheRowThatJustSucceeded(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 4)
		log := fwStage4TestGroup(fwBase(&runLog{}, 3)).ok(rtPlanner, "", fwRowPlanner)

		info := requireFinishedWork(t, f.next(mode, log, false), fwRowPlanReview-1, "")

		if info.CurrentRow != fwRowPlanner-1 {
			t.Errorf("CurrentRow: want the row that just succeeded (%d), got %d", fwRowPlanner-1, info.CurrentRow)
		}
		if info.CurrentPhase != "PLANNING" {
			t.Errorf("CurrentPhase: want PLANNING, got %q", info.CurrentPhase)
		}
		if info.CurrentStage != "" {
			t.Errorf("CurrentStage: want the recorded iteration value of that row (empty), got %q", info.CurrentStage)
		}
		if info.ErrorReason != "" {
			t.Errorf("ErrorReason: want empty, got %q", info.ErrorReason)
		}
		if got := len(info.ArtifactState.ExecutionLog); got != len(log.entries) {
			t.Errorf("ArtifactState must be the state the engine was given: want %d log rows, got %d",
				len(log.entries), got)
		}
	})
}

func TestFinishedWork_ResponseOfLiveCallIsTheRoutedSuccess(t *testing.T) {
	f := newFinishedWorkFixture(t, 4)
	log := fwStage4TestGroup(fwBase(&runLog{}, 3)).ok(rtPlanner, "", fwRowPlanner)

	info := requireFinishedWork(t, f.next(domain.ExecutionModeAuto, log, false), fwRowPlanReview-1, "")

	if info.Response.StatusCode != domain.StatusSUCCESS {
		t.Errorf("Response: want the routed SUCCESS response, got status %q", info.Response.StatusCode)
	}
}

func TestFinishedWork_PlanReviewSucceedsAfterDetour_ContractsDesignerFinished(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 4)
		log := fwStage4TestGroup(fwBase(&runLog{}, 3)).
			ok(rtPlanner, "", fwRowPlanner).ok(rtPlanReview, "", fwRowPlanReview)

		dec := f.next(mode, log, false)

		requireFinishedWork(t, dec, fwRowDesigner-1, "")
	})
}

func TestFinishedWork_ImplementationSucceeds_ReviewNotFinishedAtThatIteration_Dispatches(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 5)
		log := fwStage4TestGroup(fwBase(&runLog{}, 3)).
			ok(rtPlanner, "", fwRowPlanner).
			ok(fwImplementer, "4.Implementation", fwRowImplementer)

		dec := f.next(mode, log, false)

		// implementation-review has a SUCCESS at iterations 1-3 but not at 4.
		requireDispatchAt(t, dec, fwRowImplReview-1, "4.Implementation")
	})
}

func TestFinishedWork_ImplementationReviewSucceeds_NextIterationRoutedAsToday(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 5)
		log := fwStage4TestGroup(fwBase(&runLog{}, 3)).
			ok(fwImplementer, "4.Implementation", fwRowImplementer).
			ok(fwImplReview, "4.Implementation", fwRowImplReview)

		dec := f.next(mode, log, false)

		requireDispatchAt(t, dec, fwRowTestWriter-1, "5.Test")
	})
}

func TestFinishedWork_BlockReEntryOntoFinishedRow_Deviates(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 4)
		log := fwStage4TestGroup(fwBase(&runLog{}, 3)).
			ok(fwDesigner, "", fwRowDesigner).ok(fwContractReview, "", fwRowContractReview)

		dec := f.next(mode, log, false)

		// Iterations 1-3 are completed and skipped; the re-entry lands on
		// iteration 4, whose test-writer row has finished.
		requireFinishedWork(t, dec, fwRowTestWriter-1, "4.Test")
	})
}

func TestFinishedWork_NewIterationFirstRows_Dispatch(t *testing.T) {
	cases := []struct {
		name      string
		after     int
		wantRow   int
		wantStage string
	}{
		{"after the first row of the new iteration", 1, fwRowTestsReview - 1, "6.Test"},
		{"after its test group", 2, fwRowImplementer - 1, "6.Implementation"},
		{"after its implementation row", 3, fwRowImplReview - 1, "6.Implementation"},
	}
	for _, tc := range cases {
		forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
			t.Run(tc.name, func(t *testing.T) {
				f := newFinishedWorkFixture(t, 6)

				dec := f.next(mode, fwNewIteration(tc.after), false)

				requireDispatchAt(t, dec, tc.wantRow, tc.wantStage)
			})
		})
	}
}

func TestFinishedWork_NewIterationEnds_TestRunnerFinished(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 6)

		dec := f.next(mode, fwNewIteration(4), false)

		requireFinishedWork(t, dec, fwRowTestRunner-1, "")
	})
}

func TestFinishedWork_ReviewerAlreadyPassedAtIteration_CreatorRerun_Deviates(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 4)
		log := fwBase(&runLog{}, 3).
			ok(rtTestWriter, "4.Test", fwRowTestWriter).
			ok(rtTestsReview, "4.Test", fwRowTestsReview).
			blocked(fwImplementer, "4.Implementation", domain.ErrorPERMISSION_DENIED, fwRowImplementer).
			ok(rtTestWriter, "4.Test", fwRowTestWriter)

		dec := f.next(mode, log, false)

		requireFinishedWork(t, dec, fwRowTestsReview-1, "4.Test")
	})
}

func TestFinishedWork_Mode3FindingsRouteBack_FinishedCreatorIsDispatched(t *testing.T) {
	f := newFinishedWorkFixture(t, 4)
	log := fwBase(&runLog{}, 3).
		ok(rtTestWriter, "4.Test", fwRowTestWriter).
		stepWith(rtTestsReview, "4.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone,
			fwRowTestsReview, "needs work")

	dec := f.next(domain.ExecutionModeAutoReview, log, false)

	// The On Findings route is exempt although the creator has finished.
	requireDispatchAt(t, dec, fwRowTestWriter-1, "4.Test")
}

func TestFinishedWork_Mode3AfterRouteBackCreatorSucceeds_ReviewerHasNoSuccessAtIteration_Dispatches(t *testing.T) {
	f := newFinishedWorkFixture(t, 4)
	log := fwBase(&runLog{}, 3).
		ok(rtTestWriter, "4.Test", fwRowTestWriter).
		stepWith(rtTestsReview, "4.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone,
			fwRowTestsReview, "needs work").
		ok(rtTestWriter, "4.Test", fwRowTestWriter)

	dec := f.next(domain.ExecutionModeAutoReview, log, false)

	// tests-review has SUCCESS at iterations 1-3 and CNA at 4, no SUCCESS at 4.
	requireDispatchAt(t, dec, fwRowTestsReview-1, "4.Test")
}

func TestFinishedWork_Mode2ReviewerFindings_DeviatesAsNonSuccess(t *testing.T) {
	f := newFinishedWorkFixture(t, 4)
	log := fwBase(&runLog{}, 3).
		ok(rtTestWriter, "4.Test", fwRowTestWriter).
		stepWith(rtTestsReview, "4.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone,
			fwRowTestsReview, "needs work")

	dec := f.next(domain.ExecutionModeAuto, log, false)

	requireNonSuccessDeviation(t, dec)
}

func TestFinishedWork_Mode2AfterOrchestratorCreatorSucceeds_ReviewerHasNoSuccessAtIteration_Dispatches(t *testing.T) {
	f := newFinishedWorkFixture(t, 4)
	log := fwBase(&runLog{}, 3).
		ok(rtTestWriter, "4.Test", fwRowTestWriter).
		stepWith(rtTestsReview, "4.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone,
			fwRowTestsReview, "needs work").
		ok(rtTestWriter, "4.Test", fwRowTestWriter)

	dec := f.next(domain.ExecutionModeAuto, log, false)

	requireDispatchAt(t, dec, fwRowTestsReview-1, "4.Test")
}

func TestFinishedWork_OrchestratedMode_StillConsultsWithoutFinishedWorkKind(t *testing.T) {
	f := newFinishedWorkFixture(t, 4)
	log := fwStage4TestGroup(fwBase(&runLog{}, 3)).ok(rtPlanner, "", fwRowPlanner)

	dec := f.next(domain.ExecutionModeOrchestrated, log, false)

	requireConsult(t, dec)
}

func TestFinishedWork_HappyPathWithoutRouteBack_NeverDeviates(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 2)

		trace := scopeRun{aw: f.aw, schedules: f.schedules, mode: mode}.play(t)

		requireRowsThenComplete(t, trace, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 7, 8, 9, 10, 11})
	})
}
