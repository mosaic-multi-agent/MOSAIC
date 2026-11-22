package screens_test

// promoteagents_test.go verifies PromoteAgentScreen, the multi-select list of the batch
// promote: rows show the agent key and the "already in <layer>" hint, the keys are those of
// the Deploy agents list, SelectedIDs follows option order, and an empty confirmation is
// refused with a validation message.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/screens"
)

func promoteAgentOptions() []domain.Option {
	return []domain.Option{
		{ID: "alpha.md", Label: "alpha-agent", Description: "alpha.md"},
		{ID: "beta.agent.md", Label: "beta-agent", Description: "beta.agent.md", Hint: "already in Team Layer (team)"},
		{ID: "gamma.md", Label: "gamma-agent", Description: "gamma.md"},
	}
}

func newPromoteAgents() *screens.PromoteAgentScreen {
	return screens.NewPromoteAgentScreen(promoteAgentOptions(), 100, 30, plainStyles())
}

func paSpace(s *screens.PromoteAgentScreen) {
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
}

func paKey(s *screens.PromoteAgentScreen, k tea.KeyType) { s.Update(tea.KeyMsg{Type: k}) }

func TestPromoteAgentScreen_View_ShowsEveryAgentKey(t *testing.T) {
	s := newPromoteAgents()

	view := s.View()

	for _, o := range promoteAgentOptions() {
		if !strings.Contains(view, o.Label) {
			t.Errorf("view does not show the agent key %q:\n%s", o.Label, view)
		}
	}
}

func TestPromoteAgentScreen_View_ShowsTheExistingLayerHintOnItsRow(t *testing.T) {
	s := newPromoteAgents()

	view := s.View()

	if !strings.Contains(view, "already in Team Layer (team)") {
		t.Errorf("view does not show the hint of the agent that exists in a layer:\n%s", view)
	}
}

func TestPromoteAgentScreen_DetailPane_ShowsDescriptionAndHintOfFocusedRow(t *testing.T) {
	s := newPromoteAgents()
	paKey(s, tea.KeyDown) // focus beta-agent

	view := s.View()

	// The description of a row is shown only in the detail pane, and only for the focused row.
	if !strings.Contains(view, "beta.agent.md") {
		t.Errorf("detail pane does not show the description of the focused row:\n%s", view)
	}
	if strings.Contains(view, "gamma.md") {
		t.Errorf("detail pane shows the description of a row that is not focused:\n%s", view)
	}
	// The hint is on the row itself and repeated in the detail pane.
	if strings.Count(view, "already in Team Layer (team)") < 2 {
		t.Errorf("hint of the focused row is not shown in the detail pane next to the row:\n%s", view)
	}
}

func TestPromoteAgentScreen_SpaceToggles_EnterConfirmsSelectionInOptionOrder(t *testing.T) {
	s := newPromoteAgents()
	paKey(s, tea.KeyDown)
	paKey(s, tea.KeyDown)
	paSpace(s) // gamma
	paKey(s, tea.KeyUp)
	paKey(s, tea.KeyUp)
	paSpace(s) // alpha

	paKey(s, tea.KeyEnter)

	if !s.Done() {
		t.Fatal("Done() = false after enter with a selection; want true")
	}
	got := s.SelectedIDs()
	if len(got) != 2 || got[0] != "alpha.md" || got[1] != "gamma.md" {
		t.Errorf("SelectedIDs() = %v; want [alpha.md gamma.md] (option order, file base names)", got)
	}
}

func TestPromoteAgentScreen_SpaceTwice_DeselectsTheRow(t *testing.T) {
	s := newPromoteAgents()
	paSpace(s)
	paSpace(s)
	paKey(s, tea.KeyDown)
	paSpace(s)

	paKey(s, tea.KeyEnter)

	got := s.SelectedIDs()
	if len(got) != 1 || got[0] != "beta.agent.md" {
		t.Errorf("SelectedIDs() = %v; want only [beta.agent.md]", got)
	}
}

