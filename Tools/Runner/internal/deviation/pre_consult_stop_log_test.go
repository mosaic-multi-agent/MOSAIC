package deviation_test

// Tests that a pre-consultation stop reply stays visible in the dispatch log
// like any other pre-consultation reply, and that a rejected reply is not retried.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
)

func preConsultRequest() domain.ConsultationRequest {
	return domain.ConsultationRequest{
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextPreConsultation,
	}
}

// TestPreConsult_StopReply_LoggedAsRequestAndResponse verifies that the request
// and the stop reply are both recorded in the dispatch log, in order, and no
// error entry is written for the stop.
func TestPreConsult_StopReply_LoggedAsRequestAndResponse(t *testing.T) {
	invoker, logger := newOrderingPair([]byte(`{"action":"stop","reason":"branch is dirty"}`), nil)
	c := newTestConsultantWithLogger(invoker, mustParseTable(t), logger)

	advice, err := c.PreConsult(context.Background(), preConsultRequest())

	if err != nil {
		t.Fatalf("want no error for a stop reply, got %v", err)
	}
	if advice.Stop == nil {
		t.Fatal("want the stop returned, got nil Stop")
	}
	events := *invoker.events
	want := []string{"log-request", "invoke-raw", "log-response"}
	if len(events) != len(want) {
		t.Fatalf("want events %v, got %v", want, events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("events[%d] = %q, want %q (all events: %v)", i, events[i], want[i], events)
		}
	}
	if len(logger.errs) != 0 {
		t.Errorf("want no LogError entry for a stop reply, got %d", len(logger.errs))
	}
}

// TestPreConsult_RejectedAction_InvokedOnceAndNotRetried verifies that a
// malformed pre-consultation reply is reported after a single invocation.
func TestPreConsult_RejectedAction_InvokedOnceAndNotRetried(t *testing.T) {
	invoker, logger := newOrderingPair([]byte(`{"action":"dispatch","agent":"agent-a"}`), nil)
	c := newTestConsultantWithLogger(invoker, mustParseTable(t), logger)

	_, err := c.PreConsult(context.Background(), preConsultRequest())

	assertConsultationError(t, err, domain.ConsultFailUnknownAction)
	if n := len(logger.requests); n != 1 {
		t.Errorf("want 1 logged request (no retry), got %d", n)
	}
}
