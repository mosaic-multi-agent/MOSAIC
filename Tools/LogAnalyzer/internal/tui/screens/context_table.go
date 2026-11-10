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
	xcModel
	xcRewrites
	xcCalls
	xcFirst
	xcFirstWrite
	xcPeak
	xcFinal
	xcLogged
	xcHarness
	xcDetected
	xcUnapplied
	xcCompactionCost
	xcSpark
	xcCount
)

// sparkWidth is the fixed width of the context sparkline column.
const sparkWidth = 16

// compactionGroup labels the header line above the compaction columns.
const compactionGroup = "compactions"

// contextColumns lists the columns with the priority at which each is dropped on
// a narrow terminal: the identity and the peak stay.
func contextColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, xcCount)
	cols[xcInstance] = table.Column{Title: "instance", Flex: true, Identity: true, MinWidth: identityMinWidth}
	cols[xcModel] = table.Column{Title: "model", Priority: 1, Flex: true, MinWidth: 8}
	cols[xcRewrites] = table.Column{Title: "rewrites", Align: right, Priority: 6}
	cols[xcCalls] = table.Column{Title: "API calls", Align: right, Priority: 4}
	cols[xcFirst] = table.Column{Title: "first context", Align: right, Priority: 5}
	cols[xcFirstWrite] = table.Column{Title: "context at first write", Align: right, Priority: 5}
	cols[xcPeak] = table.Column{Title: "peak context", Align: right, Priority: 2}
	cols[xcFinal] = table.Column{Title: "final context", Align: right, Priority: 3}
	cols[xcLogged] = table.Column{Title: "logged", Align: right, Priority: 6, Group: compactionGroup}
	cols[xcHarness] = table.Column{Title: "harness", Align: right, Priority: 6, Group: compactionGroup}
	cols[xcDetected] = table.Column{Title: "detected only", Align: right, Priority: 6, Group: compactionGroup}
	cols[xcUnapplied] = table.Column{Title: "unapplied", Align: right, Priority: 6, Group: compactionGroup}
	cols[xcCompactionCost] = table.Column{Title: "cost", Align: right, Priority: 6, Group: compactionGroup}
	cols[xcSpark] = table.Column{Title: "context", Priority: 7, Width: sparkWidth}
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

// isMainCall reports whether a call belongs to the main loop; a call without a
// kind is one.
func isMainCall(c domain.ContextCall) bool {
	return c.Kind == "" || c.Kind == domain.CallMain
}

// modelsOf lists the distinct models of the main calls in order of first use.
func modelsOf(calls []domain.ContextCall) []string {
	var out []string
	seen := map[domain.ModelID]bool{}
	for _, c := range calls {
		if isMainCall(c) && c.Model != "" && !seen[c.Model] {
			seen[c.Model] = true
			out = append(out, string(c.Model))
		}
	}
	return out
}

// contextSparkCell draws the context of every call as a sparkline coloured per
// point by band, the first write call drawn with the marker.
func contextSparkCell(a domain.ActorContext, theme tuicommon.Theme) table.Cell {
	var values []float64
	var roles []tuicommon.StyleRole
	marker := -1
	for _, c := range a.Calls {
		if !isMainCall(c) {
			continue
		}
		v, ok := c.Context.Value()
		role := tuicommon.RoleSignalMuted
		if ok {
			role = BandRole(domain.BandOf(v))
		}
		values = append(values, float64(v))
		roles = append(roles, role)
		if c.FirstWrite && marker < 0 {
			marker = len(values) - 1
		}
	}
	return table.Cell{Fill: func(width int) string {
		return chart.Sparkline(values, roles, marker, width, theme)
	}}
}

// countCell is a plain count.
func countCell(n int) table.Cell { return table.Cell{Text: strconv.Itoa(n)} }

// contextCells builds the cells of one actor's row.
func contextCells(cur domain.CurrencyCode, a domain.ActorContext, label string, labelRole tuicommon.StyleRole, theme tuicommon.Theme) []table.Cell {
	cells := make([]table.Cell, xcCount)
	cells[xcInstance] = table.Cell{Text: label, Role: labelRole}
	cells[xcModel] = mutedCell(AbsentMarker)
	if models := modelsOf(a.Calls); len(models) > 0 {
		cells[xcModel] = table.Cell{Text: strings.Join(models, ",")}
	}
	cells[xcCalls] = countCell(len(a.Calls))
	cells[xcRewrites] = rewriteCountCell(len(a.Rewrites))
	cells[xcFirst] = contextCell(a.First)
	cells[xcFirstWrite] = contextCell(a.FirstWrite)
	cells[xcPeak] = contextCell(a.Peak)
	cells[xcFinal] = contextCell(a.Final)
	cells[xcLogged] = countCell(a.LoggedCount())
	cells[xcHarness] = countCell(a.HarnessCount())
	cells[xcDetected] = countCell(a.DetectedOnlyCount())
	cells[xcUnapplied] = countCell(a.CompactionCalls.UnappliedCount)
	cells[xcCompactionCost] = moneyCell(cur, a.CompactionCalls.Cost, true)
	cells[xcSpark] = contextSparkCell(a, theme)
	return cells
}

// orchestratorRowLabel is the label of the orchestrator's row.
const orchestratorRowLabel = "orchestrator"

// contextTableRows builds the rows: the orchestrator leads, then the invocations
// in the order given.
func contextTableRows(cur domain.CurrencyCode, rc domain.RunContext, order []int, theme tuicommon.Theme) [][]table.Cell {
	rows := [][]table.Cell{contextCells(cur, rc.Orchestrator, orchestratorRowLabel, tuicommon.RoleDataLead, theme)}
	for _, i := range order {
		a := rc.Invocations[i]
		rows = append(rows, contextCells(cur, a, a.Actor.Label(), tuicommon.RoleDataActor, theme))
	}
	return rows
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
