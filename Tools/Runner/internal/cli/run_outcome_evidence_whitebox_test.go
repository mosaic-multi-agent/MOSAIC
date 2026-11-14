package cli

// The COMPLETED-phase write of a CLI run, with no injected store, keeps a
// last-good copy outside the run folder.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

type completedSession struct{}

func (completedSession) Start(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
	return domain.RunOutcome{Status: domain.RunCompleted}, nil
}

func TestRunSessionAndReport_CompletedWithoutInjectedStore_KeepsALastGoodCopy(t *testing.T) {
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
	var out, errOut bytes.Buffer

	runSessionAndReport(context.Background(), domain.RunConfig{}, nil, completedSession{}, &out, &errOut, runFolder)

	copyPath := filepath.Join(ws, "RunnerLogs", runID, "Orchestration.last-good.md")
	data, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatalf("no last-good copy after the COMPLETED write: %v (stderr: %s)", err, errOut.String())
	}
	if !strings.Contains(string(data), "COMPLETED") {
		t.Errorf("the last-good copy lacks the COMPLETED marker:\n%s", data)
	}
}
