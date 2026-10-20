package session_test

// A consultation made after the HITL gate refused a step twice is announced
// and ended like any other, and no consultation is open while the gate reads
// the human's approval.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

const evApprovalRead = "approval-read"

// tracedApprovals refuses every approval and records each read in the trace.
type tracedApprovals struct{ tr *consultTrace }

func (a tracedApprovals) ReadApproval(context.Context, string) domain.HumanApproval {
	a.tr.add(evApprovalRead)
	return domain.ApprovalFalse
}

func TestConsultAnnounce_HITLGateRefusedTwice_EscalationConsultationIsBracketed(t *testing.T) {
	rig := newConsultRig(t, "hitl-linear-orch.md", linearAgents, func(d *session.Deps, tr *consultTrace) {
		d.Approvals = tracedApprovals{tr: tr}
	})
	rig.consult.queueDispatch("agent-a", "do the work", 0)
	rig.consult.queueStop("done")
	successAgentEntries(rig.f, "agent-a", 2)

	got, err := rig.ses.Start(context.Background(), baseOrchestratedConfig(rig.orchPath))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	requireNotOpenAt(t, rig.tr, evApprovalRead, 2)
	if n := len(rig.requests()); n < 2 {
		t.Fatalf("scenario did not run as intended: want the dispatch consultation and the escalation, got %d consultations", n)
	}
	requireBracketed(t, rig.tr, evCall, 2)
	requireBalanced(t, rig.tr)
	requireKindsMatch(t, rig.tr, rig.requests())
}
