package session_test

// Test of the failure list a resumed run reports: an infrastructure failure
// recorded by an earlier session stays on the outcome next to the failures of
// the resumed session.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

const earlierCheckpointInstance = infraCheckpointAgent + "#2"

// seedEarlierSessionFailure stores a run whose first session finished a
// workflow step and then recorded a failed checkpoint step.
func seedEarlierSessionFailure(store *memStore) {
	store.state = domain.ArtifactState{
		Workflow:        "linear",
		WorkflowVersion: "1.0",
		Task:            "test task",
		GlobalSequence:  2,
		RunSettings:     domain.RunSettings{Mode: domain.ExecutionModeAuto, Checkpoints: true},
		CurrentState: domain.CurrentState{
			Phase: "PLANNING", LastStatus: domain.StatusSUCCESS, LastAgent: "agent-a#1",
		},
		ExecutionLog: []domain.ExecutionLogEntry{
			{Seq: 1, Agent: "agent-a#1", Phase: "PLANNING", Status: domain.StatusSUCCESS},
			{
				Seq: 2, Agent: earlierCheckpointInstance, Phase: "PLANNING",
				WorkflowRow: domain.NoWorkflowRow, Status: domain.StatusBLOCKED,
				ErrorCode: domain.ErrorTOOL_UNAVAILABLE, Summary: "earlier session failure",
			},
		},
	}
	store.exists = true
}

func TestSession_InfraDispatch_ResumedRun_OutcomeListsEarlierSessionFailureAndTheNewOne(t *testing.T) {
	rig := newInfraReportRig(t, infraContinueFixture, true, nil)
	seedEarlierSessionFailure(rig.store)
	rig.adapter.Queue("agent-b", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-b#x", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
	}})
	for range 2 {
		rig.queueCheckpoint(checkpointReply(domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "new failure", "resumed reason"))
	}
	cfg := baseLinearConfig(rig.orchPath)
	cfg.Checkpoints = true
	markResume(&cfg)

	got, err := rig.ses.Start(t.Context(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	var listed []string
	for _, f := range got.InfraFailures {
		listed = append(listed, f.AgentInstance)
	}
	if len(listed) < 2 || listed[0] != earlierCheckpointInstance {
		t.Fatalf("outcome InfraFailures = %v, want the earlier-session failure %s first, then the new one", listed, earlierCheckpointInstance)
	}
	if first := got.InfraFailures[0]; first.Status != domain.StatusBLOCKED || first.ErrorCode != domain.ErrorTOOL_UNAVAILABLE {
		t.Errorf("earlier failure = %+v, want BLOCKED E501 as recorded", first)
	}
	if last := got.InfraFailures[len(got.InfraFailures)-1]; last.ErrorCode != domain.ErrorINPUT_NOT_FOUND {
		t.Errorf("last listed failure = %+v, want the failure of the resumed session (E101)", last)
	}
}
