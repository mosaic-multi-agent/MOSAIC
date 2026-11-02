package app_test

// ghcp_agent_migration_state_test.go verifies that a GHCP agent migrated from X.md to
// X.agent.md keeps the state a normal run leaves for an agent that already was X.agent.md:
// project and custom regions, model, tool choices, infrastructure declarations and workflow
// set. Every flow runs over a real planted workspace on both GHCP targets, and the content of
// each written file is asserted, not only the plan.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// TestGhcpMigration_MigratedAgentFile_KeepsTheLegacyState verifies each agent the run writes
// holds the planted markers of its legacy file, equals the file a run over the same state
// named X.agent.md produces, and that agents the run does not write are left alone.
func TestGhcpMigration_MigratedAgentFile_KeepsTheLegacyState(t *testing.T) {
	for _, h := range gamTargets {
		for _, r := range gsRuns() {
			t.Run(h+"/"+r.name, func(t *testing.T) {
				// Arrange
				ws, planted := gsPlantedWorkspace(t, h, false)
				refWs, _ := gsPlantedWorkspace(t, h, true)
				var sum, refSum domain.RunSummary

				// Act
				err := r.run(t, ws, h, gsOpts{out: &sum})
				refErr := r.run(t, refWs, h, gsOpts{out: &refSum})

				// Assert
				if err != nil || refErr != nil {
					t.Fatalf("%s for %s: legacy workspace error %v, reference error %v", r.name, h, err, refErr)
				}
				for _, key := range r.must {
					newRel := gamRel(key, ".agent.md")
					if !gamExists(ws, newRel) || gamExists(ws, gamRel(key, ".md")) {
						t.Errorf("%s: not migrated to %s", key, newRel)
						continue
					}
					got := string(gamRead(t, ws, newRel))
					for _, marker := range gsWantState(r, string(planted[key])) {
						if !strings.Contains(got, marker) {
							t.Errorf("%s: migrated file lost %q", key, marker)
						}
					}
					if want := string(gamRead(t, refWs, newRel)); got != want {
						t.Errorf("%s: migrated file differs from the file of an unmigrated agent:\n%s",
							key, parityFirstDiff(want, got))
					}
				}
				gsAssertNoModelTodo(t, sum, r.must)
				gamAssertNoAgentInBothForms(t, ws)
			})
		}
	}
}

// TestGhcpMigration_AgentWithoutNumericID_KeepsTheSameState verifies the agents of the sample
// catalog that declare no numeric id keep their regions, model and tools exactly as the agent
// that declares one does, in every flow that writes them.
func TestGhcpMigration_AgentWithoutNumericID_KeepsTheSameState(t *testing.T) {
	const withID, withoutID = "parity-runner", "parity-helper"
	for _, h := range gamTargets {
		for _, r := range gsRuns() {
			if !gsWrites(r, withID) || !gsWrites(r, withoutID) {
				continue
			}
			t.Run(h+"/"+r.name, func(t *testing.T) {
				// Arrange
				ws, planted := gsPlantedWorkspace(t, h, false)
				var sum domain.RunSummary

				// Act
				err := r.run(t, ws, h, gsOpts{out: &sum})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", r.name, h, err)
				}
				for _, key := range []string{withID, withoutID} {
					got := string(gamRead(t, ws, gamRel(key, ".agent.md")))
					for _, marker := range []string{gamProjectSentinel, gamCustomSentinel, gamHandSetTools, "model: " + gsPlantedModel} {
						if strings.Contains(string(planted[key]), marker) && !strings.Contains(got, marker) {
							t.Errorf("%s: migrated file lost %q", key, marker)
						}
					}
				}
				if !strings.Contains(string(planted[withoutID]), "model: "+gsPlantedModel) {
					t.Fatalf("fixture: %s carries no planted model", withoutID)
				}
			})
		}
	}
}

func gsWrites(r gsRun, key string) bool {
	for _, k := range r.must {
		if k == key {
			return true
		}
	}
	return false
}
