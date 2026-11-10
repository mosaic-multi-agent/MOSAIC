package engine_test

// A row of a named group records its iteration in the 3.0 spelling
// "{ID}.{Group}". Next reads such entries with the row's group, so the
// PARTIALLY_DONE run length and the E501 budget count them. The old spellings
// ("Stage-N", "{Group}.{ID}") are not iteration 1 of the group. The log
// counters themselves are covered in iteration_value_counters_test.go.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// requireRetryOfWriter requires a dispatch of the test-writer row of the given
// retry kind.
func requireRetryOfWriter(t *testing.T, dec domain.EngineDecision, kind domain.RetryKind) {
	t.Helper()
	step := requireDispatch(t, dec)
	if step.RowIndex != rtRowTestWriter-1 {
		t.Errorf("dispatched row index: want %d, got %d", rtRowTestWriter-1, step.RowIndex)
	}
	if step.Retry != kind {
		t.Errorf("dispatch Retry: want %q, got %q", kind, step.Retry)
	}
}

func TestNext_GroupedRow_PartiallyDoneEntriesRecordedThreeOh_FormOneRun(t *testing.T) {
	f := newRetryFixture(t)
	for _, mode := range autoModes {
		t.Run(modeName(mode), func(t *testing.T) {
			atLimit := ivWriterLog(ivPartialWriter, "1.Test", "1.Test", "1.Test")
			overLimit := ivWriterLog(ivPartialWriter, "1.Test", "1.Test", "1.Test", "1.Test")

			requireRetryOfWriter(t, f.next(mode, atLimit, false), domain.RetryPartiallyDone)
			requireNonSuccessDeviation(t, f.next(mode, overLimit, false))
		})
	}
}

// Values in the old spellings are no longer read as iteration 1 of the row's
// group: they end the run of 3.0 entries instead of joining it.
func TestNext_GroupedRow_PartiallyDoneOldSpellingEntries_DoNotJoinTheRun(t *testing.T) {
	f := newRetryFixture(t)
	for _, old := range []string{"Stage-1", "Test.1"} {
		t.Run(old, func(t *testing.T) {
			log := ivWriterLog(ivPartialWriter, "1.Test", "1.Test", old, "1.Test")

			dec := f.next(domain.ExecutionModeAuto, log, false)

			requireRetryOfWriter(t, dec, domain.RetryPartiallyDone)
		})
	}
}

func TestNext_GroupedRow_PartiallyDoneEntriesOfAnotherIteration_DoNotJoinTheRun(t *testing.T) {
	f := newRetryFixture(t)
	log := ivWriterLog(ivPartialWriter, "1.Test", "1.Test", "2.Test", "1.Test", "1.Test")

	dec := f.next(domain.ExecutionModeAuto, log, false)

	requireRetryOfWriter(t, dec, domain.RetryPartiallyDone)
}

func TestNext_GroupedRow_E501EntriesRecordedThreeOh_SpendOneBudget(t *testing.T) {
	f := newRetryFixture(t)
	forEachAutoMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		t.Run("budget left", func(t *testing.T) {
			log := ivWriterLog(ivBlockedWriter, "1.Test", "1.Test")

			requireRetryOfWriter(t, f.next(mode, log, false), domain.RetryToolUnavailable)
		})
		t.Run("limit reached", func(t *testing.T) {
			log := ivWriterLog(ivBlockedWriter, "1.Test", "1.Test", "1.Test")

			requireNonSuccessDeviation(t, f.next(mode, log, false))
		})
		t.Run("old spellings spend no budget", func(t *testing.T) {
			log := ivWriterLog(ivBlockedWriter, "Stage-1", "Test.1", "1.Test")

			requireRetryOfWriter(t, f.next(mode, log, false), domain.RetryToolUnavailable)
		})
	})
}
