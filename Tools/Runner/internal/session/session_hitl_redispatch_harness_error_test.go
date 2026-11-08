package session_test

// Tests of a harness error during the HITL repair re-dispatch, on the engine
// path (mechanical modes) and the consultant path (orchestrated mode): the
// failure is recorded and consulted as HARNESS_ERROR with a failure-kind
// Summary, never as a BLOCKED that looks subagent-returned, and the routing
// stays what it was: no mechanical re-dispatch, an immediate consultation.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// repairTimeout is the harness error the repair re-dispatch fails with.
func repairTimeout() harness.ScriptedEntry {
	return harness.ScriptedEntry{Err: fmt.Errorf("repair stalled: %w", commonharness.ErrTimeout)}
}

// requireNoSubagentLookingBlocked asserts no BLOCKED row was recorded for agent.
func requireNoSubagentLookingBlocked(t *testing.T, store *memStore, agent string) {
	t.Helper()
	for _, s := range store.Applied {
		if s.Status == domain.StatusBLOCKED && strings.HasPrefix(s.AgentInstance, agent+"#") {
			t.Errorf("want no BLOCKED row for a failure the subagent never returned, got %+v", s)
		}
	}
}

func TestSession_HITLRepairRedispatchHarnessError_EnginePath_RowIsHarnessErrorNamingTheKind(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("done")
	ses, f, store, orchPath := newAutoHITLSession(t, firstDisapprovedThenApproved(), consultant)
	f.Queue("agent-a", successEntry("plan v1"), repairTimeout())

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	row := requireHarnessFailureStep(t, store, "engine-path repair re-dispatch")
	requireSummaryNamesKind(t, row.Summary, "timeout", "repair stalled")
	if row.AgentInstance == "" || row.WorkflowRow == domain.NoWorkflowRow {
		t.Errorf("want the row recorded at the dispatched workflow row, got %+v", row)
	}
	requireNoSubagentLookingBlocked(t, store, "agent-a")
	requireCurrentStateUntouchedByFailure(t, store)
}

func TestSession_HITLRepairRedispatchHarnessError_EnginePath_ConsultsAtOnceWithHarnessErrorAndNoMechanicalRedispatch(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("done")
	ses, f, _, orchPath := newAutoHITLSession(t, firstDisapprovedThenApproved(), consultant)
	f.Queue("agent-a", successEntry("plan v1"), repairTimeout(), successEntry("never dispatched"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(consultant.Requests) != 1 {
		t.Fatalf("want one consultation straight after the repair failure, got %d", len(consultant.Requests))
	}
	req := consultant.Requests[0]
	requireHarnessErrorDeviation(t, req)
	requireHarnessErrorRequest(t, req, "timeout", "repair stalled")
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 2 {
		t.Errorf("want the first attempt and the failed repair only (no mechanical re-dispatch), got %d", n)
	}
}

func TestSession_HITLRepairRedispatchHarnessError_EnginePath_NoRoutingConsultant_EndsDeviationUnresolved(t *testing.T) {
	ses, f, store, orchPath := newAutoHITLSession(t, firstDisapprovedThenApproved(), nil)
	f.Queue("agent-a", successEntry("plan v1"), repairTimeout())

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunDeviationUnresolved)
	requireHarnessFailureStep(t, store, "engine-path repair re-dispatch without a consultant")
	requireNoSubagentLookingBlocked(t, store, "agent-a")
}

func TestSession_HITLRepairRedispatchHarnessError_ConsultantPath_DeviationCarriesHarnessErrorNamingTheKind(t *testing.T) {
	yes := true
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatchWithHITL("agent-a", "first try", 0, &yes)
	consultant.queueStop("done")
	ses, f, store, orchPath := newHITLLinearSession(t, consultant, firstDisapprovedThenApproved())
	f.Queue("agent-a", successEntry("plan v1"), repairTimeout())

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(consultant.Requests) != 2 {
		t.Fatalf("want the dispatch consultation and one consultation after the repair failure, got %d", len(consultant.Requests))
	}
	dev := consultant.Requests[1].Deviation
	if dev == nil {
		t.Fatal("want the consultation after the repair failure to carry a deviation, got nil")
	}
	if dev.Kind != domain.DeviationHarnessError {
		t.Errorf("want deviation kind %q, got %q", domain.DeviationHarnessError, dev.Kind)
	}
	if dev.Response.StatusCode != domain.StatusHARNESS_ERROR {
		t.Errorf("want the deviating response HARNESS_ERROR, got %q", dev.Response.StatusCode)
	}
	requireSummaryNamesKind(t, dev.Response.StatusMessage, "timeout", "repair stalled")
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 2 {
		t.Errorf("want the first attempt and the failed repair only, got %d", n)
	}
	row := requireHarnessFailureStep(t, store, "consultant-path repair re-dispatch")
	requireSummaryNamesKind(t, row.Summary, "timeout", "repair stalled")
	requireNoSubagentLookingBlocked(t, store, "agent-a")
	if last := store.state.CurrentState.LastStatus; last == domain.StatusHARNESS_ERROR {
		t.Errorf("want the workflow position untouched by the failure row, got last_status=%q", last)
	}
}
