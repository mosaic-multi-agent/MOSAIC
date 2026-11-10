package app_test

// ghcp_agent_orchestrator_test.go verifies the orchestrator and script orchestrator files that
// the deploy-agents flow rewrites on its own when an infrastructure agent is selected: for both
// GHCP targets they are written as X.agent.md, never beside a second X.md, and a legacy
// recorded X.md is migrated with a byte-identical backup.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// gaoInfraKey is the infrastructure agent of the tool-local parity catalog.
const gaoInfraKey = "parity-checker"

// gaoOrchestrators are the orchestrator-role agents the flow rewrites.
var gaoOrchestrators = []string{"orchestrator", "orchestrator-script"}

// gaoDeployInfra runs DeployAgents selecting only the infrastructure agent.
func gaoDeployInfra(t *testing.T, ws, harness string, cd domain.ConflictDecision) error {
	t.Helper()
	models := gamModels()
	models[gaoInfraKey] = parityModel
	_, err := parityService(t).DeployAgents(context.Background(), app.DeployAgentsRequest{
		HarnessID: harness, WorkspacePath: ws, Scope: domain.ScopeProject,
		SubagentIDs: []string{}, UtilityAgentIDs: []string{},
		InfrastructureAgentIDs: []string{gaoInfraKey}, StandaloneAgentIDs: []string{},
		AgentModels: models, AutoConfirmPlan: true, ConflictDefault: cd})
	return err
}

// gaoAssertDeclared checks the orchestrator file of key was rewritten to declare the
// infrastructure agent and that the infrastructure agent itself is a recorded X.agent.md.
func gaoAssertDeclared(t *testing.T, ws, key string) {
	t.Helper()
	rel := gamRel(key, ".agent.md")
	if !gamExists(ws, rel) {
		t.Errorf("%s: %s was not written", key, rel)
		return
	}
	if !strings.Contains(string(gamRead(t, ws, rel)), gaoInfraKey) {
		t.Errorf("%s: %s does not declare the %s infrastructure agent", key, rel, gaoInfraKey)
	}
}

// TestGhcpDeployAgents_InfrastructureSelected_RewritesOrchestratorsAsAgentDotMd covers a
// workspace whose orchestrators were deployed under the new name: the rewrite targets
// X.agent.md, no X.md appears, and the manifest keeps one entry per orchestrator.
func TestGhcpDeployAgents_InfrastructureSelected_RewritesOrchestratorsAsAgentDotMd(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws := parityDeploy(t, h)

			// Act
			err := gaoDeployInfra(t, ws, h, domain.DecisionBackupThenOverwrite)

			// Assert
			if err != nil {
				t.Fatalf("DeployAgents for %s: %v", h, err)
			}
			for _, key := range gaoOrchestrators {
				gaoAssertDeclared(t, ws, key)
				if gamExists(ws, gamRel(key, ".md")) {
					t.Errorf("%s: a plain %s was written", key, gamRel(key, ".md"))
				}
			}
			gamAssertNoAgentInBothForms(t, ws)
			gamAssertAgentsRecorded(t, ws, append([]string{gaoInfraKey}, gaoOrchestrators...))
			gamAssertOnlyAgentExt(t, ws)
		})
	}
}

// TestGhcpDeployAgents_InfrastructureSelected_MigratesLegacyOrchestrators covers orchestrators
// recorded as X.md: each is replaced by a declaring X.agent.md, the manifest entry is replaced
// and the legacy bytes are recoverable from backup, whatever the conflict default.
func TestGhcpDeployAgents_InfrastructureSelected_MigratesLegacyOrchestrators(t *testing.T) {
	for _, h := range gamTargets {
		for _, cd := range gamConflictDefaults {
			t.Run(h+"/"+string(cd), func(t *testing.T) {
				// Arrange
				ws, legacy := gamLegacyWorkspace(t, h)

				// Act
				err := gaoDeployInfra(t, ws, h, cd)

				// Assert
				if err != nil {
					t.Fatalf("DeployAgents for %s: %v", h, err)
				}
				for _, key := range gaoOrchestrators {
					gamAssertMigrated(t, ws, key, legacy[key])
					gaoAssertDeclared(t, ws, key)
				}
				gamAssertAgentsRecorded(t, ws, []string{gaoInfraKey})
				gamAssertNoAgentInBothForms(t, ws)
			})
		}
	}
}
