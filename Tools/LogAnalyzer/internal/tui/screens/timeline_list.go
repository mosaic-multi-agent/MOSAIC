package screens

import (
	"time"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The list version of the timeline: a table of the invocations followed by a
// table of the compactions, one row per compaction.

// Columns of the invocation table, in display order.
const (
	ivInstance = iota
	ivStart
	ivEnd
	ivDuration
	ivStatus
	ivOverlap
	ivCount
)

// Columns of the compactions table, in display order.
const (
	cpTime = iota
	cpActor
	cpTrigger
	cpBefore
	cpAfter
	cpShape
	cpCount
)

// newTimelineSortCycle offers the sorts of the invocation table, start first.
func newTimelineSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortStart, false}, SortOrder{SortDuration, true}, SortOrder{SortName, false})
}

// listBody renders every line of the list version.
func (s *TimelineScreen) listBody() []string {
	st, width := s.styles, s.width
	tl := s.run.Timeline
	if len(tl.Entries) == 0 && len(tl.Compactions) == 0 {
		return []string{clip(st.Body, width, "No invocations or compactions in this run.")}
	}
	var out []string
	if len(tl.Entries) > 0 {
		out = append(out, clip(st.Subtitle, width, "Invocations"))
		out = append(out, s.invocationTableLines()...)
	}
	if len(tl.Compactions) > 0 {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, clip(st.Subtitle, width, "Compactions"))
		out = append(out, s.compactionTableLines()...)
	}
	return out
}

// timelineEntryOrder returns the indexes of the entries in the order of the sort.
func timelineEntryOrder(entries []domain.TimelineEntry, o SortOrder) []int {
	return sortedOrder(len(entries), o,
		func(i int) string { return entries[i].Invocation.Actor.Label() },
		func(key SortKey, i int) (int64, bool) {
			e := entries[i]
			switch key {
			case SortStart:
				return e.Invocation.StartedAt.UnixNano(), !e.Invocation.StartedAt.IsZero()
			case SortDuration:
				return int64(e.Duration), e.HasDuration
			}
			return 0, false
		})
}

func timelineColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, ivCount)
	cols[ivInstance] = table.Column{Title: "instance", Flex: true, Identity: true, MinWidth: 8}
	cols[ivStart] = table.Column{Title: "start", Priority: 2}
	cols[ivEnd] = table.Column{Title: "end", Priority: 2}
	cols[ivDuration] = table.Column{Title: "duration", Align: right}
	cols[ivStatus] = table.Column{Title: "status"}
	cols[ivOverlap] = table.Column{Title: "overlap", Align: right, Priority: 3}
	return cols
}

// timelineSortColumn is the column that shows the sort key.
func timelineSortColumn(key SortKey) int {
	switch key {
	case SortStart:
		return ivStart
	case SortDuration:
		return ivDuration
	case SortName:
		return ivInstance
	}
	return table.NoSort
}

// invocationCells builds the cells of one invocation row; an open invocation has
// no end and no duration, and the overlap count is the analysis's own.
func invocationCells(e domain.TimelineEntry) []table.Cell {
	inv := e.Invocation
	cells := make([]table.Cell, ivCount)
	cells[ivInstance] = table.Cell{Text: inv.Actor.Label(), Role: tuicommon.RoleDataActor}
	cells[ivStart] = clockCell(inv.StartedAt)
	cells[ivEnd] = clockCell(inv.EndedAt)
	if inv.State == domain.InvocationOpen {
		cells[ivEnd] = table.Cell{Text: OpenLabel, Role: StatusRole(inv)}
	}
	cells[ivDuration] = durationCell(e.Duration, e.HasDuration)
	cells[ivStatus] = table.Cell{Text: "[" + statusText(inv) + "]", Role: StatusRole(inv)}
	cells[ivOverlap] = table.Cell{Text: formatGrouped(int64(len(e.Overlaps)))}
	return cells
}

// clockCell is a logged time, the absent marker when it is unknown.
func clockCell(t time.Time) table.Cell {
	if t.IsZero() {
		return mutedCell(AbsentMarker)
	}
	return table.Cell{Text: formatClock(t), Role: tuicommon.RoleDataTime}
}

func (s *TimelineScreen) invocationTableLines() []string {
	entries := s.run.Timeline.Entries
	o := s.sort.Active()
	rows := make([][]table.Cell, 0, len(entries))
	for _, i := range timelineEntryOrder(entries, o) {
		rows = append(rows, invocationCells(entries[i]))
	}
	t := table.Table{Columns: timelineColumns(), Rows: rows, Cursor: table.NoCursor,
		SortColumn: timelineSortColumn(o.Key), SortDesc: o.Desc}
	return renderTableLines(t, s.width, len(rows), s.styles)
}

// shapeText names how a compaction's halves were logged.
func shapeText(sh domain.CompactionShape) string {
	switch sh {
	case domain.CompactionPaired:
		return "paired"
	case domain.CompactionPreOnly:
		return "pre only"
	case domain.CompactionPostOnly:
		return "post only"
	}
	return "single"
}

func (s *TimelineScreen) compactionTableLines() []string {
	right := table.AlignRight
	cols := make([]table.Column, cpCount)
	cols[cpTime] = table.Column{Title: "time"}
	cols[cpActor] = table.Column{Title: "actor", Flex: true, MinWidth: 8}
	cols[cpTrigger] = table.Column{Title: "trigger", Priority: 1}
	cols[cpBefore] = table.Column{Title: "before", Align: right, Priority: 2}
	cols[cpAfter] = table.Column{Title: "after", Align: right, Priority: 2}
	cols[cpShape] = table.Column{Title: "shape", Priority: 3}
	var rows [][]table.Cell
	for _, c := range s.run.Timeline.Compactions {
		cells := make([]table.Cell, cpCount)
		cells[cpTime] = clockCell(c.At)
		cells[cpActor] = table.Cell{Text: c.Actor.Label(), Role: tuicommon.RoleDataActor}
		cells[cpTrigger] = table.Cell{Text: c.Trigger}
		cells[cpBefore] = tokensCell(c.TokensBefore)
		cells[cpAfter] = tokensCell(c.TokensAfter)
		cells[cpShape] = table.Cell{Text: shapeText(c.Shape)}
		rows = append(rows, cells)
	}
	t := table.Table{Columns: cols, Rows: rows, Cursor: table.NoCursor, SortColumn: table.NoSort}
	return renderTableLines(t, s.width, len(rows), s.styles)
}
