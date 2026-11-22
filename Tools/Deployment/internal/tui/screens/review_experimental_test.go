package screens_test

// review_experimental_test.go verifies that the plan review shows a warning line for every
// experimental harness the plan exposes (including a transform's source), shows it once, keeps
// the review confirmable exactly as before, and renders a plan without experimental
// information unchanged.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func experimentalPlan(mode domain.RunMode, exp ...domain.ExperimentalHarness) domain.Plan {
	p := planWithItems(mode, []domain.PlanItem{
		{Ref: agentRef("new-agent"), Action: domain.ActionCreate, TargetPath: "Agents/new-agent.agent.md"},
	})
	p.ExperimentalHarnesses = exp
	return p
}

func codexExperimental() domain.ExperimentalHarness {
	return domain.ExperimentalHarness{
		HarnessID: "codex", DisplayName: "Codex",
		Reason: "agents deploy to .codex/agents; no MOSAIC orchestration",
	}
}

// wideReview renders the plan on a screen wide and tall enough to show every line unwrapped.
func wideReview(p domain.Plan) *screens.ReviewScreen {
	return screens.NewReviewScreen(p, 240, 100, plainStyles())
}

// TestReviewScreen_ExperimentalHarness_ShowsWarningOnce verifies the warning text appears
// exactly once in the review.
func TestReviewScreen_ExperimentalHarness_ShowsWarningOnce(t *testing.T) {
	e := codexExperimental()
	s := wideReview(experimentalPlan(domain.ModeDeployWorkspace, e))

	view := collapseWhitespace(s.View())

	if got := strings.Count(view, warningText(e)); got != 1 {
		t.Errorf("warning %q appears %d times, want exactly 1:\n%s", warningText(e), got, view)
	}
}

// TestReviewScreen_ExperimentalHarness_WarningFollowsHeaderBeforeCounts verifies the placement:
// after the Mode/Harness header line and before the item counts.
func TestReviewScreen_ExperimentalHarness_WarningFollowsHeaderBeforeCounts(t *testing.T) {
	e := codexExperimental()
	s := wideReview(experimentalPlan(domain.ModeDeployWorkspace, e))

	view := collapseWhitespace(s.View())

	header := strings.Index(view, "Mode:")
	warn := strings.Index(view, warningText(e))
	counts := strings.Index(view, "to create")
	if header < 0 || warn < 0 || counts < 0 {
		t.Fatalf("missing header (%d), warning (%d) or counts (%d):\n%s", header, warn, counts, view)
	}
	if !(header < warn && warn < counts) {
		t.Errorf("order wrong: header=%d warning=%d counts=%d", header, warn, counts)
	}
}

// TestReviewScreen_ExperimentalHarness_MultiLineReasonIsCollapsedToOneLine verifies a reason
// with inner newlines is shown with its whitespace collapsed.
func TestReviewScreen_ExperimentalHarness_MultiLineReasonIsCollapsedToOneLine(t *testing.T) {
	e := domain.ExperimentalHarness{HarnessID: "x", DisplayName: "Xtool", Reason: "first part\nsecond   part"}
	s := wideReview(experimentalPlan(domain.ModeDeployWorkspace, e))

	view := s.View()

	if !strings.Contains(view, "Xtool is experimental: first part second part") {
		t.Errorf("multi-line reason not shown as one collapsed line:\n%s", view)
	}
}

// TestReviewScreen_ExperimentalHarness_ConfirmStillProceeds verifies confirming needs no extra
// step: Enter and 'y' complete with a confirm answer.
func TestReviewScreen_ExperimentalHarness_ConfirmStillProceeds(t *testing.T) {
	keys := map[string]tea.KeyMsg{
		"enter": {Type: tea.KeyEnter},
		"y":     {Type: tea.KeyRunes, Runes: []rune{'y'}},
	}
	for name, key := range keys {
		t.Run(name, func(t *testing.T) {
			s := wideReview(experimentalPlan(domain.ModeDeployWorkspace, codexExperimental()))

			s.Update(key)

			if !s.Done() {
				t.Fatal("Done() = false after confirming an experimental plan")
			}
			if ans := s.Answer(); ans.Status != domain.Answered || !ans.Confirm {
				t.Errorf("Answer() = {%v, %v}, want {Answered, true}", ans.Status, ans.Confirm)
			}
		})
	}
}

// TestReviewScreen_TransformSourceOnlyExperimental_ShowsSourceWarning verifies a transform plan
// whose target is not experimental shows the source's warning.
func TestReviewScreen_TransformSourceOnlyExperimental_ShowsSourceWarning(t *testing.T) {
	e := codexExperimental()
	p := experimentalPlan(domain.ModeTransformHarness, e)
	p.Harness = domain.HarnessRef{ID: "claude-code", DisplayName: "Claude Code"}

	view := collapseWhitespace(wideReview(p).View())

	if !strings.Contains(view, warningText(e)) {
		t.Errorf("transform review does not show the source's warning %q:\n%s", warningText(e), view)
	}
}

