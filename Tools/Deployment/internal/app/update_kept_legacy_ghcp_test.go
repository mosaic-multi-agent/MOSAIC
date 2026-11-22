package app_test

// update_kept_legacy_ghcp_test.go specifies Update on a GHCP workspace still in the pre-migration
// form (agents recorded as X.md) that holds a workflow the catalog stack does not supply: the run
// completes, migrates the agents to X.agent.md, keeps the workflow byte for byte in its position
// in both the orchestrator and the orchestrator-script, records the kept warning, refreshes the
// catalog workflows, and leaves nothing to do for a second update. Both GHCP targets run.

import (
	"testing"
)

func TestUpdate_LegacyGhcpWorkspace_KeepsRemovedWorkflowAndMigratesAgents(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, legacy := klgWorkspace(t, h, true)
			wantIDs := map[string][]string{}
			for _, key := range wpsPairKeys {
				wantIDs[key] = klgIDsOf(t, key, legacy[key])
			}

			// Act
			sum := klgUpdate(t, ws, h)

			// Assert
			for _, key := range wpsPairKeys {
				gamAssertMigrated(t, ws, key, legacy[key])
				klgAssertKeptInFile(t, ws, h, key, wantIDs[key])
			}
			klgAssertKeptWarning(t, sum)
		})
	}
}

func TestUpdate_LegacyGhcpWorkspace_SecondUpdateReportsNothingToDo(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, legacy := klgWorkspace(t, h, true)
			wantIDs := map[string][]string{}
			for _, key := range wpsPairKeys {
				wantIDs[key] = klgIDsOf(t, key, legacy[key])
			}
			klgUpdate(t, ws, h)

			// Act
			sum := klgUpdate(t, ws, h)

			// Assert
			klgAssertAllUnchanged(t, sum)
			for _, key := range wpsPairKeys {
				klgAssertKeptInFile(t, ws, h, key, wantIDs[key])
			}
		})
	}
}
