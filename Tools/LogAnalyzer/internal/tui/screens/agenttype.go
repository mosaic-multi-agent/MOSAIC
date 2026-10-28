package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	tuicommon "mosaic-common/tui"

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
	runChrome

	typeSort SortCycle
	cursor   int // selected row of the sorted type list
	offset   int // first visible type row

	drilled     bool
	drillSort   SortCycle
	drillCursor int // selected row of the sorted dispatches of the drilled type
	drillOffset int // first visible dispatch row
}

// NewAgentTypesScreen creates the by-agent-type screen for one run.
func NewAgentTypesScreen(run domain.RunReport, width, height int, styles Styles) *AgentTypesScreen {
	return &AgentTypesScreen{
		run: run, width: width, height: height, styles: styles, nav: viewNav{self: ScreenAgentTypes},
		typeSort: newTypeSortCycle(), drillSort: newDispatchSortCycle(),
	}
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
		if !s.drilled && len(s.run.ByAgentType) > 0 {
			s.drilled, s.drillCursor, s.drillOffset = true, 0, 0
			s.drillSort = newDispatchSortCycle()
		}
	case KeySort:
		if s.drilled {
			s.drillSort = s.drillSort.Next()
		} else {
			s.typeSort = s.typeSort.Next()
		}
	default:
		s.nav.request(k)
	}
	return s, nil
}

// moveCursor moves the cursor of the list currently shown, staying in range.
func (s *AgentTypesScreen) moveCursor(delta int) {
	if s.drilled {
		s.drillCursor = clampIndex(s.drillCursor+delta, len(s.drilledGroup().Dispatches))
		return
	}
	s.cursor = clampIndex(s.cursor+delta, len(s.run.ByAgentType))
}

// drilledGroup is the agent type group under the cursor of the sorted type list.
func (s *AgentTypesScreen) drilledGroup() domain.AgentTypeGroup {
	order := typeOrder(s.run.ByAgentType, s.typeSort.Active())
	return s.run.ByAgentType[order[clampIndex(s.cursor, len(order))]]
}

// View implements Screen.
func (s *AgentTypesScreen) View(width, height int) string {
	s.width, s.height = width, height
	if s.drilled {
		return s.viewDrill()
	}
	st := s.styles
	var head []string
	if len(s.run.ByAgentType) == 0 {
		head = append(head, st.Body.Render(noTypesLine))
	} else {
		for _, l := range strings.Split(tuicommon.Wrap(typeNote, width), "\n") {
			head = append(head, st.Muted.Render(l))
		}
	}
	order := s.typeSort.Active()
	groups := typeOrder(s.run.ByAgentType, order)
	budget := bodyBudget(height) - len(head) - 1
	if s.run.Unattributed != nil {
		budget-- // the unattributed row stays inside the viewport budget
	}
	body := head
	var w Window
	if len(s.run.ByAgentType) > 0 || s.run.Unattributed != nil {
		var lines []string
		lines, w = typeTableLines(s.run, groups, s.cursor, &s.offset, order, width, budget, st)
		body = append(body, lines...)
	}
	spec := frameSpec{run: s.run, active: ScreenAgentTypes, keys: s.helpKeys(), screenKeys: s.screenKeys(), escDesc: s.escDesc(),
		hint: FormatScrollHint(w, len(groups)), sort: s.typeSort.Label()}
	return s.render(spec, body, width, height, st)
}

// helpKeys are the short help keys of the list currently shown.
func (s *AgentTypesScreen) helpKeys() []tuicommon.HelpEntry {
	if s.drilled {
		return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "select"}, {Key: KeySort, Desc: "sort"}, {Key: "esc", Desc: "types"}}
	}
	return []tuicommon.HelpEntry{{Key: "↑/↓", Desc: "select"}, {Key: "enter", Desc: "dispatches"},
		{Key: KeySort, Desc: "sort"}, {Key: "esc", Desc: "back"}}
}

// screenKeys are the overlay entries of the keys this view has of its own.
func (s *AgentTypesScreen) screenKeys() []tuicommon.HelpEntry {
	if s.drilled {
		return []tuicommon.HelpEntry{sortKeyEntry}
	}
	return []tuicommon.HelpEntry{sortKeyEntry, {Key: "enter", Desc: "list the dispatches of the selected type"}}
}

// escDesc says what esc does on the list currently shown.
func (s *AgentTypesScreen) escDesc() string {
	if s.drilled {
		return "back to the agent types"
	}
	return "back to the runs list"
}

// Help implements Screen.
func (s *AgentTypesScreen) Help() string { return shortHelpText("", s.helpKeys()) }

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
	return s.drilledGroup().AgentType, true
}
