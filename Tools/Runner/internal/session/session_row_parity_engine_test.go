package session_test

// Row parity on the engine-routed (auto and auto-review) path: every Execution
// Log row the session writes is announced to the step-record observer exactly
// as recorded, including the rows no running notice precedes (commit setup,
// infrastructure dispatches, HITL-rejected attempts, failed bypass attempts)
// and the failed attempts whose running notice is never followed by a
// completion notice.

import (
	"errors"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

var rowLinearAgents = []string{"agent-a", "agent-b"}

// withEmptyConsultant wires a routing consultant that answers nothing: the
// engine-routed failure paths need a consultant to exist, not to answer.
func withEmptyConsultant(d *session.Deps) { d.Routing = &scriptedRoutingConsultant{} }

func TestRowParity_Engine_AcceptedWorkflowSteps_AreAnnounced(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newRowRig(t, "linear-orch.md", rowLinearAgents, nil)
			rig.f.Queue("agent-a", successEntry("a done"))
			rig.f.Queue("agent-b", successEntry("b done"))

			rig.start(configForMode(rig.orchPath, mode))

			rig.requireParity(t, 2)
		})
	}
}

func TestRowParity_Engine_CommitSetupRow_IsAnnounced(t *testing.T) {
	rig := newRowRig(t, "commit-agent-orch.md", append([]string{"commit-manager-git"}, rowLinearAgents...), nil)
	rig.f.Queue("commit-manager-git", statusEntry(domain.StatusSUCCESS, domain.ErrorNone, "ready [branch:mosaic/run/parity]"))
	rig.f.Queue("agent-a", successEntry("a done"))
	rig.f.Queue("agent-b", successEntry("b done"))
	cfg := baseCommitConfig(rig.orchPath)
	cfg.RunID = "parity"

	rig.start(cfg)

	rig.requireParity(t, 3)
	log := rig.store.state.ExecutionLog
	if len(log) == 0 || log[0].Agent != "commit-manager-git#1" {
		t.Fatalf("scenario did not run as intended: want commit setup as first row, got %+v", log)
	}
}

func TestRowParity_Engine_InfrastructureDispatchRows_AreAnnounced(t *testing.T) {
	rig := newRowRig(t, "interval-agent-orch.md", append([]string{"checkpoint-manager-git"}, rowLinearAgents...), nil)
	rig.f.Queue("agent-a", successEntry("a done"))
	rig.f.Queue("agent-b", successEntry("b done"))
	queueRepeated(rig.f, "checkpoint-manager-git", 4, successEntry("checkpointed"))

	cfg := configForMode(rig.orchPath, domain.ExecutionModeAuto)
	cfg.Checkpoints = true

	rig.start(cfg)

	rig.requireParity(t, 3)
	infra := 0
	for _, s := range rig.store.Applied {
		if s.IsInfrastructure && !s.HITLRejected {
			infra++
		}
	}
	if infra == 0 {
		t.Fatal("scenario did not run as intended: no infrastructure dispatch was recorded")
	}
}

func TestRowParity_Engine_HITLRejectedAttemptAndRedispatch_AreAnnounced(t *testing.T) {
	rig := newRowRig(t, "hitl-linear-orch.md", rowLinearAgents, func(d *session.Deps) {
		d.Approvals = firstDisapprovedThenApproved()
	})
	rig.f.Queue("agent-a", successEntry("plan v1"), successEntry("plan v2"))
	rig.f.Queue("agent-b", successEntry("done"))

	rig.start(configForMode(rig.orchPath, domain.ExecutionModeAuto))

	requireHITLRejectedSteps(t, rig.store.memStore)
	rig.requireParity(t, 3)
}

func TestRowParity_Engine_FailedHarnessAttempt_IsAnnouncedAndNextAttemptFollows(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newRowRig(t, "linear-orch.md", rowLinearAgents, withEmptyConsultant)
			rig.f.Queue("agent-a", harness.ScriptedEntry{Err: errors.New("harness: subprocess timed out")}, successEntry("a done"))
			rig.f.Queue("agent-b", successEntry("b done"))

			rig.start(configForMode(rig.orchPath, mode))

			requireHarnessErrorStep(t, rig.store.memStore, "engine path")
			rig.requireParity(t, 3)
		})
	}
}

func TestRowParity_Engine_FailedBypassAttempt_IsAnnounced(t *testing.T) {
	for _, mode := range autoModes {
		t.Run(string(mode), func(t *testing.T) {
			rig := newRowRig(t, "linear-orch.md", rowLinearAgents, withEmptyConsultant)
			raw := harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrProtocolNotExtractable)}
			rig.f.Queue("agent-a", raw, raw, successEntry("a done"))
			rig.f.Queue("agent-b", successEntry("b done"))

			rig.start(configForMode(rig.orchPath, mode))

			if n := countE501RowsFor(rig.store.memStore, "agent-a"); n < 2 {
				t.Fatalf("scenario did not run as intended: want the failed attempt and the failed bypass recorded, got %d E501 rows", n)
			}
			rig.requireParity(t, 4)
		})
	}
}

// withEmptyConsultantAndFirstHITLRejection wires the failure-path consultant
// and an approval reader that rejects the first attempt and accepts later ones.
func withEmptyConsultantAndFirstHITLRejection(d *session.Deps) {
	withEmptyConsultant(d)
	d.Approvals = firstDisapprovedThenApproved()
}

func TestRowParity_Engine_FailedHITLRedispatch_IsAnnounced(t *testing.T) {
	rig := newRowRig(t, "hitl-linear-orch.md", rowLinearAgents, withEmptyConsultantAndFirstHITLRejection)
	rig.f.Queue("agent-a", successEntry("plan v1"), harness.ScriptedEntry{Err: errors.New("harness: subprocess timed out")})
	rig.f.Queue("agent-b", successEntry("done"))

	rig.start(configForMode(rig.orchPath, domain.ExecutionModeAuto))

	rejected, failedAfter := 0, 0
	for _, s := range rig.store.Applied {
		if s.HITLRejected {
			rejected++
		} else if rejected > 0 && s.Status == domain.StatusBLOCKED && s.AgentInstance != "" {
			failedAfter++
		}
	}
	if rejected == 0 || failedAfter == 0 {
		t.Fatalf("scenario did not run as intended: want a rejected attempt followed by a failed re-dispatch row, got %+v", rig.store.Applied)
	}
	rig.requireParity(t, 2)
}

func TestRowParity_Engine_HITLEscalationRejectedRow_IsAnnounced(t *testing.T) {
	c := &scriptedRoutingConsultant{}
	c.queueDispatch("agent-b", "execute after escalation", 1)
	c.queueStop("done")
	rig := newRowRig(t, "hitl-linear-orch.md", rowLinearAgents, func(d *session.Deps) {
		d.Routing = c
		d.Approvals = &perPathApprovalReader{
			specific: map[string]domain.HumanApproval{"plan.md": domain.ApprovalFalse, "result.md": domain.ApprovalTrue},
			fallback: domain.ApprovalFalse,
		}
	})
	rig.f.Queue("agent-a", successEntry("plan v1"), successEntry("plan v2"))
	rig.f.Queue("agent-b", successEntry("done"))

	rig.start(configForMode(rig.orchPath, domain.ExecutionModeAuto))

	rejected := 0
	for _, s := range rig.store.Applied {
		if s.HITLRejected {
			rejected++
		}
	}
	if rejected < 2 {
		t.Fatalf("scenario did not run as intended: want the rejected attempt and the escalation row, got %d rejected rows", rejected)
	}
	rig.requireParity(t, 2)
}
