package screens

import (
	"fmt"
	"sort"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The comparison table: one row per agent type and a run-totals row, each with
// the figures of run A and run B and the change between them.

// Columns of the comparison table, in display order.
const (
	cmType = iota
	cmNote
	cmInvA
	cmInvB
	cmInvChange
	cmTokensA
	cmTokensB
	cmTokensChange
	cmCostA
	cmCostB
	cmCostChange
	cmCostPercent
	cmPeakA
	cmPeakB
	cmPeakChange
	cmPeakPercent
	cmCount
)

// totalsLabel titles the run-totals row.
const totalsLabel = "Total"

// Notes of a row whose agent type is in one run only.
const (
	onlyInANote = "only in A"
	onlyInBNote = "only in B"
)

// compareColumns lists the columns with the priority at which each is dropped on
// a narrow terminal: the type and the cost figures stay. The runs' own figures of
// invocations and tokens go first, then the changes of the average peak context,
// invocations and tokens, so the changes outlast the figures they come from.
func compareColumns() []table.Column {
	right := table.AlignRight
	num := func(title string, priority int) table.Column {
		return table.Column{Title: title, Align: right, Priority: priority}
	}
	cols := make([]table.Column, cmCount)
	cols[cmType] = table.Column{Title: "type", Flex: true, MinWidth: 10}
	cols[cmNote] = table.Column{Title: "note", Priority: 1}
	cols[cmInvA], cols[cmInvB], cols[cmInvChange] = num("inv A", 9), num("inv B", 9), num("inv chg", 5)
	cols[cmTokensA], cols[cmTokensB], cols[cmTokensChange] = num("tok A", 8), num("tok B", 8), num("tok chg", 4)
	cols[cmCostA], cols[cmCostB], cols[cmCostChange], cols[cmCostPercent] = num("cost A", 0), num("cost B", 0), num("cost chg", 0), num("cost %", 2)
	cols[cmPeakA], cols[cmPeakB], cols[cmPeakChange], cols[cmPeakPercent] = num("peak A", 3), num("peak B", 3), num("peak chg", 6), num("peak %", 7)
	return cols
}

// newCompareSortCycle offers the sorts of the comparison, largest cost change first.
func newCompareSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortCostChange, true}, SortOrder{SortTokens, true},
		SortOrder{SortInvocations, true}, SortOrder{SortName, false})
}

// compareSortColumn is the column that shows the sort key.
func compareSortColumn(key SortKey) int {
	switch key {
	case SortCostChange:
		return cmCostChange
	case SortTokens:
		return cmTokensChange
	case SortInvocations:
		return cmInvChange
	case SortName:
		return cmType
	}
	return table.NoSort
}

// changeMagnitude is what a row is ranked by: the size of the change when both
// runs have the figure, otherwise (known false) the larger figure that exists.
func changeMagnitude(r domain.ComparisonRow, key SortKey) (value int64, known bool) {
	switch key {
	case SortCostChange:
		if d, ok := moneyNanos(r.Cost.Delta); ok {
			return absInt(d), true
		}
		a, _ := moneyNanos(r.Cost.A)
		b, _ := moneyNanos(r.Cost.B)
		return max(a, b), false
	case SortTokens:
		return quantityMagnitude(r.Tokens)
	case SortInvocations:
		return quantityMagnitude(r.Invocations)
	}
	return 0, false
}

func quantityMagnitude(q domain.ComparedQuantity) (int64, bool) {
	if q.HasA && q.HasB {
		return absInt(q.Delta), true
	}
	return max(q.A, q.B), false
}

func absInt(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// compareOrder returns the indexes of the rows in the order of the sort: the
// largest change first by default; rows without a known change follow, the larger
// figure first, and ties order by type name.
func compareOrder(rows []domain.ComparisonRow, o SortOrder) []int {
	name := func(i int) string { return typeLabel(rows[i].AgentType) }
	if o.Key == SortName {
		return sortedOrder(len(rows), o, name, nil)
	}
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ia, ib := idx[a], idx[b]
		va, okA := changeMagnitude(rows[ia], o.Key)
		vb, okB := changeMagnitude(rows[ib], o.Key)
		switch {
		case okA != okB:
			return okA
		case va != vb && okA:
			return (va > vb) == o.Desc
		case va != vb:
			return va > vb
		}
		return name(ia) < name(ib)
	})
	return idx
}