func TestPromoteAgentScreen_JKMoveTheCursorLikeArrows(t *testing.T) {
	s := newPromoteAgents()
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	paSpace(s)
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	paSpace(s)

	paKey(s, tea.KeyEnter)

	got := s.SelectedIDs()
	if len(got) != 2 || got[0] != "alpha.md" || got[1] != "beta.agent.md" {
		t.Errorf("SelectedIDs() = %v; want [alpha.md beta.agent.md]", got)
	}
}

func TestPromoteAgentScreen_EnterWithNothingSelected_DoesNotConfirmAndExplains(t *testing.T) {
	s := newPromoteAgents()

	paKey(s, tea.KeyEnter)

	if s.Done() {
		t.Error("Done() = true after enter with nothing selected; want false")
	}
	if !strings.Contains(s.View(), screens.MsgNoAgentsSelected) {
		t.Errorf("view does not show the no-agents-selected message:\n%s", s.View())
	}
}

func TestPromoteAgentScreen_Esc_GoesBackWithoutConfirming(t *testing.T) {
	s := newPromoteAgents()
	paSpace(s)

	paKey(s, tea.KeyEsc)

	if !s.Back() {
		t.Error("Back() = false after esc; want true")
	}
	if s.Done() {
		t.Error("Done() = true after esc; a partial selection must not be confirmed")
	}
}

func TestPromoteAgentScreen_Summary_StatesTheSelectedCount(t *testing.T) {
	s := newPromoteAgents()
	paSpace(s)
	paKey(s, tea.KeyDown)
	paSpace(s)

	view := s.View()

	if !strings.Contains(view, "Selected 2") {
		t.Errorf("summary does not state the selected count:\n%s", view)
	}
}

func TestPromoteAgentScreen_Reset_ClearsDoneAndBack(t *testing.T) {
	s := newPromoteAgents()
	paSpace(s)
	paKey(s, tea.KeyEnter)

	s.Reset()

	if s.Done() || s.Back() {
		t.Error("Done() or Back() still true after Reset()")
	}
}

// ---------------------------------------------------------------------------
// Layout: the multi-select height budget
// ---------------------------------------------------------------------------

// promoteAgentsLayout describes the screen for the shared layout scenarios. The screen title is
// not fixed by the contract; it must name the promotion. withProv gives every row the
// "already in" hint, the only per-row relabelling this screen has.
func promoteAgentsLayout() layoutScreen {
	return layoutScreen{
		name:       "promote-agents",
		title:      "romote",
		hasSummary: true,
		topMarks:   []string{layoutRowName(0)},
		build: func(w, h, n int, withProv bool) layoutDriver {
			opts := make([]domain.Option, n)
			for i := range opts {
				opts[i] = domain.Option{ID: layoutRowKey(i) + ".md", Label: layoutRowName(i), Description: layoutDescription()}
				if withProv {
					opts[i].Hint = "already in catalog-x"
				}
			}
			return screens.NewPromoteAgentScreen(opts, w, h, plainStyles())
		},
	}
}

func TestPromoteAgentScreenLayout_FitsWhateverIsSelected(t *testing.T) {
	layoutRunFitsWhateverSelected(t, promoteAgentsLayout())
}

func TestPromoteAgentScreenLayout_CursorStaysVisibleWhileScrolling(t *testing.T) {
	layoutRunCursorStaysVisible(t, promoteAgentsLayout())
}

func TestPromoteAgentScreenLayout_ResizeKeepsFit(t *testing.T) {
	layoutRunResizeKeepsFit(t, promoteAgentsLayout())
}

func TestPromoteAgentScreenLayout_HintsOnEveryRowKeepFit(t *testing.T) {
	layoutRunProvenanceKeepsFit(t, promoteAgentsLayout())
}

func TestPromoteAgentScreenLayout_ShortListIsNotPadded(t *testing.T) {
	layoutRunShortListIsNotPadded(t, promoteAgentsLayout())
}

func TestPromoteAgentScreenLayout_ValidationMessageKeepsFit(t *testing.T) {
	layoutRunValidationFits(t, promoteAgentsLayout())
}
