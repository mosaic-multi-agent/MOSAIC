package runflow

import (
	"github.com/charmbracelet/lipgloss"

	"mosaic-run/internal/domain"
)

// isRunnerRecordedStatus reports whether status is a failure status the Runner
// records itself (no usable subagent response), as opposed to one a subagent
// returned.
func isRunnerRecordedStatus(status string) bool {
	return domain.StatusCode(status).IsRunnerRecorded()
}

// runnerRecordedStyle is the style of a Runner-recorded failure status: the
// Error colour made bold and italic, so it reads as a failure yet never
// looks like a subagent-returned BLOCKED.
func (s *ProgressScreen) runnerRecordedStyle() lipgloss.Style {
	return s.styles.Error.Bold(true).Italic(true)
}
