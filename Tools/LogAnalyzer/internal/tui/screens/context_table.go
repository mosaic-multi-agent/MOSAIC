package screens

import (
	"strconv"
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The Context table: one row per actor with how its context grew, led by the
// orchestrator's own row.

// Columns of the Context table, in display order.
const (
	xcInstance = iota
	xcType
	xcModel
	xcCalls
	xcFirst
	xcFirstWrite
	xcPeak
	xcFinal
	xcLogged
	xcDetected
	xcSpark
	xcCount
)

// sparkWidth is the fixed width of the context sparkline column.
const sparkWidth = 16

// contextColumns lists the columns with the priority at which each is dropped on
// a narrow terminal: the identity and the peak stay.
func contextColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, xcCount)
	cols[xcInstance] = table.Column{Title: "instance", Flex: true, MinWidth: 10}
	cols[xcType] = table.Column{Title: "type", Priority: 3, Flex: true, MinWidth: 8}
	cols[xcModel] = table.Column{Title: "model", Priority: 5, Flex: true, MinWidth: 8}
	cols[xcCalls] = table.Column{Title: "calls", Align: right, Priority: 2}
	cols[xcFirst] = table.Column{Title: "first", Align: right, Priority: 4}
	cols[xcFirstWrite] = table.Column{Title: "first write*", Align: right, Priority: 4}
	cols[xcPeak] = table.Column{Title: "peak", Align: right}
	cols[xcFinal] = table.Column{Title: "final", Align: right, Priority: 1}
	cols[xcLogged] = table.Column{Title: "logged", Align: right, Priority: 5}
	cols[xcDetected] = table.Column{Title: "detected", Align: right, Priority: 5}
	cols[xcSpark] = table.Column{Title: "context", Priority: 6, Width: sparkWidth}
	return cols
}

// newContextSortCycle offers the sorts of the Context table, peak context first.
func newContextSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortPeakContext, true}, SortOrder{SortFinal, true},
		SortOrder{SortCalls, true}, SortOrder{SortName, false})
}

// contextSortColumn is the column that shows the sort key.
func contextSortColumn(key SortKey) int {
	switch key {
	case SortPeakContext:
		return xcPeak
	case SortFinal:
		return xcFinal
	case SortCalls:
		return xcCalls
	case SortName:
		return xcInstance
	}
	return table.NoSort
}

// contextOrder returns the indexes of the actors in the order of the sort.
func contextOrder(actors []domain.ActorContext, o SortOrder) []int {
	return sortedOrder(len(actors), o,
		func(i int) string { return actors[i].Actor.Label() },
		func(key SortKey, i int) (int64, bool) {
			a := actors[i]
			switch key {
			case SortPeakContext:
				return a.Peak.Tokens.Value()
			case SortFinal:
				return a.Final.Tokens.Value()
			case SortCalls:
				return int64(len(a.Calls)), true
			}
			return 0, false
		})
}

// modelsOf lists the distinct models of the calls in order of first use.
func modelsOf(calls []domain.ContextCall) []string {
	var out []string
	seen := map[domain.ModelID]bool{}
	for _, c := range calls {
		if c.Model != "" && !seen[c.Model] {
			seen[c.Model] = true
			out = append(out, string(c.Model))
		}
	}
	return out
}

// contextSparkCell draws the context of every call as a sparkline coloured per
// point by band, the first write call drawn with the marker.
func contextSparkCell(a domain.ActorContext, theme tuicommon.Theme) table.Cell {
	values := make([]float64, len(a.Calls))
	roles := make([]tuicommon.StyleRole, len(a.Calls))
	marker := -1
	for i, c := range a.Calls {
		v, ok := c.Context.Value()
		values[i] = float64(v)
		roles[i] = tuicommon.RoleSignalMuted
		if ok {
			roles[i] = BandRole(domain.BandOf(v))
		}
		if c.FirstWrite && marker < 0 {
			marker = i
		}
	}
	return table.Cell{Fill: func(width int) string {
		return chart.Sparkline(values, roles, marker, width, theme)
	}}
}

// countCell is a plain count.
func countCell(n int) table.Cell { return table.Cell{Text: strconv.Itoa(n)} }

// contextCells builds the cells of one actor's row.
func contextCells(a domain.ActorContext, label string, labelRole tuicommon.StyleRole, theme tuicommon.Theme) []table.Cell {
	cells := make([]table.Cell, xcCount)
	cells[xcInstance] = table.Cell{Text: label, Role: labelRole}
	if label != orchestratorRowLabel {
		cells[xcType] = table.Cell{Text: typeLabel(a.Actor.Type)}
	}
	cells[xcModel] = mutedCell(AbsentMarker)
	if models := modelsOf(a.Calls); len(models) > 0 {
		cells[xcModel] = table.Cell{Text: strings.Join(models, ",")}
	}
	cells[xcCalls] = countCell(len(a.Calls))
	cells[xcFirst] = contextCell(a.First)
	cells[xcFirstWrite] = contextCell(a.FirstWrite)
	cells[xcPeak] = contextCell(a.Peak)
	cells[xcFinal] = contextCell(a.Final)
	cells[xcLogged] = countCell(a.LoggedCount())
	cells[xcDetected] = countCell(a.DetectedCount())
	cells[xcSpark] = contextSparkCell(a, theme)
	return cells
}

// orchestratorRowLabel is the label of the orchestrator's row.
const orchestratorRowLabel = "orchestrator"

// contextTableRows builds the rows: the orchestrator leads, then the invocations
// in the order given.
func contextTableRows(rc domain.RunContext, order []int, theme tuicommon.Theme) [][]table.Cell {
	rows := [][]table.Cell{contextCells(rc.Orchestrator, orchestratorRowLabel, tuicommon.RoleDataLead, theme)}
	for _, i := range order {
		a := rc.Invocations[i]
		rows = append(rows, contextCells(a, a.Actor.Label(), tuicommon.RoleDataActor, theme))
	}
	return rows
}

// contextNotes are the lines that explain the table: the first write heuristic,
// the detected compactions and the definition of final context.
func contextNotes(width int) []string {
	notes := []string{
		"* first write is a heuristic: the context at the first write-class tool call; it is not computed for the orchestrator.",
		"logged compactions were recorded by the harness; detected ones are inferred from a drop in context between calls.",
		"final context = " + domain.FinalContextDefinition + ".",
	}
	var out []string
	for _, n := range notes {
		out = append(out, strings.Split(tuicommon.Wrap(n, width), "\n")...)
	}
	return out
}

// hasContextCalls reports whether any actor of the run has per-call records.
func hasContextCalls(rc domain.RunContext) bool {
	if len(rc.Orchestrator.Calls) > 0 {
		return true
	}
	for _, a := range rc.Invocations {
		if len(a.Calls) > 0 {
			return true
		}
	}
	return false
}
