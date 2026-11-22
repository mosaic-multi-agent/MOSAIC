package harness_test

// Tests for DiscoverOrchestrator building the orchestrator-script file name
// from the harness's agent file extension. A harness without a catalog entry
// gets format data only through the test-only seam; without it the harness is
// refused as unknown.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/agentfile"
	"mosaic-run/internal/harness"
)

const seamAgentsDir = ".seam/agents"

// seamWorkspace registers format data for a harness without a catalog entry and
// returns a workspace whose agents directory holds the named files.
func seamWorkspace(t *testing.T, harnessID string, f agentfile.Format, files ...string) string {
	t.Helper()
	unregister := agentfile.RegisterTestOnly(harnessID, f)
	t.Cleanup(unregister)
	workDir := t.TempDir()
	dir := filepath.Join(workDir, filepath.FromSlash(seamAgentsDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seamWorkspace: mkdir: %v", err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("placeholder\n"), 0o644); err != nil {
			t.Fatalf("seamWorkspace: write %s: %v", name, err)
		}
	}
	return workDir
}

func TestDiscoverOrchestrator_NonMarkdownExtension_ReturnsExtensionFile(t *testing.T) {
	f := agentfile.Format{AgentsDir: seamAgentsDir, FormatID: "mosaic-test-json", Extension: ".json"}
	workDir := seamWorkspace(t, "seam-discovery-json", f, "orchestrator-script.json")

	got, err := harness.DiscoverOrchestrator(workDir, "seam-discovery-json")

	if err != nil {
		t.Fatalf("DiscoverOrchestrator: %v", err)
	}
	want := filepath.Join(workDir, filepath.FromSlash(seamAgentsDir), "orchestrator-script.json")
	if got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestDiscoverOrchestrator_NonMarkdownExtension_IgnoresMarkdownFile(t *testing.T) {
	f := agentfile.Format{AgentsDir: seamAgentsDir, FormatID: "mosaic-test-json", Extension: ".json"}
	workDir := seamWorkspace(t, "seam-discovery-ignore", f, "orchestrator-script.json", "orchestrator-script.md")

	got, err := harness.DiscoverOrchestrator(workDir, "seam-discovery-ignore")

	if err != nil {
		t.Fatalf("DiscoverOrchestrator: want the .md file ignored, not ambiguous: %v", err)
	}
	if filepath.Base(got) != "orchestrator-script.json" {
		t.Errorf("path = %q, want the .json file", got)
	}
}

func TestDiscoverOrchestrator_NonMarkdownExtension_OnlyMarkdownFile_RefusesAsNotDeployed(t *testing.T) {
	f := agentfile.Format{AgentsDir: seamAgentsDir, FormatID: "mosaic-test-json", Extension: ".json"}
	workDir := seamWorkspace(t, "seam-discovery-notdeployed", f, "orchestrator-script.md")

	_, err := harness.DiscoverOrchestrator(workDir, "seam-discovery-notdeployed")

	re := asRefusalError(t, err)
	if re.Component != "harness" {
		t.Errorf("Component = %q, want harness", re.Component)
	}
	if !strings.Contains(err.Error(), "orchestrator-script.json") {
		t.Errorf("refusal %q does not name the expected .json file", err.Error())
	}
	if strings.Contains(err.Error(), "unknown harness") {
		t.Errorf("a harness with format data is not an unknown harness: %v", err)
	}
}

func TestDiscoverOrchestrator_MarkdownWithOtherExtension_ReturnsThatExtension(t *testing.T) {
	f := agentfile.Format{AgentsDir: seamAgentsDir, FormatID: "markdown", Extension: ".mdx"}
	workDir := seamWorkspace(t, "seam-discovery-mdx", f, "orchestrator-script.mdx")

	got, err := harness.DiscoverOrchestrator(workDir, "seam-discovery-mdx")

	if err != nil {
		t.Fatalf("DiscoverOrchestrator: %v", err)
	}
	if filepath.Base(got) != "orchestrator-script.mdx" {
		t.Errorf("path = %q, want the .mdx file", got)
	}
}

func TestDiscoverOrchestrator_MarkdownWithOtherExtension_BothCandidates_RefusesAsAmbiguous(t *testing.T) {
	f := agentfile.Format{AgentsDir: seamAgentsDir, FormatID: "markdown", Extension: ".mdx"}
	workDir := seamWorkspace(t, "seam-discovery-ambiguous", f, "orchestrator-script.md", "orchestrator-script.mdx")

	_, err := harness.DiscoverOrchestrator(workDir, "seam-discovery-ambiguous")

	re := asRefusalError(t, err)
	if re.Component != "harness" {
		t.Errorf("Component = %q, want harness", re.Component)
	}
	for _, name := range []string{"orchestrator-script.md", "orchestrator-script.mdx"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("ambiguity refusal %q does not list candidate %s", err.Error(), name)
		}
	}
}

func TestDiscoverOrchestrator_NonMarkdownExtension_NoFile_RefusesAsNotDeployed(t *testing.T) {
	f := agentfile.Format{AgentsDir: seamAgentsDir, FormatID: "mosaic-test-json", Extension: ".json"}
	workDir := seamWorkspace(t, "seam-discovery-empty", f)

	_, err := harness.DiscoverOrchestrator(workDir, "seam-discovery-empty")

	asRefusalError(t, err)
	if !strings.Contains(err.Error(), "orchestrator-script.json") {
		t.Errorf("refusal %q does not name the expected .json file", err.Error())
	}
}

func TestDiscoverOrchestrator_HarnessWithoutFormatData_StillRefusedAsUnknown(t *testing.T) {
	workDir := t.TempDir()

	_, err := harness.DiscoverOrchestrator(workDir, "harness-without-format-data")

	re := asRefusalError(t, err)
	if re.Component != "harness" || re.Resource != "harness-without-format-data" {
		t.Errorf("refusal = %+v, want Component harness and Resource the harness id", re)
	}
}

func TestDiscoverOrchestrator_SeamHarness_RefusedAgainAfterUnregister(t *testing.T) {
	f := agentfile.Format{AgentsDir: seamAgentsDir, FormatID: "mosaic-test-json", Extension: ".json"}
	unregister := agentfile.RegisterTestOnly("seam-discovery-lifecycle", f)
	workDir := t.TempDir()
	dir := filepath.Join(workDir, filepath.FromSlash(seamAgentsDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orchestrator-script.json"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errWhileRegistered := harness.DiscoverOrchestrator(workDir, "seam-discovery-lifecycle")
	unregister()

	_, errAfter := harness.DiscoverOrchestrator(workDir, "seam-discovery-lifecycle")

	if errWhileRegistered != nil {
		t.Errorf("want discovery to succeed while registered, got %v", errWhileRegistered)
	}
	if errAfter == nil {
		t.Error("want the harness refused as unknown once the seam registration is removed")
	}
}
