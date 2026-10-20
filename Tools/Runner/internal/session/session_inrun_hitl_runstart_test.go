package session_test

// Tests that the run-start dispatches (commit setup and pre-consultation)
// neither receive nor consume a pending in-run HITL request: the first
// workflow step after them is the one that is raised.

import (
	"context"
	"slices"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

// pendingProbePreConsultant records whether the HITL request was still pending
// when pre-consultation ran.
type pendingProbePreConsultant struct {
	hitl          *requestFlag
	calls         int
	pendingAtCall bool
}

func (p *pendingProbePreConsultant) PreConsult(_ context.Context, _ domain.ConsultationRequest) (domain.PreConsultationAdvice, error) {
	p.calls++
	p.pendingAtCall = p.hitl.isPending()
	return domain.PreConsultationAdvice{}, nil
}

func TestSession_InRunHITL_CommitSetup_NeitherReceivesNorConsumesTheRequest(t *testing.T) {
	rig := newInRunRig(t, newManualUser(t), inRunOptions{
		fixture: "commit-agent-orch.md", agents: []string{"agent-a", "agent-b", "commit-manager-git"},
		approvals: &approvalLog{},
	})
	queueSuccessfulSetupAndWorkflow(rig.f, "mosaic/run/test-run-id")
	rig.hitl.request()
	cfg := baseCommitConfig(rig.orchPath)
	cfg.RunID = "test-run-id"

	got, err := rig.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	want := []string{"commit-manager-git=false", "agent-a=true", "agent-b=false"}
	if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
		t.Errorf("want the commit setup unraised and the first workflow step raised: %v, got %v", want, seq)
	}
	if rig.hitl.isPending() {
		t.Error("want the request consumed by the first workflow step")
	}
	if n := rig.appliedCount(); n != 1 {
		t.Errorf("want exactly one application logged, got %d", n)
	}
}

func TestSession_InRunHITL_PreConsultation_NeitherReceivesNorConsumesTheRequest(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newInRunRig(t, newManualUser(t), inRunOptions{
				fixture: "linear-orch.md", agents: linearAgents, approvals: &approvalLog{},
			})
			probe := &pendingProbePreConsultant{hitl: rig.hitl}
			rig.ses = session.New(session.Deps{
				Harness: rig.f, Store: rig.store, Clock: fixedClock{t: epoch},
				Interact: &noopInteraction{}, Debug: rig.logger, Approvals: &approvalLog{},
				PreConsult: probe, TakeHITLRequest: rig.hitl.take,
			})
			rig.queueOK("agent-a", 1)
			rig.queueOK("agent-b", 1)
			rig.hitl.request()
			cfg := configForMode(rig.orchPath, mode)
			cfg.PreConsultation = true

			got, err := rig.ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunCompleted)
			if probe.calls != 1 {
				t.Fatalf("want pre-consultation run once, got %d", probe.calls)
			}
			if !probe.pendingAtCall {
				t.Error("want the request still pending while pre-consultation ran")
			}
			want := []string{"agent-a=true", "agent-b=false"}
			if seq := hitlSequence(rig.f); !slices.Equal(seq, want) {
				t.Errorf("want the first workflow step raised: %v, got %v", want, seq)
			}
			if n := rig.appliedCount(); n != 1 {
				t.Errorf("want exactly one application logged, got %d", n)
			}
		})
	}
}

