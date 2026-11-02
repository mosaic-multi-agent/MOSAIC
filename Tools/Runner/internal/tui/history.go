package tui

import (
	"mosaic-run/internal/domain"
	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/tui/screens"
	"mosaic-run/internal/tui/screens/runflow"
)

// historyRows maps Execution Log entries, in log order, to progress rows,
// without telling infrastructure steps apart from workflow steps.
func historyRows(log []domain.ExecutionLogEntry) []runflow.ProgressRow {
	return historyRowsFor(log, nil)
}

// historyRowsFor maps Execution Log entries, in log order, to progress rows.
// Entries of agents in infra are flagged as infrastructure rows. Every row is
// built from persisted data only, so a live row and a rebuilt row for the same
// entry are equal.
func historyRowsFor(log []domain.ExecutionLogEntry, infra domain.InfraAgentSet) []runflow.ProgressRow {
	rows := make([]runflow.ProgressRow, 0, len(log))
	for _, e := range log {
		rows = append(rows, runflow.ProgressRow{
			AgentInstance:  e.Agent,
			Phase:          e.Phase,
			Stage:          e.Stage,
			Status:         string(e.Status),
			Infrastructure: infra != nil && infra.IsInfrastructureAgent(e.Agent),
			ErrorCode:      string(e.ErrorCode),
			Reason:         e.Summary,
		})
	}
	return rows
}

// declaredInfraSet returns the infrastructure agents declared by the run's
// orchestrator file; an empty set when the file is unknown or unreadable.
func (m *rootModel) declaredInfraSet() domain.InfraAgentSet {
	if m.selections.orchestratorFile == "" {
		return nil
	}
	declared, err := orchfile.EnumerateInfrastructureAgents(m.selections.orchestratorFile)
	if err != nil {
		return nil
	}
	return domain.NewInfraAgentSet(declared)
}

// loadHistory reads the run's artifact and returns the rows of its Execution
// Log; any read error yields nil.
func (m *rootModel) loadHistory() []runflow.ProgressRow {
	if m.selections.runFolder == "" {
		return nil
	}
	state, err := m.resolveArtifactStore(m.selections.runFolder).Read(m.ctx)
	if err != nil {
		return nil
	}
	return historyRowsFor(state.ExecutionLog, m.declaredInfraSet())
}

// newProgressScreenWithHistory returns a new progress screen; for a resumed
// run (selections.isNewRun false) its rows are set to loadHistory().
func (m *rootModel) newProgressScreenWithHistory(style screens.Styles) *runflow.ProgressScreen {
	screen := runflow.NewProgressScreen(m.width, m.height, style)
	if !m.selections.isNewRun {
		screen.SetHistory(m.loadHistory())
	}
	return screen
}
