package engine_test

// A force-stopped attempt is recorded as an unaccepted PARTIALLY_DONE row with
// the force-stop summary. Every log-derived counter reads the log as if that
// row were absent: the PARTIALLY_DONE run length, the E501 budget, the
// schedule-defect attempts and the review-loop count.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// forceStopped appends the unaccepted row of a killed attempt of a workflow row.
func (l *runLog) forceStopped(name, stage string, row int) *runLog {
	return l.stepWith(name, stage, domain.StatusPARTIALLY_DONE, domain.ErrorNone, row, domain.ForceStoppedSummary)
}

func forceStoppedEntry(index int, status domain.StatusCode) domain.ExecutionLogEntry {
	e := sdEntry(index, "", status)
	e.Status, e.Summary = domain.StatusPARTIALLY_DONE, domain.ForceStoppedSummary
	return e
}

// The bound counts the real PARTIALLY_DONE attempts only: the limit's worth of
// them around a force-stopped row still allows the mechanical re-dispatch.
func TestNext_PartiallyDone_ForceStoppedRowInsideTheRun_DoesNotCountTowardTheBound(t *testing.T) {
	f := newRetryFixture(t)
	for _, resume := range []bool{false, true} {
		log := (&runLog{}).
			repeat(engine.PartiallyDoneRedispatchLimit-1, func(l *runLog) { l.partial(rtPlanner, "", rtRowPlanner) }).
			forceStopped(rtPlanner, "", rtRowPlanner).
			partial(rtPlanner, "", rtRowPlanner).
			inPhase(rtPlanPhase)

		dec := f.next(domain.ExecutionModeAuto, log, resume)

		requireRetryStep(t, dec, rtRowPlanner, "", domain.RetryPartiallyDone)
	}
}

// The E501 budget is the one the log without the force-stopped row yields,
// wherever the row sits.
func TestE501BudgetRemaining_ForceStoppedRow_DoesNotChangeTheBudget(t *testing.T) {
	row := domain.RoutingRow{Index: rtRowPlanner - 1}
	oneE501 := func() *runLog { return (&runLog{}).blocked(rtPlanner, "", e501, rtRowPlanner) }
	without := oneE501().blocked(rtPlanner, "", e501, rtRowPlanner).inPhase(rtPlanPhase)
	want := engine.E501BudgetRemaining(without.entries, row, "")
	cases := map[string]*runLog{
		"between the E501 rows": oneE501().forceStopped(rtPlanner, "", rtRowPlanner).
			blocked(rtPlanner, "", e501, rtRowPlanner).inPhase(rtPlanPhase),
		"after the E501 rows": oneE501().blocked(rtPlanner, "", e501, rtRowPlanner).
			forceStopped(rtPlanner, "", rtRowPlanner).inPhase(rtPlanPhase),
		"before the E501 rows": (&runLog{}).forceStopped(rtPlanner, "", rtRowPlanner).
			blocked(rtPlanner, "", e501, rtRowPlanner).blocked(rtPlanner, "", e501, rtRowPlanner).inPhase(rtPlanPhase),
	}

	for name, log := range cases {
		if got := engine.E501BudgetRemaining(log.entries, row, ""); got != want {
			t.Errorf("%s: want the budget %d of the log without the force-stopped row, got %d", name, want, got)
		}
	}
}

// A force-stopped row of another routing-table row does not end the producer's
// trailing run, so its attempts on both sides still count together.
func TestScheduleDefectAttempts_ForceStoppedRowOfAnotherRow_DoesNotEndTheRun(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
		forceStoppedEntry(sdOtherIndex, domain.StatusPARTIALLY_DONE),
		sdEntry(sdProducerIndex, "", domain.StatusSUCCESS),
	}

	got := engine.ScheduleDefectAttempts(log, sdProducerRow(), "")

	if got != 2 {
		t.Errorf("want both producer attempts counted around the force-stopped row, got %d", got)
	}
}

// A reviewer row that directly follows the reviewer's findings is a re-dispatch
// and does not count. A force-stopped row between them is absent for the count,
// so the adjacency holds and the limit is not reached early.
func TestNext_Mode3_ReviewLoopLimit_ForceStoppedRow_DoesNotCountAsAnIteration(t *testing.T) {
	without := []domain.ExecutionLogEntry{
		rllRow(1, rllFixer, rllPhase, "", domain.StatusSUCCESS),
		rllRow(2, rllReviewer, rllPhase, "", domain.StatusCOMPLETED_NEEDS_ACTION),
		rllRow(4, rllReviewer, rllPhase, "", domain.StatusCOMPLETED_NEEDS_ACTION),
	}
	stopped := rllRow(3, rllReviewer, rllPhase, "", domain.StatusPARTIALLY_DONE)
	stopped.Summary = domain.ForceStoppedSummary
	with := []domain.ExecutionLogEntry{without[0], without[1], stopped, without[2]}

	requireRoutedTo(t, rllNext(t, domain.ExecutionModeAutoReview, 2, without), rllFixer)
	requireRoutedTo(t, rllNext(t, domain.ExecutionModeAutoReview, 2, with), rllFixer)
}
