package engine_test

// The limits observable through Next (E501 budget, PARTIALLY_DONE run length,
// review-loop limit) count entries of one row at one iteration as one, in the
// 3.0 spelling "{ID}.{Group}". An entry whose value its row does not accept,
// including the pre-3.0 spelling "{Group}.{ID}", is treated like an entry of
// another iteration.

import (
	"strconv"
	"testing"

	"mosaic-run/internal/domain"
)

// ivWriterLog builds a log of the test-writer row, one entry per recorded
// value, each with the given appender (blocked / partial / ok).
func ivWriterLog(add func(l *runLog, value string), values ...string) *runLog {
	l := &runLog{}
	for _, v := range values {
		add(l, v)
	}
	return l
}

func ivBlockedWriter(l *runLog, value string) {
	l.blocked(rtTestWriter, value, e501, rtRowTestWriter)
}

func ivPartialWriter(l *runLog, value string) { l.partial(rtTestWriter, value, rtRowTestWriter) }

func TestNext_E501_EntriesOfOneIteration_SpendOneBudget(t *testing.T) {
	f := newRetryFixture(t)
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		t.Run("budget left", func(t *testing.T) {
			log := ivWriterLog(ivBlockedWriter, "1.Test", "1.Test")

			requireRetryStep(t, f.next(mode, log, false), rtRowTestWriter, "1.Test", domain.RetryToolUnavailable)
		})
		t.Run("limit reached", func(t *testing.T) {
			log := ivWriterLog(ivBlockedWriter, "1.Test", "1.Test", "1.Test")

			requireNonSuccessDeviation(t, f.next(mode, log, false))
		})
		t.Run("limit reached on resume", func(t *testing.T) {
			log := ivWriterLog(ivBlockedWriter, "1.Test", "1.Test", "1.Test")

			requireNonSuccessDeviation(t, f.next(mode, log, true))
		})
		t.Run("another iteration has its own budget", func(t *testing.T) {
			log := ivWriterLog(ivBlockedWriter, "1.Test", "1.Test", "2.Test")

			requireRetryStep(t, f.next(mode, log, false), rtRowTestWriter, "2.Test", domain.RetryToolUnavailable)
		})
		t.Run("pre-3.0 entries spend no budget", func(t *testing.T) {
			log := ivWriterLog(ivBlockedWriter, "Test.1", "Test.1", "1.Test")

			requireRetryStep(t, f.next(mode, log, false), rtRowTestWriter, "1.Test", domain.RetryToolUnavailable)
		})
	})
}

func TestNext_E501_SuccessOfTheIteration_ResetsTheBudget(t *testing.T) {
	f := newRetryFixture(t)
	log := ivWriterLog(ivBlockedWriter, "1.Test", "1.Test")
	log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
	log.blocked(rtTestWriter, "1.Test", e501, rtRowTestWriter)
	log.blocked(rtTestWriter, "1.Test", e501, rtRowTestWriter)

	dec := f.next(domain.ExecutionModeAuto, log, false)

	requireRetryStep(t, dec, rtRowTestWriter, "1.Test", domain.RetryToolUnavailable)
}

// A SUCCESS entry recorded in the pre-3.0 spelling is a value the row does not
// accept: it does not reset the budget.
func TestNext_E501_PreThreeOhSuccess_DoesNotResetTheBudget(t *testing.T) {
	f := newRetryFixture(t)
	log := ivWriterLog(ivBlockedWriter, "1.Test", "1.Test")
	log.ok(rtTestWriter, "Test.1", rtRowTestWriter)
	log.blocked(rtTestWriter, "1.Test", e501, rtRowTestWriter)

	dec := f.next(domain.ExecutionModeAuto, log, false)

	requireNonSuccessDeviation(t, dec)
}

func TestNext_PartiallyDone_EntriesOfOneIteration_FormOneRun(t *testing.T) {
	f := newRetryFixture(t)
	for _, mode := range autoModes {
		t.Run(modeName(mode), func(t *testing.T) {
			atLimit := ivWriterLog(ivPartialWriter, "1.Test", "1.Test", "1.Test")
			overLimit := ivWriterLog(ivPartialWriter, "1.Test", "1.Test", "1.Test", "1.Test")

			requireRetryStep(t, f.next(mode, atLimit, false), rtRowTestWriter, "1.Test", domain.RetryPartiallyDone)
			requireNonSuccessDeviation(t, f.next(mode, overLimit, false))
		})
	}
}

// Regression coverage for an unreadable entry ending the run: it fails if the
// entry were skipped.
func TestNext_PartiallyDone_EntryTheRowDoesNotAccept_EndsTheRun(t *testing.T) {
	f := newRetryFixture(t)
	values := []string{"1.Test", "1.Test", "Wrong.1", "1.Test", "1.Test"}
	log := ivWriterLog(ivPartialWriter, values...)

	dec := f.next(domain.ExecutionModeAuto, log, false)

	requireRetryStep(t, dec, rtRowTestWriter, "1.Test", domain.RetryPartiallyDone)
}

// A pre-3.0 entry is a value the row does not accept: it ends the run instead
// of joining it. Read as the same iteration the five entries would be over the
// limit; ending the run leaves two.
func TestNext_PartiallyDone_PreThreeOhEntry_EndsTheRun(t *testing.T) {
	f := newRetryFixture(t)
	log := ivWriterLog(ivPartialWriter, "1.Test", "1.Test", "Test.1", "1.Test", "1.Test")

	dec := f.next(domain.ExecutionModeAuto, log, false)

	requireRetryStep(t, dec, rtRowTestWriter, "1.Test", domain.RetryPartiallyDone)
}

