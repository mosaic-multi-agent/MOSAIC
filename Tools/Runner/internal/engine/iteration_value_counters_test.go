package engine_test

// The log counters compare recorded values as parsed iterations, each entry's
// value read with the row it belongs to. Only the 3.0 spelling is read: a
// pre-3.0 spelling is a value the row does not accept. An entry whose value its
// row does not accept matches no value; a recorded value the row does not
// accept exhausts the counter.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

const ivGroupedIndex = 7

type ivBudgetCase struct {
	name     string
	log      []domain.ExecutionLogEntry
	recorded string
	want     int
}

func runBudgetCases(t *testing.T, row domain.RoutingRow, cases []ivBudgetCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := engine.E501BudgetRemaining(tc.log, row, tc.recorded)

			if got != tc.want {
				t.Errorf("E501BudgetRemaining: want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestE501BudgetRemaining_ThreeOhEntriesOfOneIterationAreOneBudget(t *testing.T) {
	row := ivGroupedRow(ivGroupedIndex, "Test")
	limit := engine.E501AttemptLimit

	runBudgetCases(t, row, []ivBudgetCase{
		{"two entries of one iteration", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "1.Test"), ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "1.Test", limit - 2},
		{"non-numeric ID", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "Acme.Test"), ivBlockedE501(ivGroupedIndex, "Acme.Test"),
		}, "Acme.Test", limit - 2},
		{"ID in another letter case", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "a.Test"), ivBlockedE501(ivGroupedIndex, "A.Test"),
		}, "A.Test", limit - 2},
		{"limit reached", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "1.Test"), ivBlockedE501(ivGroupedIndex, "1.Test"),
			ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "1.Test", 0},
		{"another iteration is another budget", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "1.Test"), ivBlockedE501(ivGroupedIndex, "2.Test"),
		}, "1.Test", limit - 1},
	})
}

func TestE501BudgetRemaining_PreThreeOhSpelling_IsNotTheIteration(t *testing.T) {
	row := ivGroupedRow(ivGroupedIndex, "Test")

	runBudgetCases(t, row, []ivBudgetCase{
		{"pre-3.0 entry beside a 3.0 entry is not counted", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "Test.1"), ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "1.Test", engine.E501AttemptLimit - 1},
		{"pre-3.0 position is exhausted", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "Test.1", 0},
	})
}

func TestE501BudgetRemaining_SuccessResetsTheBudgetOfItsIteration(t *testing.T) {
	row := ivGroupedRow(ivGroupedIndex, "Test")

	runBudgetCases(t, row, []ivBudgetCase{
		{"3.0 success after failures", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "1.Test"), ivBlockedE501(ivGroupedIndex, "1.Test"),
			ivEntry(ivGroupedIndex, "1.Test", domain.StatusSUCCESS),
			ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "1.Test", engine.E501AttemptLimit - 1},
		{"pre-3.0 success does not reset", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "1.Test"), ivBlockedE501(ivGroupedIndex, "1.Test"),
			ivEntry(ivGroupedIndex, "Test.1", domain.StatusSUCCESS),
			ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "1.Test", 0},
	})
}

func TestE501BudgetRemaining_EntryTheRowDoesNotAccept_IsNeitherCountedNorAReset(t *testing.T) {
	row := ivGroupedRow(ivGroupedIndex, "Test")

	runBudgetCases(t, row, []ivBudgetCase{
		{"unreadable failure is not counted", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "Wrong.1"), ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "1.Test", engine.E501AttemptLimit - 1},
		{"unreadable success does not reset", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "1.Test"), ivBlockedE501(ivGroupedIndex, "1.Test"),
			ivEntry(ivGroupedIndex, "Wrong.1", domain.StatusSUCCESS),
			ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "1.Test", 0},
		{"value without a group on a grouped row", []domain.ExecutionLogEntry{
			ivBlockedE501(ivGroupedIndex, "1"), ivBlockedE501(ivGroupedIndex, "1.Test"),
		}, "1.Test", engine.E501AttemptLimit - 1},
	})
}

func TestE501BudgetRemaining_RecordedValueTheRowDoesNotAccept_IsExhausted(t *testing.T) {
	grouped := ivGroupedRow(ivGroupedIndex, "Test")
	outside := ivOutsideRow(3)

	t.Run("wrong group", func(t *testing.T) {
		runBudgetCases(t, grouped, []ivBudgetCase{{"empty log", nil, "Wrong.1", 0}})
	})
	t.Run("empty value on a block row", func(t *testing.T) {
		runBudgetCases(t, grouped, []ivBudgetCase{{"empty log", nil, "", 0}})
	})
	t.Run("value on a row outside every block", func(t *testing.T) {
		runBudgetCases(t, outside, []ivBudgetCase{{"empty log", nil, "1", 0}})
	})
}

