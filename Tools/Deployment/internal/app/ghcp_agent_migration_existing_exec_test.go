package app_test

// ghcp_agent_migration_existing_exec_test.go verifies what an executed deploy flow leaves on disk
// and in the manifest for an X.agent.md that exists beside a recorded, present X.md: an
// unchanged file is not rewritten, an outdated or edited one is updated with a backup, a
// conflicting one follows the conflict decision, and the legacy X.md is always backed up,
// removed and its entry dropped unless the run skipped the conflicting agent.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// gmeDecisions returns the conflict decisions a state is run with: only a conflicting state
// depends on the decision.
func gmeDecisions(st gmeState) []domain.ConflictDecision {
	if st.action == domain.ActionConflict {
		return gamConflictDefaults
	}
	return []domain.ConflictDecision{domain.DecisionSkip}
}

// gmeWantWritten reports whether the run rewrites the existing X.agent.md.
func gmeWantWritten(st gmeState, cd domain.ConflictDecision) bool {
	if st.action == domain.ActionConflict {
		return cd != domain.DecisionSkip
	}
	return st.action == domain.ActionUpdate
}

// gmeWantBackup reports whether the run backs up the existing X.agent.md before writing.
func gmeWantBackup(st gmeState, cd domain.ConflictDecision) bool {
	if st.action == domain.ActionConflict {
		return cd == domain.DecisionBackupThenOverwrite
	}
	return st.action == domain.ActionUpdate
}

// TestGhcpDeployFlows_ExistingAgentFile_ExecutedRunKeepsItsOwnState covers every flow, state
// and conflict decision.
func TestGhcpDeployFlows_ExistingAgentFile_ExecutedRunKeepsItsOwnState(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			for _, st := range gmeStates() {
				for _, cd := range gmeDecisions(st) {
					t.Run(h+"/"+flow.name+"/"+st.name+"/"+string(cd), func(t *testing.T) {
						// Arrange
						ws := gmeWorkspace(t, h)
						st.prepare(t, ws, flow.subject)
						newRel, oldRel := gamRel(flow.subject, ".agent.md"), gamRel(flow.subject, ".md")
						before, legacy := gamRead(t, ws, newRel), gamRead(t, ws, oldRel)

						// Act
						err := flow.run(t, ws, h, gamOpts{conflict: cd})

						// Assert
						if err != nil {
							t.Fatalf("%s for %s: %v", flow.name, h, err)
						}
						gmeAssertExecuted(t, ws, flow.subject, st, cd, before, legacy)
					})
				}
			}
		}
	}
}

// gmeAssertExecuted checks the agent file, its backups and the legacy file after the run.
func gmeAssertExecuted(t *testing.T, ws, key string, st gmeState, cd domain.ConflictDecision, before, legacy []byte) {
	t.Helper()
	newRel := gamRel(key, ".agent.md")
	got := gamRead(t, ws, newRel)
	if written := string(got) != string(before); written != gmeWantWritten(st, cd) {
		t.Errorf("%s: rewritten = %t, want %t", newRel, written, gmeWantWritten(st, cd))
	}
	backups := len(gamBackups(t, ws, newRel))
	if gmeWantBackup(st, cd) {
		if !gamHasBackup(t, ws, newRel, before) {
			t.Errorf("%s: no backup holds the bytes it had before the run", newRel)
		}
	} else if backups != 0 {
		t.Errorf("%s: %d backups made, want none", newRel, backups)
	}
	if st.action == domain.ActionConflict && cd == domain.DecisionSkip {
		gamAssertRecoverable(t, ws, key, legacy)
		return
	}
	gamAssertMigrated(t, ws, key, legacy)
}

// TestGhcpDeployFlows_ExistingAgentFiles_OfOtherAgentsAreMigratedAndLeftAlone verifies that for
// the agents a run deploys whose X.agent.md is up to date nothing is rewritten and the legacy
// X.md is removed, while the agents the run does not deploy keep both files.
func TestGhcpDeployFlows_ExistingAgentFiles_OfOtherAgentsAreMigratedAndLeftAlone(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws := gmeWorkspace(t, h)
				current, legacy := map[string][]byte{}, map[string][]byte{}
				for _, a := range parityAgents {
					current[a.key] = gamRead(t, ws, gamRel(a.key, ".agent.md"))
					legacy[a.key] = gamRead(t, ws, gamRel(a.key, ".md"))
				}

				// Act
				err := flow.run(t, ws, h, gamOpts{})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				for _, key := range flow.migrates {
					gamAssertMigrated(t, ws, key, legacy[key])
				}
				for _, a := range parityAgents {
					newRel := gamRel(a.key, ".agent.md")
					if string(gamRead(t, ws, newRel)) != string(current[a.key]) {
						t.Errorf("%s: an up-to-date agent was rewritten", newRel)
					}
					if n := len(gamBackups(t, ws, newRel)); n != 0 {
						t.Errorf("%s: %d backups of an up-to-date agent", newRel, n)
					}
				}
				for _, key := range flow.keeps {
					if !gamExists(ws, gamRel(key, ".md")) {
						t.Errorf("%s: legacy file of an agent the run does not deploy was removed", key)
					}
				}
			})
		}
	}
}
