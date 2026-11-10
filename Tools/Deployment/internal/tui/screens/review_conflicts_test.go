package screens_test

// review_conflicts_test.go verifies that the plan review shows the catalog-stack conflict report
// of the plan before anything is written: one line per entry with the item, the winning and the
// shadowed catalogs, the severity and the affected workflows, the orchestrator pair as a single
// line, and no report section when the report is empty. The line text follows the human
// conflict line of the command line.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func conflictIdentity(id, version string) domain.CatalogIdentity {
	return domain.CatalogIdentity{ID: id, Name: id, Version: version}
}

// reviewWithReport builds a review screen tall and wide enough to show every line unwrapped.
func reviewWithReport(report domain.ConflictReport) *screens.ReviewScreen {
	p := planWithItems(domain.ModeDeployWorkspace, []domain.PlanItem{
		{Ref: agentRef("new-agent"), Action: domain.ActionCreate, TargetPath: "Agents/new-agent.agent.md"},
	})
	p.ConflictReport = report
	return screens.NewReviewScreen(p, 240, 100, plainStyles())
}

func requireLine(t *testing.T, view, want string) {
	t.Helper()
	if !strings.Contains(collapseWhitespace(view), want) {
		t.Errorf("review does not show the report line %q:\n%s", want, view)
	}
}

// TestReviewScreen_ConflictReport_ShowsWarningLineWithAffectedWorkflows verifies the line of an
// undeclared override: severity, item, winner, shadowed catalog and affected workflows.
func TestReviewScreen_ConflictReport_ShowsWarningLineWithAffectedWorkflows(t *testing.T) {
	s := reviewWithReport(domain.ConflictReport{Lines: []domain.ConflictLine{{
		Item:              domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: "test-runner"},
		Winner:            conflictIdentity("user-a", "2.0.0"),
		Shadowed:          []domain.CatalogIdentity{conflictIdentity("mosaic-default", "1.0.0")},
		Severity:          domain.ConflictWarning,
		AffectedWorkflows: []string{"feature-dev", "quick-fix"},
	}}})

	requireLine(t, s.View(), "warning: agent:test-runner from user-a 2.0.0 shadows mosaic-default 1.0.0; affects workflows: feature-dev, quick-fix")
}

// TestReviewScreen_ConflictReport_DeclaredOverrideIsInfo verifies that a declared override is
// shown with the info severity and, without affected workflows, without that clause.
func TestReviewScreen_ConflictReport_DeclaredOverrideIsInfo(t *testing.T) {
	s := reviewWithReport(domain.ConflictReport{Lines: []domain.ConflictLine{{
		Item:     domain.CatalogItem{Kind: domain.CatalogItemWorkflow, Key: "quick-fix"},
		Winner:   conflictIdentity("user-a", "2.0.0"),
		Shadowed: []domain.CatalogIdentity{conflictIdentity("mosaic-default", "1.0.0")},
		Severity: domain.ConflictInfo,
	}}})

	view := s.View()

	requireLine(t, view, "info: workflow:quick-fix from user-a 2.0.0 shadows mosaic-default 1.0.0")
	if strings.Contains(view, "affects workflows") {
		t.Errorf("a line without affected workflows must not carry the clause:\n%s", view)
	}
}

// TestReviewScreen_ConflictReport_ListsEveryShadowedCatalog verifies that all shadowed catalogs
// of one item appear on its line, in order.
func TestReviewScreen_ConflictReport_ListsEveryShadowedCatalog(t *testing.T) {
	s := reviewWithReport(domain.ConflictReport{Lines: []domain.ConflictLine{{
		Item:   domain.CatalogItem{Kind: domain.CatalogItemHook, Key: "pre-commit-guard"},
		Winner: conflictIdentity("top", "3.0.0"),
		Shadowed: []domain.CatalogIdentity{
			conflictIdentity("middle", "2.0.0"), conflictIdentity("mosaic-default", "1.0.0"),
		},
		Severity: domain.ConflictWarning,
	}}})

	requireLine(t, s.View(), "warning: hook:pre-commit-guard from top 3.0.0 shadows middle 2.0.0, mosaic-default 1.0.0")
}

