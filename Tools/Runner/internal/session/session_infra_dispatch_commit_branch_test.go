package session_test

// The recorded commit branch in the task description of a commit-class
// ITERATION_END dispatch: the previous description followed by
// "; commit branch: {branch}", the branch inserted byte for byte. Without a
// recorded branch such a dispatch is not sent and the run fails naming the
// missing commit branch. Dispatches of other classes and commit dispatches
// fired only by other triggers keep their description.

import (
	"context"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const branchSuffixPrefix = "; commit branch: "

// userOwnBranches are recorded branches a user is already on: they carry '/'
// and '-' and must reach the commit agent unchanged.
var userOwnBranches = []string{
	"PBI/RegulatedCurrentTriggerPath",
	"team-a/fix-login/step-2",
}

// requireBranchSuffix asserts the description ends with the branch suffix.
func requireBranchSuffix(t *testing.T, req domain.ProtocolRequest, branch string) {
	t.Helper()
	if !strings.HasSuffix(req.TaskDescription, branchSuffixPrefix+branch) {
		t.Errorf("want task description to end with %q, got %q", branchSuffixPrefix+branch, req.TaskDescription)
	}
}

// requireNoBranchInDescription asserts the description names no commit branch.
func requireNoBranchInDescription(t *testing.T, req domain.ProtocolRequest) {
	t.Helper()
	if strings.Contains(req.TaskDescription, "commit branch") {
		t.Errorf("want no commit branch in the task description, got %q", req.TaskDescription)
	}
}

// resumeProductWithBranch is the commit gate rig resumed from a run that
// recorded commits enabled with the given branch.
func resumeProductWithBranch(t *testing.T, branch string) *blockRig {
	t.Helper()
	rig := resumeCommitGateRig(t, commitGateFixture, true)
	rig.store.state.CommitBranch = branch
	return rig
}

func TestSession_Start_CommitOnIterationEnd_TaskDescriptionIsThePreviousTextPlusTheRecordedBranch(t *testing.T) {
	rig := newStageGroupedRig(t, "2")
	rig.queueCommitSetup()
	rig.writeRunFile(t, "Stage-2/Plan.md", "# Plan\n")
	queueStageRows(rig, map[string]string{"test-writer": "Stage-2/PlanProgress.md"})
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	req := rig.onlyRequestOf(t, infraCommitAgent)
	want := "infrastructure agent dispatch: " + infraCommitAgent +
		", commit the artifacts of Stage 2 (triggered by ITERATION_END); commit branch: " + commitSetupBranch
	if req.TaskDescription != want {
		t.Errorf("want task description %q, got %q", want, req.TaskDescription)
	}
}

func TestSession_Start_CommitOnIterationEnd_EveryIterationsDispatchNamesTheBranch(t *testing.T) {
	rig := newCommitGateRig(t, commitGateFixture)
	rig.queueCommitSetup()
	rig.queueOKTimes(infraCommitAgent, 2)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	reqs := rig.requestsOf(infraCommitAgent)
	if len(reqs) != 2 {
		t.Fatalf("want 2 commit dispatches, got %d", len(reqs))
	}
	for i, id := range []string{"A", "B"} {
		requireBranchSuffix(t, reqs[i], commitSetupBranch)
		requireDescriptionNames(t, reqs[i], "Product "+id)
	}
}

func TestSession_Start_CommitOnIterationEnd_BranchDoesNotChangeInputsOutputsOrHITL(t *testing.T) {
	rig := newStageGroupedRig(t, "2")
	rig.queueCommitSetup()
	rig.writeRunFile(t, "Stage-2/Plan.md", "# Plan\n")
	queueStageRows(rig, map[string]string{"test-writer": "Stage-2/PlanProgress.md"})
	rig.queueOK(infraCommitAgent)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig("stage-grouped", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	req := rig.onlyRequestOf(t, infraCommitAgent)
	requireBranchSuffix(t, req, commitSetupBranch)
	requireTriggerDispatchShape(t, req)
	requireSequence(t, "commit input_artifacts", req.InputArtifacts, wildcardRunPaths("Stage-2/Plan.md", "Stage-2/PlanProgress.md"))
	rows := infraRowsOf(rig.store, infraCommitAgent)
	want := dispatchPath("Stage-2/Plan.md") + ", " + dispatchPath("Stage-2/PlanProgress.md")
	if len(rows) != 1 || rows[0].Inputs != want {
		t.Errorf("want one commit row with Inputs %q, got %+v", want, rows)
	}
}

func TestSession_Start_CommitOnIterationEnd_UserOwnBranchFromSetupReply_PassedByteForByte(t *testing.T) {
	for _, branch := range userOwnBranches {
		t.Run(branch, func(t *testing.T) {
			rig := newCommitGateRig(t, commitGateFixture)
			rig.adapter.Queue(infraCommitAgent, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
				AgentInstanceID: infraCommitAgent + "#setup",
				StatusCode:      domain.StatusSUCCESS,
				StatusMessage:   "on your branch [branch:" + branch + "]",
			}})
			rig.queueOKTimes(infraCommitAgent, 2)
			cfg := rig.commitsConfig(commitGateWorkflow, domain.ExecutionModeAuto)
			cfg.CommitBranchVariant = domain.CommitBranchUserOwn

			got, err := rig.ses.Start(context.Background(), cfg)

			requireRunStatus(t, got, err, domain.RunCompleted)
			reqs := rig.requestsOf(infraCommitAgent)
			if len(reqs) != 2 {
				t.Fatalf("want 2 commit dispatches, got %d", len(reqs))
			}
			for _, req := range reqs {
				requireBranchSuffix(t, req, branch)
			}
		})
	}
}

