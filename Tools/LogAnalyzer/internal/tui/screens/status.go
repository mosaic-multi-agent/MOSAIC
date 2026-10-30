package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"

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
	runChrome

	sort SortCycle
}

// NewStatusScreen creates the status screen for one run.
func NewStatusScreen(run domain.RunReport, width, height int, styles Styles) *StatusScreen {
	return &StatusScreen{run: run, width: width, height: height, styles: styles, nav: viewNav{self: ScreenStatus},
		sort: newStatusSortCycle()}
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
	k := key.String()
	if s.help.handle(k) {
		return s, nil
	}
	switch k {
	case "esc", "q":
		s.back = true
	case KeySort:
		s.sort = s.sort.Next()
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
	spec := frameSpec{run: s.run, active: ScreenStatus, keys: statusKeys, screenKeys: []tuicommon.HelpEntry{sortKeyEntry}, escDesc: "back to the runs list",
		hint: FormatScrollHint(w, len(body)), sort: s.sort.Label()}
	return s.render(spec, body[w.Offset:w.End()], width, height, s.styles)
}

// body renders every line of the view; the caller scrolls through it.
func (s *StatusScreen) body() []string {
	st, width := s.styles, s.width
	bs := s.run.ByStatus
	if len(bs.Run) == 0 && len(bs.ByAgentType) == 0 {
		return []string{clip(st.Body, width, "No subagent invocations in this run.")}
	}
	order := s.sort.Active()
	var out []string
	if len(bs.Run) > 0 {
		out = append(out, clip(st.Subtitle, width, "Run"))
		out = append(out, statusTableLines(bs.Run, order, width, st)...)
	}
	for _, t := range bs.ByAgentType {
		out = append(out, clip(st.Subtitle, width, typeLabel(t.AgentType)))
		out = append(out, statusTableLines(t.Groups, order, width, st)...)
	}
	return out
}

// statusKeys are the short help keys of the view.
var statusKeys = []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "scroll"}, {Key: KeySort, Desc: "sort"}, exportKeyEntry, {Key: "esc", Desc: "back"}}

// Help implements Screen.
func (s *StatusScreen) Help() string {
	return shortHelpText("", statusKeys)
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
