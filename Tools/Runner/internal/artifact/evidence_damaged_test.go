package artifact_test

// Tests for saving a damaged copy: a timestamped copy of the artifact as it is
// on disk, beside the last-good copy, never overwriting an earlier copy and
// never touching the artifact.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
)

// damagedFixture is a run-scoped store whose file is damaged on disk, with a
// last-good copy already written. Returns the store, artifact path and the
// damaged content.
func damagedFixture(t *testing.T) (domain.ArtifactStore, string, string) {
	t.Helper()
	store, _, path, good := runScopedFixture(t)
	if err := guardWriters[1].write(context.Background(), store, good); err != nil {
		t.Fatalf("write: %v", err)
	}
	damaged := guardDamages[1].doc()
	writeFileT(t, path, damaged)
	return store, path, damaged
}

func TestSaveDamagedCopy_CopiesTheCurrentFileBesideTheLastGoodCopy(t *testing.T) {
	store, path, damaged := damagedFixture(t)
	lastGood := requireLastGood(t, store)

	saved, err := evidenceOf(t, store).SaveDamagedCopy(guardNow)

	if err != nil {
		t.Fatalf("SaveDamagedCopy: %v", err)
	}
	if !filepath.IsAbs(saved) {
		t.Errorf("returned path %q is not absolute", saved)
	}
	if got := readFileT(t, saved); got != damaged {
		t.Errorf("the damaged copy differs from the file on disk:\n%s", got)
	}
	if filepath.Dir(saved) != filepath.Dir(lastGood) {
		t.Errorf("damaged copy %q is not beside the last-good copy %q", saved, lastGood)
	}
	if saved == lastGood {
		t.Error("the damaged copy is the last-good copy")
	}
	if got := readFileT(t, path); got != damaged {
		t.Error("the artifact was touched")
	}
	if got := readFileT(t, lastGood); got == damaged {
		t.Error("the last-good copy was overwritten with the damaged content")
	}
}

func TestSaveDamagedCopy_NameCarriesTheUTCTimestamp(t *testing.T) {
	store, _, _ := damagedFixture(t)
	local := time.Date(2026, 11, 9, 14, 30, 15, 0, time.FixedZone("plus2", 2*3600))

	saved, err := evidenceOf(t, store).SaveDamagedCopy(local)

	if err != nil {
		t.Fatalf("SaveDamagedCopy: %v", err)
	}
	base := filepath.Base(saved)
	if !strings.Contains(base, "damaged") || !strings.Contains(base, "20261109T123015Z") {
		t.Errorf("name %q should say damaged and carry the UTC timestamp 20261109T123015Z", base)
	}
}

func TestSaveDamagedCopy_SecondSaveAtADifferentTime_KeepsTheFirstCopy(t *testing.T) {
	store, path, damaged := damagedFixture(t)
	ev := evidenceOf(t, store)
	first, err := ev.SaveDamagedCopy(guardNow)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	changed := damaged + "\nstill damaged\n"
	writeFileT(t, path, changed)

	second, err := ev.SaveDamagedCopy(guardNow.Add(time.Minute))

	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if first == second {
		t.Fatalf("both saves returned %q", first)
	}
	if got := readFileT(t, first); got != damaged {
		t.Errorf("the first copy was overwritten:\n%s", got)
	}
	if got := readFileT(t, second); got != changed {
		t.Errorf("the second copy differs from the file at that time:\n%s", got)
	}
}

func TestSaveDamagedCopy_SecondSaveAtTheSameTime_DoesNotOverwrite(t *testing.T) {
	store, path, damaged := damagedFixture(t)
	ev := evidenceOf(t, store)
	first, err := ev.SaveDamagedCopy(guardNow)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	changed := damaged + "\nstill damaged\n"
	writeFileT(t, path, changed)

	second, err := ev.SaveDamagedCopy(guardNow)

	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if first == second {
		t.Fatalf("both saves returned %q", first)
	}
	if got := readFileT(t, first); got != damaged {
		t.Errorf("the first copy was overwritten:\n%s", got)
	}
	if got := readFileT(t, second); got != changed {
		t.Errorf("the second copy differs from the file at that time:\n%s", got)
	}
}

func TestSaveDamagedCopy_WithoutALastGoodCopy_StillSavesInTheEvidenceFolder(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, domain.RunScopedFolder(testRunID), "Orchestration.md")
	damaged := guardDamages[1].doc()
	writeFileT(t, path, damaged)
	store := artifact.NewFileStore(path)

	saved, err := evidenceOf(t, store).SaveDamagedCopy(guardNow)

	if err != nil {
		t.Fatalf("SaveDamagedCopy: %v", err)
	}
	if want := filepath.Join(ws, "RunnerLogs", testRunID); filepath.Dir(saved) != want {
		t.Errorf("damaged copy %q is not in %q", saved, want)
	}
	if got := readFileT(t, saved); got != damaged {
		t.Errorf("the damaged copy differs from the file:\n%s", got)
	}
}

func TestSaveDamagedCopy_PathNotRunScoped_ReportsNoEvidenceLocation(t *testing.T) {
	store, path, _ := guardFixture(t)
	before := readFileT(t, path)

	saved, err := evidenceOf(t, store).SaveDamagedCopy(guardNow)

	if !errors.Is(err, domain.ErrNoEvidenceLocation) {
		t.Errorf("error = %v, want domain.ErrNoEvidenceLocation", err)
	}
	if saved != "" {
		t.Errorf("returned path %q, want none", saved)
	}
	if readFileT(t, path) != before {
		t.Error("the artifact was touched")
	}
}

func TestSaveDamagedCopy_NoArtifactFile_ReturnsAnIOError(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, domain.RunScopedFolder(testRunID), "Orchestration.md")
	store := artifact.NewFileStore(path)

	saved, err := evidenceOf(t, store).SaveDamagedCopy(guardNow)

	if err == nil || errors.Is(err, domain.ErrNoEvidenceLocation) {
		t.Errorf("error = %v, want an I/O error", err)
	}
	if saved != "" {
		t.Errorf("returned path %q, want none", saved)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "RunnerLogs", testRunID)); statErr == nil {
		entries, _ := os.ReadDir(filepath.Join(ws, "RunnerLogs", testRunID))
		if len(entries) != 0 {
			t.Errorf("a failed save left %d file(s) behind", len(entries))
		}
	}
}
