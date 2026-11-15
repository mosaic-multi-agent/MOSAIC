package setup

import (
	tea "github.com/charmbracelet/bubbletea"
)

func keyEnter() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }
