package plan_test

// build_kept_test.go verifies that Build treats kept workflow ids as deployed rather than
// drifted, while still reporting drift for catalog workflows.

import (
	"context"
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/plan"
)

const (
	keptOrchPath = "agents/orchestrator.md"
	keptOrchHash = "sha256:keptorch01"
)

// keptBuildInput returns a Build input for an up-to-date orchestrator whose deployed file
// carries the given workflows. The catalog supplies only "quick-fix" (version 1.0).
func keptBuildInput(deployed domain.DeployedWorkflows, workflowIDs, keptIDs []string) plan.Input {
	entry := makeManifestEntry(agentRef("orchestrator"), keptOrchPath, "1.0", keptOrchHash)
	entry.HarnessVersion = "1.0"
	entry.InjectionsVersion = "1.0"
	snap := presentSnapshot(domain.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		HarnessID:     "test-harness",
		UpdatedAt:     time.Now(),
		Entries:       []domain.ManifestEntry{entry},
	})
	return plan.Input{
		Catalog: &fakeCatalog{
			orchestrator: makeOrchestrator(),
			workflows:    []domain.Workflow{makeWorkflow("quick-fix", "orchestrator")},
		},
		Module:          newFakeModule(),
		Mode:            domain.ModeUpdateWorkspace,
		WorkspacePath:   "/fake/workspace",
		Scope:           domain.ScopeProject,
		GOOS:            "linux",
		Manifest:        snap,
		WorkflowIDs:     workflowIDs,
		KeptWorkflowIDs: keptIDs,
		Models: map[string]domain.ModelSelection{
			"orchestrator": {ModelID: "test-model", Origin: domain.OriginHarnessList},
		},
		DeployedState: map[string]domain.DeployedArtifactState{
			keptOrchPath: {
				Present:           true,
				ContentHash:       keptOrchHash,
				Version:           "1.0",
				HarnessVersion:    "1.0",
				InjectionsVersion: "1.0",
				Workflows:         deployed,
			},
		},
	}
}

func buildOrchestratorItem(t *testing.T, in plan.Input) domain.PlanItem {
	t.Helper()
	p, err := plan.New().Build(context.Background(), in)
	must(t, err)
	item, ok := findItem(p.Items, "orchestrator")
	if !ok {
		t.Fatal("plan has no item for orchestrator")
	}
	return item
}

func hasWorkflowDelta(item domain.PlanItem, id string) bool {
	for _, d := range item.Stale {
		if d.Field == plan.WorkflowDeltaFieldPrefix+id {
			return true
		}
	}
	return false
}

func TestBuild_KeptIdDeployedOnly_NoDriftAndOrchestratorUnchanged(t *testing.T) {
	in := keptBuildInput(
		makeDeployedWorkflows("quick-fix", "1.0", "my-custom", "3.0"),
		[]string{"quick-fix"}, []string{"my-custom"})

	item := buildOrchestratorItem(t, in)

	if hasWorkflowDelta(item, "my-custom") {
		t.Errorf("Stale = %v; a kept id must produce no workflow delta", item.Stale)
	}
	if item.Action != domain.ActionUnchanged {
		t.Errorf("orchestrator Action = %q, want %q; a deployed kept id is not drift", item.Action, domain.ActionUnchanged)
	}
}

func TestBuild_KeptIdAlsoInWorkflowIDs_Succeeds_NoDrift(t *testing.T) {
	in := keptBuildInput(
		makeDeployedWorkflows("quick-fix", "1.0", "my-custom", "3.0"),
		[]string{"quick-fix", "my-custom"}, []string{"my-custom"})

	item := buildOrchestratorItem(t, in)

	if hasWorkflowDelta(item, "my-custom") {
		t.Errorf("Stale = %v; a kept id must produce no workflow delta", item.Stale)
	}
	if item.Action != domain.ActionUnchanged {
		t.Errorf("orchestrator Action = %q, want %q", item.Action, domain.ActionUnchanged)
	}
}

func TestBuild_KeptIdAlsoInCatalogAtDifferentVersion_NoDrift(t *testing.T) {
	in := keptBuildInput(
		makeDeployedWorkflows("quick-fix", "1.0", "my-custom", "0.5"),
		[]string{"quick-fix", "my-custom"}, []string{"my-custom"})
	cat := in.Catalog.(*fakeCatalog)
	cat.workflows = append(cat.workflows, makeWorkflow("my-custom", "orchestrator"))

	item := buildOrchestratorItem(t, in)

	if hasWorkflowDelta(item, "my-custom") {
		t.Errorf("Stale = %v; a kept id wins over the catalog and must produce no workflow delta", item.Stale)
	}
	if item.Action != domain.ActionUnchanged {
		t.Errorf("orchestrator Action = %q, want %q", item.Action, domain.ActionUnchanged)
	}
}

func TestBuild_MultipleKeptIds_DeployedAndNot_NoDelta(t *testing.T) {
	in := keptBuildInput(
		makeDeployedWorkflows("quick-fix", "1.0", "my-custom", "3.0"),
		[]string{"quick-fix"}, []string{"my-custom", "never-deployed"})

	item := buildOrchestratorItem(t, in)

	for _, id := range []string{"my-custom", "never-deployed"} {
		if hasWorkflowDelta(item, id) {
			t.Errorf("Stale = %v; kept id %q must produce no workflow delta", item.Stale, id)
		}
	}
	if item.Action != domain.ActionUnchanged {
		t.Errorf("orchestrator Action = %q, want %q", item.Action, domain.ActionUnchanged)
	}
}

func TestBuild_KeptIdBesideCatalogDrift_ReportsOnlyCatalogDrift(t *testing.T) {
	in := keptBuildInput(
		makeDeployedWorkflows("quick-fix", "0.9", "my-custom", "3.0"),
		[]string{"quick-fix"}, []string{"my-custom"})

	item := buildOrchestratorItem(t, in)

	if item.Action != domain.ActionUpdate {
		t.Fatalf("orchestrator Action = %q, want %q; catalog workflow quick-fix drifted", item.Action, domain.ActionUpdate)
	}
	if !hasWorkflowDelta(item, "quick-fix") {
		t.Errorf("Stale = %v; want a workflow delta for the drifted catalog workflow quick-fix", item.Stale)
	}
	if hasWorkflowDelta(item, "my-custom") {
		t.Errorf("Stale = %v; a kept id must produce no workflow delta", item.Stale)
	}
	if strings.Contains(item.Reason, "my-custom") {
		t.Errorf("Reason %q names the kept id; kept ids are not drift", item.Reason)
	}
}

func TestBuild_NoKeptIds_DeployedUnselectedWorkflowIsStillRemovedDrift(t *testing.T) {
	in := keptBuildInput(
		makeDeployedWorkflows("quick-fix", "1.0", "old-wf", "3.0"),
		[]string{"quick-fix"}, nil)

	item := buildOrchestratorItem(t, in)

	if item.Action != domain.ActionUpdate || !hasWorkflowDelta(item, "old-wf") {
		t.Errorf("Action = %q, Stale = %v; with no kept ids a deployed workflow absent from the selection must stay removed drift",
			item.Action, item.Stale)
	}
}
