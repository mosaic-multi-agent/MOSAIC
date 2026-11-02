package app_test

// ghcp_agent_migration_safety_test.go covers what surrounds the legacy X.md migration: no
// agent is lost when a deploy fails part-way, the plan the user reviews carries the migration's
// removals and replacements, and deploying hooks alone leaves recorded X.md agents untouched.

import (
	"context"
	"sort"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// gamSpyReview makes the service hand every reviewed plan to the returned spy.
func gamSpyReview() (*crReviewSpy, func(*app.Deps)) {
	spy := &crReviewSpy{Stub: interactiontest.NewBuilder().Build()}
	return spy, func(d *app.Deps) { d.Interaction = spy }
}

// gamAssertRecoverable checks the legacy X.md of key still exists with its bytes or a backup
// holds them, so the agent is never lost.
func gamAssertRecoverable(t *testing.T, ws, key string, legacy []byte) {
	t.Helper()
	oldRel := gamRel(key, ".md")
	if gamExists(ws, oldRel) && string(gamRead(t, ws, oldRel)) == string(legacy) {
		return
	}
	if !gamHasBackup(t, ws, oldRel, legacy) {
		t.Errorf("%s: %s is gone or changed and no backup holds its bytes", key, oldRel)
	}
}

// gamFailure injects a failure into a legacy workspace the migration would hit.
type gamFailure struct {
	name   string
	inject func(t *testing.T, ws, subject string)
}

// gamFailures are failures that stop the migration part-way: the new file cannot be written,
// and the backup folder cannot be created.
var gamFailures = []gamFailure{
	{"new file blocked", func(t *testing.T, ws, subject string) {
		gamWrite(t, ws, gamRel(subject, ".agent.md")+"/keep.txt", "blocker")
	}},
	{"backup folder blocked", func(t *testing.T, ws, _ string) {
		gamWrite(t, ws, ".mosaic/backups", "not a folder")
	}},
}

// TestGhcpDeployFlows_FailureDuringMigration_NeverLosesAnAgent verifies a deploy that fails
// part-way surfaces the failure and leaves every legacy agent either in place with its bytes
// or recoverable from a backup.
func TestGhcpDeployFlows_FailureDuringMigration_NeverLosesAnAgent(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			for _, f := range gamFailures {
				t.Run(h+"/"+flow.name+"/"+f.name, func(t *testing.T) {
					// Arrange
					ws, legacy := gamLegacyWorkspace(t, h)
					f.inject(t, ws, flow.subject)
					var sum domain.RunSummary

					// Act
					err := flow.run(t, ws, h, gamOpts{out: &sum})

					// Assert
					if err == nil && sum.Outcome != domain.OutcomeFailed {
						t.Errorf("the failure was not surfaced: error nil, outcome %q", sum.Outcome)
					}
					for _, a := range parityAgents {
						gamAssertRecoverable(t, ws, a.key, legacy[a.key])
					}
				})
			}
		}
	}
}

// TestGhcpDeployFlows_ReviewPlan_CarriesTheMigration verifies the plan shown for review lists
// the removal of every migrated X.md and the replacement of an existing X.agent.md.
func TestGhcpDeployFlows_ReviewPlan_CarriesTheMigration(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gamFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, _ := gamLegacyWorkspace(t, h)
				existing := gamRel(flow.subject, ".agent.md")
				gamWrite(t, ws, existing, "stale user copy\n")
				var wantRemovals []string
				for _, key := range flow.migrates {
					wantRemovals = append(wantRemovals, gamRel(key, ".md"))
				}
				sort.Strings(wantRemovals)
				spy, withSpy := gamSpyReview()

				// Act
				err := flow.run(t, ws, h, gamOpts{deps: []func(*app.Deps){withSpy}})

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				p := onlyReviewedPlan(t, spy)
				if got := trPlanPaths(p.Removals); !trSamePaths(got, wantRemovals) {
					t.Errorf("review plan removals = %v, want %v", got, wantRemovals)
				}
				if got := trPlanPaths(p.Replacements); !trSamePaths(got, []string{existing}) {
					t.Errorf("review plan replacements = %v, want [%s]", got, existing)
				}
			})
		}
	}
}

// TestGhcpDeployHooks_LegacyWorkspace_LeavesRecordedAgentsAlone verifies deploying hooks only
// does not touch agents a workspace still records as X.md.
func TestGhcpDeployHooks_LegacyWorkspace_LeavesRecordedAgentsAlone(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, legacy := gamLegacyWorkspace(t, h)

			// Act
			_, err := parityService(t).DeployHooks(context.Background(), app.DeployHooksRequest{
				HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
				HookIDs: []string{}, AutoConfirmPlan: true})

			// Assert
			if err != nil {
				t.Fatalf("DeployHooks for %s: %v", h, err)
			}
			for _, a := range parityAgents {
				gamAssertKept(t, ws, a.key, legacy[a.key])
			}
		})
	}
}
