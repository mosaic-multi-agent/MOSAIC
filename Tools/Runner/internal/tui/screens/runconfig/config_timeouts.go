package runconfig

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/tui/widgets"

	"mosaic-run/internal/domain"
)

// newTimeoutInputs builds the two invocation-timeout inputs: the non-HITL
// timeout and the timeout for human_in_the_loop: true dispatches. Both accept a
// positive Go duration and reject anything else.
func newTimeoutInputs(width int, styles widgets.TextInputStyles) (nonHITL, hitl *widgets.TextInput) {
	nonHITL = widgets.NewTextInput("Invocation timeout (e.g. 30m, 1h):", "30m", width, styles)
	nonHITL.SetValidate(validateTimeout)
	hitl = widgets.NewTextInput(
		"HITL invocation timeout for human_in_the_loop: true dispatches (e.g. 4h, 90m):",
		"4h", width, styles,
	)
	hitl.SetValidate(validateTimeout)
	return nonHITL, hitl
}

// validateTimeout accepts a positive Go duration.
func validateTimeout(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return errors.New("timeout cannot be empty")
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("invalid duration %q: must be like 30m, 1h, 90m", v)
	}
	if d <= 0 {
		return errors.New("timeout must be greater than zero")
	}
	return nil
}

// parseTimeout returns the duration typed into a timeout input, or fallback
// when the text is not a positive duration.
func parseTimeout(text string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(text)); err == nil && d > 0 {
		return d
	}
	return fallback
}

// updateTimeoutStep handles a message on one of the two timeout steps. handled
// is false when the screen is on another step.
func (s *ConfigScreen) updateTimeoutStep(msg tea.Msg) (cmd tea.Cmd, handled bool) {
	switch s.step {
	case configStepHarnessTimeout:
		return s.updateNonHITLTimeout(msg), true
	case configStepHITLTimeout:
		return s.updateHITLTimeout(msg), true
	}
	return nil, false
}

// updateNonHITLTimeout delegates all key handling to the text input widget.
func (s *ConfigScreen) updateNonHITLTimeout(msg tea.Msg) tea.Cmd {
	cmd := s.timeoutInput.Update(msg)
	if s.timeoutInput.Back() {
		s.timeoutInput.Reset()
		if s.firstStep() == configStepHarnessTimeout {
			s.back = true
		} else if s.harnessPreselected {
			s.step = configStepMode
			s.cursor = s.modeIndex()
		} else {
			s.step = configStepHarness
			s.cursor = 0
		}
		return nil
	}
	if s.timeoutInput.Done() {
		s.sel.Timeout = parseTimeout(s.timeoutInput.Value(), domain.DefaultTimeout)
		s.timeoutInput.Reset()
		s.step = configStepHITLTimeout
		s.cursor = 0
		s.hitlTimeoutInput.Reset()
		s.hitlTimeoutInput.SetValue("")
		return s.hitlTimeoutInput.Init()
	}
	return cmd
}

// updateHITLTimeout handles the HITL timeout step: back returns to the
// non-HITL timeout, a valid answer moves on to the step after the timeouts.
func (s *ConfigScreen) updateHITLTimeout(msg tea.Msg) tea.Cmd {
	cmd := s.hitlTimeoutInput.Update(msg)
	if s.hitlTimeoutInput.Back() {
		s.hitlTimeoutInput.Reset()
		s.step = configStepHarnessTimeout
		s.cursor = 0
		return s.timeoutStepInit()
	}
	if s.hitlTimeoutInput.Done() {
		s.sel.HITLTimeout = parseTimeout(s.hitlTimeoutInput.Value(), domain.DefaultHITLTimeout)
		s.hitlTimeoutInput.Reset()
		s.step = s.stepAfterTimeout()
		s.cursor = s.entryCursor(s.step)
		return nil
	}
	return cmd
}

// timeoutStepInit starts the cursor blink of the timeout input the screen has
// returned to (by back navigation or reopening). The HITL input shows the
// answer already given. It returns nil on any other step.
func (s *ConfigScreen) timeoutStepInit() tea.Cmd {
	switch s.step {
	case configStepHarnessTimeout:
		return s.timeoutInput.Init()
	case configStepHITLTimeout:
		s.hitlTimeoutInput.Reset()
		s.hitlTimeoutInput.SetValue(s.sel.HITLTimeout.String())
		return s.hitlTimeoutInput.Init()
	}
	return nil
}
