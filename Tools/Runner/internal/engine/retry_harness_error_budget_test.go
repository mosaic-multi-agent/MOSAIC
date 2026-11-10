package engine_test

// HARNESS_ERROR budget: at most HarnessErrorAttemptLimit HARNESS_ERROR attempts
// per workflow row and iteration value since the last SUCCESS there, derived
// from the Execution Log alone. The E501 budget keeps counting BLOCKED/E501
// rows only: HARNESS_ERROR rows never spend it, while subagent-returned and old
// synthetic BLOCKED/E501 rows still do.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

const (
	heRowIndex = 3 // zero-based, row outside every block
	heLimit    = engine.HarnessErrorAttemptLimit
)

// heEntry is a HARNESS_ERROR entry of the row at the zero-based index.
func heEntry(index int, value string) domain.ExecutionLogEntry {
	return ivEntry(index, value, domain.StatusHARNESS_ERROR)
}

func heForceStopped(index int, value string) domain.ExecutionLogEntry {
	e := ivEntry(index, value, domain.StatusPARTIALLY_DONE)
	e.Summary = domain.ForceStoppedSummary
	return e
}

func runHarnessBudgetCases(t *testing.T, row domain.RoutingRow, cases []ivBudgetCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := engine.HarnessErrorBudgetRemaining(tc.log, row, tc.recorded)

			if got != tc.want {
				t.Errorf("HarnessErrorBudgetRemaining: want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestHarnessErrorAttemptLimit_IsThree(t *testing.T) {
	if engine.HarnessErrorAttemptLimit != 3 {
		t.Errorf("want 3 attempts, got %d", engine.HarnessErrorAttemptLimit)
	}
}

func TestHarnessErrorBudgetRemaining_CountsRowsSinceLastSuccessAtRow(t *testing.T) {
	row := ivOutsideRow(heRowIndex)
	other := heRowIndex + 1

	runHarnessBudgetCases(t, row, []ivBudgetCase{
		{"empty log", nil, "", heLimit},
		{"one failure", []domain.ExecutionLogEntry{heEntry(heRowIndex, "")}, "", heLimit - 1},
		{"limit reached", []domain.ExecutionLogEntry{
			heEntry(heRowIndex, ""), heEntry(heRowIndex, ""), heEntry(heRowIndex, ""),
		}, "", 0},
		{"never negative", []domain.ExecutionLogEntry{
			heEntry(heRowIndex, ""), heEntry(heRowIndex, ""), heEntry(heRowIndex, ""),
			heEntry(heRowIndex, ""), heEntry(heRowIndex, ""),
		}, "", 0},
		{"success resets", []domain.ExecutionLogEntry{
			heEntry(heRowIndex, ""), heEntry(heRowIndex, ""),
			ivEntry(heRowIndex, "", domain.StatusSUCCESS), heEntry(heRowIndex, ""),
		}, "", heLimit - 1},
		{"failures after a success use a fresh budget", []domain.ExecutionLogEntry{
			heEntry(heRowIndex, ""), ivEntry(heRowIndex, "", domain.StatusSUCCESS),
			heEntry(heRowIndex, ""), heEntry(heRowIndex, ""), heEntry(heRowIndex, ""),
		}, "", 0},
		{"success at another row does not reset", []domain.ExecutionLogEntry{
			heEntry(heRowIndex, ""), ivEntry(other, "", domain.StatusSUCCESS), heEntry(heRowIndex, ""),
		}, "", heLimit - 2},
		{"failures of another row are ignored", []domain.ExecutionLogEntry{
			heEntry(other, ""), heEntry(heRowIndex, ""),
		}, "", heLimit - 1},
		{"rows without a workflow row are ignored", []domain.ExecutionLogEntry{
			{Agent: "checkpoint-manager-git#1", Status: domain.StatusHARNESS_ERROR, WorkflowRow: domain.NoWorkflowRow},
			heEntry(heRowIndex, ""),
		}, "", heLimit - 1},
	})
}

func TestHarnessErrorBudgetRemaining_ForceStoppedRow_DoesNotChangeTheBudget(t *testing.T) {
	row := ivOutsideRow(heRowIndex)
	without := []domain.ExecutionLogEntry{heEntry(heRowIndex, ""), heEntry(heRowIndex, "")}
	want := engine.HarnessErrorBudgetRemaining(without, row, "")
	if want != heLimit-2 {
		t.Fatalf("fixture: want budget %d for two failures, got %d", heLimit-2, want)
	}

	runHarnessBudgetCases(t, row, []ivBudgetCase{
		{"between the failures", []domain.ExecutionLogEntry{
			heEntry(heRowIndex, ""), heForceStopped(heRowIndex, ""), heEntry(heRowIndex, ""),
		}, "", want},
		{"after the failures", []domain.ExecutionLogEntry{
			heEntry(heRowIndex, ""), heEntry(heRowIndex, ""), heForceStopped(heRowIndex, ""),
		}, "", want},
	})
}

func TestHarnessErrorBudgetRemaining_OnlyHarnessErrorRowsAreCounted(t *testing.T) {
	row := ivOutsideRow(heRowIndex)
	blocked := func(code domain.ErrorCode) domain.ExecutionLogEntry {
		e := ivEntry(heRowIndex, "", domain.StatusBLOCKED)
		e.ErrorCode = code
		return e
	}

	runHarnessBudgetCases(t, row, []ivBudgetCase{
		{"BLOCKED/E501 is not counted", []domain.ExecutionLogEntry{
			blocked(domain.ErrorTOOL_UNAVAILABLE), heEntry(heRowIndex, ""),
		}, "", heLimit - 1},
		{"other statuses are not counted", []domain.ExecutionLogEntry{
			blocked(domain.ErrorPERMISSION_DENIED), ivEntry(heRowIndex, "", domain.StatusPARTIALLY_DONE),
			ivEntry(heRowIndex, "", domain.StatusCOMPLETED_NEEDS_ACTION), heEntry(heRowIndex, ""),
		}, "", heLimit - 1},
	})
}

func TestHarnessErrorBudgetRemaining_IterationValues(t *testing.T) {
	row := ivGroupedRow(ivGroupedIndex, "Test")

	runHarnessBudgetCases(t, row, []ivBudgetCase{
		{"two entries of one iteration", []domain.ExecutionLogEntry{
			heEntry(ivGroupedIndex, "1.Test"), heEntry(ivGroupedIndex, "1.Test"),
		}, "1.Test", heLimit - 2},
		{"another iteration is another budget", []domain.ExecutionLogEntry{
			heEntry(ivGroupedIndex, "1.Test"), heEntry(ivGroupedIndex, "2.Test"),
		}, "1.Test", heLimit - 1},
		{"spellings of one iteration share one budget", []domain.ExecutionLogEntry{
			heEntry(ivGroupedIndex, "a.Test"), heEntry(ivGroupedIndex, "A.Test"),
		}, "A.Test", heLimit - 2},
		{"success at another iteration does not reset", []domain.ExecutionLogEntry{
			heEntry(ivGroupedIndex, "1.Test"), ivEntry(ivGroupedIndex, "2.Test", domain.StatusSUCCESS),
			heEntry(ivGroupedIndex, "1.Test"),
		}, "1.Test", heLimit - 2},
		{"success at the same iteration resets", []domain.ExecutionLogEntry{
			heEntry(ivGroupedIndex, "1.Test"), heEntry(ivGroupedIndex, "1.Test"),
			ivEntry(ivGroupedIndex, "1.Test", domain.StatusSUCCESS), heEntry(ivGroupedIndex, "1.Test"),
		}, "1.Test", heLimit - 1},
		{"entry whose value the row does not accept is ignored", []domain.ExecutionLogEntry{
			heEntry(ivGroupedIndex, "Wrong.1"), heEntry(ivGroupedIndex, "1.Test"),
		}, "1.Test", heLimit - 1},
	})
}

func TestHarnessErrorBudgetRemaining_RecordedValueTheRowDoesNotAccept_IsExhausted(t *testing.T) {
	t.Run("wrong group", func(t *testing.T) {
		runHarnessBudgetCases(t, ivGroupedRow(ivGroupedIndex, "Test"),
			[]ivBudgetCase{{"empty log", nil, "Wrong.1", 0}})
	})
	t.Run("empty value on a block row", func(t *testing.T) {
		runHarnessBudgetCases(t, ivGroupedRow(ivGroupedIndex, "Test"),
			[]ivBudgetCase{{"empty log", nil, "", 0}})
	})
	t.Run("value on a row outside every block", func(t *testing.T) {
		runHarnessBudgetCases(t, ivOutsideRow(heRowIndex),
			[]ivBudgetCase{{"empty log", nil, "1.Test", 0}})
	})
}

// HARNESS_ERROR rows do not spend the E501 budget; subagent-returned and old
// synthetic BLOCKED/E501 rows (indistinguishable once recorded) still do.
func TestE501BudgetRemaining_HarnessErrorRows_DoNotSpendIt(t *testing.T) {
	row := ivOutsideRow(heRowIndex)
	log := []domain.ExecutionLogEntry{
		heEntry(heRowIndex, ""), heEntry(heRowIndex, ""), heEntry(heRowIndex, ""),
		ivBlockedE501(heRowIndex, ""),
	}

	if got := engine.E501BudgetRemaining(log, row, ""); got != engine.E501AttemptLimit-1 {
		t.Errorf("E501BudgetRemaining: want %d (only the BLOCKED/E501 row counts), got %d",
			engine.E501AttemptLimit-1, got)
	}
}

func TestE501BudgetRemaining_OldSyntheticHarnessRows_StillSpendIt(t *testing.T) {
	row := ivOutsideRow(heRowIndex)
	synthetic := ivBlockedE501(heRowIndex, "")
	synthetic.Summary = "harness: subprocess timed out after 60s"
	log := []domain.ExecutionLogEntry{synthetic, synthetic, ivBlockedE501(heRowIndex, "")}

	if got := engine.E501BudgetRemaining(log, row, ""); got != 0 {
		t.Errorf("E501BudgetRemaining: want 0 after three BLOCKED/E501 rows, got %d", got)
	}
	if got := engine.HarnessErrorBudgetRemaining(log, row, ""); got != heLimit {
		t.Errorf("HarnessErrorBudgetRemaining: old synthetic rows must not spend it, want %d, got %d", heLimit, got)
	}
}
