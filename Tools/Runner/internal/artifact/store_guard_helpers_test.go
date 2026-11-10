package artifact_test

// Shared builders for the write-guard and evidence-copy tests: a valid document
// the four writers can all work on, the damages the guard must react to, and a
// table of the writers.

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

var guardNow = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

// guardDoc renders a valid artifact with commits enabled, no commit branch yet
// and no recorded Runner settings, so every writer can succeed on it.
func guardDoc(d strictDoc) string {
	return strings.Replace(d.String(), "checkpoints: disabled\n", "checkpoints: disabled\ncommits: enabled\n", 1)
}

// guardDamage damages the valid document in one way.
type guardDamage struct {
	name string
	doc  func() string
}

func damagedDoc(mutate func(d *strictDoc)) func() string {
	return func() string {
		d := newStrictDoc()
		mutate(&d)
		return guardDoc(d)
	}
}

var guardDamages = []guardDamage{
	{"short row", damagedDoc(func(d *strictDoc) {
		d.logRows = append(d.logRows, "| 3 | worker#3 | EXECUTION | - | 1 | SUCCESS | short |")
	})},
	{"non-pipe line inside a table", damagedDoc(func(d *strictDoc) { d.logTail = []string{"stray prose line"} })},
	{"non-increasing Seq", damagedDoc(func(d *strictDoc) {
		d.logRows = append(d.logRows, rawLogRow("2", "worker#2", "repeated"))
	})},
	{"free-text Agent", damagedDoc(func(d *strictDoc) {
		d.logRows = append(d.logRows, rawLogRow("3", "the orchestrator did this", "free text"))
	})},
	{"missing closing tag", func() string {
		return strings.Replace(guardDoc(newStrictDoc()), "</ExecutionLog>\n", "", 1)
	}},
	{"broken frontmatter", func() string {
		return strings.Replace(guardDoc(newStrictDoc()), "type: orchestration-artifact\n", "type: [unclosed\n", 1)
	}},
}

// guardWriter is one artifact writer. good is a state read from the valid file.
type guardWriter struct {
	name  string
	write func(ctx context.Context, s domain.ArtifactStore, good domain.ArtifactState) error
}

var guardWriters = []guardWriter{
	{"Apply", func(ctx context.Context, s domain.ArtifactStore, good domain.ArtifactState) error {
		_, err := s.Apply(ctx, good, newTestStep(3, "worker#3", "EXECUTION", "", domain.StatusSUCCESS, guardNow, nil))
		return err
	}},
	{"SetPhase", func(ctx context.Context, s domain.ArtifactStore, good domain.ArtifactState) error {
		_, err := s.SetPhase(ctx, good, "COMPLETED", guardNow)
		return err
	}},
	{"SetCommitBranch", func(ctx context.Context, s domain.ArtifactStore, _ domain.ArtifactState) error {
		_, err := s.SetCommitBranch(ctx, "mosaic/run/"+testRunID, guardNow)
		return err
	}},
	{"AdoptRunnerSettings", func(ctx context.Context, s domain.ArtifactStore, _ domain.ArtifactState) error {
		_, err := s.AdoptRunnerSettings(ctx, domain.ExecutionModeAuto, false, false, guardNow)
		return err
	}},
}

// rereadingWriters are the writers that re-read the artifact themselves.
func rereadingWriters() []guardWriter { return guardWriters[1:] }

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// guardFixture writes the valid document to a plain temp path and returns a
// store over it, the path and the state read from the valid file.
func guardFixture(t *testing.T) (domain.ArtifactStore, string, domain.ArtifactState) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Orchestration.md")
	writeFileT(t, path, guardDoc(newStrictDoc()))
	store := artifact.NewFileStore(path)
	good, err := store.Read(context.Background())
	if err != nil {
		t.Fatalf("reading the valid fixture: %v", err)
	}
	return store, path, good
}

// runScopedFixture writes the valid document to {ws}/Orchestration-{run_id}/
// Orchestration.md (no evidence copy yet) and returns a store, the paths and the
// state read from the valid file.
func runScopedFixture(t *testing.T) (store domain.ArtifactStore, ws, path string, good domain.ArtifactState) {
	t.Helper()
	ws = t.TempDir()
	path = filepath.Join(ws, domain.RunScopedFolder(testRunID), "Orchestration.md")
	writeFileT(t, path, guardDoc(newStrictDoc()))
	store = artifact.NewFileStore(path)
	good, err := store.Read(context.Background())
	if err != nil {
		t.Fatalf("reading the valid fixture: %v", err)
	}
	return store, ws, path, good
}

func evidenceOf(t *testing.T, store domain.ArtifactStore) domain.ArtifactEvidence {
	t.Helper()
	ev, ok := store.(domain.ArtifactEvidence)
	if !ok {
		t.Fatalf("the store from artifact.NewFileStore (%T) does not implement domain.ArtifactEvidence", store)
	}
	return ev
}
