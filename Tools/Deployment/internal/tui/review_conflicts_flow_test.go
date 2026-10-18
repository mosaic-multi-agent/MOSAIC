package tui

// review_conflicts_flow_test.go verifies the plan review flow in the root model: the conflict
// report of the plan is on screen before the user answers, and declining after seeing it sends
// the app a decline, which the app turns into a run that writes nothing.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
)

func conflictPlan() domain.Plan {
	return domain.Plan{
		Mode:          domain.ModeDeployWorkspace,
		WorkspacePath: "/workspace",
		Harness:       domain.HarnessRef{ID: "claude-code", DisplayName: "Claude Code"},
		Items: []domain.PlanItem{
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "new-agent"}, Action: domain.ActionCreate, TargetPath: "Agents/new-agent.md"},
		},
		ConflictReport: domain.ConflictReport{Lines: []domain.ConflictLine{{
			Item:              domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: "test-runner"},
			Winner:            domain.CatalogIdentity{ID: "user-a", Version: "2.0.0"},
			Shadowed:          []domain.CatalogIdentity{{ID: "mosaic-default", Version: "1.0.0"}},
			Severity:          domain.ConflictWarning,
			AffectedWorkflows: []string{"quick-fix"},
		}}},
	}
}

const conflictPlanLine = "warning: agent:test-runner from user-a 2.0.0 shadows mosaic-default 1.0.0; affects workflows: quick-fix"

func reviewModel() *rootModel {
	m := newRoutingModel()
	m.Update(tea.WindowSizeMsg{Width: 240, Height: 60})
	return m
}

// TestRootModel_Review_ShowsTheConflictReportBeforeTheUserAnswers verifies that the plan review
// on screen carries the report line and has sent no answer yet.
func TestRootModel_Review_ShowsTheConflictReportBeforeTheUserAnswers(t *testing.T) {
	m := reviewModel()
	msg := buildReviewMsg(conflictPlan())

	m.Update(msg)
	view := m.View()

	if !strings.Contains(strings.Join(strings.Fields(view), " "), conflictPlanLine) {
		t.Errorf("plan review does not show the conflict report line %q:\n%s", conflictPlanLine, view)
	}
	select {
	case <-msg.reply:
		t.Error("an answer was sent before the user acted")
	default:
	}
}

// TestRootModel_Review_DecliningAfterTheReportSendsADecline verifies that n on the review sends
// an answered, non-confirming reply, so the app run ends without writing.
func TestRootModel_Review_DecliningAfterTheReportSendsADecline(t *testing.T) {
	m := reviewModel()
	msg := buildReviewMsg(conflictPlan())
	m.Update(msg)
	if !strings.Contains(strings.Join(strings.Fields(m.View()), " "), conflictPlanLine) {
		t.Fatalf("the report is not on screen:\n%s", m.View())
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	select {
	case ans := <-msg.reply:
		if ans.confirmAns.Confirm {
			t.Error("declining the review must not confirm the plan")
		}
	default:
		t.Fatal("no answer was sent after declining")
	}
}
