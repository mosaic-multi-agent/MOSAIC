package domain_test

// Tests for domain.RecordedPhaseName -- the pure function that owns the
// recorded phase value's spelling (canonical phase recording).
//
// Coverage:
//
//   RecordedPhaseName:
//   - Target bare form ("EXECUTION") -> "EXECUTION" (no-op).
//   - Empty string -> "".
//   - A non-EXECUTION phase is returned unchanged.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestRecordedPhaseName_TargetBareForm_IsNoOp(t *testing.T) {
	got := domain.RecordedPhaseName("EXECUTION")
	if got != "EXECUTION" {
		t.Errorf("RecordedPhaseName(%q): want %q, got %q", "EXECUTION", "EXECUTION", got)
	}
}

func TestRecordedPhaseName_Empty_ReturnsEmpty(t *testing.T) {
	got := domain.RecordedPhaseName("")
	if got != "" {
		t.Errorf("RecordedPhaseName(\"\"): want %q, got %q", "", got)
	}
}

func TestRecordedPhaseName_NonExecutionPhase_ReturnedUnchanged(t *testing.T) {
	got := domain.RecordedPhaseName("PLANNING")
	if got != "PLANNING" {
		t.Errorf("RecordedPhaseName(%q): want unchanged %q, got %q", "PLANNING", "PLANNING", got)
	}
}
