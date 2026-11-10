package runscan_test

// A run whose Orchestration.md is damaged stays listed as a resumable
// candidate, marked damaged, and is told apart from runs that cannot be read
// for other reasons. The pre-3.0 layout, run-identity refusals and completed
// runs keep their existing classification.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/runscan"
)

// damagedRunID is the run_id the damage fixtures record.
const damagedRunID = "20260727T170000Z-a3f9"

// writeDamageFixture copies the named fixture under testdata/damaged into
// dir/Orchestration.md, applying each old->new replacement pair first.
func writeDamageFixture(t *testing.T, dir, name string, replacements ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "damaged", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	text := string(data)
	for i := 0; i+1 < len(replacements); i += 2 {
		if !strings.Contains(text, replacements[i]) {
			t.Fatalf("fixture %s does not contain %q", name, replacements[i])
		}
		text = strings.Replace(text, replacements[i], replacements[i+1], 1)
	}
	if err := os.WriteFile(filepath.Join(dir, "Orchestration.md"), []byte(text), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
}

func scanOne(t *testing.T, root string) runscan.ScanResult {
	t.Helper()
	result, err := runscan.NewDirScanner().Scan(root)
	if err != nil {
		t.Fatalf("Scan error = %v, want nil", err)
	}
	return result
}

func TestScan_DamagedArtifactWithIntactFrontmatter_IsAResumableCandidateMarkedDamaged(t *testing.T) {
	root := t.TempDir()
	writeDamageFixture(t, newTestRunFolder(t, root, damagedRunID), "frontmatter_intact.md")

	result := scanOne(t, root)

	if len(result.Unresumable) != 0 {
		t.Fatalf("Unresumable = %+v, want none: a damaged run is not refused", result.Unresumable)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %d, want the damaged run listed", len(result.Candidates))
	}
	c := result.Candidates[0]
	if !c.Damaged {
		t.Error("Damaged = false, want true for a run whose artifact is damaged")
	}
	var damage *domain.ArtifactDamageError
	if !errors.As(c.ParseError, &damage) {
		t.Errorf("ParseError = %v (%T), want it to carry the *domain.ArtifactDamageError", c.ParseError, c.ParseError)
	}
	if c.RunID != damagedRunID {
		t.Errorf("RunID = %q, want %q", c.RunID, damagedRunID)
	}
}

func TestScan_DamagedArtifactWithIntactFrontmatter_CarriesTheRecordedFacts(t *testing.T) {
	root := t.TempDir()
	writeDamageFixture(t, newTestRunFolder(t, root, damagedRunID), "frontmatter_intact.md")

	result := scanOne(t, root)

	if len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %d, want 1", len(result.Candidates))
	}
	c := result.Candidates[0]
	if c.Workflow != "test-workflow" || c.Task != "test task" {
		t.Errorf("Workflow/Task = %q/%q, want test-workflow/test task from the frontmatter", c.Workflow, c.Task)
	}
	if c.Phase != "EXECUTION" || c.LastAgent != "agent#2" {
		t.Errorf("Phase/LastAgent = %q/%q, want EXECUTION/agent#2 from the frontmatter", c.Phase, c.LastAgent)
	}
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !c.LastUpdated.Equal(want) {
		t.Errorf("LastUpdated = %v, want %v from the frontmatter", c.LastUpdated, want)
	}
	if !c.RunnerSettingsRecorded {
		t.Error("RunnerSettingsRecorded = false, want true: the frontmatter records the runner settings")
	}
}

func TestScan_DamagedArtifactRecordingNoRunnerSettings_IsNotMarkedAsRecorded(t *testing.T) {
	root := t.TempDir()
	writeDamageFixture(t, newTestRunFolder(t, root, damagedRunID), "no_runner_settings.md")

	result := scanOne(t, root)

	if len(result.Candidates) != 1 || !result.Candidates[0].Damaged {
		t.Fatalf("Candidates = %+v, want the damaged run listed and marked damaged", result.Candidates)
	}
	if result.Candidates[0].RunnerSettingsRecorded {
		t.Error("RunnerSettingsRecorded = true, want false for a frontmatter without runner settings")
	}
}

func TestScan_DamagedArtifactEnablingCommitsWithoutBranch_ReportsCommitSetupPending(t *testing.T) {
	root := t.TempDir()
	writeDamageFixture(t, newTestRunFolder(t, root, damagedRunID), "commit_setup_pending.md")

	result := scanOne(t, root)

	if len(result.Candidates) != 1 || !result.Candidates[0].Damaged {
		t.Fatalf("Candidates = %+v, want the damaged run listed and marked damaged", result.Candidates)
	}
	if !result.Candidates[0].CommitSetupPending {
		t.Error("CommitSetupPending = false, want true from the frontmatter (commits enabled, no branch)")
	}
}

