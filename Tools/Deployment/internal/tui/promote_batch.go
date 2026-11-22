package tui

// promote_batch.go holds the TUI side of a batch promote: the service call that runs it, the
// result message, and the overlay of its agent selection question. The other questions of the
// batch use the dialogs that already exist (model selection, text prompt, plain selection).

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/tui/screens"
)

// promoteDoneMsg signals that Service.PromoteBatch completed and carries its result, which
// accounts for every selected agent (also after an abort).
type promoteDoneMsg struct {
	result app.PromoteBatchResult
}

// promoteBatchMsg runs the batch promote of the selected harness over the selected agents
// folder and returns the message that ends the run.
func promoteBatchMsg(ctx context.Context, svc app.Service, sel entrySelections) tea.Msg {
	result, err := svc.PromoteBatch(ctx, app.PromoteBatchRequest{
		HarnessID: sel.harnessID,
		AgentsDir: sel.workspacePath,
	})
	if err != nil {
		return runErrorMsg{err: err}
	}
	return promoteDoneMsg{result: result}
}

// openPromoteAgentList shows the multi-select agent list for the batch's agent selection.
func (m *rootModel) openPromoteAgentList(opts questionMsg, style screens.Styles) {
	m.promoteAgentOverlay = screens.NewPromoteAgentScreen(opts.choiceQ.Options, m.width, m.height, style)
}

// updatePromoteAgentOverlay drives the agent list. Enter answers the selected file names, Esc
// answers Cancelled, which aborts the batch. It reports whether the list was active.
func (m *rootModel) updatePromoteAgentOverlay(msg tea.Msg) (tea.Cmd, bool) {
	if m.promoteAgentOverlay == nil {
		return nil, false
	}
	cmd := m.promoteAgentOverlay.Update(msg)
	if m.promoteAgentOverlay.Done() {
		m.dispatchMultiChoiceAnswer(m.promoteAgentOverlay.SelectedIDs())
		m.promoteAgentOverlay = nil
		m.screen = screenRunning
	} else if m.promoteAgentOverlay.Back() {
		m.dispatchMultiChoiceCancelled()
		m.promoteAgentOverlay = nil
		m.screen = screenRunning
	}
	return cmd, true
}
