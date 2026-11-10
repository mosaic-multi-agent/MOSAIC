package agentresolve_test

// Guard: the orchestrator fixture content used by this package's tests is
// valid WorkflowFormat 2.16 under the strict parser.

import (
	"testing"

	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/workflow"
)

func TestOrchestratorFixture_WorkflowRegionIsAcceptedByTheStrictParser(t *testing.T) {
	path := writeOrchFile(t, t.TempDir(), "guard-orchestrator.md")

	regions, err := orchfile.EnumerateWorkflows(path)
	if err != nil {
		t.Fatalf("EnumerateWorkflows: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("fixture holds no workflow region; the guard would pass vacuously")
	}
	for _, region := range regions {
		if _, err := workflow.Parse(region.Content, region.Info); err != nil {
			t.Errorf("workflow %q is not valid WorkflowFormat 2.16: %v", region.Info.ID, err)
		}
	}
}
