package session_test

// Tests that a subagent-returned BLOCKED/E501 stays a BLOCKED row with its own
// E501 budget, and that it and the HARNESS_ERROR budget are spent independently
// of each other on the same row.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
	"mosaic-run/internal/harness"
)

func TestSession_SubagentBlockedE501_StaysBlockedAndSpendsOnlyTheE501Budget(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			consultant := &scriptedRoutingConsultant{}
			consultant.queueStop("budget used up")
			ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
			queueRepeated(f, "agent-a", engine.E501AttemptLimit+3, e501Entry())

			got, err := ses.Start(context.Background(), configForMode(orchPath, mode))

			requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
			if n := countE501RowsFor(store, "agent-a"); n != engine.E501AttemptLimit {
				t.Errorf("want %d BLOCKED/E501 rows, got %d", engine.E501AttemptLimit, n)
			}
			if n := countHarnessErrorSteps(store); n != 0 {
				t.Errorf("want no HARNESS_ERROR row for a subagent reply, got %d", n)
			}
			requireBlockedE501Deviation(t, consultant.Requests[0])
		})
	}
}

// Harness errors first, then agent E501 replies: the harness errors leave the
// whole E501 budget, and the E501 replies are bounded by it, not by what is left
// of the harness budget.
func TestSession_E501Budget_IsNotSpentByEarlierHarnessErrors(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("budget used up")
	ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
	f.Queue("agent-a", failEntry("harness: first"), failEntry("harness: second"))
	queueRepeated(f, "agent-a", engine.E501AttemptLimit+2, e501Entry())

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := 2 + engine.E501AttemptLimit
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != want {
		t.Errorf("want 2 harness errors plus the full E501 budget (%d dispatches), got %d", want, n)
	}
	if n := countE501RowsFor(store, "agent-a"); n != engine.E501AttemptLimit {
		t.Errorf("want %d BLOCKED/E501 rows, got %d", engine.E501AttemptLimit, n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != 2 {
		t.Errorf("want 2 HARNESS_ERROR rows, got %d", n)
	}
	if consultant.CallCount != 1 {
		t.Fatalf("want one consultation, got %d", consultant.CallCount)
	}
	requireBlockedE501Deviation(t, consultant.Requests[0])
}

// Agent E501 replies first, then harness errors: the replies leave the whole
// HARNESS_ERROR budget.
func TestSession_HarnessErrorBudget_IsNotSpentByEarlierE501Replies(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueStop("budget used up")
	ses, f, store, orchPath := newAutoSessionWithConsultant(t, consultant)
	f.Queue("agent-a", e501Entry(), e501Entry())
	queueRepeated(f, "agent-a", engine.HarnessErrorAttemptLimit+2, failEntry("harness: subprocess timed out"))

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	want := 2 + engine.HarnessErrorAttemptLimit
	if n := countInvocationsFor(f.Invocations(), "agent-a"); n != want {
		t.Errorf("want 2 E501 replies plus the full HARNESS_ERROR budget (%d dispatches), got %d", want, n)
	}
	if n := countHarnessErrorRowsFor(store, "agent-a"); n != engine.HarnessErrorAttemptLimit {
		t.Errorf("want %d HARNESS_ERROR rows, got %d", engine.HarnessErrorAttemptLimit, n)
	}
	if n := countE501RowsFor(store, "agent-a"); n != 2 {
		t.Errorf("want 2 BLOCKED/E501 rows, got %d", n)
	}
	if consultant.CallCount != 1 {
		t.Fatalf("want one consultation, got %d", consultant.CallCount)
	}
	requireHarnessErrorDeviation(t, consultant.Requests[0])
}

// Consultant-routed dispatch: an agent that returns BLOCKED/E501 is recorded as
// BLOCKED, not as a harness error.
func TestSession_SubagentBlockedE501_ConsultPath_IsRecordedAsBlocked(t *testing.T) {
	consultant := &scriptedRoutingConsultant{}
	consultant.queueDispatch("agent-a", "do the work", 0)
	consultant.queueStop("decided after the failure")
	ses, f, store, orchPath := newOrchestratedSession(t, consultant)
	f.Queue("agent-a", harness.ScriptedEntry{Response: blockedResponse("agent-a#1", domain.ErrorTOOL_UNAVAILABLE, "tool down", "tool down")})

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	rows := agentRows(store, "agent-a")
	if len(rows) != 1 || rows[0].Status != domain.StatusBLOCKED || rows[0].ErrorCode != domain.ErrorTOOL_UNAVAILABLE {
		t.Fatalf("want a single BLOCKED/E501 row, got %+v", rows)
	}
	if n := countHarnessErrorSteps(store); n != 0 {
		t.Errorf("want no HARNESS_ERROR row, got %d", n)
	}
}
