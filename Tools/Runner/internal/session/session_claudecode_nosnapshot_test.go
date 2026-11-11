package session_test

// Tests for run start on the claude-code harness: agents are invoked
// directly from the deployed agents directory. No per-run copy of the agents
// directory is created, no backup or transform is applied, and every agent
// reference names the deployed file.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
	"mosaic-run/internal/snapshot/backup"
)

func queueLinearAgentsDone(f *harness.MockAdapter) {
	f.Queue("agent-a", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-a#1",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "done",
	}})
	f.Queue("agent-b", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
		AgentInstanceID: "agent-b#2",
		StatusCode:      domain.StatusSUCCESS,
		StatusMessage:   "done",
	}})
}

// TestSession_Start_ClaudeCode_NoPerRunCopyDuringOrAfterRun verifies that no
// agents-runner-* directory exists while agents are dispatched or after the
// run completes.
func TestSession_Start_ClaudeCode_NoPerRunCopyDuringOrAfterRun(t *testing.T) {
	const runID = "cc-nosnap-01"
	workDir, orchPath := writeCLIHarnessDir(t)

	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	var copiesDuringRun []string
	cb := &beforeInvokeHarness{delegate: f, before: func(string) {
		copiesDuringRun = append(copiesDuringRun, runnerCopyDirs(t, workDir)...)
	}}

	ses := session.New(session.Deps{
		Harness:  cb,
		Store:    &memStore{},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})

	got, err := ses.Start(context.Background(), baseCLIHarnessConfig(orchPath, runID))
	requireRunStatus(t, got, err, domain.RunCompleted)

	if len(f.Invocations()) == 0 {
		t.Fatal("want at least one invocation, got none")
	}
	if len(copiesDuringRun) != 0 {
		t.Errorf("per-run copy directories existed during dispatch: %v", copiesDuringRun)
	}
	if after := runnerCopyDirs(t, workDir); len(after) != 0 {
		t.Errorf("per-run copy directories exist after the run: %v", after)
	}
}

// TestSession_Start_ClaudeCode_AgentReferencesNameDeployedFiles verifies that
// every dispatched agent reference points at the deployed file in the agents
// directory.
func TestSession_Start_ClaudeCode_AgentReferencesNameDeployedFiles(t *testing.T) {
	const runID = "cc-nosnap-02"
	workDir, orchPath := writeCLIHarnessDir(t)
	agentsDir := filepath.Join(workDir, ".claude", "agents")

	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    &memStore{},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})

	got, err := ses.Start(context.Background(), baseCLIHarnessConfig(orchPath, runID))
	requireRunStatus(t, got, err, domain.RunCompleted)

	invs := f.Invocations()
	if len(invs) == 0 {
		t.Fatal("want at least one invocation, got none")
	}
	for _, inv := range invs {
		want := filepath.Join(agentsDir, inv.Agent.Identifier+".md")
		if filepath.Clean(inv.Agent.DefinitionPath) != filepath.Clean(want) {
			t.Errorf("agent %q: DefinitionPath %q, want deployed file %q",
				inv.Agent.Identifier, inv.Agent.DefinitionPath, want)
		}
	}
}

// TestSession_Start_ClaudeCode_OrchestratorRefNamesDeployedFile verifies that
// the orchestrator reference bound to consultants is the deployed
// orchestrator file, not a copy.
func TestSession_Start_ClaudeCode_OrchestratorRefNamesDeployedFile(t *testing.T) {
	const runID = "cc-nosnap-03"
	_, orchPath := writeCLIHarnessDir(t)

	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	capture := &orchRefCaptureConsultant{}
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    &memStore{},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
		Routing:  capture,
	})

	got, err := ses.Start(context.Background(), baseCLIHarnessConfig(orchPath, runID))
	requireRunStatus(t, got, err, domain.RunCompleted)

	if capture.bindCallCount() == 0 {
		t.Fatal("want consultants bound to a run context, got no BindRunContext call")
	}
	orchRef := capture.lastBoundOrchRef()
	if filepath.Clean(orchRef.DefinitionPath) != filepath.Clean(orchPath) {
		t.Errorf("last bound orchestrator DefinitionPath %q, want deployed file %q",
			orchRef.DefinitionPath, orchPath)
	}
}

// TestSession_Start_ClaudeCode_DeployedFilesUntouchedNoBackup verifies that
// agent files are byte-identical during and after the run and that no backup
// directory is created.
func TestSession_Start_ClaudeCode_DeployedFilesUntouchedNoBackup(t *testing.T) {
	const runID = "cc-nosnap-04"
	workDir, orchPath := writeCLIHarnessDir(t)
	agentsDir := filepath.Join(workDir, ".claude", "agents")
	original := snapshotAgentFiles(t, agentsDir)

	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	var duringRun []map[string]string
	cb := &beforeInvokeHarness{delegate: f, before: func(string) {
		duringRun = append(duringRun, snapshotAgentFiles(t, agentsDir))
	}}
	ses := session.New(session.Deps{
		Harness:  cb,
		Store:    &memStore{},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})

	got, err := ses.Start(context.Background(), baseCLIHarnessConfig(orchPath, runID))
	requireRunStatus(t, got, err, domain.RunCompleted)

	if len(duringRun) == 0 {
		t.Fatal("want at least one dispatch observation")
	}
	for i, snap := range duringRun {
		if !reflect.DeepEqual(snap, original) {
			t.Errorf("agents directory changed before dispatch %d", i)
		}
	}
	if after := snapshotAgentFiles(t, agentsDir); !reflect.DeepEqual(after, original) {
		t.Errorf("agents directory changed by the run")
	}
	backupDir := filepath.Join(workDir, ".claude", backup.BackupDirName)
	if _, statErr := os.Stat(backupDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("backup dir %q must not exist for claude-code; stat returned: %v", backupDir, statErr)
	}
}

