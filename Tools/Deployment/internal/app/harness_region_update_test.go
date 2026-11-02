package app_test

// harness_region_update_test.go specifies that the missing-region warning also reaches the
// Update workspace output: an orchestrator source without the second marker, on a harness with
// orchestrator content, updates successfully and leaves one warning per agent in the TODO output.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestUpdate_OrchestratorSourceWithoutSecondMarker_WarnsOncePerAgentAndSucceeds(t *testing.T) {
	// The content callback runs twice per item (probe plus write); the output must still hold
	// one warning per agent.
	exec, spy, deps, workspace := deployCaptureDeps(t, stampSourceSharedMarkerOnly, 2)
	// Update refreshes deployed files, so both orchestrators exist in the workspace already.
	crWriteDeployed(t, workspace, "orchestrator.md", minimalWorkflow.ID)
	crWriteDeployed(t, workspace, "orchestrator-script.md", minimalWorkflow.ID)
	plan := scriptPlan(workspace, "orchestrator.md", "orchestrator-script.md")
	plan.Mode = domain.ModeUpdateWorkspace
	deps.Planner = &stubPlanner{plan: plan}

	_, err := app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID:       minimalHarness.ID,
		WorkspacePath:   workspace,
		AutoConfirmPlan: true,
	})

	if err != nil {
		t.Fatalf("Update must succeed despite the missing region: %v", err)
	}
	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if cerr := exec.errors[key]; cerr != nil {
			t.Fatalf("content callback for %s failed: %v", key, cerr)
		}
		if strings.Contains(string(exec.contents[key]), stampOrchText) {
			t.Errorf("%s: orchestrator text appears in output although the source has no region for it", key)
		}
		assertOneUndeliveredWarning(t, key, spy.gaps)
	}
}
