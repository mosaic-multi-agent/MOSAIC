package session_test

// Tests of a commit setup invocation that ends in a harness error: the setup
// row is a HARNESS_ERROR naming the failure kind (not a BLOCKED that looks
// subagent-returned), the run still ends as a resumable start failure, and a
// resume retries setup after such a row.

import (
	"context"
	"fmt"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

func TestSession_CommitSetup_InvocationTimeout_RowIsHarnessErrorNamingTheKind(t *testing.T) {
	r := startWithSetupEntry(t, harness.ScriptedEntry{Err: fmt.Errorf("git setup stalled: %w", commonharness.ErrTimeout)})

	requireSetupFailureKept(t, r, domain.StatusHARNESS_ERROR)
	row := r.store.Applied[0]
	if row.ErrorCode != domain.ErrorNone {
		t.Errorf("want no error code on the setup row, got %q", row.ErrorCode)
	}
	requireSummaryNamesKind(t, row.Summary, "timeout", "git setup stalled")
}

func TestSession_CommitSetup_InvocationProcessFailure_RowNamesProcessFailure(t *testing.T) {
	r := startWithSetupEntry(t, harness.ScriptedEntry{Err: fmt.Errorf("exit status 1: %w", commonharness.ErrNonZeroExit)})

	requireSetupFailureKept(t, r, domain.StatusHARNESS_ERROR)
	requireSummaryNamesKind(t, r.store.Applied[0].Summary, "process-failure", "exit status 1")
}

func TestSession_CommitSetup_InvocationFailure_RowIsNotAnyBlocked(t *testing.T) {
	r := startWithSetupEntry(t, harness.ScriptedEntry{Err: fmt.Errorf("stalled: %w", commonharness.ErrTimeout)})

	for _, row := range r.store.Applied {
		if row.Status == domain.StatusBLOCKED || row.ErrorCode != domain.ErrorNone {
			t.Errorf("want no BLOCKED or error-coded row for a failure the subagent never returned, got %+v", row)
		}
	}
}

func TestSession_Start_Resume_AfterHarnessErrorSetup_RetriesSetupWithNextSequence(t *testing.T) {
	ses, f, store, orchPath := newCommitSession(t)
	store.state = commitResumeState()
	store.state.GlobalSequence = 1
	store.state.ExecutionLog = []domain.ExecutionLogEntry{
		{Seq: 1, Agent: "commit-manager-git#1", Status: domain.StatusHARNESS_ERROR, Summary: "timeout: setup stalled"},
	}
	store.exists = true
	const wantBranch = "mosaic/run/retry-after-harness-error"
	queueSuccessfulSetupAndWorkflow(f, wantBranch)

	cfg := baseCommitConfig(orchPath)
	markResume(&cfg)
	cfg.Supplied.CommitBranchVariant = true // a pending setup retry needs the variant

	got, err := ses.Start(context.Background(), cfg)

	requireRunStatus(t, got, err, domain.RunCompleted)
	if len(store.Applied) != 3 {
		t.Fatalf("want 3 newly applied rows (retried setup + 2 workflow), got %d", len(store.Applied))
	}
	if store.Applied[0].AgentInstance != "commit-manager-git#2" || store.Applied[0].Status != domain.StatusSUCCESS {
		t.Errorf("want the setup retried as commit-manager-git#2 and succeeding, got %q %q",
			store.Applied[0].AgentInstance, store.Applied[0].Status)
	}
	if store.state.CommitBranch != wantBranch {
		t.Errorf("want commit_branch=%q after the retry, got %q", wantBranch, store.state.CommitBranch)
	}
}
