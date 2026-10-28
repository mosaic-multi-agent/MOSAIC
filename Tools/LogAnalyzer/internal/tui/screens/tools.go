package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

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
	runChrome

	sort   SortCycle
	cursor int // selected row of the run's sorted table

	mark   markRequest
	status string // one-line message about the last key, cleared by the next key
}

// NewToolsScreen creates the tools screen for one run.
func NewToolsScreen(run domain.RunReport, width, height int, styles Styles) *ToolsScreen {
	return &ToolsScreen{run: run, width: width, height: height, styles: styles, nav: viewNav{self: ScreenTools},
		sort: newToolsSortCycle()}
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
	k := key.String()
	s.status = ""
	if s.help.handle(k) {
		return s, nil
	}
	switch k {
	case "esc", "q":
		s.back = true
	case KeySort:
		s.sort = s.sort.Next()
	case KeyMarkUserTool:
		s.markSelected()
	case "up", "k", "down", "j":
		delta := 1
		if k == "up" || k == "k" {
			delta = -1
		}
		s.cursor = clampIndex(s.cursor+delta, len(s.run.Tools.Run))
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
	if s.status != "" {
		height--
	}
	w := s.scroll.window(len(body), height)
	spec := frameSpec{run: s.run, active: ScreenTools, keys: toolsKeys, screenKeys: []tuicommon.HelpEntry{sortKeyEntry, markKeyEntry},
		escDesc: "back to the runs list", hint: FormatScrollHint(w, len(body)), sort: s.sort.Label()}
	out := s.render(spec, body[w.Offset:w.End()], width, height, s.styles)
	if s.status != "" {
		out += "\n" + clip(s.styles.Muted, width, s.status)
	}
	return out
}

// body renders every line of the view; the caller scrolls through it. The cursor
// selects a row of the run's table.
func (s *ToolsScreen) body() []string {
	st, width := s.styles, s.width
	tb := s.run.Tools
	if len(tb.Run) == 0 && len(tb.Orchestrator) == 0 && len(tb.ByAgentType) == 0 {
		return []string{clip(st.Body, width, "No tool calls in this run.")}
	}
	order := s.sort.Active()
	section := func(out []string, title string, tools []domain.ToolFigures, cursor int) []string {
		out = append(out, clip(st.Subtitle, width, title))
		if len(tools) > 0 {
			out = append(out, toolsTableLines(tools, order, cursor, width, st)...)
		}
		return out
	}
	out := section(nil, "Run", tb.Run, clampIndex(s.cursor, len(tb.Run)))
	out = section(out, "Orchestrator", tb.Orchestrator, table.NoCursor)
	for _, t := range tb.ByAgentType {
		out = section(out, typeLabel(t.AgentType), t.Tools, table.NoCursor)
	}
	return out
}

// toolsKeys are the short help keys of the view.
var toolsKeys = []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "scroll"}, {Key: KeySort, Desc: "sort"}, {Key: KeyMarkUserTool, Desc: "mark"}, {Key: "esc", Desc: "back"}}

// Help implements Screen.
func (s *ToolsScreen) Help() string {
	return shortHelpText("", toolsKeys)
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

// SelectedTool returns the tool row under the cursor of the sorted table.
// The second result is false when the run has no tool calls.
func (s *ToolsScreen) SelectedTool() (domain.ToolFigures, bool) {
	tools := s.run.Tools.Run
	if len(tools) == 0 {
		return domain.ToolFigures{}, false
	}
	order := toolOrder(tools, s.sort.Active())
	return tools[order[clampIndex(s.cursor, len(order))]], true
}