// "Stage-A" is an ordinary schedule ID: it is its own iteration, never "A".
func TestE501BudgetRemaining_UngroupedRow_StageDashIDIsItsOwnIteration(t *testing.T) {
	row := ivUngroupedRow(2)
	limit := engine.E501AttemptLimit

	runBudgetCases(t, row, []ivBudgetCase{
		{"plain position does not count Stage-A entries", []domain.ExecutionLogEntry{
			ivBlockedE501(2, "A"), ivBlockedE501(2, "Stage-A"),
		}, "A", limit - 1},
		{"Stage-A position counts only Stage-A entries", []domain.ExecutionLogEntry{
			ivBlockedE501(2, "A"), ivBlockedE501(2, "Stage-A"), ivBlockedE501(2, "Stage-A"),
		}, "Stage-A", limit - 2},
	})
}

func TestE501BudgetRemaining_RowOutsideEveryBlock_EmptyAndDashAreOneValue(t *testing.T) {
	row := ivOutsideRow(3)

	runBudgetCases(t, row, []ivBudgetCase{
		{"empty and dash entries", []domain.ExecutionLogEntry{
			ivBlockedE501(3, ""), ivBlockedE501(3, "-"),
		}, "", engine.E501AttemptLimit - 2},
	})
}

type ivAttemptCase struct {
	name     string
	log      []domain.ExecutionLogEntry
	recorded string
	want     int
}

func runAttemptCases(t *testing.T, row domain.RoutingRow, cases []ivAttemptCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := engine.ScheduleDefectAttempts(tc.log, row, tc.recorded)

			if got != tc.want {
				t.Errorf("ScheduleDefectAttempts: want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestScheduleDefectAttempts_ThreeOhEntriesOfOneIterationAreOneCount(t *testing.T) {
	row := ivGroupedRow(ivGroupedIndex, "Test")
	ok := func(value string) domain.ExecutionLogEntry {
		return ivEntry(ivGroupedIndex, value, domain.StatusSUCCESS)
	}

	runAttemptCases(t, row, []ivAttemptCase{
		{"two entries of one iteration", []domain.ExecutionLogEntry{ok("1.Test"), ok("1.Test")}, "1.Test", 2},
		{"non-numeric ID", []domain.ExecutionLogEntry{ok("Acme.Test"), ok("Acme.Test"), ok("Acme.Test")}, "Acme.Test", 3},
		{"another iteration is not counted and does not end the run", []domain.ExecutionLogEntry{
			ok("1.Test"), ok("2.Test"), ok("1.Test")}, "1.Test", 2},
		{"pre-3.0 entry beside a 3.0 entry is not counted", []domain.ExecutionLogEntry{
			ok("Test.1"), ok("1.Test")}, "1.Test", 1},
		{"pre-3.0 position is exhausted", []domain.ExecutionLogEntry{ok("1.Test")}, "Test.1",
			engine.ScheduleDefectRedispatchLimit + 1},
	})
}

func TestScheduleDefectAttempts_EntryTheRowDoesNotAccept_IsNeitherCountedNorEndsTheRun(t *testing.T) {
	row := ivGroupedRow(ivGroupedIndex, "Test")
	ok := func(value string) domain.ExecutionLogEntry {
		return ivEntry(ivGroupedIndex, value, domain.StatusSUCCESS)
	}

	runAttemptCases(t, row, []ivAttemptCase{
		{"wrong group between attempts", []domain.ExecutionLogEntry{ok("1.Test"), ok("Wrong.1"), ok("1.Test")}, "1.Test", 2},
		{"empty value between attempts", []domain.ExecutionLogEntry{ok("1.Test"), ok(""), ok("1.Test")}, "1.Test", 2},
		{"pre-3.0 value between attempts", []domain.ExecutionLogEntry{ok("1.Test"), ok("Test.1"), ok("1.Test")}, "1.Test", 2},
	})
}

// "Stage-A" is an ordinary schedule ID: it is its own iteration, never "A".
func TestScheduleDefectAttempts_UngroupedRow_StageDashIDIsItsOwnIteration(t *testing.T) {
	row := ivUngroupedRow(2)
	ok := func(value string) domain.ExecutionLogEntry {
		return ivEntry(2, value, domain.StatusSUCCESS)
	}

	runAttemptCases(t, row, []ivAttemptCase{
		{"plain position does not count Stage-A entries", []domain.ExecutionLogEntry{ok("A"), ok("Stage-A")}, "A", 1},
		{"Stage-A position counts only Stage-A entries", []domain.ExecutionLogEntry{
			ok("A"), ok("Stage-A"), ok("Stage-A")}, "Stage-A", 2},
	})
}

func TestScheduleDefectAttempts_RecordedValueTheRowDoesNotAccept_IsExhausted(t *testing.T) {
	grouped := ivGroupedRow(ivGroupedIndex, "Test")
	outside := ivOutsideRow(0)
	exhausted := engine.ScheduleDefectRedispatchLimit + 1

	t.Run("wrong group", func(t *testing.T) {
		runAttemptCases(t, grouped, []ivAttemptCase{{"empty log", nil, "Wrong.1", exhausted}})
	})
	t.Run("empty value on a block row", func(t *testing.T) {
		runAttemptCases(t, grouped, []ivAttemptCase{{"empty log", nil, "", exhausted}})
	})
	t.Run("value on a row outside every block", func(t *testing.T) {
		runAttemptCases(t, outside, []ivAttemptCase{{"empty log", nil, "1", exhausted}})
	})
}