// TestReviewScreen_ConflictReport_OrchestratorPairIsOneLine verifies that the orchestrator pair
// is a single line without a key and names the workflows it affects.
func TestReviewScreen_ConflictReport_OrchestratorPairIsOneLine(t *testing.T) {
	s := reviewWithReport(domain.ConflictReport{Lines: []domain.ConflictLine{{
		Item:              domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair},
		Winner:            conflictIdentity("user-a", "2.0.0"),
		Shadowed:          []domain.CatalogIdentity{conflictIdentity("mosaic-default", "1.0.0")},
		Severity:          domain.ConflictWarning,
		AffectedWorkflows: []string{"quick-fix"},
	}}})

	view := s.View()

	requireLine(t, view, "warning: orchestrator-pair from user-a 2.0.0 shadows mosaic-default 1.0.0; affects workflows: quick-fix")
	if n := strings.Count(view, "orchestrator-pair"); n != 1 {
		t.Errorf("the orchestrator pair appears %d times, want one line:\n%s", n, view)
	}
}

// TestReviewScreen_ConflictReport_EveryEntryHasItsOwnLine verifies that two entries give two
// distinct lines.
func TestReviewScreen_ConflictReport_EveryEntryHasItsOwnLine(t *testing.T) {
	s := reviewWithReport(domain.ConflictReport{Lines: []domain.ConflictLine{
		{
			Item: domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: "agent-one"}, Winner: conflictIdentity("user-a", "2.0.0"),
			Shadowed: []domain.CatalogIdentity{conflictIdentity("mosaic-default", "1.0.0")}, Severity: domain.ConflictWarning,
		},
		{
			Item: domain.CatalogItem{Kind: domain.CatalogItemSkill, Key: "skill-two"}, Winner: conflictIdentity("user-a", "2.0.0"),
			Shadowed: []domain.CatalogIdentity{conflictIdentity("mosaic-default", "1.0.0")}, Severity: domain.ConflictInfo,
		},
	}})

	view := s.View()

	requireLine(t, view, "warning: agent:agent-one from user-a 2.0.0 shadows mosaic-default 1.0.0")
	requireLine(t, view, "info: skill:skill-two from user-a 2.0.0 shadows mosaic-default 1.0.0")
}

// TestReviewScreen_ConflictReport_AppearsBeforeConfirmation verifies that the report is on the
// screen while no answer has been given yet.
func TestReviewScreen_ConflictReport_AppearsBeforeConfirmation(t *testing.T) {
	s := reviewWithReport(domain.ConflictReport{Lines: []domain.ConflictLine{{
		Item: domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: "test-runner"}, Winner: conflictIdentity("user-a", "2.0.0"),
		Shadowed: []domain.CatalogIdentity{conflictIdentity("mosaic-default", "1.0.0")}, Severity: domain.ConflictWarning,
	}}})

	view := s.View()

	if s.Done() {
		t.Fatal("review is already answered before the user acted")
	}
	requireLine(t, view, "warning: agent:test-runner from user-a 2.0.0 shadows mosaic-default 1.0.0")
}

// TestReviewScreen_EmptyConflictReport_ShowsNoReportSection verifies that a plan without
// shadowed items shows no conflict line at all.
func TestReviewScreen_EmptyConflictReport_ShowsNoReportSection(t *testing.T) {
	s := reviewWithReport(domain.ConflictReport{})

	view := s.View()

	for _, fragment := range []string{"shadows", "info:", "warning:", "affects workflows"} {
		if strings.Contains(view, fragment) {
			t.Errorf("an empty report must leave no trace of %q:\n%s", fragment, view)
		}
	}
}

// TestReviewScreen_ConflictReport_DecliningAfterSeeingItAnswersNo verifies that the user can
// stop on the report: n gives a final answer that does not confirm.
func TestReviewScreen_ConflictReport_DecliningAfterSeeingItAnswersNo(t *testing.T) {
	s := reviewWithReport(domain.ConflictReport{Lines: []domain.ConflictLine{{
		Item: domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: "test-runner"}, Winner: conflictIdentity("user-a", "2.0.0"),
		Shadowed: []domain.CatalogIdentity{conflictIdentity("mosaic-default", "1.0.0")}, Severity: domain.ConflictWarning,
	}}})
	requireLine(t, s.View(), "shadows mosaic-default 1.0.0")

	done := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if !done || !s.Done() {
		t.Fatal("n did not finish the review")
	}
	if s.Answer().Confirm {
		t.Error("declining after the report must not confirm the plan")
	}
}
