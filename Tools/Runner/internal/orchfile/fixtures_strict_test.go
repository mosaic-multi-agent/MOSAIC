package orchfile_test

// Guard: every workflow region in the orchfile testdata fixtures is valid
// WorkflowFormat 2.16 under the strict parser. The transitional entry point
// that falls back to 1.x parsing is deliberately not used here, so a fixture
// written in 1.x notation fails this test instead of passing silently.

import (
	"os"
	"path/filepath"
	"testing"

	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/workflow"
)

// deliberatelyInvalidFixtures are the fixtures a test uses to provoke a
// refusal of the orchestrator file itself (or that carry no workflow region),
// so they cannot be enumerated into regions. Excluded by file name.
var deliberatelyInvalidFixtures = map[string]string{
	"duplicate-ids.md":   "two regions share one identifier",
	"empty-id.md":        "region with an empty name attribute",
	"missing-version.md": "region without a version attribute",
	"no-workflows.md":    "agent file without any workflow region",
}

func TestFixtures_EveryWorkflowRegionIsAcceptedByTheStrictParser(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(orchfileTestdataDir, "*.md"))
	if err != nil {
		t.Fatalf("glob orchfile fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no fixtures found under %s", orchfileTestdataDir)
	}

	checkedRegions := 0
	for _, path := range paths {
		name := filepath.Base(path)
		if _, excluded := deliberatelyInvalidFixtures[name]; excluded {
			continue
		}
		regions, err := orchfile.EnumerateWorkflows(path)
		if err != nil {
			t.Errorf("%s: EnumerateWorkflows: %v", name, err)
			continue
		}
		for _, region := range regions {
			checkedRegions++
			if _, err := workflow.Parse(region.Content, region.Info); err != nil {
				t.Errorf("%s: workflow %q is not valid WorkflowFormat 2.16: %v", name, region.Info.ID, err)
			}
		}
	}
	if checkedRegions == 0 {
		t.Error("no workflow region was checked; the guard would pass vacuously")
	}
}

func TestFixtures_ExcludedFixturesExist(t *testing.T) {
	// An exclusion for a fixture that no longer exists would hide a rename.
	for name := range deliberatelyInvalidFixtures {
		if _, err := os.Stat(filepath.Join(orchfileTestdataDir, name)); err != nil {
			t.Errorf("excluded fixture %s: %v", name, err)
		}
	}
}
