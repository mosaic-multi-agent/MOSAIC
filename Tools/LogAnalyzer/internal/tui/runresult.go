package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/fatal"
)

// runResult maps how the bubbletea program ended to the value Run returns. A
// program error means the TUI could not run or ended abnormally and is returned
// with exit code 1; an external interrupt keeps exit code 1 but is not an error.
func runResult(finalModel tea.Model, err error) (int, error) {
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return 1, nil
		}
		return 1, err
	}
	if fm, ok := fatal.UnwrapModel(finalModel).(Model); ok {
		return fm.exitCode, nil
	}
	return 0, nil
}
