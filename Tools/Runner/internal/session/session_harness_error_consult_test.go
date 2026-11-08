package session_test

// Tests for the routing consultation request that follows a HARNESS_ERROR: it
// reports the status (last_status) and carries the failure description in
// last_status_message and last_error_reason. The request after any other status
// keeps today's content and gains the status of the response it describes.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
)

// requireLastStatus asserts the request reports want as last_status.
func requireLastStatus(t *testing.T, req domain.ConsultationRequest, want domain.StatusCode) {
	t.Helper()
	if req.LastStatus == nil {
		t.Fatalf("want last_status %q, got nil", want)
	}
	if *req.LastStatus != want {
		t.Errorf("want last_status %q, got %q", want, *req.LastStatus)
	}
}

// requireHarnessErrorRequest asserts the request reports HARNESS_ERROR and
// carries the failure description (starting with kind, containing detail) in
// both last_status_message and last_error_reason.
func requireHarnessErrorRequest(t *testing.T, req domain.ConsultationRequest, kind, detail string) {
	t.Helper()
	requireLastStatus(t, req, domain.StatusHARNESS_ERROR)
	if req.LastErrorReason == nil {
		t.Fatal("want last_error_reason set to the failure description, got nil")
	}
	requireSummaryNamesKind(t, *req.LastErrorReason, kind, detail)
	if req.LastStatusMessage == nil || *req.LastStatusMessage != *req.LastErrorReason {
		t.Errorf("want last_status_message equal to last_error_reason (%q), got %v", *req.LastErrorReason, req.LastStatusMessage)
	}
}

func TestSession_ConsultRequest_AfterHarnessError_AutoModes_ReportsHarnessErrorAndDescription(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("done")
			ses, f, _, orchPath := newAutoSessionWithConsultant(t, consultant)
			queueRepeated(f, "agent-a", engine.HarnessErrorAttemptLimit+1,
				harness.ScriptedEntry{Err: fmt.Errorf("subprocess stalled: %w", commonharness.ErrTimeout)})

			ses.Start(context.Background(), configForMode(orchPath, mode)) //nolint:errcheck

			if len(consultant.Requests) != 1 {
				t.Fatalf("want exactly one consultation once the budget is used up, got %d", len(consultant.Requests))
			}
			requireHarnessErrorRequest(t, consultant.Requests[0], "timeout", "subprocess stalled")
		})
	}
}

func TestSession_ConsultRequest_AfterHarnessError_ConsultantRoutedDispatch_ReportsHarnessErrorAndDescription(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "try the work", 0)
	consultant.queueStop("done")
	ses, f, _, orchPath := newOrchestratedSession(t, consultant)
	f.Queue("agent-a", harness.ScriptedEntry{Err: fmt.Errorf("launch failed: %w", commonharness.ErrExecutableNotFound)})

	ses.Start(context.Background(), baseOrchestratedConfig(orchPath)) //nolint:errcheck

	if len(consultant.Requests) < 2 {
		t.Fatalf("want a consultation after the harness error, got %d requests", len(consultant.Requests))
	}
	requireHarnessErrorRequest(t, consultant.Requests[1], "process-failure", "launch failed")
}

func TestSession_ConsultRequest_AfterHarnessError_UnparseableResponse_DescriptionNamesNoAgentOutput(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("done")
	ses, f, _, orchPath := newAutoSessionWithConsultant(t, consultant)
	queueRepeated(f, "agent-a", 2*engine.HarnessErrorAttemptLimit,
		harness.ScriptedEntry{Err: fmt.Errorf("agent said SECRET-OUTPUT: %w", commonharness.ErrMalformedJSON)})

	ses.Start(context.Background(), baseLinearConfig(orchPath)) //nolint:errcheck

	if len(consultant.Requests) == 0 {
		t.Fatal("want a consultation after the failed attempts, got none")
	}
	req := consultant.Requests[0]
	requireLastStatus(t, req, domain.StatusHARNESS_ERROR)
	if req.LastErrorReason == nil || !strings.HasPrefix(*req.LastErrorReason, "unparseable-response") {
		t.Fatalf("want last_error_reason naming the unparseable response, got %v", req.LastErrorReason)
	}
	if strings.Contains(*req.LastErrorReason, "SECRET-OUTPUT") {
		t.Errorf("want the agent's output kept out of last_error_reason, got %q", *req.LastErrorReason)
	}
}

