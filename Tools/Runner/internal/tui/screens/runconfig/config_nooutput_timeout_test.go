package runconfig

// config_nooutput_timeout_test.go verifies the no-output timeout step: asked
// right after the HITL timeout only when the selected harness is opencode-v2,
// defaulting to 10m, accepting a non-negative duration ("0" disables it), and
// carried in the configuration selection.

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens"
)

// noOutputPromptMarker is the (lower-cased) name of the no-output timeout question.
const noOutputPromptMarker = "no-output timeout"

func showsNoOutputTimeout(s *ConfigScreen) bool {
	return strings.Contains(collapsedText(s), noOutputPromptMarker)
}

func answerNoOutputTimeout(s *ConfigScreen, text string) {
	clearInput(s)
	typeText(s, text)
	pressKey(s, tea.KeyEnter)
}

// newScreenForHarness returns a new-run screen with the harness preselected and
// the mode question answered, so the first question left is the invocation
// timeout.
func newScreenForHarness(harnessID string) *ConfigScreen {
	s := NewConfigScreen(80, 24, screens.Styles{})
	s.SetIsNewRun(true)
	s.SetPreselectedHarness(harnessID)
	driveConfigScreenModeSelect(s, 0) // mode: orchestrated
	return s
}

// newScreenAtNoOutputStep returns an opencode-v2 screen on the no-output timeout
// question, the two timeouts before it having been answered.
func newScreenAtNoOutputStep(t *testing.T) *ConfigScreen {
	t.Helper()
	s := newScreenForHarness(commonharness.HarnessIDOpenCodeV2)
	typeText(s, "30m")
	pressKey(s, tea.KeyEnter)
	answerHITLTimeout(s, "4h")
	if !showsNoOutputTimeout(s) {
		t.Fatalf("the no-output timeout question does not follow the HITL timeout; view:\n%s", s.View())
	}
	return s
}

func TestConfigScreen_NoOutputTimeout_AskedRightAfterTheHITLTimeoutForOpenCodeV2(t *testing.T) {
	s := newScreenAtNoOutputStep(t)

	text := collapsedText(s)

	if !strings.Contains(text, "opencode v2") {
		t.Errorf("the question does not name OpenCode V2; view:\n%s", s.View())
	}
	if !strings.Contains(text, "10m") {
		t.Errorf("the question does not show the 10m default; view:\n%s", s.View())
	}
	if !strings.Contains(text, "0 disables") {
		t.Errorf("the question does not say that 0 disables the watchdog; view:\n%s", s.View())
	}
}

func TestConfigScreen_NoOutputTimeout_NotAskedForOtherHarnesses(t *testing.T) {
	for _, id := range []string{commonharness.HarnessIDClaudeCode, commonharness.HarnessIDOpenCode} {
		t.Run(id, func(t *testing.T) {
			s := newScreenForHarness(id)
			typeText(s, "30m")
			pressKey(s, tea.KeyEnter)

			answerHITLTimeout(s, "4h")

			if showsNoOutputTimeout(s) {
				t.Errorf("the no-output question was asked for %s; view:\n%s", id, s.View())
			}
			if got := s.Selection().NoOutputTimeout; got != 0 {
				t.Errorf("NoOutputTimeout = %v for %s, want 0 (not asked)", got, id)
			}
		})
	}
}

func TestConfigScreen_NoOutputTimeout_AcceptedAnswers(t *testing.T) {
	cases := []struct {
		answer string
		want   time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"90s", 90 * time.Second},
		{"10m", 10 * time.Minute},
		{"0", domain.NoOutputDisabled},
		{"0s", domain.NoOutputDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.answer, func(t *testing.T) {
			s := newScreenAtNoOutputStep(t)

			answerNoOutputTimeout(s, tc.answer)

			if showsNoOutputTimeout(s) {
				t.Fatalf("the question is still shown after answering %q; view:\n%s", tc.answer, s.View())
			}
			if got := s.Selection().NoOutputTimeout; got != tc.want {
				t.Errorf("NoOutputTimeout = %v after %q, want %v", got, tc.answer, tc.want)
			}
		})
	}
}

