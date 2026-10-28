package screens

import (
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The run total, the cost share and the token split of the Overview. The visual
// version draws stacked bars whose legends carry the figures; the figures-only
// version lists the same figures in tables.

// maxBarWidth caps the width of a stacked bar so it stays a bar on wide terminals.
const maxBarWidth = 60

// heading is a section title line.
func heading(st Styles, width int, title string) string { return clip(st.Subtitle, width, title) }

// figureTable draws a table without a cursor or a sort marker.
func figureTable(cols []table.Column, rows [][]table.Cell, width int, st Styles) []string {
	t := table.Table{Columns: cols, Rows: rows, Cursor: table.NoCursor, SortColumn: table.NoSort}
	return renderTableLines(t, width, len(rows), st)
}

// hasShare reports whether any segment has a positive value, so a bar can be drawn.
func hasShare(segs []chart.Segment) bool {
	for _, s := range segs {
		if s.Value > 0 {
			return true
		}
	}
	return false
}

// stackedLines draws a stacked bar with its legend underneath, indented; a split
// with nothing to share shows the absent marker instead.
func stackedLines(segs []chart.Segment, width int, st Styles) []string {
	if !hasShare(segs) {
		return []string{clip(st.Muted, width, "  "+AbsentMarker)}
	}
	inner := max(width-2, 1)
	out := []string{"  " + chart.StackedBar(segs, min(inner, maxBarWidth), st.Theme)}
	for _, l := range strings.Split(chart.Legend(segs, inner, 8, st.Theme), "\n") {
		out = append(out, "  "+l)
	}
	return out
}

// moneyValue is the amount of a money value as a bar share; unpriced and absent
// money has none.
func moneyValue(v domain.MoneyValue) float64 {
	n, _ := moneyNanos(v)
	return float64(n)
}

// summaryLines are the run total and the labels of incomplete data. The
// figures-only version adds the orchestrator, subagent and unattributed rows.
func (s *OverviewScreen) summaryLines() []string {
	ov := s.run.Overview
	st, width := s.styles, s.width
	var out []string
	if s.figuresOnly {
		out = figureTable(summaryColumns(), s.summaryRows(), width, st)
	} else {
		out = []string{clip(st.Body, width, "Total  "+formatMoneyTotal(ov.Totals.Money))}
	}
	for _, l := range incompleteness(s.run.UnpricedModels, ov.Totals.AssumedTierCacheWrite,
		ov.Subagents.MissingUsage, ov.Subagents.Open) {
		out = append(out, clip(st.Warning, width, "  "+l))
	}
	return out
}

func summaryColumns() []table.Column {
	return []table.Column{
		{Title: "figure", Flex: true, MinWidth: 12},
		{Title: "cost", Align: table.AlignRight},
		{Title: "share", Align: table.AlignRight},
		{Title: "note"},
	}
}

func (s *OverviewScreen) summaryRows() [][]table.Cell {
	ov := s.run.Overview
	actor := func(name string) table.Cell { return table.Cell{Text: name, Role: tuicommon.RoleDataActor} }
	rows := [][]table.Cell{
		{actor("Total"), moneyTotalCell(ov.Totals.Money)},
		{actor("Orchestrator"), moneyTotalCell(ov.Orchestrator.Money), ratioCell(ov.OrchestratorShare)},
		{actor("Subagents"), moneyTotalCell(ov.Subagents.Totals.Money), ratioCell(ov.SubagentShare),
			{Text: "(" + countLabel(ov.Subagents.Invocations, "invocation") + ")"}},
	}
	if ov.Unattributed != nil {
		rows = append(rows, []table.Cell{actor("Unattributed"), moneyTotalCell(ov.Unattributed.Money),
			ratioCell(ov.UnattributedShare)})
	}
	return rows
}

// costShareLines is the stacked bar of the orchestrator, the subagents and the
// unattributed usage by cost.
func (s *OverviewScreen) costShareLines() []string {
	ov := s.run.Overview
	segs := []chart.Segment{
		{Label: "orchestrator " + FormatMoney(ov.Orchestrator.Money.Total),
			Value: moneyValue(ov.Orchestrator.Money.Total), Role: tuicommon.RoleDataLead},
		{Label: "subagents " + FormatMoney(ov.Subagents.Totals.Money.Total),
			Value: moneyValue(ov.Subagents.Totals.Money.Total), Role: tuicommon.RoleDataActor},
	}
	if u := ov.Unattributed; u != nil {
		segs = append(segs, chart.Segment{Label: "unattributed " + FormatMoney(u.Money.Total),
			Value: moneyValue(u.Money.Total), Role: tuicommon.RoleSignalWarning})
	}
	return append([]string{heading(s.styles, s.width, "Cost share")}, stackedLines(segs, s.width, s.styles)...)
}

// tokenLines is the split of the billable tokens: a stacked bar in the visual
// version, a table in the figures-only one. Reasoning is part of output and is
// shown beside it, never as a share of its own.
func (s *OverviewScreen) tokenLines() []string {
	ov := s.run.Overview
	st, width := s.styles, s.width
	out := []string{heading(st, width, "Token categories")}
	if s.figuresOnly {
		return append(out, figureTable(tokenColumns(), s.tokenRows(), width, st)...)
	}
	var segs []chart.Segment
	for _, cat := range domain.BillableCategories() {
		count, _ := ov.Totals.Tokens.Get(cat)
		segs = append(segs, chart.Segment{
			Label: cat.Label() + " " + FormatTokens(count) + " " + FormatMoney(categoryMoney(ov.Totals.Money, cat)),
			Value: float64(count.ValueOr(0)), Role: tuicommon.RoleDataTokens})
	}
	out = append(out, stackedLines(segs, width, st)...)
	return append(out, clip(st.Body, width, "  "+domain.CategoryReasoning.Label()+" "+
		FormatTokens(ov.Totals.Tokens.Reasoning)+" (part of output)"))
}

func tokenColumns() []table.Column {
	right := table.AlignRight
	return []table.Column{
		{Title: "category", Flex: true, MinWidth: 12},
		{Title: "tokens", Align: right},
		{Title: "cost", Align: right},
		{Title: "share", Align: right},
	}
}

func (s *OverviewScreen) tokenRows() [][]table.Cell {
	ov := s.run.Overview
	total := tokenSum(ov.Totals.Tokens)
	var rows [][]table.Cell
	for _, cat := range domain.BillableCategories() {
		count, _ := ov.Totals.Tokens.Get(cat)
		money := categoryMoney(ov.Totals.Money, cat)
		share := mutedCell(AbsentMarker)
		if count.IsPresent() {
			share = ratioCell(domain.Ratio{Numerator: count.ValueOr(0), Denominator: total.ValueOr(0)})
		}
		rows = append(rows, []table.Cell{{Text: cat.Label()}, tokensCell(count),
			{Text: FormatMoney(money), Role: MoneyRole(money, true)}, share})
		if cat == domain.CategoryOutput {
			rows = append(rows, []table.Cell{{Text: "  " + domain.CategoryReasoning.Label() + " (part of output)"},
				tokensCell(ov.Totals.Tokens.Reasoning)})
		}
	}
	return rows
}

// categoryMoney selects the money of one billable token category.
func categoryMoney(c domain.CategoryMoney, cat domain.TokenCategory) domain.MoneyValue {
	switch cat {
	case domain.CategoryInput:
		return c.Input
	case domain.CategoryCacheRead:
		return c.CacheRead
	case domain.CategoryCacheCreation:
		return c.CacheCreation
	case domain.CategoryOutput:
		return c.Output
	}
	return domain.NoMoneyData()
}
