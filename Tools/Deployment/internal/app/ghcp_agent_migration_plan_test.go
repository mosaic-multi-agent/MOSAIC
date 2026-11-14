package app_test

// ghcp_agent_migration_plan_test.go verifies what the plan, the review and the questions of the
// four agent-writing deploy flows say about agents an earlier deploy recorded as X.md on both
// GHCP targets: such an agent is a rename (plus any staleness), never a new artifact, and is
// never asked for a model. The content a migrated file ends up with is not checked here.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// gplFlow is one flow with one selection, and the agents it must show as renames on a
// workspace holding all four parity sample agents as recorded X.md files.
type gplFlow struct {
	name    string
	renamed []string // first entry is the one the staleness and unrecorded cases act on
	run     func(t *testing.T, ws, harness string, c *gamReviewCapture, dry bool) error
}

func gplFlows() []gplFlow {
	ctx := context.Background()
	deployNew := func(utility []string) func(*testing.T, string, string, *gamReviewCapture, bool) error {
		return func(t *testing.T, ws, h string, c *gamReviewCapture, dry bool) error {
			_, err := parityServiceWith(t, gamCaptureDeps(c)).DeployNew(ctx, app.DeployRequest{
				HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
				WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: utility,
				AutoConfirmPlan: true, DryRun: dry, ConflictDefault: domain.DecisionSkip})
			return err
		}
	}
	deployAgents := func(utility []string) func(*testing.T, string, string, *gamReviewCapture, bool) error {
		return func(t *testing.T, ws, h string, c *gamReviewCapture, dry bool) error {
			_, err := parityServiceWith(t, gamCaptureDeps(c)).DeployAgents(ctx, app.DeployAgentsRequest{
				HarnessID: h, WorkspacePath: ws, Scope: domain.ScopeProject,
				SubagentIDs: []string{"parity-runner"}, UtilityAgentIDs: utility,
				InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
				AutoConfirmPlan: true, DryRun: dry, ConflictDefault: domain.DecisionSkip})
			return err
		}
	}
	return []gplFlow{
		{name: "DeployNew/full", run: deployNew([]string{"parity-helper"}),
			renamed: []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper"}},
		{name: "DeployNew/subset", run: deployNew([]string{}),
			renamed: []string{"parity-runner", "orchestrator", "orchestrator-script"}},
		{name: "DeployAgents/full", run: deployAgents([]string{"parity-helper"}),
			renamed: []string{"parity-runner", "parity-helper"}},
		{name: "DeployAgents/subset", run: deployAgents([]string{}),
			renamed: []string{"parity-runner"}},
		{name: "Update/full",
			renamed: []string{"parity-runner", "orchestrator", "orchestrator-script", "parity-helper"},
			run: func(t *testing.T, ws, h string, c *gamReviewCapture, dry bool) error {
				_, err := parityServiceWith(t, gamCaptureDeps(c)).Update(ctx, app.UpdateRequest{
					HarnessID: h, WorkspacePath: ws, AutoConfirmPlan: true, DryRun: dry,
					ConflictDefault: domain.DecisionSkip})
				return err
			}},
		{name: "UpdateWorkflows", renamed: []string{"orchestrator"},
			run: func(t *testing.T, ws, h string, c *gamReviewCapture, dry bool) error {
				_, err := parityServiceWith(t, gamCaptureDeps(c)).UpdateWorkflows(ctx, app.WorkflowUpdateRequest{
					HarnessID: h, WorkspacePath: ws, WorkflowIDs: []string{"parity-flow"},
					AutoConfirmPlan: true, DryRun: dry, ConflictDefault: domain.DecisionSkip})
				return err
			}},
	}
}

// gplReviewedPlan returns the one plan the run showed for review.
func gplReviewedPlan(t *testing.T, c *gamReviewCapture) domain.Plan {
	t.Helper()
	if len(c.plans) != 1 {
		t.Fatalf("plan was reviewed %d times, want exactly once", len(c.plans))
	}
	return c.plans[0]
}

// gplAgentItem returns the plan item for the agent key's planned X.agent.md path.
func gplAgentItem(p domain.Plan, key string) (domain.PlanItem, bool) {
	want := gamRel(key, ".agent.md")
	for _, it := range p.Items {
		if it.Ref.Kind == domain.ArtifactAgent && strings.ReplaceAll(it.TargetPath, "\\", "/") == want {
			return it, true
		}
	}
	return domain.PlanItem{}, false
}

func gplRemovesPath(p domain.Plan, rel string) bool {
	for _, r := range p.Removals {
		if strings.ReplaceAll(r.Path, "\\", "/") == rel {
			return true
		}
	}
	return false
}

// gplAssertRename checks the item is an update reported as a rename from the agent's X.md,
// with the staleness suffix only when wantStale.
func gplAssertRename(t *testing.T, p domain.Plan, key string, wantStale bool) {
	t.Helper()
	legacy := gamRel(key, ".md")
	it, ok := gplAgentItem(p, key)
	if !ok {
		t.Errorf("%s: plan has no item for %s", key, gamRel(key, ".agent.md"))
		return
	}
	if it.Action != domain.ActionUpdate {
		t.Errorf("%s: action = %q, want %q (a rename, not a create or conflict)", key, it.Action, domain.ActionUpdate)
	}
	if it.RenamedFrom != legacy {
		t.Errorf("%s: RenamedFrom = %q, want %q", key, it.RenamedFrom, legacy)
	}
	reason := "renamed from " + legacy
	hasRename := false
	for _, r := range it.Reasons {
		if r == reason {
			hasRename = true
		}
		if strings.Contains(r, reason) && r != reason {
			t.Errorf("%s: reason %q joins the rename with another cause; each cause is its own reason", key, r)
		}
	}
	if !hasRename {
		t.Errorf("%s: reasons = %q, want the separate reason %q", key, it.Reasons, reason)
	}
	if wantStale && (len(it.Stale) == 0 || len(it.Reasons) < 2) {
		t.Errorf("%s: reasons = %q, stale = %v, want the rename plus the staleness as further reasons and version deltas", key, it.Reasons, it.Stale)
	}
	if !wantStale && len(it.Stale) != 0 {
		t.Errorf("%s: stale = %v, want no deltas", key, it.Stale)
	}
	if !gplRemovesPath(p, legacy) {
		t.Errorf("%s: plan does not remove the legacy %s", key, legacy)
	}
}

// gplAssertNoNewAgent checks no agent item of the plan is a create or says "new artifact".
func gplAssertNoNewAgent(t *testing.T, p domain.Plan) {
	t.Helper()
	for _, it := range p.Items {
		if it.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		if it.Action == domain.ActionCreate || strings.Contains(reasonText(it), "new artifact") {
			t.Errorf("agent %s is shown as new: action = %q, reason = %q", it.Ref.Key, it.Action, reasonText(it))
		}
	}
}

// TestGhcpFlows_RecordedLegacyAgents_AreShownAsRenamesNotNewArtifacts covers every flow and
// selection on both targets, several legacy agents per run, including the agent with no id.
func TestGhcpFlows_RecordedLegacyAgents_AreShownAsRenamesNotNewArtifacts(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{})
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, true)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				p := gplReviewedPlan(t, c)
				gplAssertNoNewAgent(t, p)
				for _, key := range flow.renamed {
					gplAssertRename(t, p, key, false)
				}
			})
		}
	}
}

