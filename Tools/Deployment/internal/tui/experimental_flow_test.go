package tui

// experimental_flow_test.go verifies the plan review of the root model for experimental
// harnesses: the warning is on screen before the user answers, also when the harness was
// pre-selected through the initial request (picker skipped), confirming proceeds without an
// extra step, a transform whose only experimental harness is the source still warns, and
// after a declined plan restarts the selection the next review shows the warning again.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/stackservicetest"
	"mosaic-deploy/internal/domain"
)

var experimentalFlowHarness = domain.HarnessRef{
	ID: "exp-harness", DisplayName: "Expo Harness", Tier: domain.TierBuiltin, Usable: true,
	ExperimentalReason: "only agent deployment works here",
}

const experimentalFlowWarning = "Expo Harness is experimental: only agent deployment works here"

func experimentalFlowPlan(mode domain.RunMode, target domain.HarnessRef) domain.Plan {
	return domain.Plan{
		Mode:          mode,
		WorkspacePath: "/workspace",
		Harness:       target,
		Items: []domain.PlanItem{
			{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "new-agent"}, Action: domain.ActionCreate, TargetPath: "Agents/new-agent.md"},
		},
		ExperimentalHarnesses: []domain.ExperimentalHarness{{
			HarnessID: experimentalFlowHarness.ID, DisplayName: experimentalFlowHarness.DisplayName,
			Reason: experimentalFlowHarness.ExperimentalReason,
		}},
	}
}

func experimentalFlowSvc() *stubNavService {
	return &stubNavService{
		MockStackService: stackservicetest.MockStackService{},
		harnesses:        []domain.HarnessRef{experimentalFlowHarness},
	}
}

// preselectedExperimentalModel builds a model whose harness came from the initial request, so
// the picker is skipped, and sizes it so every review line is visible.
func preselectedExperimentalModel() *rootModel {
	m := newRootModel(context.Background(), experimentalFlowSvc(), Options{
		Theme: DefaultTheme(),
		InitialRequest: &app.DeployRequest{
			HarnessID:     experimentalFlowHarness.ID,
			WorkspacePath: "/workspace",
		},
	})
	m.Update(tea.WindowSizeMsg{Width: 240, Height: 60})
	m.screen = screenRunning
	return m
}

func viewShowsWarning(m *rootModel, want string) (bool, string) {
	view := m.View()
	return strings.Count(strings.Join(strings.Fields(view), " "), want) == 1, view
}

// TestRootModel_Review_PreselectedExperimentalHarness_ShowsWarningOnce verifies the review of a
// run whose harness skipped the picker shows the warning exactly once.
func TestRootModel_Review_PreselectedExperimentalHarness_ShowsWarningOnce(t *testing.T) {
	m := preselectedExperimentalModel()
	if m.selections.harnessID != experimentalFlowHarness.ID {
		t.Fatalf("precondition: harness not pre-selected, got %q", m.selections.harnessID)
	}

	m.Update(buildReviewMsg(experimentalFlowPlan(domain.ModeDeployWorkspace, experimentalFlowHarness)))

	if ok, view := viewShowsWarning(m, experimentalFlowWarning); !ok {
		t.Errorf("review does not show %q exactly once:\n%s", experimentalFlowWarning, view)
	}
}

// TestRootModel_Review_ExperimentalWarning_ConfirmProceedsWithoutExtraStep verifies one Enter
// sends the confirm answer and returns the model to the running screen.
func TestRootModel_Review_ExperimentalWarning_ConfirmProceedsWithoutExtraStep(t *testing.T) {
	m := preselectedExperimentalModel()
	msg := buildReviewMsg(experimentalFlowPlan(domain.ModeDeployWorkspace, experimentalFlowHarness))
	m.Update(msg)

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	select {
	case ans := <-msg.reply:
		if ans.confirmAns.Status != domain.Answered || !ans.confirmAns.Confirm {
			t.Errorf("answer = {%v, %v}, want {Answered, true}", ans.confirmAns.Status, ans.confirmAns.Confirm)
		}
	default:
		t.Fatal("no answer sent after Enter on an experimental review")
	}
	if m.screen != screenRunning {
		t.Errorf("screen = %v after confirming, want screenRunning", m.screen)
	}
}

// TestRootModel_Review_TransformWithExperimentalSource_ShowsSourceWarning verifies a transform
// review whose target is not experimental shows the source's warning.
func TestRootModel_Review_TransformWithExperimentalSource_ShowsSourceWarning(t *testing.T) {
	m := preselectedExperimentalModel()
	m.selections.mode = domain.ModeTransformHarness
	target := domain.HarnessRef{ID: "claude-code", DisplayName: "Claude Code", Usable: true}

	m.Update(buildReviewMsg(experimentalFlowPlan(domain.ModeTransformHarness, target)))

	if ok, view := viewShowsWarning(m, experimentalFlowWarning); !ok {
		t.Errorf("transform review does not show the source's warning exactly once:\n%s", view)
	}
}

// TestRootModel_Review_AfterDeclineAndRestart_ShowsWarningAgain verifies that declining the
// plan restarts selection as a new operation and the next review warns again, once.
func TestRootModel_Review_AfterDeclineAndRestart_ShowsWarningAgain(t *testing.T) {
	m := preselectedExperimentalModel()
	first := buildReviewMsg(experimentalFlowPlan(domain.ModeDeployWorkspace, experimentalFlowHarness))
	m.Update(first)
	if ok, view := viewShowsWarning(m, experimentalFlowWarning); !ok {
		t.Fatalf("first review does not show the warning exactly once:\n%s", view)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}) // decline
	m.Update(runErrorMsg{err: app.ErrPlanNotConfirmed})          // app reports the declined plan
	if m.screen != screenHarness {
		t.Fatalf("screen = %v after decline, want screenHarness (restart)", m.screen)
	}
	m.screen = screenRunning // a second run reaches its review
	m.Update(buildReviewMsg(experimentalFlowPlan(domain.ModeDeployWorkspace, experimentalFlowHarness)))

	if ok, view := viewShowsWarning(m, experimentalFlowWarning); !ok {
		t.Errorf("review after restart does not show the warning exactly once:\n%s", view)
	}
}

// TestRootModel_Review_NonExperimentalPlan_ShowsNoExperimentalWarning verifies a plan without
// experimental harnesses is reviewed without any experimental text.
func TestRootModel_Review_NonExperimentalPlan_ShowsNoExperimentalWarning(t *testing.T) {
	m := newRoutingModel()
	m.Update(tea.WindowSizeMsg{Width: 240, Height: 60})
	plan := experimentalFlowPlan(domain.ModeDeployWorkspace, flowTestHarness)
	plan.ExperimentalHarnesses = nil

	m.Update(buildReviewMsg(plan))

	if view := m.View(); strings.Contains(strings.ToLower(view), "experimental") {
		t.Errorf("non-experimental review mentions experimental:\n%s", view)
	}
}

