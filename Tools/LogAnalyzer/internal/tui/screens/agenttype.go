package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
)

// AgentTypesScreen is the by-agent-type view (V2) with a drill-down to the
// individual dispatches of one type.
type AgentTypesScreen struct {
	run    domain.RunReport
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav

	cursor int // selected agent type
	offset int // first visible type row

	drilled     bool
	drillCursor int // selected dispatch within the drilled type
	drillOffset int // first visible dispatch row
}

// NewAgentTypesScreen creates the by-agent-type screen for one run.
func NewAgentTypesScreen(run domain.RunReport, width, height int, styles Styles) *AgentTypesScreen {
	return &AgentTypesScreen{run: run, width: width, height: height, styles: styles, nav: viewNav{self: ScreenAgentTypes}}
}

// ID implements Screen.
func (s *AgentTypesScreen) ID() ScreenID { return ScreenAgentTypes }

// Init implements Screen.
func (s *AgentTypesScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *AgentTypesScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	k := key.String()
	switch k {
	case "esc", "q":
		if s.drilled {
			s.drilled = false
		} else {
			s.back = true
		}
	case "up", "k":
		s.moveCursor(-1)
	case "down", "j":
		s.moveCursor(1)
	case "enter":
		if !s.drilled && len(s.run.ByAgentType) > 0 {
			s.drilled, s.drillCursor, s.drillOffset = true, 0, 0
		}
	default:
		s.nav.request(k)
	}
	return s, nil
}

// moveCursor moves the cursor of the list currently shown, staying in range.
func (s *AgentTypesScreen) moveCursor(delta int) {
	if s.drilled {
		s.drillCursor = clampIndex(s.drillCursor+delta, len(s.run.ByAgentType[s.cursor].Dispatches))
		return
	}
	s.cursor = clampIndex(s.cursor+delta, len(s.run.ByAgentType))
}

func clampIndex(i, n int) int {
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

// View implements Screen.
func (s *AgentTypesScreen) View(width, height int) string {
	s.width, s.height = width, height
	if s.drilled {
		return s.viewDrill()
	}
	rows := make([]string, len(s.run.ByAgentType))
	for i, g := range s.run.ByAgentType {
		rows[i] = typeRow(g)
	}
	if len(rows) == 0 {
		rows = []string{"No subagent invocations in this run."}
	}
	body, w := s.cursorRows(rows, s.cursor, &s.offset)
	return renderPage(s.styles, width, "Log Analyzer — By Agent Type", runLabel(s.run),
		body, w, len(rows), s.Help())
}

// cursorRows styles the rows visible around the cursor and updates the offset.
func (s *AgentTypesScreen) cursorRows(rows []string, cursor int, offset *int) ([]string, Window) {
	w := ScrollWindow(*offset, cursor, len(rows), bodyBudget(s.height))
	*offset = w.Offset
	out := make([]string, 0, w.Count)
	for i := w.Offset; i < w.End(); i++ {
		if i == cursor {
			out = append(out, clip(s.styles.Selected, s.width, "> "+rows[i]))
		} else {
			out = append(out, clip(s.styles.Body, s.width, "  "+rows[i]))
		}
	}
	return out, w
}

// typeRow renders one agent type: invocation count, total, average, minimum
// and maximum, followed by the labels of any incomplete data.
func typeRow(g domain.AgentTypeGroup) string {
	name := g.AgentType
	if name == "" {
		name = UnknownTypeLabel
	}
	stats := g.PerInvocation
	row := fmt.Sprintf("%-20s %4d invocations  total %s  avg %s  min %s  max %s",
		name, g.Figures.Invocations, formatMoneyTotal(g.Figures.Totals.Money),
		FormatMoney(stats.Average), FormatMoney(stats.Min), FormatMoney(stats.Max))
	if stats.Basis > 0 && !stats.Complete {
		row += " (partial)"
	}
	return appendLabels(row, incompleteness(g.Figures.UnpricedModels, g.Figures.Totals.AssumedTierCacheWrite,
		g.Figures.MissingUsage, g.Figures.Open))
}

// appendLabels adds bracketed incompleteness labels to a row.
func appendLabels(row string, labels []string) string {
	if len(labels) == 0 {
		return row
	}
	return row + "  [" + strings.Join(labels, "; ") + "]"
}

// Help implements Screen.
func (s *AgentTypesScreen) Help() string {
	if s.drilled {
		return "↑/↓ select  " + viewKeysHelp(ScreenAgentTypes) + "  esc types  ctrl+c quit"
	}
	return "↑/↓ select  enter dispatches  " + viewKeysHelp(ScreenAgentTypes) + "  esc back  ctrl+c quit"
}

// Back reports whether the user asked to leave the per-run views.
func (s *AgentTypesScreen) Back() bool { return s.back }

// Requested reports a view key pressed for a different per-run view.
func (s *AgentTypesScreen) Requested() (ScreenID, bool) { return s.nav.requested() }

// Reset clears Back and Requested.
func (s *AgentTypesScreen) Reset() {
	s.back = false
	s.nav.reset()
}

// DrilledType returns the agent type whose dispatches are shown; ok is true
// while the drill-down list is shown.
func (s *AgentTypesScreen) DrilledType() (agentType string, ok bool) {
	if !s.drilled {
		return "", false
	}
	return s.run.ByAgentType[s.cursor].AgentType, true
}
