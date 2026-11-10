package repair_test

// Builders for the Occurrence tests: a workspace laid out like production
// (workspace/Orchestration-{run id}/Orchestration.md with the run's logs in
// workspace/RunnerLogs/{run id}/), a damaged artifact and the occurrence over it.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/dispatchlog"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/repair"
)

const occurrenceRunID = "20261109T195225Z-41da"

// occurrenceRun is a run on disk whose artifact is damaged.
type occurrenceRun struct {
	workspace    string
	artifactPath string
	store        domain.ArtifactStore
	damage       *domain.ArtifactDamageError
	repaired     string // the repaired artifact text
	lastGood     string // path of a last-good copy holding rows 1..76
}

// newOccurrenceRun writes the incident's damaged artifact as the run's artifact
// and keeps the repaired text and a last-good copy for the tests that judge.
func newOccurrenceRun(t *testing.T) *occurrenceRun {
	t.Helper()
	ws := t.TempDir()
	folder := filepath.Join(ws, domain.RunScopedFolder(occurrenceRunID))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	r := &occurrenceRun{workspace: ws, artifactPath: filepath.Join(folder, "Orchestration.md")}
	r.repaired = incidentRepaired(t)
	r.lastGood = writeFile(t, "Orchestration.last-good.md", withUpTo(t, r.repaired, 76))
	r.writeArtifact(t, incidentDamaged(t))
	r.store = artifact.NewFileStore(r.artifactPath)
	_, err := r.store.Read(t.Context())
	if !errors.As(err, &r.damage) {
		t.Fatalf("test setup: the incident artifact should read as damaged, got %v", err)
	}
	return r
}

func (r *occurrenceRun) writeArtifact(t *testing.T, doc string) {
	t.Helper()
	if err := os.WriteFile(r.artifactPath, []byte(doc), 0o644); err != nil {
		t.Fatalf("test setup: %v", err)
	}
}

// runLogPath is where the run's own dispatch log lives.
func (r *occurrenceRun) runLogPath() string {
	return dispatchlog.RunLogPath(r.workspace, occurrenceRunID)
}

// writeRunLog creates the run's own dispatch log with one SUCCESS response per row.
func (r *occurrenceRun) writeRunLog(t *testing.T) {
	t.Helper()
	lines := append([]string{versionLine(t)}, successResponses(t, rowAgents(r.repaired))...)
	if err := os.MkdirAll(filepath.Dir(r.runLogPath()), 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := os.WriteFile(r.runLogPath(), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("test setup: %v", err)
	}
}

// occurrence opens the occurrence with the given evidence.
func (r *occurrenceRun) occurrence(lastGood, damagedCopy, reportedLog string) *repair.Occurrence {
	return repair.NewOccurrence(repair.OccurrenceInput{
		Store: r.store, ArtifactPath: r.artifactPath, RunID: occurrenceRunID, Damage: r.damage,
		LastGoodCopy: lastGood, DamagedCopy: damagedCopy, ReportedDispatchLog: reportedLog,
	})
}

func problemTexts(problems []domain.ArtifactDamageProblem) []string {
	out := make([]string, len(problems))
	for i, p := range problems {
		out[i] = p.String()
	}
	return out
}