// TestGhcpFlows_StaleRecordedLegacyAgent_IsShownAsRenamePlusStaleness verifies an X.md older
// than the catalog is reported as a rename that is also stale, while its siblings are not.
func TestGhcpFlows_StaleRecordedLegacyAgent_IsShownAsRenamePlusStaleness(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				stale := flow.renamed[0]
				ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{stale: []string{stale}})
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, true)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				p := gplReviewedPlan(t, c)
				gplAssertNoNewAgent(t, p)
				for _, key := range flow.renamed {
					gplAssertRename(t, p, key, key == stale)
				}
			})
		}
	}
}

// TestGhcpFlows_RecordedLegacyAgents_AreNotAskedForAModel verifies no per-agent model question
// is asked for an agent whose recorded X.md exists, with no model supplied by the request.
func TestGhcpFlows_RecordedLegacyAgents_AreNotAskedForAModel(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{})
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, true)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				for _, key := range flow.renamed {
					if c.WasAsked(domain.QAgentModel, key) {
						t.Errorf("%s: a model was asked for although its recorded legacy file holds one", key)
					}
				}
			})
		}
	}
}

// TestGhcpDeployFlows_TierUsedOnlyByLegacyAgents_IsNotAsked verifies deploy and agents ask no
// tier model question when every agent of the run is a recorded legacy agent.
func TestGhcpDeployFlows_TierUsedOnlyByLegacyAgents_IsNotAsked(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			if !strings.HasPrefix(flow.name, "Deploy") {
				continue
			}
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{})
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, true)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				for _, call := range c.Calls() {
					if call.ID == domain.QTierModel {
						t.Errorf("tier model question %q asked although only legacy agents are deployed", call.Subject)
					}
				}
			})
		}
	}
}

