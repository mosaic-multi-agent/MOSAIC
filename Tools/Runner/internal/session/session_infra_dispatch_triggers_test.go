package session_test

// What the checkpoint-class and the non-ITERATION_END commit-class trigger
// dispatches carry: no input artifacts and a task description naming each
// trigger that matched in the pass, with the agent dispatched once however
// many of its triggers matched. The commit setup dispatch is unchanged.
// Review-class dispatches carry the run's Orchestration.md and are covered in
// session_infra_dispatch_review_test.go.

import (
	"context"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

// runProductWithInfra runs the fixture with one Product iteration A, the
// iteration's files written by the builder and checker, and the given number
// of scripted dispatches of the infrastructure agent. Commits are enabled when
// the agent is the commit-class one, checkpoints otherwise.
func runProductWithInfra(t *testing.T, fixture, infraAgent string, dispatches int) *blockRig {
	t.Helper()
	rig := newProductRig(t, fixture, infraAgent, "A")
	cfg := rig.config("product-block", domain.ExecutionModeAuto)
	if infraAgent == infraCommitAgent {
		rig.queueCommitSetup()
		cfg = rig.commitsConfig("product-block", domain.ExecutionModeAuto)
	} else {
		cfg.Checkpoints = true
	}
	queueProductIteration(rig, "A")
	rig.queueOKTimes(infraAgent, dispatches)

	got, err := rig.ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	return rig
}

func TestSession_Start_CheckpointOnIterationEnd_CarriesNoInputsAndNamesTheTrigger(t *testing.T) {
	rig := runProductWithInfra(t, "infra-contents-checkpoint-orch.md", "checkpoint-manager-git", 1)

	req := rig.onlyRequestOf(t, "checkpoint-manager-git")
	// The iteration's own files exist (built-A.md, checked-A.md); only a
	// commit-class agent receives them.
	if len(req.InputArtifacts) != 0 {
		t.Errorf("want no input_artifacts for a checkpoint dispatch, got %q", req.InputArtifacts)
	}
	requireTriggerDispatchShape(t, req)
	requireDescriptionNames(t, req, "ITERATION_END")
	rows := infraRowsOf(rig.store, "checkpoint-manager-git")
	if len(rows) != 1 || rows[0].Inputs != "" {
		t.Errorf("want one checkpoint row with empty Inputs, got %+v", rows)
	}
}

func TestSession_Start_AgentWithTwoMatchingTriggers_DispatchedOnceNamingBoth(t *testing.T) {
	// The checker is the third workflow step and the last row of iteration A,
	// so ITERATION_END and INVOCATION_INTERVAL 3 match in the same pass.
	rig := runProductWithInfra(t, "infra-contents-two-triggers-orch.md", "checkpoint-manager-git", 1)

	req := rig.onlyRequestOf(t, "checkpoint-manager-git")
	requireDescriptionNames(t, req, "ITERATION_END", "INVOCATION_INTERVAL")
	if len(req.InputArtifacts) != 0 {
		t.Errorf("want no input_artifacts for a checkpoint dispatch, got %q", req.InputArtifacts)
	}
	requireTriggerDispatchShape(t, req)
}

func TestSession_Start_CommitFiredOnlyByInterval_CarriesNoInputsAndNamesTheTrigger(t *testing.T) {
	rig := runProductWithInfra(t, "infra-contents-commit-interval-orch.md", infraCommitAgent, 1)

	req := rig.onlyRequestOf(t, infraCommitAgent)
	if len(req.InputArtifacts) != 0 {
		t.Errorf("want no input_artifacts when ITERATION_END did not fire the commit agent, got %q", req.InputArtifacts)
	}
	requireTriggerDispatchShape(t, req)
	requireDescriptionNames(t, req, "INVOCATION_INTERVAL")
}

func TestSession_Start_CommitWithIterationEndAndIntervalMatching_DispatchedOnceWithIterationFiles(t *testing.T) {
	rig := runProductWithInfra(t, "infra-contents-commit-two-triggers-orch.md", infraCommitAgent, 1)

	req := rig.onlyRequestOf(t, infraCommitAgent)
	requireSequence(t, "commit input_artifacts", req.InputArtifacts, wildcardRunPaths("built-A.md", "checked-A.md"))
	requireDescriptionNames(t, req, "Product A")
	requireTriggerDispatchShape(t, req)
}

func TestSession_Start_CheckpointOnPhaseEnd_CarriesNoInputsAndNamesTheTrigger(t *testing.T) {
	// The planner ends the PLANNING phase and the checker ends EXECUTION: two
	// PHASE_END dispatches.
	rig := runProductWithInfra(t, "infra-contents-phase-end-orch.md", "checkpoint-manager-git", 2)

	reqs := rig.requestsOf("checkpoint-manager-git")
	if len(reqs) != 2 {
		t.Fatalf("want 2 PHASE_END dispatches (one per phase), got %d", len(reqs))
	}
	for i, req := range reqs {
		if len(req.InputArtifacts) != 0 {
			t.Errorf("dispatch %d: want no input_artifacts for a PHASE_END dispatch, got %q", i+1, req.InputArtifacts)
		}
		requireTriggerDispatchShape(t, req)
		requireDescriptionNames(t, req, "PHASE_END")
	}
}

func TestSession_Start_IntervalDispatch_CarriesNoInputsAndNamesTheTrigger(t *testing.T) {
	ses, f, store, orchPath := newIntervalAgentSession(t)
	for _, agent := range []string{"agent-a", "agent-b", "checkpoint-manager-git"} {
		f.Queue(agent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: agent + "#q", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
		}})
	}
	// The interval fixture dispatches the checkpoint agent twice; the loop
	// above scripted only one reply, so a second one is queued here.
	f.Queue("checkpoint-manager-git", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "checkpoint-manager-git#q", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
	}})
	cfg := baseLinearConfig(orchPath)
	cfg.Checkpoints = true

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	var reqs []domain.ProtocolRequest
	for _, inv := range f.Invocations() {
		if inv.Agent.Identifier == "checkpoint-manager-git" {
			reqs = append(reqs, inv.Request)
		}
	}
	if len(reqs) == 0 {
		t.Fatal("want at least one interval dispatch, got none")
	}
	for i, req := range reqs {
		if len(req.InputArtifacts) != 0 {
			t.Errorf("dispatch %d: want no input_artifacts, got %q", i+1, req.InputArtifacts)
		}
		requireTriggerDispatchShape(t, req)
		requireDescriptionNames(t, req, "INVOCATION_INTERVAL")
	}
	if rows := infraRowsOf(store, "checkpoint-manager-git"); len(rows) != len(reqs) {
		t.Errorf("want one row per dispatch (%d), got %d", len(reqs), len(rows))
	}
}