func TestScan_DamagedFrontmatterWithoutLastGoodCopy_IsUnresumableWithFolderFactsOnly(t *testing.T) {
	root := t.TempDir()
	folder := newTestRunFolder(t, root, damagedRunID)
	writeDamageFixture(t, folder, "frontmatter_broken.md")

	result := scanOne(t, root)

	if len(result.Candidates) != 0 || len(result.Unresumable) != 1 {
		t.Fatalf("Candidates = %d, Unresumable = %d, want the run listed as unresumable",
			len(result.Candidates), len(result.Unresumable))
	}
	u := result.Unresumable[0]
	if u.Reason != runscan.ReasonDamagedFrontmatter {
		t.Errorf("Reason = %q, want %q", u.Reason, runscan.ReasonDamagedFrontmatter)
	}
	if !u.Damaged {
		t.Error("Damaged = false, want true for a run whose frontmatter is damaged")
	}
	if u.RunID != damagedRunID || u.FolderPath != folder {
		t.Errorf("RunID/FolderPath = %q/%q, want %q/%q from the folder", u.RunID, u.FolderPath, damagedRunID, folder)
	}
	if u.Workflow != "" || !u.LastUpdated.IsZero() {
		t.Errorf("Workflow/LastUpdated = %q/%v, want none: the frontmatter cannot be read", u.Workflow, u.LastUpdated)
	}
}

func TestScan_UndamagedArtifact_IsNotMarkedDamaged(t *testing.T) {
	root := t.TempDir()
	writeArtifact(t, newTestRunFolder(t, root, runID1), "EXECUTION", t1)

	result := scanOne(t, root)

	if len(result.Candidates) != 1 || result.Candidates[0].Damaged {
		t.Fatalf("Candidates = %+v, want one candidate that is not marked damaged", result.Candidates)
	}
}

func TestScan_MissingArtifact_IsNotMarkedDamaged(t *testing.T) {
	root := t.TempDir()
	newTestRunFolder(t, root, runID1)

	result := scanOne(t, root)

	if len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %d, want the run listed", len(result.Candidates))
	}
	c := result.Candidates[0]
	if c.Damaged {
		t.Error("Damaged = true for a missing artifact, want a damage mark only for damage")
	}
	if c.ParseError == nil {
		t.Error("ParseError = nil, want the read error")
	}
}

func TestScan_UnreadableArtifact_IsNotMarkedDamaged(t *testing.T) {
	root := t.TempDir()
	folder := newTestRunFolder(t, root, runID1)
	// A directory where the file should be: reading it fails, whatever the platform.
	if err := os.Mkdir(filepath.Join(folder, "Orchestration.md"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	result := scanOne(t, root)

	if len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %d, want the run listed", len(result.Candidates))
	}
	if result.Candidates[0].Damaged {
		t.Error("Damaged = true for an unreadable artifact, want a damage mark only for damage")
	}
}

func TestScan_Pre30LayoutWithDamage_KeepsItsExistingClassification(t *testing.T) {
	root := t.TempDir()
	writeDamageFixture(t, newTestRunFolder(t, root, damagedRunID), "pre30_layout_and_damage.md")

	result := scanOne(t, root)

	if len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %d, want the run listed as before", len(result.Candidates))
	}
	if result.Candidates[0].Damaged {
		t.Error("Damaged = true for the pre-3.0 layout, want the layout refusal kept distinct from damage")
	}
}

func TestScan_DamagedArtifactWithMismatchedRunID_IsRefusedForItsIdentity(t *testing.T) {
	root := t.TempDir()
	// The folder carries another run_id than the artifact's frontmatter.
	writeDamageFixture(t, newTestRunFolder(t, root, runID1), "frontmatter_intact.md")

	result := scanOne(t, root)

	if len(result.Candidates) != 0 {
		t.Fatalf("Candidates = %+v, want none: a folder mismatch is refused", result.Candidates)
	}
	if len(result.Unresumable) != 1 || result.Unresumable[0].Reason != runscan.ReasonInvalidRunIdentity {
		t.Fatalf("Unresumable = %+v, want one run refused for its run identity", result.Unresumable)
	}
}

func TestScan_DamagedArtifactRecordedAsCompleted_IsNotOffered(t *testing.T) {
	root := t.TempDir()
	writeDamageFixture(t, newTestRunFolder(t, root, damagedRunID), "frontmatter_intact.md",
		"phase: EXECUTION", "phase: COMPLETED")

	result := scanOne(t, root)

	if len(result.Candidates) != 0 {
		t.Fatalf("Candidates = %+v, want none for a run the frontmatter records as COMPLETED", result.Candidates)
	}
	if len(result.Unresumable) != 1 || result.Unresumable[0].Reason != runscan.ReasonCompleted {
		t.Fatalf("Unresumable = %+v, want one completed run", result.Unresumable)
	}
}

func TestScan_DamagedRunOrdersByItsRecordedLastUpdated(t *testing.T) {
	root := t.TempDir()
	// The damaged run was last updated 2026-01-01; the healthy one two days before.
	writeDamageFixture(t, newTestRunFolder(t, root, damagedRunID), "frontmatter_intact.md")
	writeArtifact(t, newTestRunFolder(t, root, runID2), "EXECUTION", t1.Add(-48*time.Hour))

	result := scanOne(t, root)

	if len(result.Candidates) != 2 {
		t.Fatalf("Candidates = %d, want 2", len(result.Candidates))
	}
	if result.Candidates[0].RunID != damagedRunID || result.Candidates[1].RunID != runID2 {
		t.Errorf("order = %s, %s; want the damaged run first by its recorded last_updated",
			result.Candidates[0].RunID, result.Candidates[1].RunID)
	}
}
