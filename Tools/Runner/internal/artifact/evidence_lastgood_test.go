package artifact_test

// Tests for the last-good copy: after every successful write the store keeps
// the written bytes outside the run folder, so they survive a restart. The copy
// is evidence only; a refused or failed write never touches it.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

func requireLastGood(t *testing.T, store domain.ArtifactStore) string {
	t.Helper()
	path, ok := evidenceOf(t, store).LastGoodCopy()
	if !ok || path == "" {
		t.Fatalf("LastGoodCopy() = (%q, %v), want an existing copy", path, ok)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the reported last-good copy is not on disk: %v", err)
	}
	return path
}

func TestEvidenceDir_RunScopedPath_IsRunnerLogsFolderOfTheRun(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, domain.RunScopedFolder(testRunID), "Orchestration.md")

	dir, ok := artifact.EvidenceDir(path)

	if !ok {
		t.Fatal("EvidenceDir reported no location for a run-scoped path")
	}
	if want := filepath.Join(ws, "RunnerLogs", testRunID); dir != want {
		t.Errorf("EvidenceDir = %q, want %q", dir, want)
	}
}

func TestEvidenceDir_PathThatIsNotRunScoped_HasNoLocation(t *testing.T) {
	dir, ok := artifact.EvidenceDir(filepath.Join(t.TempDir(), "Orchestration.md"))

	if ok || dir != "" {
		t.Errorf("EvidenceDir = (%q, %v), want (\"\", false)", dir, ok)
	}
}

func TestCreate_KeepsALastGoodCopyWithTheWrittenContent(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, domain.RunScopedFolder(testRunID), "Orchestration.md")
	writeFileT(t, filepath.Join(filepath.Dir(path), "marker"), "") // run folder exists
	store := artifact.NewFileStore(path)

	_, err := store.Create(context.Background(), domain.WorkflowInfo{ID: "quick-fix", Version: "1.0"}, "task",
		domain.RunSettings{Mode: domain.ExecutionModeAuto}, guardNow, testRunID)

	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	copyPath := requireLastGood(t, store)
	if got, want := readFileT(t, copyPath), readFileT(t, path); got != want {
		t.Errorf("last-good copy differs from the written artifact:\n%s", got)
	}
}

func TestLastGoodCopy_AfterEachWriter_HoldsExactlyTheWrittenContent(t *testing.T) {
	for _, w := range guardWriters {
		t.Run(w.name, func(t *testing.T) {
			store, _, path, good := runScopedFixture(t)

			if err := w.write(context.Background(), store, good); err != nil {
				t.Fatalf("write: %v", err)
			}

			copyPath := requireLastGood(t, store)
			if got, want := readFileT(t, copyPath), readFileT(t, path); got != want {
				t.Errorf("last-good copy differs from the written artifact:\n%s", got)
			}
		})
	}
}

func TestLastGoodCopy_SecondWrite_ReplacesTheFirstContent(t *testing.T) {
	store, _, path, good := runScopedFixture(t)
	ctx := context.Background()
	if err := guardWriters[1].write(ctx, store, good); err != nil { // SetPhase
		t.Fatalf("first write: %v", err)
	}
	firstCopy := readFileT(t, requireLastGood(t, store))

	if err := guardWriters[2].write(ctx, store, good); err != nil { // SetCommitBranch
		t.Fatalf("second write: %v", err)
	}

	secondCopy := readFileT(t, requireLastGood(t, store))
	if secondCopy == firstCopy {
		t.Error("the last-good copy was not refreshed by the second write")
	}
	if secondCopy != readFileT(t, path) {
		t.Error("the last-good copy differs from the artifact after the second write")
	}
}

func TestLastGoodCopy_IsOutsideTheRunFolder(t *testing.T) {
	store, ws, path, good := runScopedFixture(t)
	if err := guardWriters[1].write(context.Background(), store, good); err != nil {
		t.Fatalf("write: %v", err)
	}

	copyPath := requireLastGood(t, store)

	rel, err := filepath.Rel(filepath.Dir(path), copyPath)
	if err != nil || !strings.HasPrefix(rel, "..") {
		t.Errorf("last-good copy %q is inside the run folder %q", copyPath, filepath.Dir(path))
	}
	if !strings.HasPrefix(copyPath, filepath.Join(ws, "RunnerLogs", testRunID)+string(filepath.Separator)) {
		t.Errorf("last-good copy %q is not in the run's evidence folder", copyPath)
	}
}

