package app_test

// kept_legacy_ghcp_regression_test.go holds the regression coverage around the kept workflow of
// a pre-migration GHCP workspace: an already migrated GHCP workspace and a non-GHCP harness keep
// a removed workflow as before, and the agents flow on a pre-migration GHCP workspace still
// succeeds, keeps the removed workflow, and leaves the next update able to keep it too.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestUpdate_MigratedGhcpAndOtherHarness_KeepRemovedWorkflow(t *testing.T) {
	for _, h := range []string{hCLI, hVSCode, hClaude} {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, _ := klgWorkspace(t, h, false)
			wantIDs := map[string][]string{}
			for _, key := range wpsPairKeys {
				wantIDs[key] = klgIDsOf(t, key, gamRead(t, ws, wpsRel(t, h, key)))
			}

			// Act
			sum := klgUpdate(t, ws, h)

			// Assert
			for _, key := range wpsPairKeys {
				klgAssertKeptInFile(t, ws, h, key, wantIDs[key])
			}
			klgAssertKeptWarning(t, sum)
		})
	}
}

func TestDeployAgents_LegacyGhcpWorkspace_KeepsRemovedWorkflowAndNextUpdateSucceeds(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, legacy := klgWorkspace(t, h, true)
			wantIDs := map[string][]string{}
			for _, key := range wpsPairKeys {
				wantIDs[key] = klgIDsOf(t, key, legacy[key])
			}

			// Act
			_, err := wpsService(t).DeployAgents(context.Background(), app.DeployAgentsRequest{
				HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
				SubagentIDs: []string{"parity-runner"}, UtilityAgentIDs: []string{},
				InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
				AgentModels: gamModels(), AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})

			// Assert
			if err != nil {
				t.Fatalf("DeployAgents: %v", err)
			}
			for _, key := range wpsPairKeys {
				gamAssertKept(t, ws, key, legacy[key])
			}
			sum := klgUpdate(t, ws, h)
			for _, key := range wpsPairKeys {
				gamAssertMigrated(t, ws, key, legacy[key])
				klgAssertKeptInFile(t, ws, h, key, wantIDs[key])
			}
			klgAssertKeptWarning(t, sum)
		})
	}
}
