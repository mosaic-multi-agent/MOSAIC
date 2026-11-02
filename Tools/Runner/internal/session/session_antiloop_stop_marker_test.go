package session_test

// A guard-caused ending is marked on the run outcome, so a frontend recognises
// it without matching message text. Endings that look alike (a graceful stop,
// an unresolved engine deviation, the consultant's own stop) are not marked.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

func TestAntiLoopStop_GuardEscalationExhausted_IsMarked(t *testing.T) {
	consultant := &sameDispatchConsultant{agent: "agent-b", rowIndex: 1, safetyCap: consultantSafetyCap}
	ses, f, _, orchPath := newOrchestratedSession(t, consultant)
	queueRepeated(f, "agent-b", 12, failingAgentEntry(domain.ErrorDEPENDENCY_MISSING))

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireGuardEscalationEnd(t, got, err, consultant, 2)
	if got.Status != domain.RunStopped {
		t.Fatalf("precondition: want the guard escalation limit to end the run as %q, got %q", domain.RunStopped, got.Status)
	}
	if !got.AntiLoopStop {
		t.Error("AntiLoopStop = false for a run ended by the guard escalation limit")
	}
}

func TestAntiLoopStop_GuardEscalationExhaustedFromHITLRedispatch_IsMarked(t *testing.T) {
	consultant := &sameDispatchConsultant{agent: "agent-a", rowIndex: 0, safetyCap: consultantSafetyCap}
	ses, f, _, orchPath := newHITLLinearSession(t, consultant, &fixedApprovalReader{domain.ApprovalFalse})
	queueRepeated(f, "agent-a", guardBound-1, failingAgentEntry(domain.ErrorDEPENDENCY_MISSING))
	queueRepeated(f, "agent-a", 12, statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "done"))

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireGuardEscalationEnd(t, got, err, consultant, 1)
	if !got.AntiLoopStop {
		t.Error("AntiLoopStop = false for a run ended by the guard, entered from a HITL re-dispatch")
	}
}

func TestAntiLoopStop_OtherEndings_AreNotMarked(t *testing.T) {
	t.Run("the consultant's own stop", func(t *testing.T) {
		consultant := &scriptedRoutingConsultant{}
		consultant.queueDispatch("agent-a", "attempt", 0)
		consultant.queueStop("consultant finished the run")
		ses, f, _, orchPath := newOrchestratedSession(t, consultant)
		f.Queue("agent-a", partiallyDoneEntries(2)...)

		got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

		requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
		if got.AntiLoopStop {
			t.Error("AntiLoopStop = true for the consultant's stop")
		}
	})
	t.Run("graceful stop", func(t *testing.T) {
		dir := t.TempDir()
		orchPath := copyOrchestratorFile(t, dir, "linear-orch.md")
		writeAgentFile(t, dir, "agent-a")
		writeAgentFile(t, dir, "agent-b")
		ses := session.New(session.Deps{
			Harness: harness.NewMockAdapter(), Store: &memStore{},
			Clock: fixedClock{t: epoch}, Interact: &noopInteraction{},
			StopRequested: func() bool { return true },
		})

		got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

		requireRunStatus(t, got, err, domain.RunStopped)
		if got.AntiLoopStop {
			t.Error("AntiLoopStop = true for a graceful stop")
		}
	})
	t.Run("engine deviation with no routing consultant", func(t *testing.T) {
		ses, f, _, orchPath := newLinearSession(t)
		queueRepeated(f, "agent-a", 6, failingAgentEntry(domain.ErrorDEPENDENCY_MISSING))

		got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))

		requireRunStatus(t, got, err, domain.RunDeviationUnresolved)
		if got.AntiLoopStop {
			t.Error("AntiLoopStop = true for an engine deviation that no guard caused")
		}
	})
}