func TestLastGoodCopy_ARefusedWrite_LeavesTheCopyUntouched(t *testing.T) {
	store, _, path, good := runScopedFixture(t)
	ctx := context.Background()
	if err := guardWriters[1].write(ctx, store, good); err != nil {
		t.Fatalf("write: %v", err)
	}
	copyPath := requireLastGood(t, store)
	before := readFileT(t, copyPath)
	writeFileT(t, path, guardDamages[1].doc())
	if _, err := store.Read(ctx); err == nil {
		t.Fatal("the damaged fixture was read without error")
	}

	for _, w := range guardWriters {
		requireRefusedAfterDamage(t, w.write(ctx, store, good))
	}

	if got := readFileT(t, copyPath); got != before {
		t.Errorf("a refused write changed the last-good copy:\n%s", got)
	}
}

func TestLastGoodCopy_AFailedWrite_LeavesTheCopyUntouched(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, domain.RunScopedFolder(testRunID), "Orchestration.md")
	writeFileT(t, path, guardDoc(newStrictDoc()))
	store := artifact.NewFileStore(path)
	ctx := context.Background()
	good, err := store.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if err := guardWriters[1].write(ctx, store, good); err != nil {
		t.Fatalf("write: %v", err)
	}
	copyPath := requireLastGood(t, store)
	before := readFileT(t, copyPath)

	// Adopt once (succeeds), then again: the second is refused (already recorded).
	if err := guardWriters[3].write(ctx, store, good); err != nil {
		t.Fatalf("first adoption: %v", err)
	}
	afterAdopt := readFileT(t, copyPath)
	if afterAdopt == before {
		t.Fatal("the adoption did not refresh the last-good copy")
	}
	if err := guardWriters[3].write(ctx, store, good); err == nil {
		t.Fatal("the second adoption succeeded, want a refusal")
	}

	if got := readFileT(t, copyPath); got != afterAdopt {
		t.Errorf("a failed write changed the last-good copy:\n%s", got)
	}
}

func TestLastGoodCopy_SurvivesAStoreRestart(t *testing.T) {
	store, _, path, good := runScopedFixture(t)
	if err := guardWriters[1].write(context.Background(), store, good); err != nil {
		t.Fatalf("write: %v", err)
	}
	want := requireLastGood(t, store)

	restarted := artifact.NewFileStore(path)

	got := requireLastGood(t, restarted)
	if got != want {
		t.Errorf("restarted store reports %q, want %q", got, want)
	}
}

func TestLastGoodCopy_NeverWritten_IsReportedAbsent(t *testing.T) {
	store, _, _, _ := runScopedFixture(t)

	path, ok := evidenceOf(t, store).LastGoodCopy()

	if ok || path != "" {
		t.Errorf("LastGoodCopy() = (%q, %v), want (\"\", false)", path, ok)
	}
}

func TestLastGoodCopy_PathNotRunScoped_WritesNothingAndReportsAbsent(t *testing.T) {
	store, _, good := guardFixture(t)

	if err := guardWriters[1].write(context.Background(), store, good); err != nil {
		t.Fatalf("write: %v", err)
	}

	if path, ok := evidenceOf(t, store).LastGoodCopy(); ok || path != "" {
		t.Errorf("LastGoodCopy() = (%q, %v), want (\"\", false)", path, ok)
	}
}

func TestLastGoodCopy_ARefreshThatCannotBeWritten_DoesNotFailTheArtifactWrite(t *testing.T) {
	store, ws, path, good := runScopedFixture(t)
	writeFileT(t, filepath.Join(ws, "RunnerLogs"), "a file where the evidence folder must go")
	before := readFileT(t, path)

	err := guardWriters[1].write(context.Background(), store, good)

	if err != nil {
		t.Fatalf("the write failed because the evidence copy could not be written: %v", err)
	}
	if readFileT(t, path) == before {
		t.Error("the artifact was not written")
	}
	if p, ok := evidenceOf(t, store).LastGoodCopy(); ok {
		t.Errorf("LastGoodCopy() reports %q although none could be written", p)
	}
}

