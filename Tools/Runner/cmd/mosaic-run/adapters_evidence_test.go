package main

// Wiring tests: the store the composition root builds keeps the evidence copies
// outside the run folder, and the path-anomaly logging stays unchanged.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

const evidenceTestRunID = "20260805T143029Z-9bc0"

func TestNewLoggedArtifactStore_RunScopedPath_KeepsALastGoodCopyInRunnerLogs(t *testing.T) {
	ws := t.TempDir()
	runFolder := filepath.Join(ws, domain.RunScopedFolder(evidenceTestRunID))
	if err := os.MkdirAll(runFolder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	logger := &recordingLogger{}
	store := newLoggedArtifactStore(filepath.Join(runFolder, "Orchestration.md"), logger)

	evidence, ok := store.(domain.ArtifactEvidence)
	if !ok {
		t.Fatalf("the production store (%T) does not implement domain.ArtifactEvidence", store)
	}
	if _, err := store.Create(context.Background(), domain.WorkflowInfo{ID: "quick-fix", Version: "1.0"}, "task",
		domain.RunSettings{Mode: domain.ExecutionModeAuto}, time.Now().UTC(), evidenceTestRunID); err != nil {
		t.Fatalf("Create: %v", err)
	}

	path, found := evidence.LastGoodCopy()

	if !found {
		t.Fatal("no last-good copy after Create")
	}
	if want := filepath.Join(ws, "RunnerLogs", evidenceTestRunID); filepath.Dir(path) != want {
		t.Errorf("last-good copy %q is not in %q", path, want)
	}
	if n := len(logger.snapshot()); n != 0 {
		t.Errorf("a run-scoped path logged %d anomaly event(s), want none", n)
	}
}

func TestNewLoggedArtifactStore_AnomalousPath_StillLogsAndStillImplementsEvidence(t *testing.T) {
	logger := &recordingLogger{}

	store := newLoggedArtifactStore(filepath.Join(t.TempDir(), "Orchestration.md"), logger)

	if _, ok := store.(domain.ArtifactEvidence); !ok {
		t.Errorf("the store (%T) does not implement domain.ArtifactEvidence", store)
	}
	if n := len(logger.snapshot()); n != 1 {
		t.Errorf("logged %d anomaly event(s), want 1", n)
	}
}
