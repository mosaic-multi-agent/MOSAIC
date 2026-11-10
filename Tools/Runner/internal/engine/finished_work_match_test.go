package engine_test

// Finished-work check: which Execution Log entries mark a step finished.
// The recorded WorkflowRow is 1-based and the dispatched row index 0-based;
// the first and last rows of a table pin the conversion. Recorded iteration
// values are compared as read for the row; unreadable ones never count.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// iterationLog is the log of a finished iteration of the fwEdge table.
func iterationLog(id string) []fwRec {
	return []fwRec{
		{agent: "worker-one", stage: id, row: edgeWorkerOneRow},
		{agent: "worker-two", stage: id, row: edgeWorkerTwoRow},
		{agent: "checker", stage: id, row: edgeCheckerRow},
	}
}

// bothIterations is the log of both iterations finished, preceded by first.
func bothIterations(first fwRec) []fwRec {
	return append(append([]fwRec{first}, iterationLog("A")...), iterationLog("B")...)
}

// ---- Row numbering ----

// firstRowLog is the log after which the next step is the first row of the
// table at iteration "B"; first is an earlier entry of that step.
func firstRowLog(first fwRec) []fwRec {
	return append([]fwRec{first}, iterationLog("A")...)
}

func TestFinishedWork_FirstRowOfTable_RecordedRowOne_Deviates(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		dec := e.next(firstRowLog(fwRec{agent: "worker-one", stage: "B", row: edgeWorkerOneRow}), false)

		requireFinishedWork(t, dec, 0, "B")
	})
}

func TestFinishedWork_FirstRowOfTable_EntryOnTheNeighbouringRow_Dispatches(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		dec := e.next(firstRowLog(fwRec{agent: "worker-one", stage: "B", row: edgeWorkerTwoRow}), false)

		requireDispatchAt(t, dec, 0, "B")
	})
}

func TestFinishedWork_LastRowOfTable_RecordedLastRow_Deviates(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		dec := e.next(bothIterations(fwRec{agent: "final-review", row: edgeFinalRow}), false)

		requireFinishedWork(t, dec, 3, "")
	})
}

func TestFinishedWork_LastRowOfTable_EntryOnNeighbouringOrOutOfTableRow_Dispatches(t *testing.T) {
	for _, row := range []int{edgeCheckerRow, edgeFinalRow + 1} {
		forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
			e := newFinishedWorkEdge(t, mode)

			dec := e.next(bothIterations(fwRec{agent: "final-review", row: row}), false)

			requireDispatchAt(t, dec, 3, "")
		})
	}
}

func TestFinishedWork_LastRowFinishedButRunCompletes_CompleteIsNotChecked(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)
		recs := append(bothIterations(fwRec{agent: "final-review", row: edgeFinalRow}),
			fwRec{agent: "final-review", row: edgeFinalRow})

		dec := e.next(recs, false)

		requireComplete(t, dec)
	})
}

// ---- Recorded entries that do not count ----

func TestFinishedWork_EntryWithoutRecordedRow_DoesNotCount(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		// Row "-" with the step's own agent and iteration: no fallback by agent.
		dec := e.next(firstRowLog(fwRec{agent: "worker-one", stage: "B", row: 0}), false)

		requireDispatchAt(t, dec, 0, "B")
	})
}

func TestFinishedWork_NonBlockRowEntryNamingAnIteration_DoesNotCount(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		dec := e.next(bothIterations(fwRec{agent: "final-review", stage: "A", row: edgeFinalRow}), false)

		requireDispatchAt(t, dec, 3, "")
	})
}

// checkerRerun is the log of a worker-two rerun after the checker's entry x:
// the next step is the checker at iteration "A".
func checkerRerun(x fwRec) []fwRec {
	return []fwRec{
		{agent: "worker-one", stage: "A", row: edgeWorkerOneRow},
		{agent: "worker-two", stage: "A", row: edgeWorkerTwoRow},
		x,
		{agent: "worker-two", stage: "A", row: edgeWorkerTwoRow},
	}
}

