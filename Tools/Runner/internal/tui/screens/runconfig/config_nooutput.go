package runconfig

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	commonharness "mosaic-common/harness"
	"mosaic-common/tui/widgets"

	"mosaic-run/internal/domain"
)

// configStepNoOutputTimeout asks the no-output watchdog duration of
// opencode-v2 invocations. It is shown only for that harness, right after the
// HITL timeout. It is numbered after configStepDone so no other step moves.
const configStepNoOutputTimeout configStep = configStepDone + 1

// newNoOutputInput builds the no-output timeout input. The 10m default
// (domain.DefaultNoOutputTimeout) is well above V2's legitimate silences: a
// long tool call, a text block or a background wait emit nothing for a while.
// "0" disables the watchdog.
func newNoOutputInput(width int, styles widgets.TextInputStyles) *widgets.TextInput {
	in := widgets.NewTextInput(
		"No-output timeout for OpenCode V2 invocations (e.g. 10m; 0 disables):",
		"10m", width, styles,
	)
	in.SetValidate(validateNoOutputTimeout)
	return in
}

// validateNoOutputTimeout accepts a non-negative Go duration; "0" disables.
func validateNoOutputTimeout(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return errors.New("timeout cannot be empty")
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("invalid duration %q: must be like 10m, 90s, or 0 to disable", v)
	}
	if d < 0 {
		return errors.New("timeout must not be negative")
	}
	return nil
}

// noOutputSelection maps the typed text to the stored value: a positive
// duration as given, domain.NoOutputDisabled for zero.
func noOutputSelection(text string) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(text))
	if err != nil || d == 0 {
		return domain.NoOutputDisabled
	}
	return d
}

// asksNoOutputTimeout reports whether the selected harness has the watchdog.
func (s *ConfigScreen) asksNoOutputTimeout() bool {
	return s.sel.Harness == commonharness.HarnessIDOpenCodeV2
}

// lastTimeoutStep returns the last of the timeout questions: the no-output
// timeout for opencode-v2, else the HITL timeout.
func (s *ConfigScreen) lastTimeoutStep() configStep {
	if s.asksNoOutputTimeout() {
		return configStepNoOutputTimeout
	}
	return configStepHITLTimeout
}

// updateNoOutputTimeout handles the no-output timeout step: back returns to the
// HITL timeout, a valid answer moves on to the step after the timeouts.
func (s *ConfigScreen) updateNoOutputTimeout(msg tea.Msg) tea.Cmd {
	cmd := s.noOutputInput.Update(msg)
	if s.noOutputInput.Back() {
		s.noOutputInput.Reset()
		s.step = configStepHITLTimeout
		s.cursor = 0
		return s.timeoutStepInit()
	}
	if s.noOutputInput.Done() {
		s.sel.NoOutputTimeout = noOutputSelection(s.noOutputInput.Value())
		s.noOutputInput.Reset()
		s.step = s.stepAfterTimeout()
		s.cursor = s.entryCursor(s.step)
		return nil
	}
	return cmd
}

// startNoOutputInput prepares the input for display: it shows the last answer,
// or nothing (the 10m placeholder) when none was given.
func (s *ConfigScreen) startNoOutputInput() tea.Cmd {
	s.noOutputInput.Reset()
	switch d := s.sel.NoOutputTimeout; {
	case d == domain.NoOutputDisabled:
		s.noOutputInput.SetValue("0")
	case d > 0:
		s.noOutputInput.SetValue(d.String())
	default:
		s.noOutputInput.SetValue("")
	}
	return s.noOutputInput.Init()
}

// resetTimeoutInputs clears the timeout inputs.
func (s *ConfigScreen) resetTimeoutInputs() {
	s.timeoutInput.Reset()
	s.hitlTimeoutInput.Reset()
	s.noOutputInput.Reset()
}

// resizeTimeoutInputs fits the timeout inputs to width.
func (s *ConfigScreen) resizeTimeoutInputs(width int) {
	s.timeoutInput.Resize(width)
	s.hitlTimeoutInput.Resize(width)
	s.noOutputInput.Resize(width)
}