// TestGhcpFlows_UnrecordedLegacyFile_IsLeftAloneAndNotShownAsRename verifies an X.md the
// manifest has no record of stays untouched, is not removed by the plan, and is not reported as
// a rename, while the recorded agents of the same run still are.
func TestGhcpFlows_UnrecordedLegacyFile_IsLeftAloneAndNotShownAsRename(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				loose := flow.renamed[0]
				ws, legacy := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{unrecorded: []string{loose}})
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, false)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				p := gplReviewedPlan(t, c)
				if it, ok := gplAgentItem(p, loose); ok && (it.RenamedFrom != "" || strings.Contains(reasonText(it), "renamed from")) {
					t.Errorf("%s: unrecorded %s reported as a rename: %q", loose, gamRel(loose, ".md"), reasonText(it))
				}
				if gplRemovesPath(p, gamRel(loose, ".md")) {
					t.Errorf("%s: plan removes the unrecorded %s", loose, gamRel(loose, ".md"))
				}
				if !gamExists(ws, gamRel(loose, ".md")) || string(gamRead(t, ws, gamRel(loose, ".md"))) != string(legacy[loose]) {
					t.Errorf("%s: the unrecorded %s was removed or changed", loose, gamRel(loose, ".md"))
				}
				for _, key := range flow.renamed[1:] {
					gplAssertRename(t, p, key, false)
				}
			})
		}
	}
}

// TestGhcpFlows_RecordedLegacyAgents_AreNotRenamedOrWrittenBeforeReview verifies the agents
// folder still holds only the X.md files when the plan is shown.
func TestGhcpFlows_RecordedLegacyAgents_AreNotRenamedOrWrittenBeforeReview(t *testing.T) {
	for _, h := range gamTargets {
		for _, flow := range gplFlows() {
			t.Run(h+"/"+flow.name, func(t *testing.T) {
				// Arrange
				ws, _ := gamPlantedLegacyWorkspace(t, h, gamLegacyOpts{})
				c := gamNewReviewCapture(ws)

				// Act
				err := flow.run(t, ws, h, c, false)

				// Assert
				if err != nil {
					t.Fatalf("%s for %s: %v", flow.name, h, err)
				}
				if len(c.folder) != 1 {
					t.Fatalf("plan was reviewed %d times, want once", len(c.folder))
				}
				have := map[string]bool{}
				for _, name := range c.folder[0] {
					have[name] = true
					if strings.HasSuffix(name, ".agent.md") {
						t.Errorf("%s existed when the plan was shown", name)
					}
				}
				for _, a := range parityAgents {
					if !have[a.key+".md"] {
						t.Errorf("%s.md was gone when the plan was shown", a.key)
					}
				}
			})
		}
	}
}
