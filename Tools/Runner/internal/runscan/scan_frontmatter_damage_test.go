package runscan_test

// A run whose frontmatter is damaged is listed from the recorded values of its
// last-good copy when that copy is usable: a resumable candidate marked damaged.
// Without a usable copy it is listed as unresumable with its own reason, which
// names the damage. The copy lives where the Runner keeps run evidence:
// {root}/RunnerLogs/{run_id}/Orchestration.last-good.md.

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

// writeLastGoodFixture copies the named fixture under testdata/damaged to the
// run's last-good copy, applying each old->new replacement pair first.
func writeLastGoodFixture(t *testing.T, root, runID, name string, replacements ...string) {
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
	dir := filepath.Join(root, "RunnerLogs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Orchestration.last-good.md"), []byte(text), 0o600); err != nil {
		t.Fatalf("write last-good copy: %v", err)
	}
}

// damagedFrontmatterRun lays out a run whose frontmatter is damaged and returns
// the workspace root and the run folder.
func damagedFrontmatterRun(t *testing.T) (root, folder string) {
	t.Helper()
	root = t.TempDir()
	folder = newTestRunFolder(t, root, damagedRunID)
	writeDamageFixture(t, folder, "frontmatter_broken.md")
	return root, folder
}

func TestScan_DamagedFrontmatterWithUsableLastGoodCopy_IsAResumableCandidateMarkedDamaged(t *testing.T) {
	root, folder := damagedFrontmatterRun(t)
	writeLastGoodFixture(t, root, damagedRunID, "frontmatter_intact.md")

	result := scanOne(t, root)

	if len(result.Unresumable) != 0 || len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %d, Unresumable = %d, want the run listed as a candidate",
			len(result.Candidates), len(result.Unresumable))
	}
	c := result.Candidates[0]
	if !c.Damaged {
		t.Error("Damaged = false, want true for a run whose frontmatter is damaged")
	}
	var damage *domain.ArtifactDamageError
	if !errors.As(c.ParseError, &damage) {
		t.Errorf("ParseError = %v (%T), want the *domain.ArtifactDamageError", c.ParseError, c.ParseError)
	}
	if c.RunID != damagedRunID || c.FolderPath != folder {
		t.Errorf("RunID/FolderPath = %q/%q, want %q/%q", c.RunID, c.FolderPath, damagedRunID, folder)
	}
}

func TestScan_DamagedFrontmatterWithUsableLastGoodCopy_CarriesTheCopysValues(t *testing.T) {
	root, _ := damagedFrontmatterRun(t)
	// The task differs from anything the damaged artifact itself could supply.
	writeLastGoodFixture(t, root, damagedRunID, "frontmatter_intact.md",
		`task: "test task"`, `task: "copy task"`)

	result := scanOne(t, root)

	if len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %d, want 1", len(result.Candidates))
	}
	c := result.Candidates[0]
	if c.Workflow != "test-workflow" || c.Task != "copy task" {
		t.Errorf("Workflow/Task = %q/%q, want test-workflow/copy task from the last-good copy", c.Workflow, c.Task)
	}
	if c.Phase != "EXECUTION" || c.LastAgent != "agent#2" {
		t.Errorf("Phase/LastAgent = %q/%q, want EXECUTION/agent#2 from the last-good copy", c.Phase, c.LastAgent)
	}
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !c.LastUpdated.Equal(want) {
		t.Errorf("LastUpdated = %v, want %v from the last-good copy", c.LastUpdated, want)
	}
	if !c.RunnerSettingsRecorded {
		t.Error("RunnerSettingsRecorded = false, want true: the copy records the runner settings")
	}
}

func TestScan_DamagedFrontmatterWithUsableLastGoodCopy_ReportsTheCommitSetupTheCopyLeavesPending(t *testing.T) {
	root, _ := damagedFrontmatterRun(t)
	writeLastGoodFixture(t, root, damagedRunID, "commit_setup_pending.md")

	result := scanOne(t, root)

	if len(result.Candidates) != 1 || !result.Candidates[0].CommitSetupPending {
		t.Fatalf("Candidates = %+v, want one candidate with CommitSetupPending from the copy", result.Candidates)
	}
}

func TestScan_DamagedFrontmatterWithLastGoodCopyRecordedAsCompleted_IsNotOffered(t *testing.T) {
	root, _ := damagedFrontmatterRun(t)
	writeLastGoodFixture(t, root, damagedRunID, "frontmatter_intact.md", "phase: EXECUTION", "phase: COMPLETED")

	result := scanOne(t, root)

	if len(result.Candidates) != 0 {
		t.Fatalf("Candidates = %+v, want none for a run the last-good copy records as COMPLETED", result.Candidates)
	}
	if len(result.Unresumable) != 1 || result.Unresumable[0].Reason != runscan.ReasonCompleted {
		t.Fatalf("Unresumable = %+v, want one completed run", result.Unresumable)
	}
}

func TestScan_DamagedFrontmatterWithUnusableLastGoodCopy_IsUnresumable(t *testing.T) {
	tests := []struct {
		name         string
		fixture      string
		replacements []string
	}{
		{"copy of another run", "frontmatter_intact.md",
			[]string{"run_id: " + damagedRunID, "run_id: 20260101T000000Z-abcd"}},
		{"copy with damaged frontmatter", "frontmatter_broken.md", nil},
		{"copy in the pre-3.0 layout", "pre30_layout_and_damage.md", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := damagedFrontmatterRun(t)
			writeLastGoodFixture(t, root, damagedRunID, tc.fixture, tc.replacements...)

			result := scanOne(t, root)

			if len(result.Candidates) != 0 || len(result.Unresumable) != 1 {
				t.Fatalf("Candidates = %d, Unresumable = %d, want the run listed as unresumable",
					len(result.Candidates), len(result.Unresumable))
			}
			if got := result.Unresumable[0].Reason; got != runscan.ReasonDamagedFrontmatter {
				t.Errorf("Reason = %q, want %q", got, runscan.ReasonDamagedFrontmatter)
			}
		})
	}
}

func TestScan_DamagedFrontmatterWithoutLastGoodCopy_DetailNamesTheDamage(t *testing.T) {
	root, _ := damagedFrontmatterRun(t)

	result := scanOne(t, root)

	if len(result.Unresumable) != 1 {
		t.Fatalf("Unresumable = %d, want 1", len(result.Unresumable))
	}
	if !strings.Contains(strings.ToLower(result.Unresumable[0].Detail), "damaged") {
		t.Errorf("Detail = %q, want the damage error's text naming the damage", result.Unresumable[0].Detail)
	}
}

func TestScan_DamagedBodyWithLastGoodCopyRecordingAnotherWorkflow_UsesTheArtifactsOwnValues(t *testing.T) {
	root := t.TempDir()
	writeDamageFixture(t, newTestRunFolder(t, root, damagedRunID), "frontmatter_intact.md")
	writeLastGoodFixture(t, root, damagedRunID, "frontmatter_intact.md", "workflow: test-workflow", "workflow: copy-workflow")

	result := scanOne(t, root)

	if len(result.Candidates) != 1 || result.Candidates[0].Workflow != "test-workflow" {
		t.Fatalf("Candidates = %+v, want the artifact's own intact frontmatter preferred over the copy", result.Candidates)
	}
}

func TestUnresumableReason_DamagedFrontmatter_DescriptionSaysDamaged(t *testing.T) {
	got := runscan.ReasonDamagedFrontmatter.Description()

	if !strings.Contains(strings.ToLower(got), "damaged") {
		t.Errorf("Description() = %q, want a phrase naming the damage", got)
	}
}
