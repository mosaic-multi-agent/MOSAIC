package tui

// Guard: the orchestrator fixture content used by the setup-screen tests is
// valid WorkflowFormat 2.16 under the strict parser.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/workflow"
)

func TestOrchestratorFixtures_WorkflowRegionsAreAcceptedByTheStrictParser(t *testing.T) {
	harnessPath := filepath.Join(t.TempDir(), "orchestrator-script.md")
	if err := os.WriteFile(harnessPath, []byte(navCoreOrchestratorContent), 0o600); err != nil {
		t.Fatalf("write harness fixture: %v", err)
	}
	fixtures := map[string]string{
		"resume fixture":  writeOrchestratorFixture(t),
		"harness fixture": harnessPath,
	}

	for name, path := range fixtures {
		regions, err := orchfile.EnumerateWorkflows(path)
		if err != nil {
			t.Errorf("%s: EnumerateWorkflows: %v", name, err)
			continue
		}
		if len(regions) == 0 {
			t.Errorf("%s: holds no workflow region; the guard would pass vacuously", name)
		}
		for _, region := range regions {
			if _, err := workflow.Parse(region.Content, region.Info); err != nil {
				t.Errorf("%s: workflow %q is not valid WorkflowFormat 2.16: %v", name, region.Info.ID, err)
			}
		}
	}
}
