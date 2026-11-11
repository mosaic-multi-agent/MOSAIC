package app_test

// reconcile_orchestrator_probe_test.go verifies that the orchestrator probe of the deploy-agents
// flow, which looks at the manifest on its own, does not act on entries of files that are not on
// disk: an orchestrator recorded but missing is listed as dropped and never planned as an update
// or conflict, and a recorded legacy X.md that does not exist is not a rename source.

import (
	"context"
	"os"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const rpInfraKey = "parity-checker"

func rpDeployAgents(t *testing.T, ws, harness string, c *gamReviewCapture, dry bool) {
	t.Helper()
	_, err := parityServiceWith(t, gamCaptureDeps(c)).DeployAgents(context.Background(), app.DeployAgentsRequest{
		HarnessID: harness, WorkspacePath: ws, Scope: domain.ScopeProject,
		SubagentIDs: []string{}, UtilityAgentIDs: []string{}, StandaloneAgentIDs: []string{},
		InfrastructureAgentIDs: []string{rpInfraKey},
		AutoConfirmPlan:        true, DryRun: dry, ConflictDefault: domain.DecisionSkip})
	if err != nil {
		t.Fatalf("DeployAgents: %v", err)
	}
}

func rpPlanActionAt(p domain.Plan, rel string) (domain.PlanAction, bool) {
	for _, it := range p.Items {
		if rrHasPath([]string{it.TargetPath}, rel) {
			return it.Action, true
		}
	}
	return "", false
}

func TestReconcileOrchestratorProbe_RecordedOrchestratorWhoseFileIsMissing_IsDroppedNotUpdatedOrInConflict(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			ws := parityDeploy(t, h)
			orchestrator := gamRel("orchestrator", ".agent.md")
			if err := os.Remove(gamAbs(ws, orchestrator)); err != nil {
				t.Fatalf("remove deployed orchestrator: %v", err)
			}
			c := gamNewReviewCapture(ws)

			rpDeployAgents(t, ws, h, c, true)

			p := gplReviewedPlan(t, c)
			if !rrHasPath(rrDroppedPaths(p), orchestrator) {
				t.Errorf("dropped entries %v lack the missing orchestrator %s", rrDroppedPaths(p), orchestrator)
			}
			if action, ok := rpPlanActionAt(p, orchestrator); ok &&
				(action == domain.ActionUpdate || action == domain.ActionConflict) {
				t.Errorf("the missing orchestrator is planned as %q", action)
			}
		})
	}
}

func TestReconcileOrchestratorProbe_PresentOrchestratorWithGhostLegacyEntry_IsUpdatedInPlaceNotRenamed(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			ws := parityDeploy(t, h)
			legacy := gamRel("orchestrator", ".md")
			m := gamManifest(t, ws)
			m.Entries = append(m.Entries, rfEntry(h, "orchestrator", legacy))
			gamSaveManifest(t, ws, m)
			c := gamNewReviewCapture(ws)

			rpDeployAgents(t, ws, h, c, true)

			p := gplReviewedPlan(t, c)
			if !rrHasPath(rrDroppedPaths(p), legacy) {
				t.Errorf("dropped entries %v lack the missing legacy file %s", rrDroppedPaths(p), legacy)
			}
			for _, it := range p.Items {
				if it.RenamedFrom == legacy {
					t.Errorf("%s is planned as a rename from the missing %s", it.Ref.Key, legacy)
				}
				if it.Action == domain.ActionConflict {
					t.Errorf("%s is planned as a conflict: %v", it.Ref.Key, it.Reasons)
				}
			}
		})
	}
}
