package session_test

// Session tests that the Input wildcard listing applies on every path that
// hands Inputs to a subagent besides the first engine-routed dispatch:
// consulted dispatch in Mode 1, consulted dispatch after a Mode 2
// consultation, the dispatch again after a rejected human approval (engine
// routed and consulted), and the mechanical retry of a PARTIALLY_DONE step.

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// wildcardListing is the Inputs every dispatch of agent-b of the fixtures in
// this file must receive for the files in wildcardRunFolderFiles.
func wildcardListing() []string {
	return wildcardRunPaths("Stage-A/Plan.md", "Stage-b-2/Plan.md", "alpha-Research.md", "beta-Research.md")
}

// wildcardRunFolderFiles are the files present in the run folder.
func wildcardRunFolderFiles() []string {
	return append(append([]string{}, wildcardStageFiles...), wildcardResearchFiles...)
}

// TestSession_Start_ConsultedDispatchInMode1_ReceivesTheListedInputs verifies
// that a row the routing consultant dispatches in Mode 1 receives the listed
// artifacts in place of the Input wildcards of its row defaults.
func TestSession_Start_ConsultedDispatchInMode1_ReceivesTheListedInputs(t *testing.T) {
	rig := newBlockRig(t, "wildcard-inputs-orch.md", nil, "agent-a", "agent-b")
	rig.writeRunFiles(t, wildcardRunFolderFiles()...)
	rig.queueOK("agent-b")
	rig.consult.queueDispatch("agent-b", "review the plans", 1)
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-inputs", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	rig.requireConsultedDispatchRan(t, "agent-b")
	rig.requireInputsOfEveryDispatch(t, "agent-b", 1, wildcardListing())
	wantCell := strings.Join(wildcardListing(), ", ")
	if cell := rig.inputsCellOf(t, "agent-b"); cell != wantCell {
		t.Errorf("Inputs cell: want the listed artifacts %q, got %q", wantCell, cell)
	}
}

// TestSession_Start_ConsultedDispatchAfterMode2Consultation_ReceivesTheListedInputs
// verifies the same for a row dispatched on a consultant's instruction after
// a review-class agent triggered a consultation in Mode 2.
func TestSession_Start_ConsultedDispatchAfterMode2Consultation_ReceivesTheListedInputs(t *testing.T) {
	rig := newBlockRig(t, "wildcard-review-consult-orch.md", nil, "agent-a", "agent-b", "review-agent")
	rig.writeRunFiles(t, wildcardRunFolderFiles()...)
	rig.queueOK("agent-a")
	rig.queueOKTimes("review-agent", 2)
	rig.queueOK("agent-b")
	rig.consult.queueDispatch("agent-b", "review the plans", 1)
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-review-consult", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	rig.requireConsultedDispatchRan(t, "agent-b")
	rig.requireInputsOfEveryDispatch(t, "agent-b", 1, wildcardListing())
}

// TestSession_Start_DispatchAfterRejectedApproval_ReceivesTheListedInputs
// verifies that the dispatch again of a step whose human approval was
// rejected receives the listed artifacts, as the first attempt did.
func TestSession_Start_DispatchAfterRejectedApproval_ReceivesTheListedInputs(t *testing.T) {
	approvals := &rejectFirstReadOf{suffix: "result.md"}
	rig := newBlockRig(t, "wildcard-inputs-hitl-orch.md", approvals, "agent-a", "agent-b")
	rig.writeRunFiles(t, "Stage-b-2/Plan.md", "Stage-A/Plan.md")
	rig.queueOK("agent-a")
	rig.queueOKTimes("agent-b", 2)

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-inputs-hitl", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	rig.requireInputsOfEveryDispatch(t, "agent-b", 2, wildcardRunPaths("Stage-A/Plan.md", "Stage-b-2/Plan.md"))
}

// TestSession_Start_ConsultedDispatchAfterRejectedApproval_ReceivesTheListedInputs
// verifies the same for a consulted dispatch whose approval was rejected.
func TestSession_Start_ConsultedDispatchAfterRejectedApproval_ReceivesTheListedInputs(t *testing.T) {
	approvals := &switchingApprovalReader{firstApproval: domain.ApprovalFalse, restApproval: domain.ApprovalTrue}
	rig := newBlockRig(t, "wildcard-inputs-hitl-orch.md", approvals, "agent-a", "agent-b")
	rig.writeRunFiles(t, "Stage-b-2/Plan.md", "Stage-A/Plan.md")
	rig.queueOKTimes("agent-b", 2)
	rig.consult.queueDispatch("agent-b", "review the plans", 1)
	rig.consult.queueStop("done")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-inputs-hitl", domain.ExecutionModeOrchestrated))

	requireRunStatus(t, got, err, domain.RunStoppedByConsultant)
	rig.requireInputsOfEveryDispatch(t, "agent-b", 2, wildcardRunPaths("Stage-A/Plan.md", "Stage-b-2/Plan.md"))
}

// TestSession_Start_MechanicalRetryAfterPartiallyDone_ReceivesTheListedInputs
// verifies that the engine's immediate re-dispatch of a step that returned
// PARTIALLY_DONE receives the listed artifacts, as the first attempt did, plus
// the previous attempt's output handed over as partially-done context.
func TestSession_Start_MechanicalRetryAfterPartiallyDone_ReceivesTheListedInputs(t *testing.T) {
	rig := newBlockRig(t, "wildcard-inputs-orch.md", nil, "agent-a", "agent-b")
	rig.writeRunFiles(t, wildcardRunFolderFiles()...)
	rig.queueOK("agent-a")
	rig.adapter.Queue("agent-b", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-b#queued",
		StatusCode:      domain.StatusPARTIALLY_DONE,
		StatusMessage:   "half done",
	}})
	rig.queueOK("agent-b")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-inputs", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	dispatches := rig.dispatchedInputs("agent-b")
	if len(dispatches) != 2 {
		t.Fatalf("want 2 dispatches of agent-b, got %d", len(dispatches))
	}
	requireSequence(t, "agent-b first dispatch inputs", dispatches[0], wildcardListing())
	retryWant := append(wildcardListing(), dispatchPath("result.md"))
	requireSequence(t, "agent-b retry inputs", dispatches[1], retryWant)
}
