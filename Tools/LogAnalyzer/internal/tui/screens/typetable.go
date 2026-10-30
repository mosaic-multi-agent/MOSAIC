package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The By agent type table: one row per agent type, with the run's unattributed
// usage as a last row of its own.

// Columns of the By agent type table, in display order.
const (
	tcType = iota
	tcInvocations
	tcTotal
	tcAverage
	tcMin
	tcMax
	tcShare
	tcFlags
	tcAvgPeak
	tcMaxPeak
	tcHit
	tcRewrites
	tcWrite
	tcWriteCost
	tcRepeats
	tcRepeatSpend
	tcRework
	tcReworkCost
	tcCount
)

// typeNote states what the repeat and rework figures mean and where they stop
// being exact; it sits above the table.
const typeNote = "repeat spend includes planned repeats; likely rework is a heuristic; " + domain.ReworkLimitNote

// typeColumns lists the columns with the priority at which each is dropped on a
// narrow terminal: type, invocations and total stay.
func typeColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, tcCount)
	cols[tcType] = table.Column{Title: "type", Flex: true, MinWidth: 10}
	cols[tcInvocations] = table.Column{Title: "invocations", Align: right}
	cols[tcTotal] = table.Column{Title: "total", Align: right}
	cols[tcAverage] = table.Column{Title: "average", Align: right, Priority: 1}
	cols[tcMin] = table.Column{Title: "min", Align: right, Priority: 3}
	cols[tcMax] = table.Column{Title: "max", Align: right, Priority: 3}
	cols[tcShare] = shareBarColumn(5)
	cols[tcFlags] = table.Column{Title: "flags", Priority: 1}
	cols[tcAvgPeak] = table.Column{Title: "avg peak", Align: right, Priority: 2}
	cols[tcMaxPeak] = table.Column{Title: "max peak", Align: right, Priority: 2}
	cols[tcHit] = table.Column{Title: "cache hit", Align: right, Priority: 4}
	cols[tcRewrites] = table.Column{Title: "rewrites", Align: right, Priority: 4}
	cols[tcWrite] = table.Column{Title: "cache write", Align: right, Priority: 4}
	cols[tcWriteCost] = table.Column{Title: "write cost", Align: right, Priority: 4}
	cols[tcRepeats] = table.Column{Title: "repeats", Align: right, Priority: 3}
	cols[tcRepeatSpend] = table.Column{Title: "repeat spend", Align: right, Priority: 3}
	cols[tcRework] = table.Column{Title: "likely rework", Align: right, Priority: 3}
	cols[tcReworkCost] = table.Column{Title: "rework cost", Align: right, Priority: 3}
	return cols
}

// newTypeSortCycle offers the sorts of the type table, cost first.
func newTypeSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortCost, true}, SortOrder{SortInvocations, true},
		SortOrder{SortPeakContext, true}, SortOrder{SortName, false})
}

// typeSortColumn is the column that shows the sort key.
func typeSortColumn(key SortKey) int {
	switch key {
	case SortCost:
		return tcTotal
	case SortInvocations:
		return tcInvocations
	case SortPeakContext:
		return tcMaxPeak
	case SortName:
		return tcType
	}
	return table.NoSort
}

// typeOrder returns the indexes of the groups in the order of the sort.
func typeOrder(groups []domain.AgentTypeGroup, o SortOrder) []int {
	return sortedOrder(len(groups), o,
		func(i int) string { return typeLabel(groups[i].AgentType) },
		func(key SortKey, i int) (int64, bool) {
			g := groups[i]
			switch key {
			case SortCost:
				return moneyNanos(g.Figures.Totals.Money.Total)
			case SortInvocations:
				return int64(g.Figures.Invocations), true
			case SortPeakContext:
				return g.PeakContext.Max.Tokens.Value()
			}
			return 0, false
		})
}

