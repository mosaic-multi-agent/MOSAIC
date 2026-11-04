package analysis_test

// Tests for repeat dispatches and the likely-rework heuristic.
//
// Dispatches are ordered by start time (zero start last, then instance id). A
// repeat is a dispatch whose agent type has an earlier dispatch. A repeat with
// a start time is likely rework when the previous dispatch of its type did not
// end SUCCESS, or when the most recent other dispatch that had ended by its
// start ended COMPLETED_NEEDS_ACTION. First dispatches are never likely rework.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestRepeats_ExcludeEachTypesFirstDispatchInStartOrder(t *testing.T) {
	// coder#9 started first, so coder#1 is the repeat despite its lower id.
	run := rwPriceRun(t,
		rwClosed("coder#9", "coder", "SUCCESS", 0, 10, 1000),
		rwClosed("coder#1", "coder", "SUCCESS", 100, 110, 2000),
		rwClosed("tester#1", "tester", "SUCCESS", 50, 60, 4000),
	)

	if rwDispatchOf(t, run, "coder#9").Repeat {
		t.Error("coder#9 (first of its type by start time) is marked as a repeat")
	}
	if !rwDispatchOf(t, run, "coder#1").Repeat {
		t.Error("coder#1 (second of its type by start time) is not marked as a repeat")
	}
	if rwDispatchOf(t, run, "tester#1").Repeat {
		t.Error("tester#1 (only dispatch of its type) is marked as a repeat")
	}
	rwAssertGroup(t, "coder repeats", rwTypeGroup(t, run, "coder").Rework.Repeats, 1, 2000*3000)
	rwAssertEmptyGroup(t, "tester repeats", rwTypeGroup(t, run, "tester").Rework.Repeats)
}

func TestRepeats_EqualStartTimesOrderByInstanceId(t *testing.T) {
	run := rwPriceRun(t,
		rwClosed("coder#2", "coder", "SUCCESS", 10, 20, 2000),
		rwClosed("coder#1", "coder", "SUCCESS", 10, 20, 1000),
	)

	if rwDispatchOf(t, run, "coder#1").Repeat {
		t.Error("coder#1 is marked as a repeat, want the first of the tie")
	}
	if !rwDispatchOf(t, run, "coder#2").Repeat {
		t.Error("coder#2 is not marked as a repeat")
	}
}

func TestRepeats_DispatchWithoutStartTimeComesLast(t *testing.T) {
	// coder#1 has no start time, so it sorts after coder#2 despite its lower id.
	run := rwPriceRun(t,
		rwClosed("coder#1", "coder", "SUCCESS", rwNoTime, 10, 1000),
		rwClosed("coder#2", "coder", "SUCCESS", 5, 10, 2000),
	)

	if rwDispatchOf(t, run, "coder#2").Repeat {
		t.Error("coder#2 (the only dispatch with a start time) is marked as a repeat")
	}
	if !rwDispatchOf(t, run, "coder#1").Repeat {
		t.Error("coder#1 (no start time, ordered last) is not marked as a repeat")
	}
}

func TestRepeats_UnknownTypeGroupsLikeAnyOtherType(t *testing.T) {
	run := rwPriceRun(t,
		rwClosed("a#1", "", "SUCCESS", 0, 10, 1000),
		rwClosed("b#1", "", "SUCCESS", 20, 30, 2000),
	)

	rwAssertGroup(t, "unknown-type repeats", rwTypeGroup(t, run, "").Rework.Repeats, 1, 2000*3000)
}

func TestLikelyRework_RepeatAfterPreviousOfSameTypeNotSucceeding(t *testing.T) {
	cases := []struct {
		name     string
		previous rwDispatch
	}{
		{"other status", rwClosed("coder#1", "coder", "FAILED", 0, 10, 1000)},
		{"no status", rwClosed("coder#1", "coder", "", 0, 10, 1000)},
		{"open", rwOpen("coder#1", "coder", 0, 1000)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := rwPriceRun(t, tc.previous, rwClosed("coder#2", "coder", "SUCCESS", 20, 30, 2000))

			if !rwDispatchOf(t, run, "coder#2").LikelyRework {
				t.Errorf("coder#2 after a %s dispatch is not likely rework", tc.name)
			}
			if rwDispatchOf(t, run, "coder#1").LikelyRework {
				t.Error("coder#1 (first of its type) is likely rework")
			}
		})
	}
}

