package screens

import (
	"fmt"

	"mosaic-log-analyzer/internal/domain"
)

// viewDrill renders the individual dispatches of the drilled agent type.
func (s *AgentTypesScreen) viewDrill() string {
	g := s.run.ByAgentType[s.cursor]
	name := g.AgentType
	if name == "" {
		name = UnknownTypeLabel
	}
	rows := make([]string, len(g.Dispatches))
	for i, d := range g.Dispatches {
		rows[i] = dispatchRow(d)
	}
	if len(rows) == 0 {
		rows = []string{"No dispatches of this type."}
	}
	body, w := s.cursorRows(rows, s.drillCursor, &s.drillOffset, 0)
	label := fmt.Sprintf("%s / %s", runLabel(s.run), name)
	return renderPage(s.styles, s.width, "Log Analyzer — Dispatches", label, body, w, len(rows), s.Help())
}

// dispatchRow renders one dispatch: instance, status, total and the labels of
// any incomplete data.
func dispatchRow(d domain.Dispatch) string {
	inv := d.Invocation
	status := inv.StatusCode
	switch {
	case inv.State == domain.InvocationOpen:
		status = OpenLabel
	case status == "":
		status = NoStatusLabel
	}
	row := fmt.Sprintf("%-20s %-12s %s", inv.Actor.Label(), status, formatMoneyTotal(d.Totals.Money))
	row += "  " + effortSegment(d.Effort)
	var labels []string
	if d.MissingUsage {
		labels = append(labels, MissingUsageLabel)
	}
	if len(d.UnpricedModels) > 0 {
		labels = append(labels, UnpricedMarker+" models")
	}
	if a := FormatAssumedTier(d.Totals.AssumedTierCacheWrite); a != "" {
		labels = append(labels, a)
	}
	return appendLabels(row, labels)
}

// effortSegment renders "effort: <levels>", or the absent marker when no level was reported.
func effortSegment(l domain.EffortLevels) string {
	if l.IsEmpty() {
		return "effort: " + AbsentMarker
	}
	return "effort: " + l.Label()
}
