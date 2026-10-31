package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The By status tables: one row per status group, with a bar for the group's
// share of the dispatches and one for its share of the cost. The run and every
// agent type each get a table of their own.

// Columns of the By status table, in display order.
const (
	scName = iota
	scDispatches
	scDispatchShare
	scCost
	scCostShare
	scFlags
	scCount
)

// statusColumns lists the columns with the priority at which each is dropped on a
// narrow terminal: the group, its dispatch count and its cost stay.
func statusColumns() []table.Column {
	cols := make([]table.Column, scCount)
	cols[scName] = table.Column{Title: "status", Flex: true, MinWidth: 10}
	cols[scDispatches] = table.Column{Title: "dispatches", Align: table.AlignRight}
	cols[scDispatchShare] = table.Column{Title: "dispatch share", Priority: 3, Width: shareColumnWidth}
	cols[scCost] = table.Column{Title: "cost", Align: table.AlignRight}
	cols[scCostShare] = table.Column{Title: "cost share", Priority: 2, Width: shareColumnWidth}
	cols[scFlags] = table.Column{Title: "flags", Priority: 1}
	return cols
}

// newStatusSortCycle offers the sorts of the status tables, cost first.
func newStatusSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortCost, true}, SortOrder{SortInvocations, true}, SortOrder{SortName, false})
}

// statusSortColumn is the column that shows the sort key.
func statusSortColumn(key SortKey) int {
	switch key {
	case SortCost:
		return scCost
	case SortInvocations:
		return scDispatches
	case SortName:
		return scName
	}
	return table.NoSort
}

// statusName is the label of a status group: its code, or the label of the
// no-status and open groups.
func statusName(g domain.StatusGroup) string {
	switch g.Kind {
	case domain.StatusNone:
		return NoStatusLabel
	case domain.StatusOpen:
		return OpenLabel
	}
	return g.Code
}

// statusGroupOrder returns the indexes of the groups in the order of the sort.
func statusGroupOrder(groups []domain.StatusGroup, o SortOrder) []int {
	return sortedOrder(len(groups), o,
		func(i int) string { return statusName(groups[i]) },
		func(key SortKey, i int) (int64, bool) {
			f := groups[i].Figures
			switch key {
			case SortCost:
				return moneyNanos(f.Totals.Money.Total)
			case SortInvocations:
				return int64(f.Invocations), true
			}
			return 0, false
		})
}

// statusTotals are the dispatch count and known cost the share bars of a table
// are measured against.
func statusTotals(groups []domain.StatusGroup) (dispatches, cost int64) {
	for _, g := range groups {
		dispatches += int64(g.Figures.Invocations)
		n, _ := moneyNanos(g.Figures.Totals.Money.Total)
		cost += n
	}
	return dispatches, cost
}

// barCell is a bar of part's share of whole with its whole percent.
func barCell(part, whole int64, role tuicommon.StyleRole, theme tuicommon.Theme) table.Cell {
	return shareCell(part, whole, true, role, theme)
}

// statusCells builds the cells of one status group row.
func statusCells(cur domain.CurrencyCode, g domain.StatusGroup, dispatches, cost int64, theme tuicommon.Theme) []table.Cell {
	f := g.Figures
	open := f.Open
	if g.Kind == domain.StatusOpen {
		open = 0 // the group name already says open
	}
	part, priced := moneyNanos(f.Totals.Money.Total)
	cells := make([]table.Cell, scCount)
	cells[scName] = table.Cell{Text: statusName(g), Role: StatusGroupRole(g)}
	cells[scDispatches] = table.Cell{Text: formatGrouped(int64(f.Invocations))}
	cells[scDispatchShare] = barCell(int64(f.Invocations), dispatches, tuicommon.RoleDataActor, theme)
	cells[scCost] = moneyTotalCell(cur, f.Totals.Money)
	cells[scCostShare] = shareCell(part, cost, priced, tuicommon.RoleDataCost, theme)
	cells[scFlags] = warningCell(badgeLabels(incompleteness(f.UnpricedModels, f.Totals.AssumedTierCacheWrite, f.Totals.AssumedTiers, f.MissingUsage, open)))
	return cells
}

// statusTable is the groups as a table in the order of the sort.
func statusTable(groups []domain.StatusGroup, o SortOrder, st Styles) table.Table {
	dispatches, cost := statusTotals(groups)
	var rows [][]table.Cell
	for _, i := range statusGroupOrder(groups, o) {
		rows = append(rows, statusCells(st.Currency, groups[i], dispatches, cost, st.Theme))
	}
	return table.Table{
		Columns: statusColumns(), Rows: rows, Cursor: table.NoCursor, SortColumn: statusSortColumn(o.Key), SortDesc: o.Desc,
	}
}