func TestSession_ConsultRequest_AfterSubagentBlocked_ReportsBlockedAndItsReason(t *testing.T) {
	const reason = "task_description is missing"
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("done")
	ses, f, _, orchPath := newAutoSessionWithConsultant(t, consultant)
	f.Queue("agent-a", harness.ScriptedEntry{Response: blockedResponse(
		"agent-a#1", domain.ErrorINVALID_INVOCATION, "invalid invocation", reason)})

	ses.Start(context.Background(), baseLinearConfig(orchPath)) //nolint:errcheck

	if len(consultant.Requests) == 0 {
		t.Fatal("want a consultation after the BLOCKED response, got none")
	}
	req := consultant.Requests[0]
	requireLastStatus(t, req, domain.StatusBLOCKED)
	if req.LastErrorReason == nil || *req.LastErrorReason != reason {
		t.Errorf("want last_error_reason %q, got %v", reason, req.LastErrorReason)
	}
	if req.LastStatusMessage == nil || *req.LastStatusMessage != "invalid invocation" {
		t.Errorf("want last_status_message %q, got %v", "invalid invocation", req.LastStatusMessage)
	}
}

func TestSession_ConsultRequest_AfterSubagentE501_ReportsBlockedNotHarnessError(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("done")
	ses, f, _, orchPath := newAutoSessionWithConsultant(t, consultant)
	queueRepeated(f, "agent-a", engine.E501AttemptLimit+1, e501Entry())

	ses.Start(context.Background(), baseLinearConfig(orchPath)) //nolint:errcheck

	if len(consultant.Requests) == 0 {
		t.Fatal("want a consultation once the E501 budget is used up, got none")
	}
	requireLastStatus(t, consultant.Requests[0], domain.StatusBLOCKED)
}

func TestSession_ConsultRequest_AfterPartiallyDone_ReportsStatusAndNullReason(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("done")
	ses, f, _, orchPath := newAutoSessionWithConsultant(t, consultant)
	queueRepeated(f, "agent-a", 1+engine.PartiallyDoneRedispatchLimit,
		statusEntry(domain.StatusPARTIALLY_DONE, domain.ErrorNone, "half done"))

	ses.Start(context.Background(), baseLinearConfig(orchPath)) //nolint:errcheck

	if len(consultant.Requests) == 0 {
		t.Fatal("want a consultation after the re-dispatch bound, got none")
	}
	req := consultant.Requests[0]
	requireLastStatus(t, req, domain.StatusPARTIALLY_DONE)
	if req.LastErrorReason != nil {
		t.Errorf("want last_error_reason null for PARTIALLY_DONE, got %q", *req.LastErrorReason)
	}
}

func TestSession_ConsultRequest_Orchestrated_AfterSuccess_ReportsSuccessAndNullReason(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "step one", 0)
	consultant.queueStop("done")
	ses, f, _, orchPath := newOrchestratedSession(t, consultant)
	f.Queue("agent-a", successEntry("planned"))

	ses.Start(context.Background(), baseOrchestratedConfig(orchPath)) //nolint:errcheck

	if len(consultant.Requests) != 2 {
		t.Fatalf("want 2 consultations (first step, after SUCCESS), got %d", len(consultant.Requests))
	}
	requireLastStatus(t, consultant.Requests[1], domain.StatusSUCCESS)
	if r := consultant.Requests[1].LastErrorReason; r != nil {
		t.Errorf("want last_error_reason null after SUCCESS, got %q", *r)
	}
}

func TestSession_ConsultRequest_FirstStep_HasNullLastStatus(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("stop immediately")
	ses, _, _, orchPath := newOrchestratedSession(t, consultant)

	ses.Start(context.Background(), baseOrchestratedConfig(orchPath)) //nolint:errcheck

	if len(consultant.Requests) == 0 {
		t.Fatal("want a first-step consultation, got none")
	}
	if got := consultant.Requests[0].LastStatus; got != nil {
		t.Errorf("want last_status null on the first step (no last response), got %q", *got)
	}
}
