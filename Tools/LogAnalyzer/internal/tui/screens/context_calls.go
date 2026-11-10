package screens

import (
	"sort"
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The per-call table of the Session screen: the API calls of one actor with
// the marks of the first write call and of the calls around compactions.

// Columns of the per-call table, in display order.
const (
	kcTime = iota
	kcModel
	kcKind
	kcContext
	kcInput
	kcCacheRead
	kcCacheWrite
	kcOutput
	kcCost
	kcTools
	kcGrowth
	kcMark
	kcCount
)

// Marks a call can carry.
const (
	markFirstWriteText = "first write"
	markLoggedText     = "logged compaction"
	markDetectedText   = "detected compaction"
	markHarnessText    = "harness compaction (OTel)"
	markCompactCall    = "compaction call"
	markUnappliedCall  = "unapplied compaction call"
)

// tableOf assembles a kit table from its parts.
func tableOf(cols []table.Column, rows [][]table.Cell, cursor, offset, sortCol int, desc bool) table.Table {
	return table.Table{Columns: cols, Rows: rows, Cursor: cursor, Offset: offset, SortColumn: sortCol, SortDesc: desc}
}

// callColumns lists the columns with the priority at which each is dropped on a
// narrow terminal: the time and the context stay.
func callColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, kcCount)
	cols[kcTime] = table.Column{Title: "time"}
	cols[kcModel] = table.Column{Title: "model", Priority: 1, Flex: true, MinWidth: 8}
	cols[kcKind] = table.Column{Title: "kind", Priority: 1, MinWidth: 8}
	cols[kcContext] = table.Column{Title: "context", Align: right}
	cols[kcInput] = table.Column{Title: "input", Align: right, Priority: 4}
	cols[kcCacheRead] = table.Column{Title: "cache read", Align: right, Priority: 5}
	cols[kcCacheWrite] = table.Column{Title: "cache write", Align: right, Priority: 5}
	cols[kcOutput] = table.Column{Title: "output", Align: right, Priority: 4}
	cols[kcCost] = table.Column{Title: "cost", Align: right, Priority: 3}
	cols[kcTools] = table.Column{Title: "requested tools", Priority: 2, Flex: true, MinWidth: 12}
	cols[kcGrowth] = table.Column{Title: "context growth", Align: right, Priority: 3, MinWidth: 12}
	cols[kcMark] = table.Column{Title: "mark", Priority: 2, Flex: true, MinWidth: 8}
	return cols
}

// newCallSortCycle offers the sorts of the per-call table: call order, then the
// largest cost and the largest context first.
func newCallSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortCallOrder, false}, SortOrder{SortCost, true}, SortOrder{SortContext, true})
}

// callSortColumn is the column that shows the sort key.
func callSortColumn(key SortKey) int {
	switch key {
	case SortCost:
		return kcCost
	case SortContext:
		return kcContext
	case SortCallOrder:
		return kcTime
	}
	return table.NoSort
}

// callOrder returns the indexes of the calls in the order of the sort. Calls
// without the sorted figure come last and ties keep call order.
func callOrder(calls []domain.ContextCall, o SortOrder) []int {
	idx := make([]int, len(calls))
	for i := range idx {
		idx[i] = i
	}
	if o.Key == SortCallOrder {
		return idx
	}
	figure := func(i int) (int64, bool) {
		if o.Key == SortCost {
			return moneyNanos(calls[i].Cost)
		}
		return calls[i].Context.Value()
	}
	sort.SliceStable(idx, func(a, b int) bool {
		va, okA := figure(idx[a])
		vb, okB := figure(idx[b])
		if okA != okB {
			return okA
		}
		return okA && va != vb && (va > vb) == o.Desc
	})
	return idx
}