// TestReviewScreen_TwoExperimentalHarnesses_ShowsBothInOrder verifies each entry gets a line,
// in slice order.
func TestReviewScreen_TwoExperimentalHarnesses_ShowsBothInOrder(t *testing.T) {
	first := domain.ExperimentalHarness{HarnessID: "a", DisplayName: "Alpha", Reason: "alpha reason"}
	second := domain.ExperimentalHarness{HarnessID: "b", DisplayName: "Beta", Reason: "beta reason"}
	s := wideReview(experimentalPlan(domain.ModeTransformHarness, first, second))

	view := collapseWhitespace(s.View())

	i, j := strings.Index(view, warningText(first)), strings.Index(view, warningText(second))
	if i < 0 || j < 0 || i > j {
		t.Errorf("warnings missing or out of order (first=%d second=%d):\n%s", i, j, view)
	}
}

// TestReviewScreen_ExperimentalWarning_CountsTowardScrollBudget verifies the warning lines are
// part of the scrollable content, so a short screen reports more lines than without them.
func TestReviewScreen_ExperimentalWarning_CountsTowardScrollBudget(t *testing.T) {
	without := manyItemPlan(30)
	with := manyItemPlan(30)
	with.ExperimentalHarnesses = []domain.ExperimentalHarness{codexExperimental(),
		{HarnessID: "b", DisplayName: "Beta", Reason: "beta reason"}}

	totalWithout, okWithout := scrollIndicatorTotal(screens.NewReviewScreen(without, 80, 10, plainStyles()).View())
	totalWith, okWith := scrollIndicatorTotal(screens.NewReviewScreen(with, 80, 10, plainStyles()).View())

	if !okWithout || !okWith {
		t.Fatalf("scroll indicator missing for a 30-item plan at height 10 (without=%v, with=%v)", okWithout, okWith)
	}
	if totalWith <= totalWithout {
		t.Errorf("scroll total with warnings = %d, want more than without (%d)", totalWith, totalWithout)
	}
}

// manyItemPlan builds a deploy plan with n create items, enough to overflow a short screen.
func manyItemPlan(n int) domain.Plan {
	items := make([]domain.PlanItem, 0, n)
	for i := 0; i < n; i++ {
		key := "agent-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		items = append(items, domain.PlanItem{
			Ref: agentRef(key), Action: domain.ActionCreate, TargetPath: "Agents/" + key + ".agent.md"})
	}
	return planWithItems(domain.ModeDeployWorkspace, items)
}

// TestReviewScreen_ExperimentalHarness_MultiItemPlan_ShowsWarningOnce verifies the warning is
// shown once per review, not once per plan item.
func TestReviewScreen_ExperimentalHarness_MultiItemPlan_ShowsWarningOnce(t *testing.T) {
	e := codexExperimental()
	p := manyItemPlan(3)
	p.Items[1].Action = domain.ActionUnchanged
	p.ExperimentalHarnesses = []domain.ExperimentalHarness{e}

	view := collapseWhitespace(wideReview(p).View())

	if got := strings.Count(view, warningText(e)); got != 1 {
		t.Errorf("warning %q appears %d times for a 3-item plan, want exactly 1:\n%s", warningText(e), got, view)
	}
}

// TestReviewScreen_ExperimentalHarness_NarrowWidth_ReasonWrapsWithoutTruncation verifies a long
// reason at a narrow width is wrapped, not cut off.
func TestReviewScreen_ExperimentalHarness_NarrowWidth_ReasonWrapsWithoutTruncation(t *testing.T) {
	long := strings.Repeat("word ", 30) + "final-marker"
	e := domain.ExperimentalHarness{HarnessID: "x", DisplayName: "Xtool", Reason: long}

	view := collapseWhitespace(screens.NewReviewScreen(
		experimentalPlan(domain.ModeDeployWorkspace, e), 40, 100, plainStyles()).View())

	if !strings.Contains(view, "final-marker") {
		t.Errorf("long reason truncated at narrow width:\n%s", view)
	}
}

// TestReviewScreen_NoExperimentalInformation_RendersAsBefore verifies a plan without
// experimental harnesses shows no experimental text and renders the same whether the field is
// nil or empty.
func TestReviewScreen_NoExperimentalInformation_RendersAsBefore(t *testing.T) {
	nilPlan := experimentalPlan(domain.ModeDeployWorkspace)
	emptyPlan := experimentalPlan(domain.ModeDeployWorkspace)
	emptyPlan.ExperimentalHarnesses = []domain.ExperimentalHarness{}

	viewNil := wideReview(nilPlan).View()
	viewEmpty := wideReview(emptyPlan).View()

	if viewNil != viewEmpty {
		t.Errorf("nil and empty ExperimentalHarnesses render differently:\n%s\n---\n%s", viewNil, viewEmpty)
	}
	if strings.Contains(strings.ToLower(viewNil), "experimental") {
		t.Errorf("review of a non-experimental plan mentions experimental:\n%s", viewNil)
	}
}

// warningText is the contractual one-line warning sentence for an entry whose reason has
// single-spaced words.
func warningText(e domain.ExperimentalHarness) string {
	return e.DisplayName + " is experimental: " + e.Reason
}
