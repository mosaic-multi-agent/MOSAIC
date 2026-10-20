package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// ConsultationStarted implements domain.ConsultationObserver. It posts a
// consultStartedMsg to the program and is a no-op before the program is wired.
func (r *ProgramRef) ConsultationStarted(_ context.Context, a domain.ConsultationActivity) {
	if p := r.program(); p != nil {
		p.Send(consultStartedMsg{activity: a})
	}
}

// ConsultationEnded implements domain.ConsultationObserver. It posts a
// consultEndedMsg to the program and is a no-op before the program is wired.
func (r *ProgramRef) ConsultationEnded(context.Context) {
	if p := r.program(); p != nil {
		p.Send(consultEndedMsg{})
	}
}

// handleConsultStarted shows the consultation row.
func (m *rootModel) handleConsultStarted(msg consultStartedMsg) (tea.Model, tea.Cmd) {
	if m.progressScreen != nil {
		m.progressScreen.StartConsultation(msg.activity)
	}
	return m, nil
}

// handleConsultEnded removes the consultation row.
func (m *rootModel) handleConsultEnded() (tea.Model, tea.Cmd) {
	if m.progressScreen != nil {
		m.progressScreen.EndConsultation()
	}
	return m, nil
}
