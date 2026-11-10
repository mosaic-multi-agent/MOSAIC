package engine_test

// Finished-work check: mechanical retries are exempt, and resume derives the
// same decision as the live loop.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestFinishedWork_PartiallyDoneRetryOfFinishedRow_IsNotBlocked(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 4)
		log := fwBase(&runLog{}, 3).
			ok(rtTestWriter, "4.Test", fwRowTestWriter).
			ok(rtTestsReview, "4.Test", fwRowTestsReview).
			partial(rtTestsReview, "4.Test", fwRowTestsReview)

		dec := f.next(mode, log, false)

		requireRetryStep(t, dec, fwRowTestsReview, "4.Test", domain.RetryPartiallyDone)
	})
}

func TestFinishedWork_E501RetryOfFinishedRow_IsNotBlocked(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 4)
		log := fwBase(&runLog{}, 3).
			ok(rtTestWriter, "4.Test", fwRowTestWriter).
			ok(rtTestsReview, "4.Test", fwRowTestsReview).
			blocked(rtTestsReview, "4.Test", domain.ErrorTOOL_UNAVAILABLE, fwRowTestsReview)

		dec := f.next(mode, log, false)

		requireRetryStep(t, dec, fwRowTestsReview, "4.Test", domain.RetryToolUnavailable)
	})
}

// A resumed run calls Next without a response; the decision is the one the
// live loop reached, with the same finished step.
func TestFinishedWork_ResumeYieldsTheLiveDecision(t *testing.T) {
	for _, sc := range finishedWorkScenarios() {
		forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
			t.Run(sc.name, func(t *testing.T) {
				f := newFinishedWorkFixture(t, sc.iterations)

				live := requireFinishedWork(t, f.next(mode, sc.log, false), sc.wantRow, sc.wantStage)
				resumed := requireFinishedWork(t, f.next(mode, sc.log, true), sc.wantRow, sc.wantStage)

				// Response is not compared: it is the zero value on resume.
				if live.CurrentRow != resumed.CurrentRow || live.CurrentPhase != resumed.CurrentPhase ||
					live.CurrentStage != resumed.CurrentStage {
					t.Errorf("current row/phase/stage differ: live (%d,%q,%q), resume (%d,%q,%q)",
						live.CurrentRow, live.CurrentPhase, live.CurrentStage,
						resumed.CurrentRow, resumed.CurrentPhase, resumed.CurrentStage)
				}
				if live.PlannedContinuation.Phase != resumed.PlannedContinuation.Phase {
					t.Errorf("planned phase differs: live %q, resume %q",
						live.PlannedContinuation.Phase, resumed.PlannedContinuation.Phase)
				}
			})
		})
	}
}

func TestFinishedWork_ResumeWhenNextStepNotFinished_DispatchesSameStepAsLive(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		f := newFinishedWorkFixture(t, 5)
		log := fwStage4TestGroup(fwBase(&runLog{}, 3)).
			ok(fwImplementer, "4.Implementation", fwRowImplementer)

		live := requireDispatchAt(t, f.next(mode, log, false), fwRowImplReview-1, "4.Implementation")
		resumed := requireDispatchAt(t, f.next(mode, log, true), fwRowImplReview-1, "4.Implementation")

		if live.Phase != resumed.Phase {
			t.Errorf("phase differs: live %q, resume %q", live.Phase, resumed.Phase)
		}
	})
}

// Every finished next step of the scenarios is a finished-work deviation, in
// every automatic mode and from the same log; a deviation never carries a
// dispatch, completion or stop.
func TestFinishedWork_DecisionIsOnlyTheDeviation(t *testing.T) {
	for _, sc := range finishedWorkScenarios() {
		forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
			t.Run(sc.name, func(t *testing.T) {
				f := newFinishedWorkFixture(t, sc.iterations)

				dec := f.next(mode, sc.log, false)

				requireFinishedWork(t, dec, sc.wantRow, sc.wantStage)
				if dec.Dispatch != nil || dec.Complete != nil || dec.Stop != nil || dec.Consult != nil {
					t.Errorf("decision carries more than the deviation: %+v", dec)
				}
			})
		})
	}
}