func TestLikelyRework_RepeatAfterSuccessfulPreviousIsPlanned(t *testing.T) {
	// coder#2 follows a failure; coder#3 follows coder#2's success.
	run := rwPriceRun(t,
		rwClosed("coder#1", "coder", "FAILED", 0, 10, 1000),
		rwClosed("coder#2", "coder", "SUCCESS", 20, 30, 2000),
		rwClosed("coder#3", "coder", "SUCCESS", 40, 50, 4000),
	)

	if !rwDispatchOf(t, run, "coder#2").LikelyRework {
		t.Error("coder#2 (previous failed) is not likely rework")
	}
	d3 := rwDispatchOf(t, run, "coder#3")
	if !d3.Repeat || d3.LikelyRework {
		t.Errorf("coder#3: repeat=%v likelyRework=%v, want a planned repeat (true, false)", d3.Repeat, d3.LikelyRework)
	}
}

// rwCoderAfterReview runs coder#1 (SUCCESS, ends 10), then a review dispatch
// ending at reviewEnd with reviewStatus, then coder#2 starting at coder2Start;
// it returns coder#2.
func rwCoderAfterReview(t *testing.T, reviewStatus string, reviewEnd, coder2Start int, extra ...rwDispatch) domain.Dispatch {
	t.Helper()
	dispatches := []rwDispatch{
		rwClosed("coder#1", "coder", "SUCCESS", 0, 10, 1000),
		rwClosed("review#1", "review", reviewStatus, 12, reviewEnd, 500),
		rwClosed("coder#2", "coder", "SUCCESS", coder2Start, coder2Start+10, 2000),
	}
	run := rwPriceRun(t, append(dispatches, extra...)...)
	return rwDispatchOf(t, run, "coder#2")
}

func TestLikelyRework_RepeatRightAfterAnotherAgentNeedsAction(t *testing.T) {
	got := rwCoderAfterReview(t, "COMPLETED_NEEDS_ACTION", 20, 30)

	if !got.Repeat || !got.LikelyRework {
		t.Errorf("coder#2: repeat=%v likelyRework=%v, want both true", got.Repeat, got.LikelyRework)
	}
}

func TestLikelyRework_NeedsActionEndingExactlyAtStartCounts(t *testing.T) {
	got := rwCoderAfterReview(t, "COMPLETED_NEEDS_ACTION", 30, 30)

	if !got.LikelyRework {
		t.Error("coder#2 starting exactly when the review ended is not likely rework")
	}
}

func TestLikelyRework_NeedsActionEndingAfterStartDoesNotCount(t *testing.T) {
	got := rwCoderAfterReview(t, "COMPLETED_NEEDS_ACTION", 40, 30)

	if got.LikelyRework {
		t.Error("coder#2 is likely rework although the review ended after it started")
	}
}

func TestLikelyRework_OtherAgentEndingSuccessfullyDoesNotCount(t *testing.T) {
	got := rwCoderAfterReview(t, "SUCCESS", 20, 30)

	if !got.Repeat || got.LikelyRework {
		t.Errorf("coder#2: repeat=%v likelyRework=%v, want a planned repeat (true, false)", got.Repeat, got.LikelyRework)
	}
}

func TestLikelyRework_OnlyTheMostRecentEndedDispatchDecides(t *testing.T) {
	later := rwClosed("tester#1", "tester", "SUCCESS", 21, 25, 500)

	got := rwCoderAfterReview(t, "COMPLETED_NEEDS_ACTION", 20, 30, later)

	if got.LikelyRework {
		t.Error("coder#2 is likely rework although a later dispatch ended SUCCESS before it started")
	}
}

