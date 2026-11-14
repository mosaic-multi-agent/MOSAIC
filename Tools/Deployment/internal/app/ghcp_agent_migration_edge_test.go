package app_test

// ghcp_agent_migration_edge_test.go covers the edges of the legacy X.md migration: an
// X.md the manifest does not record, agents the run does not deploy, and a dry run. An
// X.agent.md that already exists beside a recorded X.md is covered by
// ghcp_agent_migration_existing_*_test.go.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// gamExistingVariant describes an X.agent.md already in the workspace next to a recorded X.md.
type gamExistingVariant struct {
	name         string
	recorded     bool
	recordedBody string // content the manifest hash was taken from, when recorded
	body         string // content on disk
}

// gamConflictDefaults are the conflict defaults a migration must be independent of.
var gamConflictDefaults = []domain.ConflictDecision{
	domain.DecisionSkip, domain.DecisionOverwrite, domain.DecisionBackupThenOverwrite,
}

// TestGhcpDeployFlows_UnrecordedAgentMd_Stays verifies an X.md the manifest does not record is
// a user file: it is neither backed up nor removed, and neither is any other user file.
func TestGhcpDeployFlows_UnrecordedAgentMd_Stays(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, legacy := gamLegacyWorkspace(t, h)
				oldRel := gamRel(flow.subject, ".md")
				gamDropEntry(t, ws, flow.subject, oldRel)
				userRel := gamRel("mine", ".md")
				gamWrite(t, ws, userRel, "my own agent\n")

				// Act
				err := flow.run(t, ws, h, gamOpts{})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				if !gamExists(ws, oldRel) || string(gamRead(t, ws, oldRel)) != string(legacy[flow.subject]) {
					t.Errorf("unrecorded %s was removed or changed", oldRel)
				}
				if n := len(gamBackups(t, ws, oldRel)); n != 0 {
					t.Errorf("unrecorded %s was backed up %d times", oldRel, n)
				}
				if !gamExists(ws, userRel) || string(gamRead(t, ws, userRel)) != "my own agent\n" {
					t.Errorf("user file %s was removed or changed", userRel)
				}
				gamAssertAgentsRecorded(t, ws, []string{flow.subject})
			})
		}
	}
}

// TestGhcpDeployFlows_UnrecordedAgentMdBesideAgentDotMd_Stays verifies an unrecorded X.md stays
// untouched (no backup, no manifest entry) when an unrecorded X.agent.md also exists. That
// X.agent.md is an ordinary unrecorded file, so the flow's conflict handling decides its fate and
// the test does not assert whether it is replaced or recorded.
func TestGhcpDeployFlows_UnrecordedAgentMdBesideAgentDotMd_Stays(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, legacy := gamLegacyWorkspace(t, h)
				oldRel := gamRel(flow.subject, ".md")
				gamDropEntry(t, ws, flow.subject, oldRel)
				gamWrite(t, ws, gamRel(flow.subject, ".agent.md"), "stale user copy\n")

				// Act
				err := flow.run(t, ws, h, gamOpts{})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				if !gamExists(ws, oldRel) || string(gamRead(t, ws, oldRel)) != string(legacy[flow.subject]) {
					t.Errorf("unrecorded %s was removed or changed", oldRel)
				}
				if n := len(gamBackups(t, ws, oldRel)); n != 0 {
					t.Errorf("unrecorded %s was backed up %d times", oldRel, n)
				}
				for _, p := range gamEntryPaths(gamManifest(t, ws), flow.subject) {
					if p == oldRel {
						t.Errorf("unrecorded %s gained a manifest entry", oldRel)
					}
				}
			})
		}
	}
}

// TestGhcpDeployFlows_AgentsTheRunDoesNotDeploy_Stay verifies an edited legacy agent the run
// does not deploy keeps its file, its X.md record and its bytes.
func TestGhcpDeployFlows_AgentsTheRunDoesNotDeploy_Stay(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			if len(flow.keeps) == 0 {
				continue
			}
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, legacy := gamLegacyWorkspace(t, h)
				for _, key := range flow.keeps {
					edited := string(legacy[key]) + "\nlocal edit\n"
					gamWrite(t, ws, gamRel(key, ".md"), edited)
					legacy[key] = []byte(edited)
				}

				// Act
				err := flow.run(t, ws, h, gamOpts{})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				for _, key := range flow.keeps {
					gamAssertKept(t, ws, key, legacy[key])
				}
			})
		}
	}
}

// TestGhcpDeployFlows_DryRun_ChangesNothing verifies a dry run neither migrates, backs up nor
// writes anything on a workspace with recorded X.md agents.
func TestGhcpDeployFlows_DryRun_ChangesNothing(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, _ := gamLegacyWorkspace(t, h)
				before := captureFS(t, ws)

				// Act
				err := flow.run(t, ws, h, gamOpts{dry: true})

				// Assert
				if err != nil {
					t.Fatalf("%s dry run for %s: %v", flow.name, h, err)
				}
				if changed, what := fsChanged(before, captureFS(t, ws)); changed {
					t.Errorf("dry run changed the workspace: %s", what)
				}
			})
		}
	}
}
