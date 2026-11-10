package session_test

// Shared helpers for the HARNESS_ERROR session tests: row counting and
// assertions over the recorded steps and the consultation requests of the two
// dispatch paths (engine-routed and consultant-routed).

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// countHarnessErrorRowsFor counts the HARNESS_ERROR workflow rows recorded for
// agent.
func countHarnessErrorRowsFor(store *memStore, agent string) int {
	n := 0
	for _, s := range agentRows(store, agent) {
		if s.Status == domain.StatusHARNESS_ERROR {
			n++
		}
	}
	return n
}

// countHarnessErrorSteps counts every recorded HARNESS_ERROR step, whatever its
// agent.
func countHarnessErrorSteps(store *memStore) int {
	n := 0
	for _, s := range store.Applied {
		if s.Status == domain.StatusHARNESS_ERROR {
			n++
		}
	}
	return n
}

// requireHarnessFailureStep asserts that store.Applied holds an accepted
// workflow CompletedStep (neither infrastructure nor HITL rejected) with Status
// HARNESS_ERROR and no error code, recording the failed harness attempt.
// Returns the first such step. Fails the test if none is found.
func requireHarnessFailureStep(t *testing.T, store *memStore, context string) domain.CompletedStep {
	t.Helper()
	for _, s := range store.Applied {
		if !s.IsInfrastructure && !s.HITLRejected && s.Status == domain.StatusHARNESS_ERROR {
			if s.ErrorCode != domain.ErrorNone {
				t.Errorf("%s: want no error code on a HARNESS_ERROR row, got %q", context, s.ErrorCode)
			}
			return s
		}
	}
	t.Errorf("%s: want a workflow CompletedStep in store.Applied with Status=HARNESS_ERROR recording "+
		"the failed harness attempt, got none", context)
	return domain.CompletedStep{}
}

// requireHarnessErrorDeviation asserts the engine routed the failure as a
// non-success deviation whose deviating response is a HARNESS_ERROR.
func requireHarnessErrorDeviation(t *testing.T, req domain.ConsultationRequest) {
	t.Helper()
	if req.Deviation == nil {
		t.Fatal("want the consultation to carry a deviation, got nil")
	}
	if req.Deviation.Kind != domain.DeviationNonSuccess {
		t.Errorf("want deviation kind %q, got %q", domain.DeviationNonSuccess, req.Deviation.Kind)
	}
	if req.Deviation.Response.StatusCode != domain.StatusHARNESS_ERROR {
		t.Errorf("want deviating response HARNESS_ERROR, got %s/%s",
			req.Deviation.Response.StatusCode, req.Deviation.Response.ErrorCode)
	}
}

// requireCurrentStateUntouchedByFailure asserts the persisted workflow position
// is still the one before any harness failure: no agent, no status. Used by runs
// whose very first dispatch fails.
func requireCurrentStateUntouchedByFailure(t *testing.T, store *memStore) {
	t.Helper()
	cs := store.state.CurrentState
	if cs.LastAgent != "" || cs.LastStatus != "" {
		t.Errorf("want current_state untouched by HARNESS_ERROR rows, got last_agent=%q last_status=%q",
			cs.LastAgent, cs.LastStatus)
	}
}

// requireSummaryNamesKind asserts a row Summary starts with the failure kind
// and carries the detail fragment.
func requireSummaryNamesKind(t *testing.T, summary, kind, detail string) {
	t.Helper()
	if !strings.HasPrefix(summary, kind) {
		t.Errorf("want Summary to start with the failure kind %q, got %q", kind, summary)
	}
	if !strings.Contains(summary, detail) {
		t.Errorf("want Summary to carry the detail %q, got %q", detail, summary)
	}
}