// typeCells builds the cells of one agent type row; runCost is the run total the
// share bar is measured against.
func typeCells(cur domain.CurrencyCode, g domain.AgentTypeGroup, runCost domain.MoneyValue, theme tuicommon.Theme) []table.Cell {
	f := g.Figures
	cells := make([]table.Cell, tcCount)
	cells[tcType] = table.Cell{Text: typeLabel(g.AgentType), Role: tuicommon.RoleDataActor}
	cells[tcInvocations] = table.Cell{Text: formatGrouped(int64(f.Invocations))}
	cells[tcTotal] = moneyTotalCell(cur, f.Totals.Money)
	stats := g.PerInvocation
	cells[tcAverage] = statCell(cur, stats, stats.Average)
	cells[tcMin] = statCell(cur, stats, stats.Min)
	cells[tcMax] = statCell(cur, stats, stats.Max)
	cells[tcShare] = shareBarCell(f.Totals.Money.Total, runCost, theme)
	cells[tcFlags] = warningCell(badgeLabels(incompleteness(f.UnpricedModels, f.Totals.AssumedTierCacheWrite, f.Totals.AssumedTiers, f.MissingUsage, f.Open)))
	// The average peak is derived from several runs of figures and cannot carry a partial marker.
	cells[tcAvgPeak] = contextCell(domain.ContextFigure{Tokens: g.PeakContext.Average, Band: domain.BandOf(g.PeakContext.Average.ValueOr(0))})
	cells[tcMaxPeak] = contextCell(g.PeakContext.Max)
	cells[tcHit] = ratioCell(g.Cache.HitRatio)
	cells[tcRewrites] = rewriteCountCell(g.Cache.Rewrites.Count())
	cells[tcWrite] = tokensCell(g.Cache.WriteTokens)
	cells[tcWriteCost] = moneyCell(cur, g.Cache.WriteCost, g.Cache.Complete)
	cells[tcRepeats] = table.Cell{Text: formatGrouped(int64(g.Rework.Repeats.Invocations))}
	cells[tcRepeatSpend] = moneyTotalCell(cur, g.Rework.Repeats.Totals.Money)
	cells[tcRework] = table.Cell{Text: formatGrouped(int64(g.Rework.LikelyRework.Invocations))}
	cells[tcReworkCost] = moneyTotalCell(cur, g.Rework.LikelyRework.Totals.Money)
	return cells
}

// statCell is a per-invocation statistic; it is marked partial when the statistics
// do not cover every member.
func statCell(cur domain.CurrencyCode, stats domain.MoneyStats, v domain.MoneyValue) table.Cell {
	c := table.Cell{Text: FormatMoney(cur, v), Role: MoneyRole(v, stats.Complete)}
	if stats.Basis > 0 && !stats.Complete {
		c.Text += " (partial)"
	}
	return c
}

// unattributedCells is the row of the run's unattributed usage: its own cost and
// share, and nothing for the figures that only agent types have.
func unattributedCells(cur domain.CurrencyCode, u domain.ActorReport, runCost domain.MoneyValue, theme tuicommon.Theme) []table.Cell {
	cells := make([]table.Cell, tcCount)
	cells[tcType] = table.Cell{Text: UnattributedRowLabel, Role: tuicommon.RoleSignalWarning}
	cells[tcTotal] = moneyTotalCell(cur, u.Totals.Money)
	cells[tcShare] = shareBarCell(u.Totals.Money.Total, runCost, theme)
	return cells
}

// typeTableLines draws the type rows, then the unattributed row when there is one,
// into lines. The window follows the cursor within rowBudget rows of types.
func typeTableLines(run domain.RunReport, order []int, cursor int, offset *int, o SortOrder, width, rowBudget int, st Styles) ([]string, Window) {
	// Only the visible types are handed to the table, so its own offset stays 0 and
	// the cursor is relative to the window.
	w := ScrollWindow(*offset, cursor, len(order), rowBudget)
	*offset = w.Offset
	var rows [][]table.Cell
	for _, i := range order[w.Offset:w.End()] {
		rows = append(rows, typeCells(st.Currency, run.ByAgentType[i], run.Totals.Money.Total, st.Theme))
	}
	if u := run.Unattributed; u != nil {
		rows = append(rows, unattributedCells(st.Currency, *u, run.Totals.Money.Total, st.Theme))
	}
	rowCursor := cursor - w.Offset
	if len(order) == 0 {
		rowCursor = table.NoCursor
	}
	t := table.Table{
		Columns: typeColumns(), Rows: rows, Cursor: rowCursor, SortColumn: typeSortColumn(o.Key), SortDesc: o.Desc,
	}
	return renderTableLines(t, width, len(rows), st), w
}

// UnattributedRowLabel titles the type table row of the run's unattributed usage.
const UnattributedRowLabel = "Unattributed"

// noTypesLine is shown in place of the table when the run has no agent types.
const noTypesLine = "No subagent invocations in this run."
