package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/domain"
)

// StatusScreen is the by-returned-status-code view (V3): cost and dispatch
// count per status code for the run and per agent type. Invocations without a
// status code and open invocations are separate groups.
type StatusScreen struct {
	run    domain.RunReport
	scroll pageScroller
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
}

// NewStatusScreen creates the status screen for one run.
func NewStatusScreen(run domain.RunReport, width, height int, styles Styles) *StatusScreen {
	return &StatusScreen{run: run, width: width, height: height, styles: styles, nav: viewNav{self: ScreenStatus}}
}

// ID implements Screen.
func (s *StatusScreen) ID() ScreenID { return ScreenStatus }

// Init implements Screen.
func (s *StatusScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *StatusScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
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
func (s *StatusScreen) View(width, height int) string {
	s.width, s.height = width, height
	body := s.body()
	w := s.scroll.window(len(body), height)
	return renderPage(s.styles, width, "Log Analyzer — By Status Code", runLabel(s.run),
		body[w.Offset:w.End()], w, len(body), s.Help())
}

// body renders every line of the view; the caller scrolls through it.
func (s *StatusScreen) body() []string {
	st, width := s.styles, s.width
	bs := s.run.ByStatus
	if len(bs.Run) == 0 && len(bs.ByAgentType) == 0 {
		return []string{clip(st.Body, width, "No subagent invocations in this run.")}
	}
	out := []string{clip(st.Subtitle, width, "Run")}
	for _, g := range bs.Run {
		out = append(out, clip(st.Body, width, "  "+statusRow(g)))
	}
	for _, t := range bs.ByAgentType {
		name := t.AgentType
		if name == "" {
			name = UnknownTypeLabel
		}
		out = append(out, "", clip(st.Subtitle, width, name))
		for _, g := range t.Groups {
			out = append(out, clip(st.Body, width, "  "+statusRow(g)))
		}
	}
	return out
}

// statusRow renders one status group: its name, dispatch count and cost,
// followed by the labels of any incomplete data.
func statusRow(g domain.StatusGroup) string {
	var name string
	open := g.Figures.Open
	switch g.Kind {
	case domain.StatusNone:
		name = NoStatusLabel
	case domain.StatusOpen:
		name = OpenLabel
		open = 0 // the group name already says open
	default:
		name = g.Code
	}
	row := fmt.Sprintf("%-16s %4d dispatches  %s", name, g.Figures.Invocations, formatMoneyTotal(g.Figures.Totals.Money))
	return appendLabels(row, incompleteness(g.Figures.UnpricedModels, g.Figures.Totals.AssumedTierCacheWrite,
		g.Figures.MissingUsage, open))
}

// Help implements Screen.
func (s *StatusScreen) Help() string {
	return "↑/↓ scroll  " + viewKeysHelp(ScreenStatus) + "  esc back  ctrl+c quit"
}

// Back reports whether the user asked to leave the per-run views.
func (s *StatusScreen) Back() bool { return s.back }

// Requested reports a view key pressed for a different per-run view.
func (s *StatusScreen) Requested() (ScreenID, bool) { return s.nav.requested() }

// Reset clears Back and Requested.
func (s *StatusScreen) Reset() {
	s.back = false
	s.nav.reset()
}
