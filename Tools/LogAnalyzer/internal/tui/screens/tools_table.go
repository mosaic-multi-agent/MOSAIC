package screens

import (
	"fmt"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
)

// The Tools tables: one row per tool, with a bar for the tool's share of the calls
// or, under the duration sort, of the total paired duration. The run, the
// orchestrator and every agent type each get a table of their own.

// Columns of the Tools table, in display order.
const (
	tlName = iota
	tlCalls
	tlErrors
	tlAverage
	tlMin
	tlMax
	tlUnpaired
	tlShare
	tlCount
)

// toolsColumns lists the columns with the priority at which each is dropped on a
// narrow terminal: the tool, its calls and its errors stay.
func toolsColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, tlCount)
	cols[tlName] = table.Column{Title: "tool", Flex: true, MinWidth: 10}
	cols[tlCalls] = table.Column{Title: "calls", Align: right}
	cols[tlErrors] = table.Column{Title: "errors", Align: right}
	cols[tlAverage] = table.Column{Title: "avg", Align: right, Priority: 2}
	cols[tlMin] = table.Column{Title: "min", Align: right, Priority: 3}
	cols[tlMax] = table.Column{Title: "max", Align: right, Priority: 3}
	cols[tlUnpaired] = table.Column{Title: "unpaired", Priority: 1}
	cols[tlShare] = shareBarColumn(4)
	return cols
}

// newToolsSortCycle offers the sorts of the tools tables, calls first.
func newToolsSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortCalls, true}, SortOrder{SortDuration, true},
		SortOrder{SortErrors, true}, SortOrder{SortName, false})
}

// toolsSortColumn is the column that shows the sort key.
func toolsSortColumn(key SortKey) int {
	switch key {
	case SortCalls:
		return tlCalls
	case SortDuration:
		return tlAverage
	case SortErrors:
		return tlErrors
	case SortName:
		return tlName
	}
	return table.NoSort
}

// toolName is the label of a tool, or "(start missing)" for calls that have
// neither a start nor a logged name.
func toolName(t domain.ToolFigures) string {
	if t.Tool == "" {
		return StartMissingLabel
	}
	return t.Tool
}

// toolOrder returns the indexes of the tools in the order of the sort. A tool
// without a paired call has no duration and comes last under the duration sort.
func toolOrder(tools []domain.ToolFigures, o SortOrder) []int {
	return sortedOrder(len(tools), o,
		func(i int) string { return toolName(tools[i]) },
		func(key SortKey, i int) (int64, bool) {
			t := tools[i]
			switch key {
			case SortCalls:
				return int64(t.Calls), true
			case SortDuration:
				return int64(t.Durations.Total), t.Durations.Count > 0
			case SortErrors:
				return int64(t.Errors), true
			}
			return 0, false
		})
}

// toolShare is the share a tool's bar shows: its part of the whole under the sort.
// The bar follows the total duration under the duration sort and the calls otherwise.
func toolShare(t domain.ToolFigures, tools []domain.ToolFigures, o SortOrder) (part, whole int64) {
	if o.Key == SortDuration {
		for _, x := range tools {
			whole += int64(x.Durations.Total)
		}
		return int64(t.Durations.Total), whole
	}
	for _, x := range tools {
		whole += int64(x.Calls)
	}
	return int64(t.Calls), whole
}

// toolCells builds the cells of one tool row.
func toolCells(t domain.ToolFigures, tools []domain.ToolFigures, o SortOrder, nameWidth int, theme tuicommon.Theme) []table.Cell {
	d := t.Durations
	ok := d.Count > 0
	cells := make([]table.Cell, tlCount)
	cells[tlName] = table.Cell{Text: textwidth.Truncate(toolName(t), nameWidth), Role: tuicommon.RoleDataTool}
	cells[tlCalls] = table.Cell{Text: formatGrouped(int64(t.Calls))}
	cells[tlErrors] = table.Cell{Text: formatGrouped(int64(t.Errors)), Role: ToolErrorsRole(t.Errors)}
	cells[tlAverage] = table.Cell{Text: FormatDuration(d.Average, ok), Role: tuicommon.RoleDataTime}
	cells[tlMin] = table.Cell{Text: FormatDuration(d.Min, ok), Role: tuicommon.RoleDataTime}
	cells[tlMax] = table.Cell{Text: FormatDuration(d.Max, ok), Role: tuicommon.RoleDataTime}
	if t.Unpaired > 0 {
		cells[tlUnpaired] = warningCell(fmt.Sprintf("%d %s", t.Unpaired, UnpairedLabel))
	}
	part, whole := toolShare(t, tools, o)
	role := tuicommon.RoleDataTool
	if o.Key == SortDuration {
		role = tuicommon.RoleDataTime
	}
	cells[tlShare] = barCell(part, whole, role, theme)
	return cells
}

// toolsTableLines draws the tools as a table in the order of the sort; cursor is
// the selected row, or table.NoCursor for a table without a selection.
func toolsTableLines(tools []domain.ToolFigures, o SortOrder, cursor, width int, st Styles) []string {
	var rows [][]table.Cell
	for _, i := range toolOrder(tools, o) {
		rows = append(rows, toolCells(tools[i], tools, o, toolNameWidth(width), st.Theme))
	}
	t := table.Table{
		Columns: toolsColumns(), Rows: rows, Cursor: cursor, SortColumn: toolsSortColumn(o.Key), SortDesc: o.Desc,
	}
	return renderTableLines(t, width, len(rows), st)
}

// toolNameWidth is the widest a tool name is drawn at the terminal width. Long
// MCP names are cut to it with an ellipsis so the figure columns are kept.
func toolNameWidth(width int) int { return max(width/4, 12) }