// callMarks lists the marks of each call: the first write call and the calls on
// either side of each compaction, logged and detected apart.
func callMarks(a domain.ActorContext) [][]string {
	marks := make([][]string, len(a.Calls))
	add := func(i int, text string) {
		if i < 0 || i >= len(marks) {
			return
		}
		for _, have := range marks[i] {
			if have == text {
				return
			}
		}
		marks[i] = append(marks[i], text)
	}
	for i, c := range a.Calls {
		if c.FirstWrite {
			add(i, markFirstWriteText)
		}
	}
	for i, c := range a.Calls {
		switch {
		case c.Kind != domain.CallCompaction:
		case c.Unapplied:
			add(i, markUnappliedCall)
		default:
			add(i, markCompactCall)
		}
	}
	for _, r := range a.Rewrites {
		add(r.Call, "rewrite: "+r.Label())
	}
	for _, m := range a.Compactions {
		text := compactionMarkText(m)
		if text == "" {
			continue
		}
		add(m.Before, text)
		add(m.After, text)
	}
	return marks
}

// compactionMarkText names the kind of a compaction on the calls around it. A
// drop matched to a logged compaction has no mark of its own: the logged mark
// stands for it.
func compactionMarkText(m domain.CompactionMark) string {
	switch {
	case m.Kind == domain.CompactionLogged:
		return markLoggedText
	case m.MatchedLogged:
		return ""
	case m.Harness:
		return markHarnessText
	}
	return markDetectedText
}

// callCells builds the cells of one call row.
func callCells(cur domain.CurrencyCode, c domain.ContextCall, marks []string) []table.Cell {
	cells := make([]table.Cell, kcCount)
	cells[kcTime] = table.Cell{Text: c.At.Format("15:04:05"), Role: tuicommon.RoleDataTime}
	cells[kcModel] = table.Cell{Text: string(c.Model)}
	if c.Model == "" {
		cells[kcModel] = mutedCell(AbsentMarker)
	}
	cells[kcKind] = table.Cell{Text: callKindText(c)}
	ctx := domain.ContextFigure{Tokens: c.Context, Partial: c.Partial}
	if v, ok := c.Context.Value(); ok {
		ctx.Band = domain.BandOf(v)
	}
	cells[kcContext] = contextCell(ctx)
	cells[kcInput] = tokensCell(c.Usage.Input)
	cells[kcCacheRead] = tokensCell(c.Usage.CacheRead)
	cells[kcCacheWrite] = tokensCell(c.Usage.CacheCreation)
	cells[kcOutput] = tokensCell(c.Usage.Output)
	cost := FormatMoney(cur, c.Cost)
	cells[kcCost] = table.Cell{Text: cost, Role: MoneyRole(c.Cost, true)}
	cells[kcTools], cells[kcGrowth] = toolsCell(c.RequestedTools), growthCell(c)
	cells[kcMark] = warningCell(strings.Join(marks, ", "))
	return cells
}

// callTableLines draws the per-call table in the order of the sort; the window
// follows the cursor within rowBudget rows and the offset carries to the next render.
func callTableLines(a domain.ActorContext, o SortOrder, cursor int, offset *int, width, rowBudget int, st Styles) ([]string, Window) {
	marks := callMarks(a)
	order := callOrder(a.Calls, o)
	rows := make([][]table.Cell, len(order))
	for r, i := range order {
		rows[r] = callCells(st.Currency, a.Calls[i], marks[i])
	}
	t := tableOf(callColumns(), rows, cursor, 0, callSortColumn(o.Key), o.Desc)
	w := ScrollWindow(*offset, cursor, len(rows), rowBudget-extraHeaderLines(t, width))
	*offset = w.Offset
	t.Offset = w.Offset
	return renderTableLines(t, width, w.Count, st), w
}

// callKindText names the kind of a helper call, with its label when it has one;
// a main call shows nothing.
func callKindText(c domain.ContextCall) string {
	if isMainCall(c) {
		return ""
	}
	if c.Label == "" {
		return string(c.Kind)
	}
	return string(c.Kind) + " (" + c.Label + ")"
}

// rewriteCountCell is the number of context rewrites of one scope.
func rewriteCountCell(n int) table.Cell {
	return table.Cell{Text: formatGrouped(int64(n))}
}
