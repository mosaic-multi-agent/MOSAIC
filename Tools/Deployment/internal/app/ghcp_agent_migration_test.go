package app_test

// ghcp_agent_migration_test.go verifies that the agent-writing deploy flows replace the
// agents an earlier deploy recorded as X.md with X.agent.md for both GHCP targets: the old
// file is backed up and removed, the manifest entry is replaced, and an agent the run does
// not deploy is left as it was.

import (
	"testing"
)

// TestGhcpDeployFlows_RecordedLegacyAgents_AreMigratedToAgentDotMd covers the four flows on
// a workspace whose agents are all recorded as X.md.
func TestGhcpDeployFlows_RecordedLegacyAgents_AreMigratedToAgentDotMd(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, legacy := gamLegacyWorkspace(t, h)

				// Act
				err := flow.run(t, ws, h, gamOpts{})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				for _, key := range flow.migrates {
					gamAssertMigrated(t, ws, key, legacy[key])
				}
				for _, key := range flow.keeps {
					gamAssertKept(t, ws, key, legacy[key])
				}
				gamAssertNoAgentInBothForms(t, ws)
			})
		}
	}
}

// TestGhcpDeployFlows_EditedRecordedLegacyAgent_IsBackedUpWithItsEdits covers a recorded X.md
// the user edited: it is still replaced by X.agent.md, and the edited bytes (not the recorded
// ones) are recoverable from backup whatever the conflict default.
func TestGhcpDeployFlows_EditedRecordedLegacyAgent_IsBackedUpWithItsEdits(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			for _, cd := range gamConflictDefaults {
				t.Run(h+"/"+flow.name+"/"+string(cd), func(t *testing.T) {
					// Arrange
					ws, legacy := gamLegacyWorkspace(t, h)
					oldRel := gamRel(flow.subject, ".md")
					edited := append(append([]byte{}, legacy[flow.subject]...), []byte("\nlocal edit by the user\n")...)
					gamWrite(t, ws, oldRel, string(edited))

					// Act
					err := flow.run(t, ws, h, gamOpts{conflict: cd})

					// Assert
					if err != nil {
						t.Fatalf("%s for %s: %v", flow.name, h, err)
					}
					gamAssertMigrated(t, ws, flow.subject, edited)
				})
			}
		}
	}
}

// TestGhcpUpdate_AfterMigration_LeavesTheMigratedAgentsAlone verifies a second Update after a
// migration sees the migrated agents as deployed: nothing is migrated again, duplicated or
// backed up a second time.
func TestGhcpUpdate_AfterMigration_LeavesTheMigratedAgentsAlone(t *testing.T) {
	for _, h := range gamTargets {
		for _, first := range []int{0, 2} { // DeployNew, Update
			flow := gamFlows()[first]
			t.Run(h+"/after "+flow.name, func(t *testing.T) {
				// Arrange
				ws, legacy := gamLegacyWorkspace(t, h)
				if err := flow.run(t, ws, h, gamOpts{}); err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				before := map[string][]byte{}
				for _, key := range gamAllAgentKeys {
					before[key] = gamRead(t, ws, gamRel(key, ".agent.md"))
				}

				// Act
				if err := gamFlows()[2].run(t, ws, h, gamOpts{}); err != nil {
					t.Fatalf("second Update for %s: %v", h, err)
				}

				// Assert
				gamAssertOnlyAgentExt(t, ws)
				gamAssertAgentsRecorded(t, ws, gamAllAgentKeys)
				for _, key := range gamAllAgentKeys {
					if string(gamRead(t, ws, gamRel(key, ".agent.md"))) != string(before[key]) {
						t.Errorf("%s: second Update changed the migrated agent", key)
					}
					if n := len(gamBackups(t, ws, gamRel(key, ".agent.md"))); n != 0 {
						t.Errorf("%s: second Update backed up the migrated agent %d times", key, n)
					}
					if !gamHasBackup(t, ws, gamRel(key, ".md"), legacy[key]) {
						t.Errorf("%s: legacy backup lost after the second Update", key)
					}
				}
			})
		}
	}
}
