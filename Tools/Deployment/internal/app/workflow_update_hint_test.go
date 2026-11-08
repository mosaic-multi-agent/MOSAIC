package app_test

// workflow_update_hint_test.go verifies that the Update workflows flow does not attach the
// removed-layer hint to the failure of an explicitly requested workflow id, also when that id
// is deployed in the workspace: the flow rejects the id, and the kept-workflow warning is the
// only deployed-workflow signal it gives. The id here is deployed only as a workflow pair
// nested inside another deployed workflow block, so the flow has no block of its own for it
// and rejects it, while the deployed-workflow listing still sees it.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/plan"
)

const (
	nestedPairID    = "nested-only"
	nestedOuterID   = "quick-fix"
	nestedOrchFile  = "orchestrator.md"
	nestedHintToken = removedLayerHintPhrase
)

// nestedPairOrchestrator is a deployed orchestrator whose catalog workflow block holds a
// whole-line workflow pair of another id.
func nestedPairOrchestrator() string {
	return "---\nversion: \"1.0\"\n---\n\n<Identity type=\"core\">\n<AvailableWorkflows type=\"managed\">\n" +
		"<Workflow type=\"managed\" name=\"" + nestedOuterID + "\" version=\"1.0\">\nOuter body.\n" +
		"<Workflow type=\"managed\" name=\"" + nestedPairID + "\" version=\"1.0\">\nInner body.\n</Workflow>\n" +
		"</Workflow>\n</AvailableWorkflows>\n</Identity>\n"
}

// snapshotWorkspace returns the content of every file below root keyed by relative path.
func snapshotWorkspace(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(root, path)
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot workspace: %v", err)
	}
	return out
}

func TestUpdateWorkflows_RejectedDeployedID_FailsWithoutRemovedLayerHintAndWritesNothing(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
	if err := os.WriteFile(filepath.Join(ws, nestedOrchFile), []byte(nestedPairOrchestrator()), 0o644); err != nil {
		t.Fatalf("write deployed orchestrator: %v", err)
	}
	before := snapshotWorkspace(t, ws)

	_, err := app.New(deps).UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: ws, WorkflowIDs: []string{nestedPairID}, AutoConfirmPlan: true})

	if err == nil {
		t.Fatalf("UpdateWorkflows accepted %q, want the unknown-workflow failure", nestedPairID)
	}
	if !errors.Is(err, plan.ErrUnknownWorkflow) {
		t.Fatalf("error = %v, want errors.Is(err, plan.ErrUnknownWorkflow)", err)
	}
	if !strings.Contains(err.Error(), nestedPairID) {
		t.Errorf("error %q does not name the rejected id %q", err.Error(), nestedPairID)
	}
	if strings.Contains(err.Error(), nestedHintToken) {
		t.Errorf("error %q must not carry the removed-layer hint in the workflows flow", err.Error())
	}
	after := snapshotWorkspace(t, ws)
	if len(after) != len(before) {
		t.Errorf("workspace file set changed: before %d files, after %d", len(before), len(after))
	}
	for rel, content := range before {
		if after[rel] != content {
			t.Errorf("file %s changed by the failed run", rel)
		}
	}
}
