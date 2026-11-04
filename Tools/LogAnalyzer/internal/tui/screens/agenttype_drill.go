package screens

import "mosaic-log-analyzer/internal/domain"

// enter drills into the type under the cursor, or, in the drill-down, asks for the
// session of the dispatch under the cursor.
func (s *AgentTypesScreen) enter() {
	if len(s.run.ByAgentType) == 0 {
		return
	}
	if !s.drilled {
		s.drilled, s.drillCursor, s.drillOffset = true, 0, 0
		s.drillSort = newDispatchSortCycle()
		return
	}
	entries := dispatchEntries([]domain.AgentTypeGroup{s.drilledGroup()})
	if len(entries) == 0 {
		return
	}
	order := dispatchOrder(entries, s.drillSort.Active())
	s.requestSession(entries[order[clampIndex(s.drillCursor, len(order))]].d.Invocation.Actor)
}

// viewDrill renders the dispatches of the drilled agent type in the table of the
// Dispatches view, without the orchestrator row.
func (s *AgentTypesScreen) viewDrill() string {
	g := s.drilledGroup()
	entries := dispatchEntries([]domain.AgentTypeGroup{g})
	order := s.drillSort.Active()
	rows := dispatchTableRows(s.styles.Currency, s.run, false, entries, dispatchOrder(entries, order), s.styles.Theme)

	head := []string{s.styles.Subtitle.Render("Dispatches of " + typeLabel(g.AgentType))}
	if len(rows) == 0 {
		head = append(head, s.styles.Body.Render("No dispatches of this type."))
	}
	lines, w := dispatchTableLines(rows, s.drillCursor, &s.drillOffset, order, s.width, bodyBudget(s.height)-len(head)-1, s.styles)

	spec := frameSpec{run: s.run, active: ScreenAgentTypes, keys: s.helpKeys(), screenKeys: s.screenKeys(), escDesc: s.escDesc(),
		legend: dispatchLegend(),
		hint:   FormatScrollHint(w, len(rows)), sort: s.drillSort.Label()}
	return s.render(spec, append(head, lines...), s.width, s.height, s.styles)
}

// effortSegment renders "effort: <levels>", or the absent marker when no level was reported.
func effortSegment(l domain.EffortLevels) string {
	if l.IsEmpty() {
		return "effort: " + AbsentMarker
	}
	return "effort: " + l.Label()
}
