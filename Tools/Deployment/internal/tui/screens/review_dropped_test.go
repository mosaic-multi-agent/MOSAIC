package screens_test

// review_dropped_test.go verifies that the plan review screen lists every manifest entry that
// was dropped before planning, with its path and reason, before the user confirms the run.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func droppedPlan() domain.Plan {
	p := removalPlan()
	p.DroppedEntries = []domain.DroppedEntry{
		{Entry: domain.ManifestEntry{Harness: "ghcp-cli", Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "ghost"},
			TargetPath: ".github/agents/ghost.agent.md"}, Reason: domain.DropReasonMissing},
		{Entry: domain.ManifestEntry{Harness: "opencode", Ref: domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "logger"},
			TargetPath: ".opencode/plugins/logger.ts"}, Reason: domain.DropReasonMissing},
		{Entry: domain.ManifestEntry{Ref: domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "notes"},
			TargetPath: "docs/notes.md"}, Reason: domain.DropReasonUnattributed},
	}
	return p
}

func TestReviewScreen_DroppedEntries_ListedWithHarnessKindKeyPathAndReason(t *testing.T) {
	view := reviewView(droppedPlan(), 240, 80)

	for _, want := range []string{
		"[DROPPED]      ghcp-cli agent:ghost .github/agents/ghost.agent.md",
		"[DROPPED]      opencode hook:logger .opencode/plugins/logger.ts",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks the line %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, reviewDetailIndent+domain.DropReasonMissing) {
		t.Errorf("view lacks the reason %q on its own indented line:\n%s", domain.DropReasonMissing, view)
	}
}

func TestReviewScreen_DroppedEntryWithoutHarness_OmitsTheHarnessAndShowsTheMigrationReason(t *testing.T) {
	view := reviewView(droppedPlan(), 240, 80)

	if !strings.Contains(view, "[DROPPED]      skill:notes docs/notes.md") {
		t.Errorf("view lacks the harness-less line:\n%s", view)
	}
	if !strings.Contains(view, reviewDetailIndent+domain.DropReasonUnattributed) {
		t.Errorf("view lacks the old-format reason on its own indented line:\n%s", view)
	}
}

func TestReviewScreen_DroppedEntries_FollowTheRemovals(t *testing.T) {
	view := reviewView(droppedPlan(), 240, 80)

	removal := strings.Index(view, "[REPLACE]")
	dropped := strings.Index(view, "[DROPPED]")
	if removal < 0 || dropped < 0 || dropped < removal {
		t.Errorf("[DROPPED] (at %d) must come after the removals and replacements (at %d):\n%s", dropped, removal, view)
	}
}

func TestReviewScreen_DroppedEntries_UseTheWarningStyleForTheEntryAndMutedForTheReason(t *testing.T) {
	styles := coloredStyles()

	view := screens.NewReviewScreen(droppedPlan(), 240, 80, styles).View()

	entry := lineWith(view, ".github/agents/ghost.agent.md")
	if !strings.Contains(entry, rolePrefix(styles.Warning)) {
		t.Errorf("entry line is not in the warning style: %q", entry)
	}
	reason := lineWith(view, domain.DropReasonMissing)
	if !strings.Contains(reason, rolePrefix(styles.Muted)) {
		t.Errorf("reason line is not in the muted style: %q", reason)
	}
}

func TestReviewScreen_PlanWithOnlyDroppedEntries_StillListsThem(t *testing.T) {
	p := domain.Plan{
		Mode:          domain.ModeUpdateWorkspace,
		Harness:       domain.HarnessRef{ID: "opencode", DisplayName: "OpenCode"},
		WorkspacePath: "/ws",
		DroppedEntries: []domain.DroppedEntry{{
			Entry: domain.ManifestEntry{Harness: "opencode", Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "ghost"},
				TargetPath: ".opencode/agents/ghost.md"},
			Reason: domain.DropReasonMissing}},
	}

	view := reviewView(p, 240, 80)

	if !strings.Contains(view, "[DROPPED]      opencode agent:ghost .opencode/agents/ghost.md") {
		t.Errorf("view of a plan whose only content is a dropped entry lacks it:\n%s", view)
	}
}

func TestReviewScreen_NoDroppedEntries_ShowsNoDroppedSection(t *testing.T) {
	view := reviewView(removalPlan(), 240, 80)

	if strings.Contains(view, "[DROPPED]") {
		t.Errorf("view of a plan without dropped entries shows a dropped line:\n%s", view)
	}
}