// Four entries follow the unreadable one and form one run over the limit.
func TestNext_PartiallyDone_EntriesAfterAnUnreadableEntry_FormOneRun(t *testing.T) {
	f := newRetryFixture(t)
	log := ivWriterLog(ivPartialWriter, "1.Test", "Wrong.1", "1.Test", "1.Test", "1.Test", "1.Test")

	dec := f.next(domain.ExecutionModeAuto, log, false)

	requireNonSuccessDeviation(t, dec)
}

// ivReviewLog records the test-writer completing and the tests-review row
// returning findings, once per value.
func ivReviewLog(values ...string) *runLog {
	l := &runLog{}
	for _, v := range values {
		l.ok(rtTestWriter, v, rtRowTestWriter)
		l.stepWith(rtTestsReview, v, domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone, rtRowTestsReview, "findings")
	}
	return l
}

// nextAtLoopLimit runs Next in Mode 3 for the findings that ended log.
func (f retryFixture) nextAtLoopLimit(log *runLog, limit int) domain.EngineDecision {
	state := log.state()
	state.ReviewLoopLimit = limit
	return nextAfterState(f.aw, f.schedules, f.agents, state, responseFor(log.last()))
}

func TestNext_Mode3_ReviewLoopLimit_EntriesOfOneIteration_CountAsOneLoop(t *testing.T) {
	for _, limit := range []int{2, 3} {
		t.Run("limit "+strconv.Itoa(limit), func(t *testing.T) {
			f := newRetryFixture(t)
			log := ivReviewLog("1.Test", "1.Test")

			dec := f.nextAtLoopLimit(log, limit)

			if limit == 2 {
				requireLoopLimitDeviation(t, dec)
			} else {
				requireRoutedTo(t, dec, rtTestWriter)
			}
		})
	}
}

func TestNext_Mode3_ReviewLoopLimit_AnotherIteration_DoesNotCount(t *testing.T) {
	f := newRetryFixture(t)
	log := ivReviewLog("1.Test", "2.Test")

	dec := f.nextAtLoopLimit(log, 2)

	requireRoutedTo(t, dec, rtTestWriter)
}

func TestNext_Mode3_ReviewLoopLimit_EntryTheRowDoesNotAccept_IsNotCounted(t *testing.T) {
	f := newRetryFixture(t)
	log := ivReviewLog("1.Test", "Wrong.1", "1.Test")

	dec := f.nextAtLoopLimit(log, 3)

	requireRoutedTo(t, dec, rtTestWriter)
}

func TestNext_Mode3_ReviewLoopLimit_PreThreeOhEntry_IsNotCounted(t *testing.T) {
	f := newRetryFixture(t)
	log := ivReviewLog("1.Test", "Test.1", "1.Test")

	dec := f.nextAtLoopLimit(log, 3)

	requireRoutedTo(t, dec, rtTestWriter)
}

func TestNext_Mode3_ReviewLoopLimit_ReviewerSuccessOfTheIteration_ResetsTheCount(t *testing.T) {
	f := newRetryFixture(t)
	log := &runLog{}
	log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
	log.stepWith(rtTestsReview, "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone, rtRowTestsReview, "findings")
	log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
	log.ok(rtTestsReview, "1.Test", rtRowTestsReview)
	log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
	log.stepWith(rtTestsReview, "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone, rtRowTestsReview, "findings")

	dec := f.nextAtLoopLimit(log, 2)

	requireRoutedTo(t, dec, rtTestWriter)
}

// A reviewer SUCCESS recorded in the pre-3.0 spelling is a value the row does
// not accept: it does not reset the count.
func TestNext_Mode3_ReviewLoopLimit_PreThreeOhReviewerSuccess_DoesNotResetTheCount(t *testing.T) {
	f := newRetryFixture(t)
	log := &runLog{}
	log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
	log.stepWith(rtTestsReview, "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone, rtRowTestsReview, "findings")
	log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
	log.ok(rtTestsReview, "Test.1", rtRowTestsReview)
	log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
	log.stepWith(rtTestsReview, "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone, rtRowTestsReview, "findings")

	dec := f.nextAtLoopLimit(log, 2)

	requireLoopLimitDeviation(t, dec)
}

// A reviewer entry without a recorded row is read with the row of the
// position, so a 3.0 value counts with the row-recorded entries of the same
// iteration, and a pre-3.0 value does not.
func TestNext_Mode3_ReviewLoopLimit_ReviewerEntryWithoutRecordedRow_CountsOnlyInTheThreeOhSpelling(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		wantLimit   bool
	}{
		{"3.0 value", "1.Test", true},
		{"pre-3.0 value", "Test.1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRetryFixture(t)
			log := &runLog{}
			log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
			log.stepWith(rtTestsReview, tc.value, domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone,
				int(domain.NoWorkflowRow), "findings")
			log.ok(rtTestWriter, "1.Test", rtRowTestWriter)
			log.stepWith(rtTestsReview, "1.Test", domain.StatusCOMPLETED_NEEDS_ACTION, domain.ErrorNone,
				rtRowTestsReview, "findings")

			dec := f.nextAtLoopLimit(log, 2)

			if tc.wantLimit {
				requireLoopLimitDeviation(t, dec)
			} else {
				requireRoutedTo(t, dec, rtTestWriter)
			}
		})
	}
}

func TestNext_Mode3_ReviewLoopLimit_ReachedWithOneIteration_ThreeLoops(t *testing.T) {
	f := newRetryFixture(t)
	log := ivReviewLog("1.Test", "1.Test", "1.Test")

	dec := f.nextAtLoopLimit(log, 3)

	requireLoopLimitDeviation(t, dec)
}
