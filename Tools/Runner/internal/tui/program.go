package tui

import tea "github.com/charmbracelet/bubbletea"

// newProgram builds the Bubble Tea program through the factory the composition root supplied,
// so panics inside the model are reported by the shared fatal reporter; without one it builds
// a plain program.
func (o Options) newProgram(m tea.Model, opts ...tea.ProgramOption) *tea.Program {
	if o.NewProgram != nil {
		return o.NewProgram(m, opts...)
	}
	return tea.NewProgram(m, opts...)
}
