package session_test

// Tests that a HARNESS_ERROR attempt on a step dispatched with
// human_in_the_loop is never treated as HITL-accepted: the failed attempt does
// not discharge the gate, and the step's next dispatch is still HITL-gated and
// still subject to the HITL compliance check.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
)

// requireAgentRequestsHITLGated asserts every dispatch of agent carried
// human_in_the_loop.
func requireAgentRequestsHITLGated(t *testing.T, f *harness.MockAdapter, agent string) {
	t.Helper()
	n := 0
	for _, inv := range f.Invocations() {
		if inv.Agent.Identifier != agent {
			continue
		}
		n++
		if !inv.Request.HumanInTheLoop {
			t.Errorf("want dispatch %d of %s to carry human_in_the_loop, it did not", n, agent)
		}
	}
	if n == 0 {
		t.Fatalf("want %s dispatched at least once", agent)
	}
}

// rejectedRowsOf returns the HITL-rejected rows of agent.
func rejectedRowsOf(store *memStore, agent string) []domain.CompletedStep {
	var out []domain.CompletedStep
	for _, s := range store.Applied {
		if s.HITLRejected && strings.HasPrefix(s.AgentInstance, agent+"#") {
			out = append(out, s)
		}
	}
	return out
}

// The failed attempt is not HITL-checked and does not use up the gate: the
// next attempt's response is the one the compliance check reads, so the first
// disapproval lands on it and not on the failed attempt.
func TestSession_HarnessError_HITLGatedStep_NextAttemptIsStillSubjectToTheComplianceCheck(t *testing.T) {
	approvals := firstDisapprovedThenApproved()
	ses, f, store, orchPath := newAutoHITLSession(t, approvals, &scriptedRoutingConsultant{})
	f.Queue("agent-a", failEntry("harness: subprocess timed out"), successEntry("plan v1"), successEntry("plan v2"))
	f.Queue("agent-b", successEntry("done"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireAgentRequestsHITLGated(t, f, "agent-a")
	rows := agentRows(store, "agent-a")
	if len(rows) == 0 || rows[0].Status != domain.StatusHARNESS_ERROR {
		t.Fatalf("want the failed attempt recorded first as HARNESS_ERROR, got %+v", rows)
	}
	rejected := rejectedRowsOf(store, "agent-a")
	if len(rejected) != 1 {
		t.Fatalf("want exactly one HITL-rejected attempt (the first one that returned), got %d: %+v", len(rejected), store.Applied)
	}
	if rejected[0].AgentInstance == rows[0].AgentInstance {
		t.Errorf("want the disapproval on the attempt after the failure, not on the failed %q", rows[0].AgentInstance)
	}
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != 3 {
		t.Errorf("want the failed attempt, the rejected attempt and the repair re-dispatch (3), got %d", n)
	}
}

// With every attempt failing, the HITL gate is never reached: no approval is
// read and nothing is HITL-rejected or accepted.
func TestSession_HarnessError_HITLGatedStep_FailedAttemptsNeverReachTheApprovalCheck(t *testing.T) {
	approvals := firstDisapprovedThenApproved()
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("budget used up")
	ses, f, store, orchPath := newAutoHITLSession(t, approvals, consultant)
	queueRepeated(f, "agent-a", engine.HarnessErrorAttemptLimit+2, failEntry("harness: subprocess timed out"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	if approvals.calls != 0 {
		t.Errorf("want no approval read for attempts that returned no response, got %d reads", approvals.calls)
	}
	if rej := rejectedRowsOf(store, "agent-a"); len(rej) != 0 {
		t.Errorf("want no HITL-rejected row for a failed attempt, got %+v", rej)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != engine.HarnessErrorAttemptLimit {
		t.Errorf("want %d HARNESS_ERROR rows, got %d", engine.HarnessErrorAttemptLimit, n)
	}
	requireCurrentStateUntouchedByFailure(t, store)
	requireAgentRequestsHITLGated(t, f, "agent-a")
}

// A step that fails and then returns an approved artifact is accepted by the
// compliance check of the later attempt.
func TestSession_HarnessError_HITLGatedStep_ApprovedAttemptAfterFailureIsAccepted(t *testing.T) {
	ses, f, store, orchPath := newAutoHITLSession(t, &fixedApprovalReader{domain.ApprovalTrue}, &scriptedRoutingConsultant{})
	f.Queue("agent-a", failEntry("harness: subprocess timed out"), successEntry("plan"))
	f.Queue("agent-b", successEntry("done"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunCompleted)
	rows := agentRows(store, "agent-a")
	if len(rows) != 2 || rows[0].Status != domain.StatusHARNESS_ERROR || rows[1].Status != domain.StatusSUCCESS {
		t.Fatalf("want HARNESS_ERROR then SUCCESS for agent-a, got %+v", rows)
	}
	if cs := store.state.CurrentState; cs.LastAgent == rows[0].AgentInstance {
		t.Errorf("want the failed attempt never to become the workflow position, got %+v", cs)
	}
}

// Consultant-routed dispatch with a human_in_the_loop override behaves the
// same: the failure does not discharge the gate, and the consultant's next
// dispatch of the step is checked.
func TestSession_HarnessError_HITLGatedConsultDispatch_NextAttemptIsStillChecked(t *testing.T) {
	yes := true
	approvals := firstDisapprovedThenApproved()
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatchWithHITL("agent-a", "first try", 0, &yes)
	consultant.queueDispatchWithHITL("agent-a", "second try", 0, &yes)
	consultant.queueStop("done")
	ses, f, store, orchPath := newHITLLinearSession(t, consultant, approvals)
	f.Queue("agent-a", failEntry("harness: subprocess timed out"), successEntry("plan v1"), successEntry("plan v2"))

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireAgentRequestsHITLGated(t, f, "agent-a")
	rows := agentRows(store, "agent-a")
	if len(rows) == 0 || rows[0].Status != domain.StatusHARNESS_ERROR {
		t.Fatalf("want the failed attempt recorded first as HARNESS_ERROR, got %+v", rows)
	}
	rejected := rejectedRowsOf(store, "agent-a")
	if len(rejected) != 1 || rejected[0].AgentInstance == rows[0].AgentInstance {
		t.Errorf("want the single disapproval on the consultant's next attempt, got %+v", rejected)
	}
}
