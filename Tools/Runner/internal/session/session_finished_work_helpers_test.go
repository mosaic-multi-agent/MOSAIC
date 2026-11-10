package session_test

// Helpers for tests whose run is routed back through a row that already
// finished: the session then consults the orchestrator instead of repeating
// the finished step.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// requireFinishedWorkConsultation asserts that req is a routing consultation
// caused by a SUCCESS whose next step (0-based table row wantRow) had already
// finished.
func requireFinishedWorkConsultation(t *testing.T, req domain.ConsultationRequest, wantRow int) {
	t.Helper()
	if req.Context != domain.ConsultContextRouting {
		t.Errorf("consultation context: want %q, got %q", domain.ConsultContextRouting, req.Context)
	}
	dev := req.Deviation
	if dev == nil || dev.Kind != domain.DeviationFinishedWork {
		t.Fatalf("want a finished-work deviation, got %+v", dev)
	}
	if dev.PlannedContinuation == nil || dev.PlannedContinuation.RowIndex != wantRow {
		t.Errorf("finished step: want row index %d, got %+v", wantRow, dev.PlannedContinuation)
	}
}