// The commit setup dispatch is not a trigger dispatch: it keeps its request.
func TestSession_Start_CommitSetupDispatch_RequestIsUnchanged(t *testing.T) {
	ses, f, _, orchPath := newCommitSession(t)
	f.Queue(infraCommitAgent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: infraCommitAgent + "#1",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "branch ready [branch:mosaic/run/test-run-id]",
	}})
	for _, agent := range []string{"agent-a", "agent-b"} {
		f.Queue(agent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: agent + "#q", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
		}})
	}
	cfg := baseCommitConfig(orchPath)
	cfg.RunID = "test-run-id"

	got, err := ses.Start(context.Background(), cfg)

	// A setup-time refusal must not masquerade as "first dispatch".
	if err != nil || got.Status == domain.RunRefused {
		t.Fatalf("want the run to start past the commit setup, got status %q, err %v", got.Status, err)
	}

	invs := f.Invocations()
	if len(invs) == 0 || invs[0].Agent.Identifier != infraCommitAgent {
		t.Fatalf("want the commit setup as the first dispatch, got %+v", invokedAgents(f))
	}
	req := invs[0].Request
	if len(req.InputArtifacts) != 0 {
		t.Errorf("want no input_artifacts on the commit setup dispatch, got %q", req.InputArtifacts)
	}
	const wantDescription = "commit setup: establish the target branch for this run"
	if req.TaskDescription != wantDescription {
		t.Errorf("want task description %q, got %q", wantDescription, req.TaskDescription)
	}
}
