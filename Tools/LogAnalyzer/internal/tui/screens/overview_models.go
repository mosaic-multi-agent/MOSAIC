package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The by-model table of the Overview. The visual version ends it with a cost
// bar; the figures-only version leaves the bar out.

// Columns of the by-model table, in display order.
const (
	mcModel = iota
	mcRequests
	mcCost
	mcTokens
	mcShare
	mcCount
)

func modelColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, mcCount)
	cols[mcModel] = table.Column{Title: "model", Flex: true, MinWidth: 10}
	cols[mcRequests] = table.Column{Title: "requests", Align: right}
	cols[mcCost] = table.Column{Title: "cost", Align: right}
	cols[mcTokens] = table.Column{Title: "tokens", Align: right, Priority: 1}
	cols[mcShare] = shareBarColumn(2)
	return cols
}

// newModelSortCycle offers the sorts of the by-model table, cost first.
func newModelSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortCost, true}, SortOrder{SortTokens, true}, SortOrder{SortName, false})
}

// modelSortColumn is the column that shows the sort key.
func modelSortColumn(key SortKey) int {
	switch key {
	case SortCost:
		return mcCost
	case SortTokens:
		return mcTokens
	case SortName:
		return mcModel
	}
	return table.NoSort
}

// modelName is the displayed name of a model row.
func modelName(m domain.ModelFigures) string {
	if m.Model == "" {
		return UnknownModelLabel
	}
	return domain.PricingKey{Provider: m.Provider, Model: m.Model}.String()
}

// modelOrder returns the indexes of the models in the order of the sort.
func modelOrder(models []domain.ModelFigures, o SortOrder) []int {
	return sortedOrder(len(models), o,
		func(i int) string { return modelName(models[i]) },
		func(key SortKey, i int) (int64, bool) {
			switch key {
			case SortCost:
				return moneyNanos(models[i].Totals.Money.Total)
			case SortTokens:
				return tokenSum(models[i].Totals.Tokens).Value()
			}
			return 0, false
		})
}

// modelCells builds the cells of one model row; runCost is the run total the
// cost bar is measured against.
func modelCells(m domain.ModelFigures, runCost domain.MoneyValue, theme tuicommon.Theme) []table.Cell {
	total := m.Totals.Money.Total
	cost := table.Cell{Text: FormatMoney(total), Role: MoneyRole(total, true)}
	if !m.Priced && total.State != domain.MoneyUnpriced {
		cost.Text += " " + UnpricedMarker
		cost.Role = tuicommon.RoleSignalWarning
	}
	cells := make([]table.Cell, mcCount)
	cells[mcModel] = table.Cell{Text: modelName(m), Role: tuicommon.RoleDataModel}
	cells[mcRequests] = table.Cell{Text: formatGrouped(int64(m.Requests))}
	cells[mcCost] = cost
	cells[mcTokens] = tokensCell(tokenSum(m.Totals.Tokens))
	cells[mcShare] = shareBarCell(total, runCost, theme)
	return cells
}

// modelLines is the by-model table in the order of the sort.
func (s *OverviewScreen) modelLines() []string {
	ov := s.run.Overview
	st := s.styles
	o := s.modelSort.Active()
	cols := modelColumns()
	var rows [][]table.Cell
	for _, i := range modelOrder(ov.ByModel, o) {
		rows = append(rows, modelCells(ov.ByModel[i], ov.Totals.Money.Total, st.Theme))
	}
	if s.figuresOnly {
		cols = cols[:mcShare]
		for i := range rows {
			rows[i] = rows[i][:mcShare]
		}
	}
	t := table.Table{Columns: cols, Rows: rows, Cursor: table.NoCursor, SortColumn: modelSortColumn(o.Key), SortDesc: o.Desc}
	return append([]string{heading(st, s.width, "By model")}, renderTableLines(t, s.width, len(rows), st)...)
}
