package main

// Run identity for a run whose Orchestration.md is damaged, on both entry
// points. The run is neither hidden nor refused: whatever the intact
// frontmatter records is carried on, and the session repairs the artifact
// when the run starts.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// damagedRunID is the run_id the damage fixtures record.
const damagedRunID = "20260727T170000Z-a3f9"

// writeDamagedRunFolder lays the named fixture under testdata/damaged out as
// the artifact of run damagedRunID in a new workspace and returns the
// workspace and the run folder.
func writeDamagedRunFolder(t *testing.T, fixture string, replacements ...string) (workDir, runFolder string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "damaged", fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	text := string(data)
	for i := 0; i+1 < len(replacements); i += 2 {
		if !strings.Contains(text, replacements[i]) {
			t.Fatalf("fixture %s does not contain %q", fixture, replacements[i])
		}
		text = strings.Replace(text, replacements[i], replacements[i+1], 1)
	}
	workDir = t.TempDir()
	runFolder = filepath.Join(workDir, domain.RunScopedFolder(damagedRunID))
	if err := os.MkdirAll(runFolder, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", runFolder, err)
	}
	if err := os.WriteFile(filepath.Join(runFolder, "Orchestration.md"), []byte(text), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	return workDir, runFolder
}

// ---- CLI entry point ----

func TestResolveRunIdentityForCLI_NamedRunWithDamagedArtifact_IsNotRefused(t *testing.T) {
	workDir, runFolder := writeDamagedRunFolder(t, "frontmatter_intact.md")

	identity, _, err := resolveRunIdentityForCLI([]string{"run", "--run", damagedRunID}, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForCLI error = %v, want nil: a damaged run stays resumable", err)
	}
	if identity == nil || identity.RunID != damagedRunID || identity.RunFolder != runFolder || identity.IsNewRun {
		t.Fatalf("identity = %+v, want the damaged run resumed from %s", identity, runFolder)
	}
}

func TestResolveRunIdentityForCLI_NamedRunWithDamagedArtifact_CarriesThePositionTheFrontmatterRecords(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "frontmatter_intact.md")

	identity, _, err := resolveRunIdentityForCLI([]string{"run", "--run", damagedRunID}, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForCLI error = %v, want nil", err)
	}
	if identity.Position == nil {
		t.Fatal("Position = nil, want the position from the intact frontmatter")
	}
	if identity.Position.Phase != "EXECUTION" || identity.Position.LastAgent != "agent#2" {
		t.Errorf("Position phase/last agent = %q/%q, want EXECUTION/agent#2", identity.Position.Phase, identity.Position.LastAgent)
	}
}

func TestResolveRunIdentityForCLI_NamedRunWithDamagedFrontmatterAndNoLastGoodCopy_IsNotRefused(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "frontmatter_broken.md")

	identity, _, err := resolveRunIdentityForCLI([]string{"run", "--run", damagedRunID}, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForCLI error = %v, want nil: the session reports the damage", err)
	}
	if identity == nil || identity.RunID != damagedRunID {
		t.Fatalf("identity = %+v, want the run resumed", identity)
	}
	if identity.Position != nil {
		t.Errorf("Position = %+v, want none: the frontmatter cannot be read", identity.Position)
	}
}

func TestResolveRunIdentityForCLI_NamedRunWithPre30Layout_KeepsItsHandling(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "pre30_layout_and_damage.md")

	identity, _, err := resolveRunIdentityForCLI([]string{"run", "--run", damagedRunID}, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForCLI error = %v, want nil: the layout refusal is the preflight's", err)
	}
	if identity == nil || identity.Position != nil {
		t.Errorf("identity = %+v, want the run resumed with no position read from a pre-3.0 artifact", identity)
	}
}

// ---- TUI entry point ----

func TestResolveRunIdentityForTUI_NamedRunWithDamagedArtifact_CarriesTheRecordedWorkflow(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "frontmatter_intact.md")

	identity, err := resolveRunIdentityForTUI([]string{"--run", damagedRunID}, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForTUI error = %v, want nil", err)
	}
	if identity.Workflow != "test-workflow" {
		t.Errorf("Workflow = %q, want the workflow the intact frontmatter records: without it the "+
			"damaged run would be refused for recording none instead of being repaired", identity.Workflow)
	}
}

func TestResolveRunIdentityForTUI_NamedRunWithDamagedFrontmatterAndNoLastGoodCopy_CarriesNoWorkflow(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "frontmatter_broken.md")

	identity, err := resolveRunIdentityForTUI([]string{"--run", damagedRunID}, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForTUI error = %v, want nil: the session refuses with the damage", err)
	}
	if identity.Workflow != "" {
		t.Errorf("Workflow = %q, want none: the frontmatter cannot be read", identity.Workflow)
	}
	if identity.RunID != damagedRunID || identity.IsNewRun {
		t.Errorf("identity = %+v, want the damaged run kept as a resume", identity)
	}
}

func TestResolveRunIdentityForTUI_DeferredScan_OffersTheDamagedRunMarkedDamaged(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "frontmatter_intact.md")

	identity, err := resolveRunIdentityForTUI(nil, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForTUI error = %v, want nil", err)
	}
	if identity.Selection == nil {
		t.Fatal("Selection = nil, want the run-select question")
	}
	for _, c := range identity.Selection.Choices {
		if c.ID != damagedRunID {
			continue
		}
		if !c.Selectable {
			t.Error("the damaged run is not selectable, want it offered for resumption")
		}
		if !c.Run.Damaged {
			t.Error("Run.Damaged = false, want the damaged run marked damaged")
		}
		return
	}
	t.Fatalf("the damaged run is missing from the choices: %+v", identity.Selection.Choices)
}