func TestConfigScreen_NoOutputTimeout_AnswerKeepsTheOtherTimeouts(t *testing.T) {
	s := newScreenAtNoOutputStep(t)

	answerNoOutputTimeout(s, "5m")

	sel := s.Selection()
	if sel.Timeout != 30*time.Minute || sel.HITLTimeout != 4*time.Hour {
		t.Errorf("Timeout = %v, HITLTimeout = %v, want 30m and 4h", sel.Timeout, sel.HITLTimeout)
	}
}

func TestConfigScreen_NoOutputTimeout_RejectsInvalidInput(t *testing.T) {
	for _, bad := range []string{"abc", "-5m", "10", "-1s"} {
		t.Run(bad, func(t *testing.T) {
			s := newScreenAtNoOutputStep(t)

			answerNoOutputTimeout(s, bad)

			if !showsNoOutputTimeout(s) {
				t.Errorf("input %q moved past the no-output question; view:\n%s", bad, s.View())
			}
		})
	}
}

func TestConfigScreen_NoOutputTimeout_RejectsEmptyInput(t *testing.T) {
	s := newScreenAtNoOutputStep(t)
	clearInput(s)

	pressKey(s, tea.KeyEnter)

	if !showsNoOutputTimeout(s) {
		t.Errorf("an empty answer moved past the no-output question; view:\n%s", s.View())
	}
}

func TestConfigScreen_NoOutputTimeout_BackReturnsToTheHITLTimeout(t *testing.T) {
	s := newScreenAtNoOutputStep(t)

	pressKey(s, tea.KeyEsc)

	if !showsHITLTimeout(s) {
		t.Errorf("esc from the no-output timeout did not return to the HITL timeout; view:\n%s", s.View())
	}
	if s.Back() {
		t.Error("Back() = true: esc from the no-output timeout must not leave the configuration screen")
	}
}

func TestConfigScreen_NoOutputTimeout_BackFromTheNextStepReturnsToIt(t *testing.T) {
	s := newScreenAtNoOutputStep(t)
	answerNoOutputTimeout(s, "5m")

	pressKey(s, tea.KeyEsc)

	if !showsNoOutputTimeout(s) {
		t.Errorf("esc from the step after the no-output timeout did not return to it; view:\n%s", s.View())
	}
}

func TestConfigScreen_NoOutputTimeout_ReopeningShowsTheLastValue(t *testing.T) {
	s := newScreenAtNoOutputStep(t)
	answerNoOutputTimeout(s, "7m")
	pressKey(s, tea.KeyEsc) // back onto the question
	if !showsNoOutputTimeout(s) {
		t.Fatalf("precondition: not back on the no-output question; view:\n%s", s.View())
	}

	if !strings.Contains(collapsedText(s), "7m") {
		t.Errorf("the reopened question does not show the last value 7m; view:\n%s", s.View())
	}
}

func TestConfigScreen_NoOutputTimeout_ReopenedResumeWizardKeepsTheAnswer(t *testing.T) {
	s := newDrivenScreen(false, recordedRunAgents())
	s.SetNeedsRunnerAdoption(false)
	s.SetVersionDriftInfo("1.0", "1.0")
	s.SetPreselectedHarness(commonharness.HarnessIDOpenCodeV2)
	typeText(s, "50m")
	pressKey(s, tea.KeyEnter)
	answerHITLTimeout(s, "3h")
	answerNoOutputTimeout(s, "7m")
	if !s.Done() {
		t.Fatalf("precondition: the resume wizard is not finished; view:\n%s", s.View())
	}

	s.Reopen()

	if !showsNoOutputTimeout(s) {
		t.Fatalf("a reopened opencode-v2 resume wizard is not on its last question, the no-output timeout; view:\n%s", s.View())
	}
	if got := s.Selection().NoOutputTimeout; got != 7*time.Minute {
		t.Errorf("NoOutputTimeout = %v after reopening, want 7m", got)
	}
}

func TestConfigScreen_NoOutputTimeout_NotAskedYetIsZero(t *testing.T) {
	s := NewConfigScreen(80, 24, screens.Styles{})

	if got := s.Selection().NoOutputTimeout; got != 0 {
		t.Errorf("NoOutputTimeout = %v before the question is answered, want 0 (default applies)", got)
	}
}
