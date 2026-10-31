package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// ContextScreen shows per actor how its context grew and whether it stayed in
// the optimal range; enter opens the Session screen of the selected actor.
type ContextScreen struct {
	run    domain.RunReport
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
	runChrome
	sessionRequest

	sort   SortCycle
	cursor int // selected row: 0 is the orchestrator, then the sorted invocations; none until a key moves it
	offset int // first visible row
}

// NewContextScreen builds the Context view of a run.
func NewContextScreen(run domain.RunReport, width, height int, st Styles) *ContextScreen {
	return &ContextScreen{run: run, width: width, height: height, styles: st, nav: viewNav{self: ScreenContext},
		sort: newContextSortCycle(), cursor: table.NoCursor}
}

// ID implements Screen.
func (s *ContextScreen) ID() ScreenID { return ScreenContext }

// Init implements Screen.
func (s *ContextScreen) Init() tea.Cmd { return nil }

// rowCount is the number of table rows: the orchestrator and every invocation.
func (s *ContextScreen) rowCount() int { return 1 + len(s.run.Context.Invocations) }

// Update implements Screen.
func (s *ContextScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
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
	case "up", "k":
		s.cursor = clampIndex(s.cursor-1, s.rowCount())
	case "down", "j":
		s.cursor = clampIndex(s.cursor+1, s.rowCount())
	case "enter":
		s.openSession()
	case KeySort:
		s.sort = s.sort.Next()
	default:
		s.nav.request(k)
	}
	return s, nil
}

// openSession asks for the session of the actor under the cursor of the table.
func (s *ContextScreen) openSession() {
	if s.cursor < 0 {
		return
	}
	rc := s.run.Context
	if s.cursor == 0 {
		s.requestSession(domain.Orchestrator())
		return
	}
	order := contextOrder(rc.Invocations, s.sort.Active())
	s.requestSession(rc.Invocations[order[clampIndex(s.cursor-1, len(order))]].Actor)
}

// View implements Screen.
func (s *ContextScreen) View(width, height int) string {
	s.width, s.height = width, height
	st := s.styles
	rc := s.run.Context
	order := s.sort.Active()
	rows := contextTableRows(st.Currency, rc, contextOrder(rc.Invocations, order), st.Theme)

	rowBudget := bodyBudget(height) - 1
	t := tableOf(contextColumns(), rows, s.cursor, 0, contextSortColumn(order.Key), order.Desc)
	rowBudget -= extraHeaderLines(t, width)
	w := ScrollWindow(s.offset, s.cursor, len(rows), rowBudget)
	s.offset = w.Offset
	t.Offset = w.Offset
	body := renderTableLines(t, width, w.Count, st)
	if !hasContextCalls(rc) {
		body = append([]string{st.Body.Render("No per-call context records in this run.")}, body...)
	}
	spec := frameSpec{run: s.run, active: ScreenContext, keys: s.helpKeys(), screenKeys: s.screenKeys(), escDesc: "back to the runs list",
		legend: contextLegend(), hint: FormatScrollHint(w, len(rows)), sort: s.sort.Label()}
	return s.render(spec, body, width, height, st)
}

// helpKeys are the short help keys of the view.
func (s *ContextScreen) helpKeys() []tuicommon.HelpEntry {
	return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "select"}, {Key: "enter", Desc: "session"},
		{Key: KeySort, Desc: "sort"}, exportKeyEntry, {Key: "esc", Desc: "back"}}
}

// screenKeys are the overlay entries of the keys this view has of its own.
func (s *ContextScreen) screenKeys() []tuicommon.HelpEntry {
	return []tuicommon.HelpEntry{sortKeyEntry, {Key: "enter", Desc: "open the session of the selected actor: its context curve and calls"}}
}

// Help implements Screen.
func (s *ContextScreen) Help() string { return shortHelpText("", s.helpKeys()) }

// Back reports whether the user asked to leave the per-run views.
func (s *ContextScreen) Back() bool { return s.back }

// Requested reports a view key pressed for a different per-run view.
func (s *ContextScreen) Requested() (ScreenID, bool) { return s.nav.requested() }

// Reset clears Back and Requested.
func (s *ContextScreen) Reset() {
	s.back = false
	s.nav.reset()
}
