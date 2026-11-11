package app_test

// update_ghcp_dangling_legacy_test.go specifies the update preview on a workspace modelled on a
// long-lived multi-harness deployment: an OpenCode deployment transformed to GHCP CLI, whose
// manifest also records .github/agents/X.md entries of the target whose files are gone. When
// nothing changed, no GHCP agent is an update, a conflict, a replacement or a removal.
//
// Fixtures are the tool-local parity catalog; nothing here reads the repository Catalog.

import (
	"context"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// udSettledWorkspace builds the workspace: OpenCode deployed, transformed to the GHCP target,
// settled by one update, then given a recorded X.md entry (file missing) for every agent.
func udSettledWorkspace(t *testing.T, target string) string {
	t.Helper()
	ws := parityDeploy(t, hOpen)
	parityTransform(t, ws, hOpen, target)
	if _, err := parityService(t).Update(context.Background(), app.UpdateRequest{
		HarnessID: target, WorkspacePath: ws, AgentModels: gamModels(),
		AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip}); err != nil {
		t.Fatalf("settling update for %s: %v", target, err)
	}
	m := gamManifest(t, ws)
	for _, a := range parityAgents {
		m.Entries = append(m.Entries, domain.ManifestEntry{
			Harness:     target,
			Ref:         domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: a.key},
			TargetPath:  gamRel(a.key, ".md"),
			Version:     "0.0.1",
			ContentHash: manifest.Hash([]byte("a file that is gone\n")),
			DeployedAt:  time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
		})
	}
	gamSaveManifest(t, ws, m)
	return ws
}

// udPreview runs an update as a dry run and returns the plan it showed.
func udPreview(t *testing.T, ws, target string) domain.Plan {
	t.Helper()
	c := gamNewReviewCapture(ws)
	sum, err := parityServiceWith(t, gamCaptureDeps(c)).Update(context.Background(), app.UpdateRequest{
		HarnessID: target, WorkspacePath: ws, AgentModels: gamModels(),
		DryRun: true, AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
	if err != nil {
		t.Fatalf("update preview for %s: %v", target, err)
	}
	if sum.DryRunPlan != nil {
		return *sum.DryRunPlan
	}
	return gplReviewedPlan(t, c)
}

// TestUpdatePreview_TransformedWorkspaceWithDanglingLegacyEntries_ListsNoGhcpAgentUpdate
// verifies the preview lists every sample agent as unchanged, and no removal or replacement.
func TestUpdatePreview_TransformedWorkspaceWithDanglingLegacyEntries_ListsNoGhcpAgentUpdate(t *testing.T) {
	for _, target := range gamTargets {
		t.Run(target, func(t *testing.T) {
			// Arrange
			ws := udSettledWorkspace(t, target)

			// Act
			p := udPreview(t, ws, target)

			// Assert
			for _, a := range parityAgents {
				it, ok := gplAgentItem(p, a.key)
				if !ok {
					t.Errorf("%s: preview has no item for %s", a.key, gamRel(a.key, ".agent.md"))
					continue
				}
				if it.Action != domain.ActionUnchanged {
					t.Errorf("%s: action = %q (%q), want unchanged when nothing changed", a.key, it.Action, it.Reasons)
				}
			}
			if len(p.Removals) != 0 || len(p.Replacements) != 0 {
				t.Errorf("removals = %+v, replacements = %+v, want none: no legacy file exists", p.Removals, p.Replacements)
			}
		})
	}
}

// TestUpdate_TransformedWorkspaceWithDanglingLegacyEntries_RewritesNoGhcpAgent verifies a real
// update over the same workspace leaves every GHCP agent file as it was and makes no backup.
func TestUpdate_TransformedWorkspaceWithDanglingLegacyEntries_RewritesNoGhcpAgent(t *testing.T) {
	for _, target := range gamTargets {
		t.Run(target, func(t *testing.T) {
			// Arrange
			ws := udSettledWorkspace(t, target)
			before := map[string]string{}
			for _, a := range parityAgents {
				before[a.key] = string(gamRead(t, ws, gamRel(a.key, ".agent.md")))
			}

			// Act
			sum, err := parityService(t).Update(context.Background(), app.UpdateRequest{
				HarnessID: target, WorkspacePath: ws, AgentModels: gamModels(),
				AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})

			// Assert
			if err != nil {
				t.Fatalf("update for %s: %v", target, err)
			}
			for _, a := range parityAgents {
				newRel := gamRel(a.key, ".agent.md")
				if got := string(gamRead(t, ws, newRel)); got != before[a.key] {
					t.Errorf("%s: the update rewrote an agent that did not change", newRel)
				}
				if n := len(gamBackups(t, ws, newRel)) + len(gamBackups(t, ws, gamRel(a.key, ".md"))); n != 0 {
					t.Errorf("%s: %d backups made although nothing changed", a.key, n)
				}
			}
			for _, act := range sum.Actions {
				if act.Taken != domain.TakenUnchanged && act.Ref.Kind == domain.ArtifactAgent {
					t.Errorf("%s: taken %q, want unchanged", act.TargetPath, act.Taken)
				}
			}
		})
	}
}