func TestLikelyRework_AnyOfSeveralDispatchesTyingOnEndTimeQualifies(t *testing.T) {
	tied := rwClosed("tester#1", "tester", "SUCCESS", 15, 20, 500)

	got := rwCoderAfterReview(t, "COMPLETED_NEEDS_ACTION", 20, 30, tied)

	if !got.LikelyRework {
		t.Error("coder#2 is not likely rework although a dispatch ending at the same instant needed action")
	}
}

func TestLikelyRework_NeedsActionWithoutEndTimeDoesNotCount(t *testing.T) {
	got := rwCoderAfterReview(t, "COMPLETED_NEEDS_ACTION", rwNoTime, 30)

	if got.LikelyRework {
		t.Error("coder#2 is likely rework although the review has no end time")
	}
}

func TestLikelyRework_FirstDispatchOfATypeIsNeverLikelyRework(t *testing.T) {
	run := rwPriceRun(t,
		rwClosed("review#1", "review", "COMPLETED_NEEDS_ACTION", 0, 10, 500),
		rwClosed("coder#1", "coder", "SUCCESS", 20, 30, 1000),
	)

	d := rwDispatchOf(t, run, "coder#1")
	if d.Repeat || d.LikelyRework {
		t.Errorf("coder#1: repeat=%v likelyRework=%v, want a first dispatch (false, false)", d.Repeat, d.LikelyRework)
	}
}

func TestLikelyRework_RepeatWithoutStartTimeIsNeverLikelyRework(t *testing.T) {
	// coder#1 has no start time, so it is ordered last, after the failed coder#2.
	run := rwPriceRun(t,
		rwClosed("coder#2", "coder", "FAILED", 5, 10, 2000),
		rwClosed("coder#1", "coder", "SUCCESS", rwNoTime, 10, 1000),
	)

	d := rwDispatchOf(t, run, "coder#1")
	if !d.Repeat {
		t.Error("coder#1 is not a repeat")
	}
	if d.LikelyRework {
		t.Error("coder#1 without a start time is likely rework")
	}
}

func TestRework_CountsAndCostsPerTypeAndForTheRun(t *testing.T) {
	run := rwPriceRun(t,
		rwClosed("coder#1", "coder", "FAILED", 0, 10, 1000),
		rwClosed("coder#2", "coder", "SUCCESS", 20, 30, 2000),
		rwClosed("coder#3", "coder", "SUCCESS", 40, 50, 4000),
		rwClosed("tester#1", "tester", "SUCCESS", 60, 70, 8000),
		rwClosed("tester#2", "tester", "SUCCESS", 80, 90, 16000),
	)

	coder := rwTypeGroup(t, run, "coder").Rework
	tester := rwTypeGroup(t, run, "tester").Rework

	rwAssertGroup(t, "coder repeats", coder.Repeats, 2, 6000*3000)
	rwAssertGroup(t, "coder likely rework", coder.LikelyRework, 1, 2000*3000)
	rwAssertGroup(t, "tester repeats", tester.Repeats, 1, 16000*3000)
	rwAssertEmptyGroup(t, "tester likely rework", tester.LikelyRework)
	rwAssertGroup(t, "run repeats", run.Rework.Repeats, 3, 22000*3000)
	rwAssertGroup(t, "run likely rework", run.Rework.LikelyRework, 1, 2000*3000)
}

func TestRework_UnpricedMemberMakesFiguresPartialNeverZero(t *testing.T) {
	second := rwClosed("coder#2", "coder", "SUCCESS", 20, 30, 0)
	second.usage = rwUnpricedUsage("coder#2", 500)

	run := rwPriceRun(t, rwClosed("coder#1", "coder", "FAILED", 0, 10, 1000), second)

	for name, g := range map[string]domain.GroupFigures{
		"repeats":       run.Rework.Repeats,
		"likely rework": run.Rework.LikelyRework,
	} {
		if g.Invocations != 1 {
			t.Errorf("%s: invocations = %d, want 1", name, g.Invocations)
		}
		assertMoneyState(t, name+" total", g.Totals.Money.Total, domain.MoneyUnpriced)
		if g.Totals.Money.Complete {
			t.Errorf("%s: money is complete, want partial", name)
		}
	}
}
