package app_test

// dryrun_ghcp_legacy_test.go verifies that a dry run over a GHCP workspace holding agents an
// earlier deploy recorded as X.md lists the planned removals, replacements and renames, without
// any plan confirmation and without writing anything.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// drgFlow is one agent-writing flow run as a dry run without plan confirmation, with the agents
// it migrates.
type drgFlow struct {
	name     string
	subject  string
	migrates []string
	run      func(t *testing.T, ws, harness string, review *drReview) (domain.RunSummary, error)
}

func drgFlows() []drgFlow {
	ctx := context.Background()
	reviewing := func(r *drReview) func(*app.Deps) { return func(d *app.Deps) { d.Interaction = r } }
	flows := gamFlows()
	byName := map[string]gamFlow{}
	for _, f := range flows {
		byName[f.name] = f
	}
	return []drgFlow{
		{name: "DeployNew", subject: "parity-runner", migrates: byName["DeployNew"].migrates,
			run: func(t *testing.T, ws, h string, r *drReview) (domain.RunSummary, error) {
				return parityServiceWith(t, reviewing(r)).DeployNew(ctx, app.DeployRequest{
					HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
					WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: []string{"parity-helper"},
					AgentModels: gamModels(), DryRun: true, ConflictDefault: domain.DecisionSkip})
			}},
		{name: "DeployAgents", subject: "parity-runner", migrates: byName["DeployAgents"].migrates,
			run: func(t *testing.T, ws, h string, r *drReview) (domain.RunSummary, error) {
				return parityServiceWith(t, reviewing(r)).DeployAgents(ctx, app.DeployAgentsRequest{
					HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
					SubagentIDs: []string{"parity-runner"}, UtilityAgentIDs: []string{},
					InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
					AgentModels: gamModels(), DryRun: true, ConflictDefault: domain.DecisionSkip})
			}},
		{name: "Update", subject: "parity-runner", migrates: byName["Update"].migrates,
			run: func(t *testing.T, ws, h string, r *drReview) (domain.RunSummary, error) {
				return parityServiceWith(t, reviewing(r)).Update(ctx, app.UpdateRequest{
					HarnessID: h, WorkspacePath: ws, AgentModels: gamModels(),
					DryRun: true, ConflictDefault: domain.DecisionSkip})
			}},
		{name: "UpdateWorkflows", subject: "orchestrator", migrates: byName["UpdateWorkflows"].migrates,
			run: func(t *testing.T, ws, h string, r *drReview) (domain.RunSummary, error) {
				return parityServiceWith(t, reviewing(r)).UpdateWorkflows(ctx, app.WorkflowUpdateRequest{
					HarnessID: h, WorkspacePath: ws, WorkflowIDs: []string{"parity-flow"},
					DryRun: true, ConflictDefault: domain.DecisionSkip})
			}},
	}
}

// drgReplacesPath reports whether the plan replaces the workspace-relative path.
func drgReplacesPath(p domain.Plan, rel string) bool {
	for _, r := range p.Replacements {
		if strings.ReplaceAll(r.Path, "\\", "/") == rel {
			return true
		}
	}
	return false
}

// TestDryRun_GhcpLegacyWorkspace_ListsRemovalsAndRenames verifies every migrated agent shows as
// a rename with the removal of its X.md, with a reason on each, and nothing is written.
func TestDryRun_GhcpLegacyWorkspace_ListsRemovalsAndRenames(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range drgFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{})
				before := crSnapshotDir(t, ws)
				review := newDrReview(domain.SkippedOne)

				// Act
				summary, err := flow.run(t, ws, h, review)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: dry run without confirmation failed: %v", flow.name, h, err)
				}
				if summary.DryRunPlan == nil {
					t.Fatalf("%s for %s: summary carries no dry-run plan", flow.name, h)
				}
				p := *summary.DryRunPlan
				for _, key := range flow.migrates {
					gplAssertRename(t, p, key, false)
				}
				for _, r := range p.Removals {
					if r.Reason == "" {
						t.Errorf("removal of %s has no reason", r.Path)
					}
				}
				if !reflect.DeepEqual(crSnapshotDir(t, ws), before) {
					t.Errorf("%s for %s: a dry run changed the workspace", flow.name, h)
				}
			})
		}
	}
}

// TestDryRun_GhcpLegacyWorkspace_ExistingUnrecordedAgentFile_StaysAConflictWithoutReplacement
// verifies an X.agent.md the manifest does not record, existing beside the recorded X.md, keeps
// its conflict (it is not listed as a replacement) next to the removal of the X.md, and stays
// untouched.
func TestDryRun_GhcpLegacyWorkspace_ExistingUnrecordedAgentFile_StaysAConflictWithoutReplacement(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range drgFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, _ := gamLegacyWorkspace(t, h)
				newRel, oldRel := gamRel(flow.subject, ".agent.md"), gamRel(flow.subject, ".md")
				gamWrite(t, ws, newRel, "stale user copy\n")
				before := crSnapshotDir(t, ws)
				review := newDrReview(domain.SkippedOne)

				// Act
				summary, err := flow.run(t, ws, h, review)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				if summary.DryRunPlan == nil {
					t.Fatalf("%s for %s: summary carries no dry-run plan", flow.name, h)
				}
				p := *summary.DryRunPlan
				if drgReplacesPath(p, newRel) {
					t.Errorf("%s for %s: replacements %+v list the unrecorded %s, which is a conflict", flow.name, h, p.Replacements, newRel)
				}
				if it, ok := gplAgentItem(p, flow.subject); !ok || it.Action != domain.ActionConflict {
					t.Errorf("%s for %s: item for %s = %+v (found %t), want a conflict", flow.name, h, newRel, it, ok)
				}
				if !gplRemovesPath(p, oldRel) {
					t.Errorf("%s for %s: removals %+v do not list %s", flow.name, h, p.Removals, oldRel)
				}
				if !reflect.DeepEqual(crSnapshotDir(t, ws), before) {
					t.Errorf("%s for %s: a dry run changed the workspace", flow.name, h)
				}
			})
		}
	}
}

// TestRealRun_GhcpLegacyWorkspace_WithoutConfirmation_RefusesAndMigratesNothing verifies the
// same workspace without --dry-run and without confirmation is refused and left as it was.
func TestRealRun_GhcpLegacyWorkspace_WithoutConfirmation_RefusesAndMigratesNothing(t *testing.T) {
	for _, h := range gamTargets {
		t.Run(h, func(t *testing.T) {
			// Arrange
			ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{})
			before := crSnapshotDir(t, ws)
			review := newDrReview(domain.SkippedOne)

			// Act
			_, err := parityServiceWith(t, func(d *app.Deps) { d.Interaction = review }).Update(
				context.Background(), app.UpdateRequest{HarnessID: h, WorkspacePath: ws, AgentModels: gamModels(),
					ConflictDefault: domain.DecisionSkip})

			// Assert
			if err == nil || !strings.Contains(err.Error(), app.ErrPlanNotConfirmed.Error()) {
				t.Fatalf("err = %v, want ErrPlanNotConfirmed", err)
			}
			if !reflect.DeepEqual(crSnapshotDir(t, ws), before) {
				t.Error("a refused run changed the workspace")
			}
		})
	}
}
