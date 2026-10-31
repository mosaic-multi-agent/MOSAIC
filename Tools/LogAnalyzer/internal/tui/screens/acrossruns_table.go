package screens

import (
	"fmt"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The across-runs tables: the runs with a bar for each run's share of the cost,
// and the agent types summed over the runs with their per-run averages and a bar.

// Columns of the runs table, in display order.
const (
	arRun = iota
	arCost
	arShare
	arSubagents
	arFlags
	arCount
)

// Columns of the agent type table, in display order.
const (
	atType = iota
	atInvocations
	atRuns
	atTotal
	atShare
	atAverage
	atPerRunInvocations
	atPerRunTokens
	atFlags
	atCount
)

// acrossRunColumns lists the runs table columns with the priority at which each is
// dropped on a narrow terminal: the run and its cost stay.
func acrossRunColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, arCount)
	cols[arRun] = table.Column{Title: "run", Flex: true, MinWidth: 12}
	cols[arCost] = table.Column{Title: "total", Align: right}
	cols[arShare] = shareBarColumn(3)
	cols[arSubagents] = table.Column{Title: "subagents", Align: right, Priority: 1}
	cols[arFlags] = table.Column{Title: "flags", Priority: 2}
	return cols
}

// acrossTypeColumns lists the agent type table columns with the priority at which
// each is dropped on a narrow terminal: the type and its total stay.
func acrossTypeColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, atCount)
	cols[atType] = table.Column{Title: "type", Flex: true, MinWidth: 10}
	cols[atInvocations] = table.Column{Title: "invocations", Align: right, Priority: 1}
	cols[atRuns] = table.Column{Title: "runs", Align: right, Priority: 1}
	cols[atTotal] = table.Column{Title: "total", Align: right}
	cols[atShare] = shareBarColumn(4)
	cols[atAverage] = table.Column{Title: "average per run", Align: right, Priority: 1}
	cols[atPerRunInvocations] = table.Column{Title: "invocations per run", Align: right, Priority: 2}
	cols[atPerRunTokens] = table.Column{Title: "tokens per run", Align: right, Priority: 2}
	cols[atFlags] = table.Column{Title: "flags", Priority: 3}
	return cols
}

// newAcrossSortCycle offers the sorts of both across-runs tables, cost first.
func newAcrossSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortCost, true}, SortOrder{SortName, false})
}

// acrossSortColumn is the column of a table that shows the sort key.
func acrossSortColumn(key SortKey, nameColumn, costColumn int) int {
	switch key {
	case SortCost:
		return costColumn
	case SortName:
		return nameColumn
	}
	return table.NoSort
}

// acrossRunOrder returns the indexes of the runs in the order of the sort.
func acrossRunOrder(runs []domain.RunSummary, o SortOrder) []int {
	return sortedOrder(len(runs), o,
		func(i int) string { return runs[i].Run.Label() },
		func(key SortKey, i int) (int64, bool) { return moneyNanos(runs[i].Totals.Money.Total) })
}

// acrossTypeOrder returns the indexes of the agent types in the order of the sort.
func acrossTypeOrder(types []domain.AgentTypeAcrossRuns, o SortOrder) []int {
	return sortedOrder(len(types), o,
		func(i int) string { return typeLabel(types[i].AgentType) },
		func(key SortKey, i int) (int64, bool) { return moneyNanos(types[i].Figures.Totals.Money.Total) })
}

// acrossRunCells builds the cells of one run row; whole is the known cost of all
// runs the bar is measured against.
func acrossRunCells(cur domain.CurrencyCode, r domain.RunSummary, whole int64, theme tuicommon.Theme) []table.Cell {
	var labels []string
	if r.Provisional {
		labels = append(labels, ProvisionalLabel)
	}
	labels = append(labels, incompleteness(r.UnpricedModels, r.Totals.AssumedTierCacheWrite, r.Totals.AssumedTiers, r.Subagents.MissingUsage, r.Subagents.Open)...)
	part, priced := moneyNanos(r.Totals.Money.Total)
	cells := make([]table.Cell, arCount)
	cells[arRun] = table.Cell{Text: r.Run.Label(), Role: tuicommon.RoleSubtitle}
	cells[arCost] = moneyTotalCell(cur, r.Totals.Money)
	cells[arShare] = shareCell(part, whole, priced, tuicommon.RoleDataCost, theme)
	cells[arSubagents] = table.Cell{Text: formatGrouped(int64(r.Subagents.Invocations))}
	cells[arFlags] = warningCell(badgeLabels(labels))
	return cells
}

