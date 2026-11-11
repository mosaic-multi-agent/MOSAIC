package screens_test

// review_removals_test.go verifies that the plan review screen shows every removal and
// replacement of a plan, with its own label, before the user approves.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func removalPlan() domain.Plan {
	return domain.Plan{
		Mode:          domain.ModeTransformHarness,
		Harness:       domain.HarnessRef{ID: "vscode-ghcp", DisplayName: "VS Code GHCP"},
		WorkspacePath: "/ws",
		Items: []domain.PlanItem{
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "writer"},
				TargetPath: ".github/agents/writer.agent.md", Action: domain.ActionCreate},
		},
		Removals: []domain.Removal{
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "reviewer"},
				Path: ".github/agents/reviewer.md", DropEntry: true, Reason: "legacy form migrated"},
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "logger"},
				Path: ".github/hooks/old.json"},
		},
		Replacements: []domain.Replacement{
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactHook, Key: "logger"},
				Path: ".github/hooks/mosaic-logger.json", Reason: "registration replaced"},
		},
	}
}

func reviewView(p domain.Plan, width, height int) string {
	return screens.NewReviewScreen(p, width, height, plainStyles()).View()
}

func TestReviewScreen_Removals_ShownWithRemoveLabel(t *testing.T) {
	view := reviewView(removalPlan(), 200, 60)

	for _, want := range []string{
		"[REMOVE]", "agent:reviewer", ".github/agents/reviewer.md", "legacy form migrated",
		"hook:logger", ".github/hooks/old.json",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestReviewScreen_RemovalsAndReplacements_ShownInEveryMode(t *testing.T) {
	for _, mode := range []domain.RunMode{domain.ModeDeployAgents, domain.ModeDeployWorkspace, domain.ModeTransformHarness} {
		t.Run(string(mode), func(t *testing.T) {
			p := removalPlan()
			p.Mode = mode

			view := reviewView(p, 200, 60)

			for _, want := range []string{"[REMOVE]", "[REPLACE]", "2 to remove", "1 to replace"} {
				if !strings.Contains(view, want) {
					t.Errorf("mode %v: view lacks %q:\n%s", mode, want, view)
				}
			}
		})
	}
}

func TestReviewScreen_Replacements_ShownWithReplaceLabel(t *testing.T) {
	view := reviewView(removalPlan(), 200, 60)

	for _, want := range []string{"[REPLACE]", ".github/hooks/mosaic-logger.json", "registration replaced"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestReviewScreen_RemovalsAndReplacements_CountsHeaderIncludesThem(t *testing.T) {
	view := reviewView(removalPlan(), 200, 60)

	if !strings.Contains(view, "2 to remove") {
		t.Errorf("counts header lacks %q:\n%s", "2 to remove", view)
	}
	if !strings.Contains(view, "1 to replace (current file backed up first)") {
		t.Errorf("counts header lacks %q:\n%s", "1 to replace (current file backed up first)", view)
	}
}

func TestReviewScreen_Removals_ComeAfterItemsBeforeGaps_RemovalsFirst(t *testing.T) {
	p := removalPlan()
	p.Gaps = []domain.Gap{{Kind: domain.GapSkippedFile, Subject: "logger"}}
	view := reviewView(p, 200, 60)

	create := strings.Index(view, "[CREATE]")
	remove := strings.Index(view, "[REMOVE]")
	replace := strings.Index(view, "[REPLACE]")
	gaps := strings.Index(view, "gap(s)")
	if create < 0 || remove < 0 || replace < 0 || gaps < 0 {
		t.Fatalf("missing sections (create=%d remove=%d replace=%d gaps=%d):\n%s", create, remove, replace, gaps, view)
	}
	if !(create < remove && remove < replace && replace < gaps) {
		t.Errorf("order must be items, removals, replacements, gaps:\n%s", view)
	}
}

func TestReviewScreen_OnlyRemovals_DoesNotSayNothingToDo(t *testing.T) {
	p := removalPlan()
	p.Items = []domain.PlanItem{{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "writer"},
		TargetPath: ".github/agents/writer.agent.md", Action: domain.ActionUnchanged}}
	p.Replacements = nil

	view := reviewView(p, 200, 60)

	if strings.Contains(view, "Nothing to do") {
		t.Errorf("plan with removals must not say there is nothing to do:\n%s", view)
	}
	if !strings.Contains(view, "[REMOVE]") {
		t.Errorf("removals must be shown:\n%s", view)
	}
}

func TestReviewScreen_NoRemovals_RendersWithoutRemovalText(t *testing.T) {
	p := removalPlan()
	p.Removals = nil
	p.Replacements = nil

	view := reviewView(p, 200, 60)

	for _, s := range []string{"[REMOVE]", "[REPLACE]", "to remove", "to replace"} {
		if strings.Contains(view, s) {
			t.Errorf("plan without removals must not show %q:\n%s", s, view)
		}
	}
}

func TestReviewScreen_ManyRemovals_ScrollToLastEntry(t *testing.T) {
	p := removalPlan()
	p.Replacements = nil
	p.Removals = nil
	for i := 0; i < 40; i++ {
		p.Removals = append(p.Removals, domain.Removal{
			Ref:  domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: fmt.Sprintf("agent%02d", i)},
			Path: fmt.Sprintf(".github/agents/agent%02d.md", i)})
	}
	s := screens.NewReviewScreen(p, 120, 20, plainStyles())
	if strings.Contains(s.View(), "agent39") {
		t.Fatalf("last entry should not fit on the first page:\n%s", s.View())
	}

	for i := 0; i < 200; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	if !strings.Contains(s.View(), "agent39") {
		t.Errorf("last removal must be reachable by scrolling:\n%s", s.View())
	}
}
