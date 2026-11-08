package tui

// hitl_timeout_helpers_test.go holds the helpers the setup-flow tests use to
// answer the HITL invocation timeout question, which follows the (non-HITL)
// invocation timeout question.

import (
	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/tui/screens/runconfig"
)

// hitlTimeoutPromptMarker is the (lower-cased) name of the HITL timeout question.
const hitlTimeoutPromptMarker = "hitl invocation timeout"

// answerHITLTimeoutOnScreen replaces whatever the focused timeout input holds
// with "4h" and confirms it. It works whether the input starts empty or prefilled.
func answerHITLTimeoutOnScreen(s *runconfig.ConfigScreen) {
	answerHITLTimeoutVia(func(msg tea.Msg) { s.Update(msg) })
}

// answerHITLTimeoutOnModel does the same through the root model.
func answerHITLTimeoutOnModel(m *rootModel) {
	answerHITLTimeoutVia(func(msg tea.Msg) { m.Update(msg) })
}

func answerHITLTimeoutVia(send func(tea.Msg)) {
	for i := 0; i < 12; i++ {
		send(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	send(tea.KeyMsg{Type: tea.KeyEnter})
}
