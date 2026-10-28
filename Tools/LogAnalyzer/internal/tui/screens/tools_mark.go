package screens

import (
	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// cannotMarkText is shown when the selected row has no tool name or harness.
const cannotMarkText = "cannot mark: tool name or harness unknown"

// markKeyEntry is the help and overlay entry of the mark key.
var markKeyEntry = tuicommon.HelpEntry{Key: KeyMarkUserTool, Desc: "mark the selected tool as user-interaction"}

// markRequest is a pending request to mark a tool as user-interaction.
type markRequest struct {
	tool      string
	harnesses []domain.Harness
	pending   bool
}

// markSelected turns the row under the cursor into a pending request; a row
// that cannot be marked sets a status message instead.
func (s *ToolsScreen) markSelected() {
	row, ok := s.SelectedTool()
	if !ok {
		return
	}
	if row.Tool == "" || len(row.Harnesses) == 0 {
		s.status = cannotMarkText
		return
	}
	s.mark = markRequest{tool: row.Tool, harnesses: append([]domain.Harness(nil), row.Harnesses...), pending: true}
}

// MarkRequested returns the tool row the user marked with the mark key; ok is
// false when none is pending.
func (s *ToolsScreen) MarkRequested() (tool string, harnesses []domain.Harness, ok bool) {
	return s.mark.tool, s.mark.harnesses, s.mark.pending
}

// ResetMark clears the pending mark request.
func (s *ToolsScreen) ResetMark() { s.mark = markRequest{} }
