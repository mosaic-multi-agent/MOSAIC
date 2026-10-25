package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
)

// ToolsScreen is the tools view (V4): call count, error count and durations
// per tool for the run, the orchestrator and each agent type. It shows exact
// figures only, never attribution of cost to tools.
type ToolsScreen struct {
	run    domain.RunReport
	scroll pageScroller
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
}

// NewToolsScreen creates the tools screen for one run.
func NewToolsScreen(run domain.RunReport, width, height int, styles Styles) *ToolsScreen {
	return &ToolsScreen{run: run, width: width, height: height, styles: styles, nav: viewNav{self: ScreenTools}}
}

// ID implements Screen.
func (s *ToolsScreen) ID() ScreenID { return ScreenTools }

// Init implements Screen.
func (s *ToolsScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *ToolsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch k := key.String(); k {
	case "esc", "q":
		s.back = true
	case "up", "k", "down", "j":
		s.scroll.handle(k, len(s.body()), s.height)
	default:
		s.nav.request(k)
	}
	return s, nil
}

// View implements Screen.
func (s *ToolsScreen) View(width, height int) string {
	s.width, s.height = width, height
	body := s.body()
	w := s.scroll.window(len(body), height)
	return renderPage(s.styles, width, "Log Analyzer — Tools", runLabel(s.run),
		body[w.Offset:w.End()], w, len(body), s.Help())
}

// body renders every line of the view; the caller scrolls through it.
func (s *ToolsScreen) body() []string {
	st, width := s.styles, s.width
	tb := s.run.Tools
	if len(tb.Run) == 0 && len(tb.Orchestrator) == 0 && len(tb.ByAgentType) == 0 {
		return []string{clip(st.Body, width, "No tool calls in this run.")}
	}
	section := func(out []string, title string, tools []domain.ToolFigures) []string {
		out = append(out, clip(st.Subtitle, width, title))
		for _, t := range tools {
			out = append(out, clip(st.Body, width, "  "+toolRow(t)))
		}
		return out
	}
	out := section(nil, "Run", tb.Run)
	out = append(out, "")
	out = section(out, "Orchestrator", tb.Orchestrator)
	for _, t := range tb.ByAgentType {
		name := t.AgentType
		if name == "" {
			name = UnknownTypeLabel
		}
		out = append(out, "")
		out = section(out, name, t.Tools)
	}
	return out
}

// toolRow renders one tool: calls, errors and the average, minimum and maximum
// duration of its paired calls, with a count of unpaired calls.
func toolRow(t domain.ToolFigures) string {
	name := t.Tool
	if name == "" {
		name = UnnamedToolLabel
	}
	d := t.Durations
	ok := d.Count > 0
	row := fmt.Sprintf("%-16s %4d calls  %3d errors  avg %s  min %s  max %s", name, t.Calls, t.Errors,
		FormatDuration(d.Average, ok), FormatDuration(d.Min, ok), FormatDuration(d.Max, ok))
	if t.Unpaired > 0 {
		row += fmt.Sprintf("  %d %s", t.Unpaired, UnpairedLabel)
	}
	return row
}

// Help implements Screen.
func (s *ToolsScreen) Help() string {
	return "↑/↓ scroll  " + viewKeysHelp(ScreenTools) + "  esc back  ctrl+c quit"
}

// Back reports whether the user asked to leave the per-run views.
func (s *ToolsScreen) Back() bool { return s.back }

// Requested reports a view key pressed for a different per-run view.
func (s *ToolsScreen) Requested() (ScreenID, bool) { return s.nav.requested() }

// Reset clears Back and Requested.
func (s *ToolsScreen) Reset() {
	s.back = false
	s.nav.reset()
}
