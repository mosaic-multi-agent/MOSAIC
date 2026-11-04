package app_test

// ghcp_agent_migration_followup_test.go covers what follows a migration that keeps the file
// content: the replaced X.md files stay recoverable from backup with their planted state, a
// second update finds everything unchanged, and after a subset run the agents it did not write
// stay X.md until the next update migrates them with their state kept.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// gsUpdate runs a plain update over a workspace and returns its summary.
func gsUpdate(t *testing.T, ws, harness string) domain.RunSummary {
	t.Helper()
	sum, err := parityService(t).Update(context.Background(), app.UpdateRequest{
		HarnessID: harness, WorkspacePath: ws, AutoConfirmPlan: true,
		ConflictDefault: domain.DecisionSkip})
	if err != nil {
		t.Fatalf("update for %s: %v", harness, err)
	}
	return sum
}

// TestGhcpMigration_ReplacedLegacyFiles_RemainInBackups verifies the backup of each replaced
// X.md holds the planted bytes, so the user state is recoverable and the backup is a copy, not
// the source the migrated file was written from.
func TestGhcpMigration_ReplacedLegacyFiles_RemainInBackups(t *testing.T) {
	for _, h := range gamTargets {
		for _, r := range gsRuns() {
			t.Run(h+"/"+r.name, func(t *testing.T) {
				// Arrange
				ws, planted := gsPlantedWorkspace(t, h, false)

				// Act
				err := r.run(t, ws, h, gsOpts{})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", r.name, h, err)
				}
				for _, key := range r.must {
					gamAssertMigrated(t, ws, key, planted[key])
					if !strings.Contains(string(gamRead(t, ws, gamRel(key, ".agent.md"))), gamCustomSentinel) {
						t.Errorf("%s: the migrated file does not carry the state of the backed-up file", key)
					}
				}
			})
		}
	}
}

// TestGhcpMigration_SecondUpdate_ReportsEverythingUnchanged verifies an update after a
// migration rewrites nothing: every action is unchanged and no agent file changes.
func TestGhcpMigration_SecondUpdate_ReportsEverythingUnchanged(t *testing.T) {
	for _, h := range gamTargets {
		for _, r := range gsRuns() {
			if len(r.must) != len(gsAllAgents) {
				continue
			}
			t.Run(h+"/"+r.name, func(t *testing.T) {
				// Arrange
				ws, _ := gsPlantedWorkspace(t, h, false)
				if err := r.run(t, ws, h, gsOpts{}); err != nil {
					t.Fatalf("%s for %s: %v", r.name, h, err)
				}
				after := map[string]string{}
				for _, key := range gsAllAgents {
					after[key] = string(gamRead(t, ws, gamRel(key, ".agent.md")))
				}

				// Act
				sum := gsUpdate(t, ws, h)

				// Assert
				for _, a := range sum.Actions {
					// The infrastructure agent is newly added by the agents run, not migrated.
					if strings.HasSuffix(a.TargetPath, gaoInfraKey+".agent.md") {
						continue
					}
					if a.Taken != domain.TakenUnchanged {
						t.Errorf("second update: %s taken %q, want unchanged", a.TargetPath, a.Taken)
					}
				}
				for key, content := range after {
					if !strings.Contains(content, gamCustomSentinel) {
						t.Errorf("%s: the migrated file lost its custom region before the second update", key)
					}
				}
				for key, want := range after {
					if got := string(gamRead(t, ws, gamRel(key, ".agent.md"))); got != want {
						t.Errorf("%s: second update changed the file:\n%s", key, parityFirstDiff(want, got))
					}
				}
			})
		}
	}
}

// TestGhcpMigration_SubsetRun_LeavesOthersForTheNextUpdate verifies agents a subset run does
// not write stay their X.md, and that the following update migrates them with the state a
// plain update of an unmigrated agent leaves.
func TestGhcpMigration_SubsetRun_LeavesOthersForTheNextUpdate(t *testing.T) {
	for _, h := range gamTargets {
		for _, r := range gsRuns() {
			// A run that changes the workflow set leaves the later update a different input than
			// the reference workspace has, so only runs that keep the set are compared.
			if !r.workflows || len(r.must) == len(gsAllAgents) {
				continue
			}
			t.Run(h+"/"+r.name, func(t *testing.T) {
				// Arrange
				ws, planted := gsPlantedWorkspace(t, h, false)
				refWs, _ := gsPlantedWorkspace(t, h, true)
				var rest []string
				for _, key := range gsAllAgents {
					if !gsWrites(r, key) {
						rest = append(rest, key)
					}
				}
				if err := r.run(t, ws, h, gsOpts{}); err != nil {
					t.Fatalf("%s for %s: %v", r.name, h, err)
				}
				for _, key := range rest {
					gamAssertKept(t, ws, key, planted[key])
				}

				// Act
				gsUpdate(t, ws, h)
				gsUpdate(t, refWs, h)

				// Assert
				for _, key := range rest {
					newRel := gamRel(key, ".agent.md")
					if !gamExists(ws, newRel) || gamExists(ws, gamRel(key, ".md")) {
						t.Errorf("%s: the next update did not migrate it", key)
						continue
					}
					got := string(gamRead(t, ws, newRel))
					for _, marker := range []string{gamProjectSentinel, gamCustomSentinel, gamHandSetTools, "model: " + gsPlantedModel} {
						if strings.Contains(string(planted[key]), marker) && !strings.Contains(got, marker) {
							t.Errorf("%s: the next update lost %q", key, marker)
						}
					}
					if want := string(gamRead(t, refWs, newRel)); got != want {
						t.Errorf("%s: differs from a plain update of an unmigrated agent:\n%s",
							key, parityFirstDiff(want, got))
					}
				}
			})
		}
	}
}
