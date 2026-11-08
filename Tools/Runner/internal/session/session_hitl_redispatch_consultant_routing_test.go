package session_test

// Tests of the routing of a failed HITL repair re-dispatch on the consultant
// path (orchestrated mode): like the engine path, a raw-text failure is
// bypassed by one direct re-dispatch that keeps the step's HITL value, gated
// only by the anti-loop guard; when the bypass also fails, the consultation
// follows at once with a harness-error deviation and no mechanical re-dispatch.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

func TestSession_HITLRepairRedispatchBypass_ConsultantPath_BypassSucceeds_NoDeviationConsultation(t *testing.T) {
	yes := true
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatchWithHITL("agent-a", "first try", 0, &yes)
	consultant.queueStop("done")
	ses, f, store, orchPath := newHITLLinearSession(t, consultant, firstDisapprovedThenApproved())
	f.Queue("agent-a", successEntry("plan v1"), repairRawText(), successEntry("plan v2"))

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	for i, req := range consultant.Requests {
		if req.Deviation != nil {
			t.Errorf("want no deviation consultation when the bypass succeeds, request %d carries %q", i, req.Deviation.Kind)
		}
	}
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 3 {
		t.Errorf("want first attempt, failed repair and bypass, got %d agent-a dispatches", n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != 1 {
		t.Errorf("want one HARNESS_ERROR row for the failed repair, got %d", n)
	}
	requireBypassKeptHITL(t, f)
}

func TestSession_HITLRepairRedispatchBypass_ConsultantPath_BypassFails_ConsultsAtOnceWithHarnessError(t *testing.T) {
	yes := true
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatchWithHITL("agent-a", "first try", 0, &yes)
	consultant.queueStop("done")
	ses, f, store, orchPath := newHITLLinearSession(t, consultant, firstDisapprovedThenApproved())
	f.Queue("agent-a", successEntry("plan v1"), repairRawText(), repairTimeout(), successEntry("never dispatched"))

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if len(consultant.Requests) != 2 {
		t.Fatalf("want the dispatch consultation and one consultation straight after the failed bypass, got %d", len(consultant.Requests))
	}
	dev := consultant.Requests[1].Deviation
	if dev == nil || dev.Kind != domain.DeviationHarnessError {
		t.Fatalf("want a harness-error deviation after the failed bypass, got %+v", dev)
	}
	if dev.Response.StatusCode != domain.StatusHARNESS_ERROR {
		t.Errorf("want the deviating response HARNESS_ERROR, got %q", dev.Response.StatusCode)
	}
	if last := consultant.Requests[1].LastStatus; last == nil || *last != domain.StatusHARNESS_ERROR {
		t.Errorf("want the consultation to see last_status HARNESS_ERROR, got %v", last)
	}
	if msg := consultant.Requests[1].LastStatusMessage; msg == nil || *msg == "" {
		t.Errorf("want the consultation to see the failure summary as last status message, got %v", msg)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 3 {
		t.Errorf("want no mechanical re-dispatch after the failed bypass, got %d agent-a dispatches", n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != 2 {
		t.Errorf("want a HARNESS_ERROR row for the failed repair and for the failed bypass, got %d", n)
	}
	requireNoSubagentLookingBlocked(t, store, "agent-a")
}
