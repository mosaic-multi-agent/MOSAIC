package screens

import (
	"fmt"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The outcome split of the Overview: how the run's dispatches ended, drawn in
// the status roles.

// outcomeShare is one part of the outcome split.
type outcomeShare struct {
	name  string
	count int
	role  tuicommon.StyleRole
}

func outcomeShares(o domain.OutcomeSplit) []outcomeShare {
	return []outcomeShare{
		{"success", o.Success.Invocations, tuicommon.RoleSignalGood},
		{"other status", o.OtherStatus.Invocations, tuicommon.RoleSignalWarning},
		{"open or no status", o.OpenOrNone.Invocations, tuicommon.RoleSignalWarning},
	}
}

// outcomeLines is the outcome split: a stacked bar in the visual version, a table
// in the figures-only one.
func (s *OverviewScreen) outcomeLines() []string {
	st, width := s.styles, s.width
	shares := outcomeShares(s.run.Overview.Outcomes)
	out := []string{heading(st, width, "Outcome of dispatches")}
	if s.figuresOnly {
		return append(out, figureTable(outcomeColumns(), outcomeRows(shares), width, st)...)
	}
	segs := make([]chart.Segment, len(shares))
	for i, sh := range shares {
		segs[i] = chart.Segment{Label: fmt.Sprintf("%s %d", sh.name, sh.count), Value: float64(sh.count), Role: sh.role}
	}
	return append(out, stackedLines(segs, width, st)...)
}

func outcomeColumns() []table.Column {
	right := table.AlignRight
	return []table.Column{
		{Title: "outcome", Flex: true, MinWidth: 12},
		{Title: "dispatches", Align: right},
		{Title: "share", Align: right},
	}
}

func outcomeRows(shares []outcomeShare) [][]table.Cell {
	var total int64
	for _, sh := range shares {
		total += int64(sh.count)
	}
	rows := make([][]table.Cell, len(shares))
	for i, sh := range shares {
		rows[i] = []table.Cell{{Text: sh.name, Role: sh.role}, {Text: formatGrouped(int64(sh.count))},
			ratioCell(domain.Ratio{Numerator: int64(sh.count), Denominator: total})}
	}
	return rows
}
