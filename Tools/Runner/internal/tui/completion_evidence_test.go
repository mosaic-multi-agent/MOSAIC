package tui

// The COMPLETED-phase write of the TUI goes through a store that keeps the
// last-good copy outside the run folder.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

func TestResolveArtifactStore_Default_KeepsALastGoodCopyAfterTheCompletedWrite(t *testing.T) {
	const runID = "20260727T170000Z-a3f9"
	ws := t.TempDir()
	runFolder := filepath.Join(ws, domain.RunScopedFolder(runID))
	if err := os.MkdirAll(runFolder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seed := artifact.NewFileStore(filepath.Join(runFolder, "Orchestration.md"))
	if _, err := seed.Create(context.Background(), domain.WorkflowInfo{ID: "test-workflow", Version: "1.0"},
		"task", domain.RunSettings{}, time.Now().UTC(), runID); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(ws, "RunnerLogs")); err != nil {
		t.Fatalf("clearing seed evidence: %v", err)
	}
	m := &rootModel{}

	store := m.resolveArtifactStore(runFolder)
	if _, err := store.SetPhase(context.Background(), domain.ArtifactState{}, "COMPLETED", time.Now().UTC()); err != nil {
		t.Fatalf("SetPhase: %v", err)
	}

	evidence, ok := store.(domain.ArtifactEvidence)
	if !ok {
		t.Fatalf("the default store (%T) does not implement domain.ArtifactEvidence", store)
	}
	path, found := evidence.LastGoodCopy()
	if !found {
		t.Fatal("no last-good copy after the COMPLETED write")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the copy: %v", err)
	}
	if !strings.Contains(string(data), "COMPLETED") {
		t.Errorf("the last-good copy lacks the COMPLETED marker:\n%s", data)
	}
	if filepath.Dir(path) != filepath.Join(ws, "RunnerLogs", runID) {
		t.Errorf("last-good copy %q is not in the run's evidence folder", path)
	}
}
