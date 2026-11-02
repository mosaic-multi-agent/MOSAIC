// Package screens implements the TUI screens for the mosaic-log-analyzer tool.
// Each screen is a self-contained model that satisfies the Screen interface.
// Screens import only domain/app types, shared widgets, and styling libraries.
// They never import the parent tui package.
package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// ScreenID names each screen for navigation and testing.
type ScreenID string

const (
	ScreenSource    ScreenID = "source"
	ScreenRuns      ScreenID = "runs"
	ScreenRunDetail ScreenID = "run-detail"
	ScreenPricing   ScreenID = "pricing"
	ScreenNoData    ScreenID = "no-data"

	ScreenOverview   ScreenID = "run-overview"    // V1 run overview
	ScreenAgentTypes ScreenID = "run-agent-types" // V2 by agent type
	ScreenStatus     ScreenID = "run-status"      // V3 by returned status code
	ScreenTools      ScreenID = "run-tools"       // V4 tools
	ScreenTimeline   ScreenID = "run-timeline"    // V5 timeline
	ScreenAcrossRuns ScreenID = "across-runs"     // V6 across runs
	ScreenContext    ScreenID = "run-context"     // V7 context
	ScreenSession    ScreenID = "run-session"     // one actor's session, opened from Dispatches, By type or Context

	ScreenCompare ScreenID = "compare" // run comparison (not a per-run view)
)

// Screen is the contract every screen satisfies so the top-level model can
// route uniformly.
type Screen interface {
	ID() ScreenID
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View(width, height int) string
	// Help returns the key hints rendered by the shared scaffold.
	Help() string
}

// Styles is the resolved set of lipgloss styles passed to every screen
// constructor. It is populated by the root model from the active Theme so
// screens are style-agnostic.
type Styles struct {
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Body     lipgloss.Style
	Muted    lipgloss.Style
	Selected lipgloss.Style
	Checked  lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Error    lipgloss.Style
	Help     lipgloss.Style
	Border   lipgloss.Style

	// Theme is the palette theme passed to kit widgets.
	Theme tuicommon.Theme

	// Currency is the effective currency every money figure is shown in; empty
	// reads as USD.
	Currency domain.CurrencyCode
}
