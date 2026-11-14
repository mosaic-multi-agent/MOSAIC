package session_test

// Fixture helpers shared across CLI-harness, OpenCode, and GHCP-CLI session tests:
// directory setup, harness config factories, content-scan wrapper, and backup
// assertion helper.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/snapshot/backup"
	"mosaic-run/internal/snapshot/manifest"
)

// ---- CLI harness helpers ----

// writeCLIHarnessDir creates a temp directory following the claude-code
// harness agents-directory convention (.claude/agents/) and populates it with
// the linear workflow orchestrator file and both agent definition files.
//
// Returns the work directory root and the orchestrator file path (inside the
// agents dir).
func writeCLIHarnessDir(t *testing.T) (workDir, orchPath string) {
	t.Helper()
	workDir = t.TempDir()
	agentsDir := filepath.Join(workDir, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("writeCLIHarnessDir: create agents dir: %v", err)
	}

	orchPath = writeOrchestratorDefinition(t, agentsDir, "claude-code", "linear-orch.md")
	writeDefinitionFile(t, agentsDir, "claude-code", "agent-a")
	writeDefinitionFile(t, agentsDir, "claude-code", "agent-b")
	return
}

// baseCLIHarnessConfig returns a RunConfig for the linear workflow with the
// claude-code harness identity and an explicit run ID.
func baseCLIHarnessConfig(orchPath, runID string) domain.RunConfig {
	return domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           "linear",
		Task:                 "snapshot integration test",
		IsNewRun:             true,
		RunID:                runID,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
		HarnessID:            "claude-code",
	}
}

// runnerCopyDirs returns every per-run agents copy directory
// (.claude/agents-runner-*) that currently exists under workDir.
func runnerCopyDirs(t *testing.T, workDir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(workDir, ".claude", "agents-runner-*"))
	if err != nil {
		t.Fatalf("runnerCopyDirs: glob: %v", err)
	}
	return matches
}

// snapshotAgentFiles reads every file in dir and returns name -> content.
func snapshotAgentFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("snapshotAgentFiles: read dir: %v", err)
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("snapshotAgentFiles: read %s: %v", e.Name(), err)
		}
		out[e.Name()] = string(data)
	}
	return out
}

// ---- OpenCode harness helpers ----

// writeOpenCodeAgentFile writes an agent definition file in dir for the given
// agent ID. The file has YAML frontmatter with mode=subagent so that the
// opencode backup-and-transform rules can rewrite it to mode=primary in-place,
// and the model every CLI-harness run requires.
// After Cleanup, the file should be restored to mode=subagent.
func writeOpenCodeAgentFile(t *testing.T, dir, agentID string) {
	t.Helper()
	writeDefinitionFile(t, dir, "opencode", agentID)
}

// writeOpenCodeHarnessDir creates a temporary working directory for the
// opencode harness (.opencode/agents/). Returns the working directory, the
// orchestrator file path, and the expected backup directory path.
//
// Agent files are written with mode=subagent frontmatter so that the
// backup-and-transform rules have a field to transform.
func writeOpenCodeHarnessDir(t *testing.T) (workDir, orchPath, backupDir string) {
	t.Helper()
	workDir = t.TempDir()
	agentsDir := filepath.Join(workDir, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("writeOpenCodeHarnessDir: create agents dir: %v", err)
	}

	orchPath = writeOrchestratorDefinition(t, agentsDir, "opencode", "linear-orch.md")

	writeOpenCodeAgentFile(t, agentsDir, "agent-a")
	writeOpenCodeAgentFile(t, agentsDir, "agent-b")

	// Backup dir is a sibling of the agents dir, following NewBackupDir:
	//   filepath.Join(filepath.Dir(agentsDir), BackupDirName)
	backupDir = filepath.Join(workDir, ".opencode", backup.BackupDirName)
	return
}

// baseOpenCodeHarnessConfig returns a RunConfig for the linear workflow with
// the opencode harness identity and an explicit run ID.
func baseOpenCodeHarnessConfig(orchPath, runID string) domain.RunConfig {
	return domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           "linear",
		Task:                 "backup-and-transform integration test",
		IsNewRun:             true,
		RunID:                runID,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
		HarnessID:            "opencode",
	}
}

// ---- GHCP-CLI harness helpers ----

// writeGHCPCLIHarnessDir creates a temporary working directory for the
// ghcp-cli harness (.github/agents/). Returns the working directory and the
// orchestrator file path. Agent files have no special frontmatter because
// TransformationsFor("ghcp-cli") returns nil (FR-3: skip backup-and-transform).
func writeGHCPCLIHarnessDir(t *testing.T) (workDir, orchPath string) {
	t.Helper()
	workDir = t.TempDir()
	agentsDir := filepath.Join(workDir, ".github", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("writeGHCPCLIHarnessDir: create agents dir: %v", err)
	}

	orchPath = writeOrchestratorDefinition(t, agentsDir, "ghcp-cli", "linear-orch.md")
	writeDefinitionFile(t, agentsDir, "ghcp-cli", "agent-a")
	writeDefinitionFile(t, agentsDir, "ghcp-cli", "agent-b")
	return
}

