package tui

// gapstatus.go shows the file the pricing-gap flow wrote as the one-line status
// below the Runs screen once the analysis completes.

import (
	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/app"
)

// withGapSavedStatus sets the status line from the analysis result when the
// pricing-gap flow saved a price; it leaves the status alone otherwise.
func (m Model) withGapSavedStatus(result app.Result) Model {
	if result.GapConfigPath == "" {
		return m
	}
	return m.setStatus("Saved prices to "+result.GapConfigPath, tuicommon.RoleSignalGood)
}
