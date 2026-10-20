package session_test

// Row parity on the consultant-routed (orchestrated) path: the same announcement
// obligations as on the engine-routed path, for the rows the consultant-routed
// dispatch, HITL loop and failure handling record.

import (
	"path/filepath"
	"errors"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

func withConsultant(c *scriptedRoutingConsultant) func(*session.Deps) {
	return func(d *session.Deps) { d.Routing = c }
}

func TestRowParity_Consulted_AcceptedWorkflowSteps_AreAnnounced(t *testing.T) {
	c := &scriptedRoutingConsultant{}
	c.queueDispatch("agent-a", "first", 0)
	c.queueDispatch("agent-b", "second", 1)
	c.queueStop("done")
	rig := newRowRig(t, "linear-orch.md", rowLinearAgents, withConsultant(c))
	rig.f.Queue("agent-a", successEntry("a done"))
	rig.f.Queue("agent-b", successEntry("b done"))

	rig.start(baseOrchestratedConfig(rig.orchPath))

	rig.requireParity(t, 2)
}

func TestRowParity_Consulted_HITLRejectedAttemptAndRedispatch_AreAnnounced(t *testing.T) {
	c := &scriptedRoutingConsultant{}
	c.queueDispatch("agent-a", "draft the plan", 0)
	c.queueDispatch("agent-b", "execute the plan", 1)
	c.queueStop("done")
	rig := newRowRig(t, "hitl-linear-orch.md", rowLinearAgents, func(d *session.Deps) {
		d.Routing = c
		d.Approvals = firstDisapprovedThenApproved()
	})
	rig.f.Queue("agent-a", successEntry("plan v1"), successEntry("plan v2"))
	rig.f.Queue("agent-b", successEntry("done"))

	rig.start(baseOrchestratedConfig(rig.orchPath))

	requireHITLRejectedSteps(t, rig.store.memStore)
	rig.requireParity(t, 3)
}

func TestRowParity_Consulted_HITLEscalationRows_AreAnnounced(t *testing.T) {
	c := &scriptedRoutingConsultant{}
	c.queueDispatch("agent-a", "draft the plan", 0)
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

	rig.start(baseOrchestratedConfig(rig.orchPath))

	rejected := 0
	for _, s := range rig.store.Applied {
		if s.HITLRejected {
			rejected++
		}
	}
	if rejected < 2 {
		t.Fatalf("scenario did not run as intended: want the rejected attempt and rejected re-dispatch, got %d rejected rows", rejected)
	}
	rig.requireParity(t, 3)
}

func TestRowParity_Consulted_FailedHarnessAttempt_IsAnnouncedAndNextAttemptFollows(t *testing.T) {
	c := &scriptedRoutingConsultant{}
	c.queueDispatch("agent-a", "first", 0)
	c.queueDispatch("agent-a", "again", 0)
	c.queueDispatch("agent-b", "second", 1)
	c.queueStop("done")
	rig := newRowRig(t, "linear-orch.md", rowLinearAgents, withConsultant(c))
	rig.f.Queue("agent-a", harness.ScriptedEntry{Err: errors.New("harness: subprocess timed out")}, successEntry("a done"))
	rig.f.Queue("agent-b", successEntry("b done"))

	rig.start(baseOrchestratedConfig(rig.orchPath))

	requireHarnessErrorStep(t, rig.store.memStore, "consultant path")
	rig.requireParity(t, 3)
}

func TestRowParity_Consulted_FailedBypassAttempt_IsAnnounced(t *testing.T) {
	c := &scriptedRoutingConsultant{}
	c.queueDispatch("agent-a", "first", 0)
	c.queueDispatch("agent-a", "again", 0)
	c.queueDispatch("agent-b", "second", 1)
	c.queueStop("done")
	rig := newRowRig(t, "linear-orch.md", rowLinearAgents, withConsultant(c))
	raw := harness.ScriptedEntry{Err: wrapSentinel(commonharness.ErrProtocolNotExtractable)}
	rig.f.Queue("agent-a", raw, raw, successEntry("a done"))
	rig.f.Queue("agent-b", successEntry("b done"))

	rig.start(baseOrchestratedConfig(rig.orchPath))

	if n := countE501RowsFor(rig.store.memStore, "agent-a"); n < 2 {
		t.Fatalf("scenario did not run as intended: want the failed attempt and the failed bypass recorded, got %d E501 rows", n)
	}
	rig.requireParity(t, 4)
}

// Dispatched rows of a staged workflow carry a non-empty recorded Stage; the
// announced entries keep it unchanged, so a live row can be compared with the
// rebuilt one on real stage values.
func TestRowParity_Consulted_StagedRows_AnnounceTheRecordedStage(t *testing.T) {
	c := &scriptedRoutingConsultant{}
	c.queueStagedDispatch("implementer", "implement stage 2", rsImplementer, "2")
	c.queueStop("done")
	rig := newRowRig(t, "consult-row-stage-orch.md", consultRowStageAgents, withConsultant(c))
	dir := filepath.Dir(rig.orchPath)
	writeConsultRowStagePlan(t, dir)
	rig.store.memStore.state = rowStageSeed("test-writer", 3, "EXECUTION", "1.Test")
	rig.store.memStore.exists = true
	rig.store.memStore.state.RunSettings = domain.RunSettings{Mode: domain.ExecutionModeOrchestrated}
	queueSuccess(rig.f, "implementer", "implementer#3")
	cfg := domain.RunConfig{
		OrchestratorFilePath: rig.orchPath, WorkflowID: consultRowStageWorkflow, Task: "test task",
		RunID: testRunID, RunFolder: dir,
		RunSettings: domain.RunSettings{Mode: domain.ExecutionModeOrchestrated},
	}

	rig.start(cfg)

	rig.requireParityAfter(t, 1, 1)
	got := rig.rec.recorded()
	if len(got) == 0 || got[len(got)-1].Stage != "2.Implementation" {
		t.Fatalf("scenario did not run as intended: want an announced row with stage 2.Implementation, got %+v", got)
	}
}