// baseGHCPCLIHarnessConfig returns a RunConfig for the linear workflow with
// the ghcp-cli harness identity and an explicit run ID.
func baseGHCPCLIHarnessConfig(orchPath, runID string) domain.RunConfig {
	return domain.RunConfig{
		OrchestratorFilePath: orchPath,
		WorkflowID:           "linear",
		Task:                 "ghcp-cli integration test",
		IsNewRun:             true,
		RunID:                runID,
		RunSettings:          domain.RunSettings{Mode: domain.ExecutionModeAuto},
		HarnessID:            "ghcp-cli",
	}
}

// ---- agentContentScanHarness ----

// agentContentScanHarness is a test-local domain.HarnessAdapter wrapper that
// reads the agent definition file from disk before returning the scripted
// response. This allows tests to verify that backup-and-transform has modified
// the agent files in-place at dispatch time, without changing MockAdapter.
type agentContentScanHarness struct {
	delegate       *harness.MockAdapter
	mu             sync.Mutex
	scannedContent map[string][]byte // agentID -> file content at invoke time
}

func (h *agentContentScanHarness) Invoke(ctx context.Context, agent domain.AgentReference, req domain.ProtocolRequest) (domain.ProtocolResponse, error) {
	if data, readErr := os.ReadFile(agent.DefinitionPath); readErr == nil {
		h.mu.Lock()
		if h.scannedContent == nil {
			h.scannedContent = make(map[string][]byte)
		}
		h.scannedContent[agent.Identifier] = data
		h.mu.Unlock()
	}
	return h.delegate.Invoke(ctx, agent, req)
}

func (h *agentContentScanHarness) contentFor(agentID string) []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.scannedContent[agentID]
}

// ---- OpenCode backup/recovery fixture helpers ----

// writeStaleOpenCodeBackupDir creates an orphaned backup directory that
// simulates a previous opencode run that completed transforms and then
// crashed before releasing the run lock. The backup directory contains:
//   - backup copies of agent-a.md and agent-b.md (same content as originals)
//   - recovery-manifest.json (valid, listing both files)
//   - .setup-complete (signal that transforms were applied)
//
// No .lock-* files are present, so RecoveryCheck will determine no run is
// active and will safely restore and remove the backup directory.
func writeStaleOpenCodeBackupDir(t *testing.T, agentsDir, backupDir string) {
	t.Helper()
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("writeStaleOpenCodeBackupDir: mkdir: %v", err)
	}

	// Copy agent files to backup (same content as originals).
	for _, name := range []string{"agent-a.md", "agent-b.md"} {
		data, err := os.ReadFile(filepath.Join(agentsDir, name))
		if err != nil {
			t.Fatalf("writeStaleOpenCodeBackupDir: read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, name), data, 0o600); err != nil {
			t.Fatalf("writeStaleOpenCodeBackupDir: write backup %s: %v", name, err)
		}
	}

	// Write a valid recovery manifest that lists both backed-up files.
	manifestJSON := fmt.Sprintf(
		`{"timestamp":"2026-01-01T00:00:00Z","agents_dir":%q,"files":[`+
			`{"filename":"agent-a.md","field":"mode","original_value":"subagent"},`+
			`{"filename":"agent-b.md","field":"mode","original_value":"subagent"}]}`,
		agentsDir,
	)
	if err := os.WriteFile(filepath.Join(backupDir, manifest.ManifestFileName), []byte(manifestJSON), 0o600); err != nil {
		t.Fatalf("writeStaleOpenCodeBackupDir: write manifest: %v", err)
	}

	// Write .setup-complete to indicate transforms were fully applied.
	if err := os.WriteFile(filepath.Join(backupDir, backup.SetupCompleteFileName), []byte(""), 0o600); err != nil {
		t.Fatalf("writeStaleOpenCodeBackupDir: write .setup-complete: %v", err)
	}
}

// writeCorruptOpenCodeBackupDir creates a backup directory with a corrupt
// manifest (invalid JSON). RecoveryCheck will return a RefusalError when
// it encounters this, causing the session to refuse the run.
func writeCorruptOpenCodeBackupDir(t *testing.T, agentsDir, backupDir string) {
	t.Helper()
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("writeCorruptOpenCodeBackupDir: mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, manifest.ManifestFileName), []byte("{invalid json}"), 0o600); err != nil {
		t.Fatalf("writeCorruptOpenCodeBackupDir: write manifest: %v", err)
	}
}

// ---- Backup cleanup assertion ----

// assertBackupCleanedUp verifies the backup-and-transform cleanup completed:
// backup dir deleted and agent-a.md restored to mode=subagent.
func assertBackupCleanedUp(t *testing.T, agentsDir, backupDir string) {
	t.Helper()

	if _, statErr := os.Stat(backupDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("backup dir %q must be deleted by Cleanup (last-out check); "+
			"stat returned: %v", backupDir, statErr)
	}

	agentAContent, readErr := os.ReadFile(filepath.Join(agentsDir, "agent-a.md"))
	if readErr != nil {
		t.Fatalf("read agent-a.md after run: %v", readErr)
	}
	if !strings.Contains(string(agentAContent), "subagent") {
		t.Errorf("agent-a.md after run: want mode=subagent (restored by Cleanup), "+
			"got %q; BackupState.Cleanup must call the last-out check which restores originals",
			string(agentAContent))
	}
}
