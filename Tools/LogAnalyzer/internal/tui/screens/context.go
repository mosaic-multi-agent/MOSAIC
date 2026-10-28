package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// ContextScreen shows per actor how its context grew and whether it stayed in
// the optimal range, with a drill-down per actor.
type ContextScreen struct {
	run    domain.RunReport
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
	runChrome

	sort   SortCycle
	cursor int // selected row: 0 is the orchestrator, then the sorted invocations; none until a key moves it
	offset int // first visible row

	drilled     bool
	drillActor  domain.ActorContext
	drillSort   SortCycle
	drillCursor int // selected call of the sorted call list
	drillOffset int // first visible call
}

// NewContextScreen builds the Context view of a run.
func NewContextScreen(run domain.RunReport, width, height int, st Styles) *ContextScreen {
	return &ContextScreen{run: run, width: width, height: height, styles: st, nav: viewNav{self: ScreenContext},
		sort: newContextSortCycle(), drillSort: newCallSortCycle(), cursor: table.NoCursor}
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
		if !s.drilled {
			s.openDrill()
		}
	case KeySort:
		if s.drilled {
			s.drillSort = s.drillSort.Next()
		} else {
			s.sort = s.sort.Next()
		}
	default:
		s.nav.request(k)
	}
	return s, nil
}

// moveCursor moves the cursor of the list currently shown, staying in range.
func (s *ContextScreen) moveCursor(delta int) {
	if s.drilled {
		s.drillCursor = clampIndex(s.drillCursor+delta, len(s.drillActor.Calls))
		return
	}
	s.cursor = clampIndex(s.cursor+delta, s.rowCount())
}

// openDrill drills into the actor under the cursor of the table.
func (s *ContextScreen) openDrill() {
	if s.cursor < 0 {
		return
	}
	rc := s.run.Context
	s.drillActor = rc.Orchestrator
	if s.cursor > 0 {
		order := contextOrder(rc.Invocations, s.sort.Active())
		s.drillActor = rc.Invocations[order[clampIndex(s.cursor-1, len(order))]]
	}
	s.drilled, s.drillCursor, s.drillOffset = true, 0, 0
	s.drillSort = newCallSortCycle()
}

// View implements Screen.
func (s *ContextScreen) View(width, height int) string {
	s.width, s.height = width, height
	if s.drilled {
		return s.viewDrill()
	}
	st := s.styles
	rc := s.run.Context
	order := s.sort.Active()
	rows := contextTableRows(rc, contextOrder(rc.Invocations, order), st.Theme)

	notes := contextNotes(width)
	budget := bodyBudget(height)
	rowBudget := budget - len(notes) - 1
	if rowBudget < 4 {
		rowBudget = budget - 1 // a short terminal keeps the table and loses the notes
	}
	w := ScrollWindow(s.offset, s.cursor, len(rows), rowBudget)
	s.offset = w.Offset
	t := tableOf(contextColumns(), rows, s.cursor, w.Offset, contextSortColumn(order.Key), order.Desc)
	body := renderTableLines(t, width, w.Count, st)
	if !hasContextCalls(rc) {
		body = append([]string{st.Body.Render("No per-call context records in this run.")}, body...)
	}
	for _, l := range notes {
		body = append(body, st.Muted.Render(l))
	}
	spec := frameSpec{run: s.run, active: ScreenContext, keys: s.helpKeys(), screenKeys: s.screenKeys(), escDesc: s.escDesc(),
		hint: FormatScrollHint(w, len(rows)), sort: s.sort.Label()}
	return s.render(spec, body, width, height, st)
}

// helpKeys are the short help keys of the list currently shown.
func (s *ContextScreen) helpKeys() []tuicommon.HelpEntry {
	if s.drilled {
		return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "select"}, {Key: KeySort, Desc: "sort"}, {Key: "esc", Desc: "actors"}}
	}
	return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "select"}, {Key: "enter", Desc: "calls"},
		{Key: KeySort, Desc: "sort"}, {Key: "esc", Desc: "back"}}
}

// screenKeys are the overlay entries of the keys this view has of its own.
func (s *ContextScreen) screenKeys() []tuicommon.HelpEntry {
	if s.drilled {
		return []tuicommon.HelpEntry{{Key: KeySort, Desc: "cycle the call order, cost and context sorts"}}
	}
	return []tuicommon.HelpEntry{sortKeyEntry, {Key: "enter", Desc: "show the context curve and calls of the selected actor"}}
}

// escDesc says what esc does on the list currently shown.
func (s *ContextScreen) escDesc() string {
	if s.drilled {
		return "back to the actors"
	}
	return "back to the runs list"
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
