package main

// Run identity for a run whose Orchestration.md has damaged frontmatter, on
// both entry points. With a usable last-good copy the identity carries what the
// copy records, so the session can repair the run at start. A deferred run
// selection offers such a run as resumable, and one without a usable copy as
// unresumable.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/runscan"
)

// writeLastGoodCopyOf places the named fixture under testdata/damaged as the
// last-good copy of run damagedRunID in workDir.
func writeLastGoodCopyOf(t *testing.T, workDir, fixture string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "damaged", fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	dir := filepath.Join(workDir, "RunnerLogs", damagedRunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Orchestration.last-good.md"), data, 0o600); err != nil {
		t.Fatalf("write last-good copy: %v", err)
	}
}

func TestResolveRunIdentityForCLI_NamedRunWithDamagedFrontmatterAndUsableLastGoodCopy_CarriesTheCopysPosition(t *testing.T) {
	workDir, runFolder := writeDamagedRunFolder(t, "frontmatter_broken.md")
	writeLastGoodCopyOf(t, workDir, "frontmatter_intact.md")

	identity, _, err := resolveRunIdentityForCLI([]string{"run", "--run", damagedRunID}, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForCLI error = %v, want nil", err)
	}
	if identity == nil || identity.RunID != damagedRunID || identity.RunFolder != runFolder || identity.IsNewRun {
		t.Fatalf("identity = %+v, want the damaged run resumed from %s", identity, runFolder)
	}
	if identity.Position == nil {
		t.Fatal("Position = nil, want the position the last-good copy records")
	}
	if identity.Position.Phase != "EXECUTION" || identity.Position.LastAgent != "agent#2" {
		t.Errorf("Position phase/last agent = %q/%q, want EXECUTION/agent#2 from the copy",
			identity.Position.Phase, identity.Position.LastAgent)
	}
}

func TestResolveRunIdentityForTUI_NamedRunWithDamagedFrontmatterAndUsableLastGoodCopy_CarriesTheCopysWorkflow(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "frontmatter_broken.md")
	writeLastGoodCopyOf(t, workDir, "frontmatter_intact.md")

	identity, err := resolveRunIdentityForTUI([]string{"--run", damagedRunID}, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForTUI error = %v, want nil", err)
	}
	if identity.Workflow != "test-workflow" {
		t.Errorf("Workflow = %q, want the workflow the last-good copy records: without it the run "+
			"would be refused for recording none instead of being repaired", identity.Workflow)
	}
	if identity.RunID != damagedRunID || identity.IsNewRun {
		t.Errorf("identity = %+v, want the damaged run kept as a resume", identity)
	}
}

func TestResolveRunIdentityForTUI_DeferredScan_OffersADamagedFrontmatterRunWithALastGoodCopyAsResumable(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "frontmatter_broken.md")
	writeLastGoodCopyOf(t, workDir, "frontmatter_intact.md")

	identity, err := resolveRunIdentityForTUI(nil, workDir)

	if err != nil {
		t.Fatalf("resolveRunIdentityForTUI error = %v, want nil", err)
	}
	if identity.Selection == nil {
		t.Fatal("Selection = nil, want the run-select question: the run is resumable")
	}
	for _, c := range identity.Selection.Choices {
		if c.ID != damagedRunID {
			continue
		}
		if !c.Selectable {
			t.Error("the damaged run is not selectable, want it offered for resumption")
		}
		if !c.Run.Damaged || c.Run.Workflow != "test-workflow" {
			t.Errorf("Run damaged/workflow = %v/%q, want it marked damaged with the copy's workflow", c.Run.Damaged, c.Run.Workflow)
		}
		return
	}
	t.Fatalf("the damaged run is missing from the choices: %+v", identity.Selection.Choices)
}

func TestResolveRunIdentityForTUI_DeferredScan_ListsADamagedFrontmatterRunWithoutALastGoodCopyAsUnresumable(t *testing.T) {
	workDir, _ := writeDamagedRunFolder(t, "frontmatter_broken.md")
	// A second, healthy run keeps the selection screen in play.
	healthy := filepath.Join(workDir, "Orchestration-20260801T090000Z-bbbb")
	if err := os.MkdirAll(healthy, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "damaged", "no_runner_settings.md"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	text := string(data)
	text = strings.Replace(text, "run_id: "+damagedRunID, "run_id: 20260801T090000Z-bbbb", 1)
	if err := os.WriteFile(filepath.Join(healthy, "Orchestration.md"), []byte(text), 0o600); err != nil {
		t.Fatalf("write healthy artifact: %v", err)
	}

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
		if c.Selectable {
			t.Error("the run without a usable last-good copy is selectable, want it unresumable")
		}
		if c.Reason != runscan.ReasonDamagedFrontmatter || !c.Run.Damaged {
			t.Errorf("Reason/Damaged = %q/%v, want the damaged-frontmatter reason and the damage mark", c.Reason, c.Run.Damaged)
		}
		return
	}
	t.Fatalf("the damaged run is missing from the choices: %+v", identity.Selection.Choices)
}