func TestFinishedWork_CheckerSucceededAtSameIteration_Deviates(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		dec := e.next(checkerRerun(fwRec{agent: "checker", stage: "A", row: edgeCheckerRow}), false)

		requireFinishedWork(t, dec, 2, "A")
	})
}

func TestFinishedWork_IterationIdMatchedIgnoringCase(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		dec := e.next(checkerRerun(fwRec{agent: "checker", stage: "a", row: edgeCheckerRow}), false)

		requireFinishedWork(t, dec, 2, "A")
	})
}

func TestFinishedWork_SuccessAtAnotherIteration_Dispatches(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		dec := e.next(checkerRerun(fwRec{agent: "checker", stage: "B", row: edgeCheckerRow}), false)

		requireDispatchAt(t, dec, 2, "A")
	})
}

func TestFinishedWork_SuccessOnAnotherRowAtSameIteration_Dispatches(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)

		// The entry names the checker's agent but is recorded on worker-one's row.
		dec := e.next(checkerRerun(fwRec{agent: "checker", stage: "A", row: edgeWorkerOneRow}), false)

		requireDispatchAt(t, dec, 2, "A")
	})
}

func TestFinishedWork_UnreadableIterationValueOnBlockRow_DoesNotCount(t *testing.T) {
	for _, stage := range []string{"-", "", "not an iteration!"} {
		forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
			t.Run("stage "+stage, func(t *testing.T) {
				e := newFinishedWorkEdge(t, mode)

				dec := e.next(checkerRerun(fwRec{agent: "checker", stage: stage, row: edgeCheckerRow}), false)

				requireDispatchAt(t, dec, 2, "A")
			})
		})
	}
}

func TestFinishedWork_GroupedValueOfAnotherGroupOrCase_DoesNotCount(t *testing.T) {
	for _, stage := range []string{"4.Implementation", "4.test"} {
		forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
			t.Run("stage "+stage, func(t *testing.T) {
				f := newFinishedWorkFixture(t, 4)
				log := fwBase(&runLog{}, 3).
					ok(rtTestWriter, "4.Test", fwRowTestWriter).
					ok(rtTestsReview, stage, fwRowTestsReview).
					ok(rtTestWriter, "4.Test", fwRowTestWriter)

				dec := f.next(mode, log, false)

				requireDispatchAt(t, dec, fwRowTestsReview-1, "4.Test")
			})
		})
	}
}

// ---- Entries that count ----

func TestFinishedWork_RejectedAttemptLoggedAsSuccess_Counts(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)
		rejected := fwRec{agent: "checker", stage: "A", row: edgeCheckerRow, summary: "rejected by the user in HITL"}

		dec := e.next(checkerRerun(rejected), false)

		requireFinishedWork(t, dec, 2, "A")
	})
}

func TestFinishedWork_UnreadableEntryDoesNotHideAReadableOne(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)
		recs := []fwRec{
			{agent: "worker-one", stage: "A", row: edgeWorkerOneRow},
			{agent: "worker-two", stage: "A", row: edgeWorkerTwoRow},
			{agent: "checker", stage: "A", row: edgeCheckerRow},
			{agent: "checker", stage: "not an iteration!", row: edgeCheckerRow},
			{agent: "worker-two", stage: "A", row: edgeWorkerTwoRow},
		}

		dec := e.next(recs, false)

		requireFinishedWork(t, dec, 2, "A")
	})
}

func TestFinishedWork_ResumeOnEdgeFixture_YieldsTheLiveDecision(t *testing.T) {
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		e := newFinishedWorkEdge(t, mode)
		recs := checkerRerun(fwRec{agent: "checker", stage: "a", row: edgeCheckerRow})

		live := requireFinishedWork(t, e.next(recs, false), 2, "A")
		resumed := requireFinishedWork(t, e.next(recs, true), 2, "A")

		if live.CurrentRow != resumed.CurrentRow || live.CurrentStage != resumed.CurrentStage {
			t.Errorf("current row/stage differ: live (%d,%q), resume (%d,%q)",
				live.CurrentRow, live.CurrentStage, resumed.CurrentRow, resumed.CurrentStage)
		}
	})
}
