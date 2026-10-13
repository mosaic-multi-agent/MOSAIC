package mosaiclog

import (
	"testing"

	"mosaic-run/internal/domain"
)

// TestOutcomeForRun_PreConsultationStop_IsStopped verifies that a consultant
// stop at pre-consultation is reported as a stopped run, exactly like a routing
// stop: the pre-consultation marker does not change the mapping.
func TestOutcomeForRun_PreConsultationStop_IsStopped(t *testing.T) {
	routing := domain.RunOutcome{Status: domain.RunStoppedByConsultant, StopReason: "X"}
	preConsult := domain.RunOutcome{Status: domain.RunStoppedByConsultant, StopReason: "X", PreConsultationStop: true}

	got := OutcomeForRun(preConsult, nil, nil)

	if got != OutcomeStopped {
		t.Errorf("got %q, want %q for a pre-consultation stop", got, OutcomeStopped)
	}
	if got != OutcomeForRun(routing, nil, nil) {
		t.Errorf("pre-consultation stop maps to %q, differs from routing stop", got)
	}
}
