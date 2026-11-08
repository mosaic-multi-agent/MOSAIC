package runconfig

// config_hitl_timeout_test.go verifies the HITL invocation timeout step: the
// second timeout question, asked right after the non-HITL timeout wherever that
// one is asked, defaulting to 4h, validated like the first, and carried in the
// configuration selection next to the non-HITL value.

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/tui/screens"
)

// hitlPromptMarker is the (lower-cased) name of the HITL timeout question.
const hitlPromptMarker = "hitl invocation timeout"

// collapsedText is the lower-cased view with runs of whitespace collapsed, so
// a prompt wrapped over several lines is still found.
func collapsedText(s *ConfigScreen) string {
	return strings.Join(strings.Fields(screenText(s)), " ")
}

// showsHITLTimeout reports whether the HITL timeout question is on screen.
func showsHITLTimeout(s *ConfigScreen) bool {
	return strings.Contains(collapsedText(s), hitlPromptMarker)
}

// showsNonHITLTimeout reports whether the first (non-HITL) timeout question is
// on screen.
func showsNonHITLTimeout(s *ConfigScreen) bool {
	text := collapsedText(s)
	return strings.Contains(text, "invocation timeout") && !strings.Contains(text, hitlPromptMarker)
}

// answerHITLTimeout replaces whatever the HITL timeout input holds with text and
// confirms it. It works whether the input starts empty or prefilled.
func answerHITLTimeout(s *ConfigScreen, text string) {
	clearInput(s)
	typeText(s, text)
	pressKey(s, tea.KeyEnter)
}

// newScreenAtNonHITLTimeoutStep returns a new-run screen on the first timeout
// question.
func newScreenAtNonHITLTimeoutStep(t *testing.T) *ConfigScreen {
	t.Helper()
	s := newConfigScreenAtHarnessStep()
	pressKey(s, tea.KeyEnter) // harness -> timeout
	if !showsNonHITLTimeout(s) {
		t.Fatalf("precondition: the non-HITL timeout question is not shown; view:\n%s", s.View())
	}
	return s
}

// newScreenAtHITLTimeoutStep returns a new-run screen on the HITL timeout
// question, the non-HITL timeout having been answered with 30m.
func newScreenAtHITLTimeoutStep(t *testing.T) *ConfigScreen {
	t.Helper()
	s := newScreenAtNonHITLTimeoutStep(t)
	typeText(s, "30m")
	pressKey(s, tea.KeyEnter)
	if !showsHITLTimeout(s) {
		t.Fatalf("the HITL timeout question does not follow the non-HITL timeout; view:\n%s", s.View())
	}
	return s
}

func TestConfigScreen_HITLTimeout_IsAskedRightAfterTheNonHITLTimeout(t *testing.T) {
	s := newScreenAtHITLTimeoutStep(t)

	text := collapsedText(s)

	if !strings.Contains(text, "human_in_the_loop: true") {
		t.Errorf("the HITL timeout question does not name human_in_the_loop: true dispatches; view:\n%s", s.View())
	}
}

func TestConfigScreen_HITLTimeout_OffersFourHoursAsTheDefault(t *testing.T) {
	s := newScreenAtHITLTimeoutStep(t)

	if !strings.Contains(collapsedText(s), "4h") {
		t.Errorf("the HITL timeout question does not show the 4h default; view:\n%s", s.View())
	}
}

func TestConfigScreen_HITLTimeout_TakesAValidDurationAndKeepsTheNonHITLValue(t *testing.T) {
	s := newScreenAtHITLTimeoutStep(t)

	answerHITLTimeout(s, "2h")

	if showsHITLTimeout(s) {
		t.Fatalf("the HITL timeout question is still shown after a valid answer; view:\n%s", s.View())
	}
	sel := s.Selection()
	if sel.HITLTimeout != 2*time.Hour {
		t.Errorf("HITLTimeout = %v, want 2h", sel.HITLTimeout)
	}
	if sel.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want the non-HITL answer 30m", sel.Timeout)
	}
}

func TestConfigScreen_HITLTimeout_RejectsInvalidInputLikeTheNonHITLStep(t *testing.T) {
	for _, bad := range []string{"abc", "0", "0s", "-5m", "10"} {
		t.Run(bad, func(t *testing.T) {
			s := newScreenAtHITLTimeoutStep(t)

			answerHITLTimeout(s, bad)

			if !showsHITLTimeout(s) {
				t.Errorf("input %q moved past the HITL timeout question; view:\n%s", bad, s.View())
			}
		})
	}
}

func TestConfigScreen_HITLTimeout_RejectsEmptyInputLikeTheNonHITLStep(t *testing.T) {
	s := newScreenAtHITLTimeoutStep(t)
	clearInput(s)

	pressKey(s, tea.KeyEnter)

	if !showsHITLTimeout(s) {
		t.Errorf("an empty answer moved past the HITL timeout question; view:\n%s", s.View())
	}
}

