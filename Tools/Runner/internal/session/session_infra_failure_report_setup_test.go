package session_test

// Tests of how a commit setup that ends without SUCCESS is reported in the
// Runner log: one dedicated failure event naming the commit agent, an empty
// trigger list and the halt policy. A successful setup reports no failure.

import (
	"errors"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

const setupInstance = "commit-manager-git#1"

// startCommitSetup runs a new commits-enabled run whose commit setup dispatch
// ends as entry, and returns the outcome with the captured debug log.
func startCommitSetup(t *testing.T, entry harness.ScriptedEntry) (domain.RunOutcome, *sessionRecordingLogger) {
	t.Helper()
	dir := t.TempDir()
	orchPath := copyOrchestratorFile(t, dir, "commit-agent-orch.md")
	for _, agent := range []string{"agent-a", "agent-b", "commit-manager-git"} {
		writeAgentFile(t, dir, agent)
	}
	f := harness.NewMockAdapter()
	logger := &sessionRecordingLogger{}
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    &memStore{},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
		Debug:    logger,
	})
	f.Queue("commit-manager-git", entry)
	cfg := baseCommitConfig(orchPath)
	cfg.RunID = "test-run-id"

	got, err := ses.Start(t.Context(), cfg)
	if err != nil && got.Status == "" {
		t.Fatalf("Start returned error %v without an outcome", err)
	}
	return got, logger
}

func TestSession_CommitSetup_Blocked_WritesOneFailureEventWithClassEmptyTriggersAndHaltPolicy(t *testing.T) {
	got, logger := startCommitSetup(t, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: setupInstance,
		StatusCode:      domain.StatusBLOCKED,
		StatusMessage:   "dirty working tree",
		ErrorCode:       domain.ErrorPERMISSION_DENIED,
		ErrorReason:     "uncommitted changes",
	}})

	if got.Status != domain.RunStartFailed {
		t.Errorf("want RunStartFailed, got %q", got.Status)
	}
	failed := logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event, got %d", domain.EventSessionInfraFailed, len(failed))
	}
	requireFields(t, failed[0], map[string]string{
		"agent": setupInstance, "class": "commit", "triggers": "", "status": "BLOCKED",
		"error_code": "E502", "policy": "halt",
	})
	if !strings.Contains(failed[0].Message, "uncommitted changes") {
		t.Errorf("event message = %q, want the response's error reason", failed[0].Message)
	}
}

func TestSession_CommitSetup_HarnessError_WritesFailureEventWithE501AndTheErrorText(t *testing.T) {
	got, logger := startCommitSetup(t, harness.ScriptedEntry{Err: errors.New("git: authentication failed")})

	if got.Status != domain.RunStartFailed {
		t.Errorf("want RunStartFailed, got %q", got.Status)
	}
	failed := logger.entriesOf(domain.EventSessionInfraFailed)
	if len(failed) != 1 {
		t.Fatalf("want exactly 1 %s event, got %d", domain.EventSessionInfraFailed, len(failed))
	}
	requireFields(t, failed[0], map[string]string{"status": "BLOCKED", "error_code": "E501", "policy": "halt", "triggers": ""})
	if !strings.Contains(failed[0].Message, "authentication failed") {
		t.Errorf("event message = %q, want the harness error text", failed[0].Message)
	}
}

func TestSession_CommitSetup_FailureIsOnTheOutcomeFailureList(t *testing.T) {
	got, _ := startCommitSetup(t, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: setupInstance,
		StatusCode:      domain.StatusBLOCKED,
		StatusMessage:   "dirty working tree",
		ErrorCode:       domain.ErrorPERMISSION_DENIED,
	}})

	if len(got.InfraFailures) != 1 {
		t.Fatalf("outcome InfraFailures = %+v, want the failed setup step", got.InfraFailures)
	}
	if f := got.InfraFailures[0]; f.AgentInstance != setupInstance || f.Status != domain.StatusBLOCKED || f.ErrorCode != domain.ErrorPERMISSION_DENIED {
		t.Errorf("listed failure = %+v, want %s BLOCKED E502", f, setupInstance)
	}
}

func TestSession_CommitSetup_SuccessWithoutBranchMarker_WritesNoFailureEvent(t *testing.T) {
	got, logger := startCommitSetup(t, harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: setupInstance,
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "setup complete (no branch marker)",
	}})

	if got.Status != domain.RunStartFailed {
		t.Errorf("want RunStartFailed for the missing branch marker, got %q", got.Status)
	}
	if n := len(logger.entriesOf(domain.EventSessionInfraFailed)); n != 0 {
		t.Errorf("want no %s event for a SUCCESS setup, got %d", domain.EventSessionInfraFailed, n)
	}
	if len(got.InfraFailures) != 0 {
		t.Errorf("outcome InfraFailures = %+v, want none for a SUCCESS setup", got.InfraFailures)
	}
}

func TestSession_CommitSetup_Success_WritesNoFailureEventAndNoNewStepDone(t *testing.T) {
	_, logger := startCommitSetup(t, commitSetupEntry())

	if n := len(logger.entriesOf(domain.EventSessionInfraFailed)); n != 0 {
		t.Errorf("want no %s event for a successful setup, got %d", domain.EventSessionInfraFailed, n)
	}
	if done := logger.stepDoneOf(setupInstance); len(done) != 0 {
		t.Errorf("want no new %s event for commit setup, got %d", domain.EventSessionStepDone, len(done))
	}
}