// TestSession_Start_ClaudeCode_NoPerRunCopyOnFailureOutcomes verifies that no
// per-run copy exists during dispatch or after the run for runs that end
// without completing.
func TestSession_Start_ClaudeCode_NoPerRunCopyOnFailureOutcomes(t *testing.T) {
	t.Run("RunFailed", func(t *testing.T) {
		workDir, orchPath := writeCLIHarnessDir(t)
		f := harness.NewMockAdapter()
		f.Queue("agent-a", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: "agent-a#1", StatusCode: domain.StatusSUCCESS, StatusMessage: "done",
		}})
		var during []string
		cb := &beforeInvokeHarness{delegate: f, before: func(string) {
			during = append(during, runnerCopyDirs(t, workDir)...)
		}}
		store := &memStore{
			applyErrOnFirst: true,
			applyFirstErr:   errors.New("test: forced apply failure to trigger RunFailed"),
		}
		ses := session.New(session.Deps{Harness: cb, Store: store, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

		got, _ := ses.Start(context.Background(), baseCLIHarnessConfig(orchPath, "cc-nosnap-fail"))

		if got.Status != domain.RunFailed {
			t.Fatalf("want RunFailed, got %q (%s)", got.Status, got.Message)
		}
		if len(during) != 0 {
			t.Errorf("per-run copy existed during dispatch: %v", during)
		}
		if after := runnerCopyDirs(t, workDir); len(after) != 0 {
			t.Errorf("per-run copy exists after the run: %v", after)
		}
	})

	t.Run("RunDeviationUnresolved", func(t *testing.T) {
		workDir, orchPath := writeCLIHarnessDir(t)
		f := harness.NewMockAdapter()
		f.Queue("agent-a", harness.ScriptedEntry{Response: &domain.ProtocolResponse{
			AgentInstanceID: "agent-a#1", StatusCode: domain.StatusPARTIALLY_DONE, StatusMessage: "partial",
		}})
		var during []string
		cb := &beforeInvokeHarness{delegate: f, before: func(string) {
			during = append(during, runnerCopyDirs(t, workDir)...)
		}}
		ses := session.New(session.Deps{Harness: cb, Store: &memStore{}, Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}})

		got, err := ses.Start(context.Background(), baseCLIHarnessConfig(orchPath, "cc-nosnap-dev"))
		requireRunStatus(t, got, err, domain.RunDeviationUnresolved)

		if len(f.Invocations()) == 0 {
			t.Fatal("want at least one invocation")
		}
		if len(during) != 0 {
			t.Errorf("per-run copy existed during dispatch: %v", during)
		}
		if after := runnerCopyDirs(t, workDir); len(after) != 0 {
			t.Errorf("per-run copy exists after the run: %v", after)
		}
	})
}

// TestSession_Start_ClaudeCode_StaleRunnerCopyIsLeftAlone verifies that a
// stale agents-runner-* directory from an older version is neither read nor
// removed.
func TestSession_Start_ClaudeCode_StaleRunnerCopyIsLeftAlone(t *testing.T) {
	const runID = "cc-nosnap-stale"
	workDir, orchPath := writeCLIHarnessDir(t)
	stale := filepath.Join(workDir, ".claude", "agents-runner-"+runID)
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatalf("create stale dir: %v", err)
	}
	marker := filepath.Join(stale, "marker.txt")
	if err := os.WriteFile(marker, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	f := harness.NewMockAdapter()
	queueLinearAgentsDone(f)
	ses := session.New(session.Deps{
		Harness:  f,
		Store:    &memStore{},
		Clock:    fixedClock{t: epoch},
		Interact: &noopInteraction{},
	})

	got, err := ses.Start(context.Background(), baseCLIHarnessConfig(orchPath, runID))
	requireRunStatus(t, got, err, domain.RunCompleted)

	data, readErr := os.ReadFile(marker)
	if readErr != nil || string(data) != "stale" {
		t.Errorf("stale directory content must be left alone; read %q, err %v", data, readErr)
	}
}

// TestSession_Start_NonCLIHarness_SnapshotStepSkipped verifies that when
// HarnessID is empty (not a known CLI harness), agents are resolved from the
// original agents directory and no per-run directory segment appears in any
// DefinitionPath.
func TestSession_Start_NonCLIHarness_SnapshotStepSkipped(t *testing.T) {
	ses, f, _, orchPath := newLinearSession(t)
	queueLinearAgentsDone(f)

	got, err := ses.Start(context.Background(), baseLinearConfig(orchPath))
	requireRunStatus(t, got, err, domain.RunCompleted)

	for _, inv := range f.Invocations() {
		if strings.Contains(filepath.ToSlash(inv.Agent.DefinitionPath), "agents-runner-") {
			t.Errorf("agent %q: DefinitionPath %q contains a per-run directory segment",
				inv.Agent.Identifier, inv.Agent.DefinitionPath)
		}
	}
}
