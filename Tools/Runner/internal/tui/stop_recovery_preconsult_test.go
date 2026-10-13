package tui

// stop_recovery_preconsult_test.go verifies the TUI's recovery behaviour when
// the orchestrator stopped the run at pre-consultation: the stop screen offers
// only a Retry that resumes the run (which repeats pre-consultation), worded
// for pre-consultation, with no routing-decision wording and no manual-dispatch
// choice.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens"
	"mosaic-run/internal/tui/screens/decision"
)

const preConsultStopReason = "Scope is unclear; ask the user first"

func newPreConsultStopScreen(reason string) *decision.StopScreen {
	return decision.NewPreConsultationStopScreen(reason, 80, 24, screens.Styles{})
}

// preConsultStopOutcome is the outcome of a run stopped at pre-consultation.
func preConsultStopOutcome(reason string) domain.RunOutcome {
	return domain.RunOutcome{
		Status:              domain.RunStoppedByConsultant,
		StopReason:          reason,
		PreConsultationStop: true,
	}
}

func assertNoRoutingOrManualWording(t *testing.T, view string) {
	t.Helper()
	lower := strings.ToLower(view)
	if strings.Contains(lower, "routing decision") {
		t.Errorf("view must not word the choice as re-invoking a routing decision:\n%s", view)
	}
	if strings.Contains(lower, "manual dispatch") {
		t.Errorf("view must not offer manual dispatch:\n%s", view)
	}
}

func TestPreConsultStopScreen_ViewShowsReason(t *testing.T) {
	view := newPreConsultStopScreen(preConsultStopReason).View()
	if !containsStr(view, preConsultStopReason) {
		t.Errorf("view does not contain stop reason %q:\n%s", preConsultStopReason, view)
	}
}

func TestPreConsultStopScreen_ViewIdentifiesOrchestratorPreConsultationStop(t *testing.T) {
	view := strings.ToLower(newPreConsultStopScreen(preConsultStopReason).View())
	if !strings.Contains(view, "pre-consultation") {
		t.Errorf("view does not mention pre-consultation:\n%s", view)
	}
	if !strings.Contains(view, "orchestrator") {
		t.Errorf("view does not say the orchestrator stopped the run:\n%s", view)
	}
}

func TestPreConsultStopScreen_ViewHasNoRoutingOrManualDispatchWording(t *testing.T) {
	assertNoRoutingOrManualWording(t, newPreConsultStopScreen(preConsultStopReason).View())
}

func TestPreConsultStopScreen_ViewOffersRetry(t *testing.T) {
	view := newPreConsultStopScreen(preConsultStopReason).View()
	if !containsAny(view, "Retry", "retry") {
		t.Errorf("view does not offer Retry:\n%s", view)
	}
}

func TestPreConsultStopScreen_EnterReportsRetry(t *testing.T) {
	s := newPreConsultStopScreen(preConsultStopReason)

	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !s.Done() {
		t.Error("Done() = false after Enter; want true")
	}
	if s.Choice() != decision.StopChoiceRetry {
		t.Errorf("Choice() = %q, want %q", s.Choice(), decision.StopChoiceRetry)
	}
}

func TestPreConsultStopScreen_CursorCannotMoveToSecondOption(t *testing.T) {
	s := newPreConsultStopScreen(preConsultStopReason)

	for i := 0; i < 3; i++ {
		s.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if s.Choice() != decision.StopChoiceRetry {
		t.Errorf("Choice() = %q after Down presses; want %q (only one option exists)",
			s.Choice(), decision.StopChoiceRetry)
	}
}

func TestPreConsultStopScreen_EscSetsBackWithoutDone(t *testing.T) {
	s := newPreConsultStopScreen(preConsultStopReason)

	s.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !s.Back() {
		t.Error("Back() = false after Esc; want true")
	}
	if s.Done() {
		t.Error("Done() = true after Esc; want false")
	}
}

// ---------------------------------------------------------------------------
// Root model
// ---------------------------------------------------------------------------

func TestPreConsultStopRecovery_ShowsStopScreenWithReasonAndPreConsultationWording(t *testing.T) {
	m := newTestModel()

	m.Update(runDoneMsg{outcome: preConsultStopOutcome(preConsultStopReason)})

	if m.screen != screenStop {
		t.Fatalf("screen = %v, want screenStop (%v)", m.screen, screenStop)
	}
	view := m.View()
	if !containsStr(view, preConsultStopReason) {
		t.Errorf("view does not contain stop reason %q:\n%s", preConsultStopReason, view)
	}
	if !strings.Contains(strings.ToLower(view), "pre-consultation") {
		t.Errorf("view does not mention pre-consultation:\n%s", view)
	}
	assertNoRoutingOrManualWording(t, view)
}

func TestPreConsultStopRecovery_RetryResumesWithoutManualDispatch(t *testing.T) {
	cap := &capturingDispatchSession{outcome: domain.RunOutcome{Status: domain.RunCompleted}}
	m := newCapturingStopModel(cap)
	m.Update(runDoneMsg{outcome: preConsultStopOutcome(preConsultStopReason)})
	if m.screen != screenStop {
		t.Fatalf("precondition: screen = %v, want screenStop", m.screen)
	}

	// Press Down first: with a single option the cursor must stay on Retry.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.startSession()()

	if len(cap.configs) == 0 {
		t.Fatal("session.Start was not called after choosing Retry")
	}
	cfg := cap.configs[len(cap.configs)-1]
	if cfg.ManualDispatch {
		t.Error("RunConfig.ManualDispatch = true after Retry at a pre-consultation stop; want false")
	}
}

func TestPreConsultStopRecovery_RetryLeavesStopScreen(t *testing.T) {
	m := newTestModel()
	m.Update(runDoneMsg{outcome: preConsultStopOutcome(preConsultStopReason)})
	if m.screen != screenStop {
		t.Fatalf("precondition: screen = %v, want screenStop", m.screen)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen == screenStop {
		t.Error("screen = screenStop after Retry; the run must resume")
	}
}

// TestPreConsultStopRecovery_RoutingStopKeepsManualDispatch guards that the
// marker, not the status alone, selects the variant.
func TestPreConsultStopRecovery_RoutingStopKeepsManualDispatch(t *testing.T) {
	m := newTestModel()

	m.Update(runDoneMsg{outcome: domain.RunOutcome{
		Status:     domain.RunStoppedByConsultant,
		StopReason: "routing stop",
	}})

	if !containsAny(m.View(), "Manual dispatch") {
		t.Errorf("routing stop must still offer manual dispatch:\n%s", m.View())
	}
}
