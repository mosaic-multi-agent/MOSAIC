package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// stepRecordedMsg is the message ProgramRef.StepRecorded posts to the program:
// one Execution Log entry exactly as the session recorded it.
type stepRecordedMsg struct {
	entry domain.ExecutionLogEntry
}

// StepRecorded implements domain.StepRecordObserver. It posts one
// stepRecordedMsg to the program, in call order, and is a no-op before the
// program is wired, exactly like Notify.
func (r *ProgramRef) StepRecorded(_ context.Context, entry domain.ExecutionLogEntry) {
	if p := r.program(); p != nil {
		p.Send(stepRecordedMsg{entry: entry})
	}
}

// handleStepRecorded shows a recorded entry as one finished progress row, the
// row historyRows would rebuild for it. It finishes the row announced as
// running or appends one when no step is running.
func (m *rootModel) handleStepRecorded(msg stepRecordedMsg) (tea.Model, tea.Cmd) {
	if m.progressScreen != nil {
		m.progressScreen.RecordStep(historyRowsFor([]domain.ExecutionLogEntry{msg.entry}, m.declaredInfraSet())[0])
		m.syncGuidanceAvailable()
	}
	return m, nil
}
