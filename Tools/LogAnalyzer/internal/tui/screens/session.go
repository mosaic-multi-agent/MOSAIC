package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"

	"mosaic-log-analyzer/internal/domain"
)

// sessionMinRows is the fewest call rows kept on screen before the head scrolls away.
const sessionMinRows = 3

// SessionScreen shows one actor's session (the orchestrator or an invocation) of a run:
// its head, cost split, context curve and per-call table.
//
// Navigation contract:
//   - ↑/↓  move the selection through the calls.
//   - o     cycle the sort order of the calls.
//   - Esc   -> Back() == true; the router returns to the opener.
type SessionScreen struct {
	run    domain.RunReport
	actor  domain.ActorRef
	opener ScreenID
	width  int
	height int
	styles Styles
	back   bool
	nav    viewNav
	runChrome

	sort   SortCycle
	cursor int // selected call of the sorted call list
	offset int // first visible call
}

// NewSessionScreen shows one actor's session of run. opener is the per-run view it was
// opened from (ScreenRunDetail, ScreenAgentTypes or ScreenContext); its tab is the one
// highlighted in the frame.
func NewSessionScreen(run domain.RunReport, actor domain.ActorRef, opener ScreenID, width, height int, st Styles) *SessionScreen {
	return &SessionScreen{run: run, actor: actor, opener: opener, width: width, height: height, styles: st,
		nav: viewNav{}, sort: newCallSortCycle()}
}

// ID implements Screen.
func (s *SessionScreen) ID() ScreenID { return ScreenSession }

// Init implements Screen.
func (s *SessionScreen) Init() tea.Cmd { return nil }

// Update implements Screen.
func (s *SessionScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
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
		s.cursor = clampIndex(s.cursor-1, s.callCount())
	case "down", "j":
		s.cursor = clampIndex(s.cursor+1, s.callCount())
	case KeySort:
		s.sort = s.sort.Next()
	default:
		s.nav.request(k)
	}
	return s, nil
}

// callCount is the number of calls of the actor.
func (s *SessionScreen) callCount() int { return len(sessionContextOf(s.run, s.actor).Calls) }

// View implements Screen.
func (s *SessionScreen) View(width, height int) string {
	s.width, s.height = width, height
	st := s.styles
	a := sessionContextOf(s.run, s.actor)
	budget := bodyBudget(height)

	body := sessionHeadLines(st, width, s.run, s.actor, sessionInfoOf(s.run, s.actor), a.Calls)
	body = append(body, sessionFindingLines(st, width, s.run, s.actor)...)
	body = append(body, contextFigureLines(st, width, a)...)

	toolsTable := sessionToolsTable(sessionToolsOf(s.run, s.actor), width, st)
	toolRows, toolsFixed := len(toolsTable.Rows), 0
	if toolRows > 0 { // the blank line and the header lines of the tools table
		toolsFixed = 1 + extraHeaderLines(toolsTable, width) + 1
	}

	var w Window
	if len(a.Calls) == 0 {
		body = append(body, st.Body.Render("No per-call records for this actor."))
		body = append(body, sessionToolsLines(toolsTable, width, toolRows, st)...)
	} else {
		curveH := min((budget-len(body)-toolsFixed-toolRows)/2, maxCurveHeight)
		if curveH >= minCurveHeight {
			body = append(body, strings.Split(chart.RenderCurve(contextCurve(a), width, curveH, st.Theme), "\n")...)
		}
		rowBudget := budget - len(body) - 1 - toolsFixed - toolRows
		for rowBudget < sessionMinRows && toolRows > 1 { // the tools make room for the calls
			toolRows--
			rowBudget++
		}
		if rowBudget < sessionMinRows { // the page scrolls: the head makes room for the calls
			drop := min(sessionMinRows-rowBudget, len(body))
			body, rowBudget = body[drop:], rowBudget+drop
		}
		var lines []string
		lines, w = callTableLines(a, s.sort.Active(), s.cursor, &s.offset, width, rowBudget, st)
		body = append(body, lines...)
		body = append(body, sessionToolsLines(toolsTable, width, toolRows, st)...)
	}
	spec := frameSpec{run: s.run, active: s.opener, keys: s.helpKeys(), screenKeys: s.screenKeys(), escDesc: s.escDesc(),
		legend: callLegend(), hint: FormatScrollHint(w, len(a.Calls)), sort: s.sort.Label()}
	return s.render(spec, body, width, height, st)
}

// helpKeys are the short help keys of the screen.
func (s *SessionScreen) helpKeys() []tuicommon.HelpEntry {
	return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "select"}, {Key: KeySort, Desc: "sort"}, exportKeyEntry, {Key: "esc", Desc: "back"}}
}

// screenKeys are the overlay entries of the keys this screen has of its own.
func (s *SessionScreen) screenKeys() []tuicommon.HelpEntry {
	return []tuicommon.HelpEntry{{Key: KeySort, Desc: "cycle the call order, cost and context sorts"}}
}

// escDesc says what esc does.
func (s *SessionScreen) escDesc() string { return "back to the view the session was opened from" }

// Help implements Screen.
func (s *SessionScreen) Help() string { return shortHelpText("", s.helpKeys()) }

// Back reports whether esc or q was pressed.
func (s *SessionScreen) Back() bool { return s.back }

// Requested reports a per-run view key pressed, the opener's own included.
func (s *SessionScreen) Requested() (ScreenID, bool) { return s.nav.requested() }

// Reset clears Back and Requested.
func (s *SessionScreen) Reset() {
	s.back = false
	s.nav.reset()
}

// Actor is the actor whose session is shown.
func (s *SessionScreen) Actor() domain.ActorRef { return s.actor }

// Opener is the per-run view the screen was opened from.
func (s *SessionScreen) Opener() ScreenID { return s.opener }