func TestSession_Start_CommitOnIterationEnd_ResumeRecordingUserOwnBranch_PassedByteForByte(t *testing.T) {
	for _, branch := range userOwnBranches {
		t.Run(branch, func(t *testing.T) {
			rig := resumeProductWithBranch(t, branch)

			got, err := rig.ses.Start(context.Background(), rig.resumeConfig(commitGateWorkflow, domain.ExecutionModeAuto))

			requireRunStatus(t, got, err, domain.RunCompleted)
			reqs := rig.requestsOf(infraCommitAgent)
			if len(reqs) != 2 {
				t.Fatalf("want 2 commit dispatches, got %d", len(reqs))
			}
			for _, req := range reqs {
				requireBranchSuffix(t, req, branch)
			}
		})
	}
}

func TestSession_Start_CommitOnIterationEndAndIntervalTogether_DispatchedOnceNamingTheBranch(t *testing.T) {
	rig := runProductWithInfra(t, "infra-contents-commit-two-triggers-orch.md", infraCommitAgent, 1)

	req := rig.onlyRequestOf(t, infraCommitAgent)

	requireDescriptionNames(t, req, "Product A", "ITERATION_END", "INVOCATION_INTERVAL")
	requireBranchSuffix(t, req, commitSetupBranch)
}

// ===== Descriptions that stay as they were =====

func TestSession_Start_CommitFiredOnlyByInterval_DescriptionNamesNoBranch(t *testing.T) {
	rig := runProductWithInfra(t, "infra-contents-commit-interval-orch.md", infraCommitAgent, 1)

	req := rig.onlyRequestOf(t, infraCommitAgent)

	want := "infrastructure agent dispatch: " + infraCommitAgent + ", triggered by INVOCATION_INTERVAL"
	if req.TaskDescription != want {
		t.Errorf("want task description %q, got %q", want, req.TaskDescription)
	}
}

func TestSession_Start_CheckpointOnIterationEnd_DescriptionNamesNoBranch(t *testing.T) {
	rig := runProductWithInfra(t, "infra-contents-checkpoint-orch.md", "checkpoint-manager-git", 1)

	req := rig.onlyRequestOf(t, "checkpoint-manager-git")

	want := "infrastructure agent dispatch: checkpoint-manager-git, triggered by ITERATION_END"
	if req.TaskDescription != want {
		t.Errorf("want task description %q, got %q", want, req.TaskDescription)
	}
	requireNoBranchInDescription(t, req)
}

func TestSession_Start_CommitSetupDispatch_DescriptionNamesNoBranch(t *testing.T) {
	rig := newCommitGateRig(t, commitGateFixture)
	rig.queueCommitSetup()
	rig.queueOKTimes(infraCommitAgent, 2)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	setup := rig.adapter.Invocations()[0].Request
	want := "commit setup: establish the target branch for this run"
	if setup.TaskDescription != want {
		t.Errorf("want setup task description %q, got %q", want, setup.TaskDescription)
	}
}

// ===== Missing recorded branch =====

// branchlessStore accepts the commit setup but leaves commit_branch empty in
// the recorded artifact, standing in for a store that lost the branch.
type branchlessStore struct {
	*memStore
}

func (s branchlessStore) SetCommitBranch(_ context.Context, branch string, _ time.Time) (domain.ArtifactState, error) {
	s.memStore.BranchCalls = append(s.memStore.BranchCalls, branch)
	return s.memStore.state, nil
}

// newBranchlessRig is the commit gate rig over a store that never records the
// commit branch.
func newBranchlessRig(t *testing.T) *blockRig {
	t.Helper()
	rig := newCommitGateRig(t, commitGateFixture)
	rig.queueCommitSetup()
	rig.queueOKTimes(infraCommitAgent, 2)
	rig.ses = session.New(session.Deps{
		Harness:  rig.adapter,
		Store:    branchlessStore{rig.store},
		Routing:  rig.consult,
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})
	return rig
}

func TestSession_Start_CommitOnIterationEnd_NoRecordedBranch_NoDispatchAndRunFailsNamingTheMissingBranch(t *testing.T) {
	rig := newBranchlessRig(t)

	got, err := rig.ses.Start(context.Background(), rig.commitsConfig(commitGateWorkflow, domain.ExecutionModeAuto))

	if err == nil {
		t.Fatal("want a non-nil error when the commit branch is missing, got nil")
	}
	if got.Status != domain.RunFailed {
		t.Errorf("want Status=%q, got %q (message: %q)", domain.RunFailed, got.Status, got.Message)
	}
	for _, tok := range []string{"commit branch", infraCommitAgent} {
		if !strings.Contains(got.Message, tok) {
			t.Errorf("want the failure message to mention %q, got %q", tok, got.Message)
		}
	}
	// The failure is raised at the first iteration end, not at setup: the
	// first iteration's rows ran and nothing after them.
	if n := rig.invocationsOf("checker"); n != 1 {
		t.Errorf("want the first iteration to have run (1 checker dispatch), got %d", n)
	}
	requireNoCommitActivity(t, rig)
}