func TestConfigScreen_HITLTimeout_BackReturnsToTheNonHITLTimeout(t *testing.T) {
	s := newScreenAtHITLTimeoutStep(t)

	pressKey(s, tea.KeyEsc)

	if !showsNonHITLTimeout(s) {
		t.Errorf("esc from the HITL timeout did not return to the non-HITL timeout; view:\n%s", s.View())
	}
	if s.Back() {
		t.Error("Back() = true: esc from the HITL timeout must not leave the configuration screen")
	}
}

func TestConfigScreen_HITLTimeout_BackFromTheNextStepReturnsToTheHITLTimeout(t *testing.T) {
	s := newScreenAtHITLTimeoutStep(t)
	answerHITLTimeout(s, "2h")

	pressKey(s, tea.KeyEsc)

	if !showsHITLTimeout(s) {
		t.Errorf("esc from the step after the HITL timeout did not return to it; view:\n%s", s.View())
	}
}

func TestConfigScreen_HITLTimeout_BackFromVersionDriftReturnsToTheHITLTimeout(t *testing.T) {
	s := NewConfigScreen(80, 24, screens.Styles{})
	s.SetIsNewRun(false)
	s.SetVersionDriftInfo("0.9", "1.0")
	s.SetNeedsRunnerAdoption(false)
	s.SetPreselectedHarness(harness.CLISelections()[0].ID)
	typeText(s, "30m")
	pressKey(s, tea.KeyEnter)
	answerHITLTimeout(s, "2h")
	if !strings.Contains(collapsedText(s), "workflow version drift") {
		t.Fatalf("precondition: the version-drift question does not follow the HITL timeout; view:\n%s", s.View())
	}

	pressKey(s, tea.KeyEsc)

	if !showsHITLTimeout(s) {
		t.Errorf("esc from version drift did not return to the HITL timeout; view:\n%s", s.View())
	}
}

func TestConfigScreen_HITLTimeout_SelectionsCarryBothValues(t *testing.T) {
	s := NewConfigScreen(80, 24, screens.Styles{})
	driveConfigScreenModeSelect(s, 0)
	pressKey(s, tea.KeyEnter) // harness
	typeText(s, "45m")
	pressKey(s, tea.KeyEnter)
	answerHITLTimeout(s, "6h")

	for i := 0; i < 30 && !s.Done(); i++ {
		advanceOnce(s)
	}

	if !s.Done() {
		t.Fatal("the wizard did not finish")
	}
	sel := s.Selection()
	if sel.Timeout != 45*time.Minute || sel.HITLTimeout != 6*time.Hour {
		t.Errorf("Timeout = %v, HITLTimeout = %v, want 45m and 6h", sel.Timeout, sel.HITLTimeout)
	}
}

func TestConfigScreen_HITLTimeout_ReopenedWizardKeepsBothAnswers(t *testing.T) {
	s := newRecordedResume("1.0", "1.0")
	typeText(s, "50m")
	pressKey(s, tea.KeyEnter)
	answerHITLTimeout(s, "3h")
	if !s.Done() {
		t.Fatalf("precondition: the recorded resume is not finished; view:\n%s", s.View())
	}

	s.Reopen()

	if !showsHITLTimeout(s) {
		t.Fatalf("a reopened resume wizard is not on its last question, the HITL timeout; view:\n%s", s.View())
	}
	if sel := s.Selection(); sel.Timeout != 50*time.Minute || sel.HITLTimeout != 3*time.Hour {
		t.Errorf("Timeout = %v, HITLTimeout = %v after reopening, want 50m and 3h", sel.Timeout, sel.HITLTimeout)
	}
}

// The wizard offers only CLI-backed harnesses, so the HITL question is never
// skipped by a harness choice; a selection read before the question has been
// answered (as for any path that does not ask it) carries the default.
func TestConfigScreen_HITLTimeout_NotAskedYetIsTheDefault(t *testing.T) {
	s := NewConfigScreen(80, 24, screens.Styles{})

	got := s.Selection().HITLTimeout

	if got != domain.DefaultHITLTimeout {
		t.Errorf("HITLTimeout = %v before the question is answered, want domain.DefaultHITLTimeout (%v)", got, domain.DefaultHITLTimeout)
	}
}

func TestConfigScreen_HITLTimeout_BackThenForwardStartsWithAnEmptyInput(t *testing.T) {
	s := newScreenAtHITLTimeoutStep(t)
	typeText(s, "zzbad")
	pressKey(s, tea.KeyEsc) // back to the non-HITL timeout
	clearInput(s)
	typeText(s, "30m")
	pressKey(s, tea.KeyEnter) // forward to the HITL timeout again
	if !showsHITLTimeout(s) {
		t.Fatalf("precondition: the HITL timeout question did not come back; view:\n%s", s.View())
	}

	if strings.Contains(collapsedText(s), "zzbad") {
		t.Errorf("the HITL timeout input kept the text typed before going back; view:\n%s", s.View())
	}
}

func TestConfigScreen_HITLTimeout_AnsweringTheDefaultGivesFourHours(t *testing.T) {
	s := newScreenAtHITLTimeoutStep(t)

	answerHITLTimeout(s, "4h")

	if got := s.Selection().HITLTimeout; got != 4*time.Hour {
		t.Errorf("HITLTimeout = %v after answering 4h, want 4h", got)
	}
}