// acrossTypeCells builds the cells of one agent type row; whole is the known cost
// of all types the bar is measured against.
func acrossTypeCells(cur domain.CurrencyCode, g domain.AgentTypeAcrossRuns, whole int64, theme tuicommon.Theme) []table.Cell {
	f := g.Figures
	labels := incompleteness(f.UnpricedModels, f.Totals.AssumedTierCacheWrite, f.Totals.AssumedTiers, f.MissingUsage, f.Open)
	if g.ProvisionalRuns > 0 {
		labels = append(labels, fmt.Sprintf("%d of %d runs %s", g.ProvisionalRuns, g.Runs, ProvisionalLabel))
	}
	avg := FormatMoney(cur, g.PerRunMoney.Average)
	if g.PerRunMoney.Basis > 0 && !g.PerRunMoney.Complete && f.Totals.Money.Complete {
		avg += " (partial)"
	}
	part, priced := moneyNanos(f.Totals.Money.Total)
	cells := make([]table.Cell, atCount)
	cells[atType] = table.Cell{Text: typeLabel(g.AgentType), Role: tuicommon.RoleDataActor}
	cells[atInvocations] = table.Cell{Text: formatGrouped(int64(f.Invocations))}
	cells[atRuns] = table.Cell{Text: formatGrouped(int64(g.Runs))}
	cells[atTotal] = moneyTotalCell(cur, f.Totals.Money)
	cells[atShare] = shareCell(part, whole, priced, tuicommon.RoleDataCost, theme)
	cells[atAverage] = table.Cell{Text: avg, Role: MoneyRole(g.PerRunMoney.Average, g.PerRunMoney.Complete)}
	cells[atPerRunInvocations] = table.Cell{Text: formatPerRun(g.PerRunInvocations, 1)}
	cells[atPerRunTokens] = table.Cell{Text: formatPerRun(g.PerRunTokens, 0)}
	cells[atFlags] = warningCell(badgeLabels(labels))
	return cells
}

// acrossRunLines draws the runs as a table in the order of the sort.
func acrossRunLines(runs []domain.RunSummary, o SortOrder, width int, st Styles) []string {
	var whole int64
	for _, r := range runs {
		n, _ := moneyNanos(r.Totals.Money.Total)
		whole += n
	}
	var rows [][]table.Cell
	for _, i := range acrossRunOrder(runs, o) {
		rows = append(rows, acrossRunCells(st.Currency, runs[i], whole, st.Theme))
	}
	t := table.Table{
		Columns: acrossRunColumns(), Rows: rows, Cursor: table.NoCursor,
		SortColumn: acrossSortColumn(o.Key, arRun, arCost), SortDesc: o.Desc,
	}
	return renderTableLines(t, width, len(rows), st)
}

// acrossTypeLines draws the agent types as a table in the order of the sort.
func acrossTypeLines(types []domain.AgentTypeAcrossRuns, o SortOrder, width int, st Styles) []string {
	var whole int64
	for _, g := range types {
		n, _ := moneyNanos(g.Figures.Totals.Money.Total)
		whole += n
	}
	var rows [][]table.Cell
	for _, i := range acrossTypeOrder(types, o) {
		rows = append(rows, acrossTypeCells(st.Currency, types[i], whole, st.Theme))
	}
	t := table.Table{
		Columns: acrossTypeColumns(), Rows: rows, Cursor: table.NoCursor,
		SortColumn: acrossSortColumn(o.Key, atType, atTotal), SortDesc: o.Desc,
	}
	return renderTableLines(t, width, len(rows), st)
}

// formatPerRun renders a per-run ratio as a decimal number; AbsentMarker when undefined.
func formatPerRun(r domain.Ratio, decimals int) string {
	if r.Denominator <= 0 {
		return AbsentMarker
	}
	return fmt.Sprintf("%.*f", decimals, float64(r.Numerator)/float64(r.Denominator))
}