// compareCells builds the cells of one row: an agent type's, or the totals row
// when label is set.
func compareCells(r domain.ComparisonRow, label string) []table.Cell {
	cells := make([]table.Cell, cmCount)
	cells[cmType] = table.Cell{Text: typeLabel(r.AgentType), Role: tuicommon.RoleDataActor}
	if label != "" {
		cells[cmType] = table.Cell{Text: label, Role: tuicommon.RoleSubtitle}
	}
	switch r.Presence {
	case domain.OnlyInA:
		cells[cmNote] = mutedCell(onlyInANote)
	case domain.OnlyInB:
		cells[cmNote] = mutedCell(onlyInBNote)
	}
	setQuantityCells(cells, cmInvA, r.Invocations, tuicommon.StyleRole(""), false)
	setQuantityCells(cells, cmTokensA, r.Tokens, tuicommon.RoleDataTokens, false)
	setMoneyCells(cells, r.Cost)
	setQuantityCells(cells, cmPeakA, r.AvgPeakContext, tuicommon.RoleDataTokens, true)
	return cells
}

// setQuantityCells fills the A, B and change cells starting at first, and the
// percent cell after them when withPercent is set.
func setQuantityCells(cells []table.Cell, first int, q domain.ComparedQuantity, role tuicommon.StyleRole, withPercent bool) {
	cells[first] = quantityCell(q.A, q.HasA, role)
	cells[first+1] = quantityCell(q.B, q.HasB, role)
	cells[first+2] = changeCell(q.Delta, q.HasA && q.HasB, q.Direction, formatSignedCount)
	if withPercent {
		cells[first+3] = percentCell(q.Percent, q.HasPercent, q.Direction)
	}
}

// setMoneyCells fills the cost cells: both runs' costs, the change and its percent.
func setMoneyCells(cells []table.Cell, m domain.ComparedMoney) {
	cells[cmCostA] = moneyCell(m.A, m.Complete)
	cells[cmCostB] = moneyCell(m.B, m.Complete)
	cells[cmCostChange] = table.Cell{Text: formatSignedMoney(m.Delta), Role: ChangeRole(m.Direction)}
	cells[cmCostPercent] = percentCell(m.Percent, m.HasPercent, m.Direction)
}

// quantityCell is one run's figure; a figure the run lacks reads as absent, never 0.
func quantityCell(v int64, has bool, role tuicommon.StyleRole) table.Cell {
	if !has {
		return mutedCell(AbsentMarker)
	}
	return table.Cell{Text: formatGrouped(v), Role: role}
}

// changeCell is a change between the runs, signed and coloured by its direction;
// absent when the runs cannot be compared.
func changeCell(delta int64, has bool, d domain.ChangeDirection, format func(int64) string) table.Cell {
	if !has {
		return mutedCell(AbsentMarker)
	}
	return table.Cell{Text: format(delta), Role: ChangeRole(d)}
}

// percentCell is a change as a signed percentage, absent when the base is 0 or missing.
func percentCell(r domain.Ratio, has bool, d domain.ChangeDirection) table.Cell {
	if !has || r.Denominator <= 0 {
		return mutedCell(AbsentMarker)
	}
	text := "0%"
	if r.Numerator != 0 {
		text = fmt.Sprintf("%+.1f%%", float64(r.Numerator)/float64(r.Denominator)*100)
	}
	return table.Cell{Text: text, Role: ChangeRole(d)}
}

// formatSignedCount renders a change in a count with its sign; no change is "0".
func formatSignedCount(n int64) string {
	if n > 0 {
		return "+" + formatGrouped(n)
	}
	return formatGrouped(n)
}

// formatSignedMoney renders a change in money with its sign; no change is "$0.00"
// and a change that cannot be known is absent.
func formatSignedMoney(v domain.MoneyValue) string {
	text := FormatMoney(v)
	if n, ok := moneyNanos(v); ok && n > 0 {
		return "+" + text
	}
	return text
}

// compareTableLines draws the rows in the order of the sort, then the totals row.
// Only the rows in the window are drawn; the totals row always is.
func compareTableLines(cmp domain.RunComparison, order []int, w Window, o SortOrder, width int, st Styles) []string {
	var rows [][]table.Cell
	for _, i := range order[w.Offset:w.End()] {
		rows = append(rows, compareCells(cmp.Rows[i], ""))
	}
	rows = append(rows, compareCells(cmp.Totals, totalsLabel))
	t := table.Table{Columns: compareColumns(), Rows: rows, Cursor: table.NoCursor,
		SortColumn: compareSortColumn(o.Key), SortDesc: o.Desc}
	return renderTableLines(t, width, len(rows), st)
}
