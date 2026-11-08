package session_test

// Tests that HARNESS_ERROR dispatches count toward the anti-loop guard exactly
// as the synthetic BLOCKED/E501 dispatches did: the engine's mechanical
// re-dispatches are not counted, the ones a consultant requests are, and the
// guard's escalation hands the consultant the failure context.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

const guardHarnessFailure = "harness: subprocess timed out"

// recordingSameDispatchConsultant is a sameDispatchConsultant that keeps the
// requests it was consulted with.
type recordingSameDispatchConsultant struct {
	sameDispatchConsultant
	requests []domain.ConsultationRequest
}

func (c *recordingSameDispatchConsultant) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.requests = append(c.requests, req)
	return c.sameDispatchConsultant.ConsultRouting(ctx, req)
}

// guardEscalationRequests returns the requests consulted because the guard
// blocked a dispatch: their deviation response is the guard's own.
func guardEscalationRequests(c *recordingSameDispatchConsultant) []domain.ConsultationRequest {
	var out []domain.ConsultationRequest
	for _, req := range c.requests {
		if req.Deviation != nil && strings.Contains(req.Deviation.Response.AgentInstanceID, "#guard") {
			out = append(out, req)
		}
	}
	return out
}

// requireGuardEscalationCarriesHarnessFailure asserts every guard escalation
// request reports HARNESS_ERROR with the failure description, and that at
// least one escalation happened.
func requireGuardEscalationCarriesHarnessFailure(t *testing.T, c *recordingSameDispatchConsultant) {
	t.Helper()
	escalations := guardEscalationRequests(c)
	if len(escalations) == 0 {
		t.Fatalf("want at least one guard escalation consultation, got none among %d requests", len(c.requests))
	}
	for i, req := range escalations {
		if req.LastStatus == nil || *req.LastStatus != domain.StatusHARNESS_ERROR {
			t.Errorf("escalation %d: want last_status HARNESS_ERROR, got %v", i, req.LastStatus)
		}
		if req.LastErrorReason == nil || !strings.Contains(*req.LastErrorReason, guardHarnessFailure) {
			t.Errorf("escalation %d: want last_error_reason carrying %q, got %v", i, guardHarnessFailure, req.LastErrorReason)
		}
		if req.LastStatusMessage == nil || !strings.Contains(*req.LastStatusMessage, guardHarnessFailure) {
			t.Errorf("escalation %d: want last_status_message carrying %q, got %v", i, guardHarnessFailure, req.LastStatusMessage)
		}
	}
}

// Every dispatch the consultant requests counts: the fifth consecutive
// same-agent dispatch is blocked although each earlier one ended in a harness
// error, and the escalation names the failure.
func TestSession_AntiLoopGuard_Orchestrated_HarnessErrorDispatchesCounted_EscalationCarriesFailure(t *testing.T) {
	consultant := &recordingSameDispatchConsultant{
		sameDispatchConsultant: sameDispatchConsultant{agent: "agent-a", rowIndex: 0, safetyCap: consultantSafetyCap},
	}
	ses, f, store, orchPath := newOrchestratedSession(t, consultant)
	queueRepeated(f, "agent-a", 12, failEntry(guardHarnessFailure))

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireGuardEscalationEnd(t, got, err, &consultant.sameDispatchConsultant, 1)
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != guardBound {
		t.Errorf("want exactly %d agent-a dispatches before the guard blocks, got %d", guardBound, n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != guardBound {
		t.Errorf("want one HARNESS_ERROR row per dispatch (blocked attempts write none), got %d for %d dispatches", n, guardBound)
	}
	requireGuardEscalationCarriesHarnessFailure(t, consultant)
}

// In the mechanical modes the engine's HARNESS_ERROR re-dispatches are not
// counted: only the first engine dispatch and the consultant-requested ones are.
func TestSession_AntiLoopGuard_Auto_MechanicalHarnessErrorRedispatchesNotCounted_EscalationCarriesFailure(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &recordingSameDispatchConsultant{
				sameDispatchConsultant: sameDispatchConsultant{agent: "agent-a", rowIndex: 0, safetyCap: consultantSafetyCap},
			}
			ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
			queueRepeated(f, "agent-a", 20, failEntry(guardHarnessFailure))

			got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

			requireGuardEscalationEnd(t, got, err, &consultant.sameDispatchConsultant, 1)
			// The engine's dispatches (the first and its mechanical re-dispatches up
			// to the budget) plus the guardBound-1 dispatches the guard allows the
			// consultant after the first dispatch was counted. Counting the
			// mechanical re-dispatches would leave only guardBound-HarnessErrorAttemptLimit
			// consultant dispatches.
			want := engine.HarnessErrorAttemptLimit + guardBound - 1
			if n := countInvocationsFor(f.Invocations(), "agent-a"); n != want {
				t.Errorf("want %d agent-a dispatches (mechanical re-dispatches not counted), got %d", want, n)
			}
			if n := countHarnessErrorRowsFor(store, "agent-a"); n != want {
				t.Errorf("want one HARNESS_ERROR row per dispatch, got %d for %d dispatches", n, want)
			}
			requireGuardEscalationCarriesHarnessFailure(t, consultant)
		})
	}
}
