package screens

import (
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// The Dispatches table: one row per dispatch, optionally led by the
// orchestrator's own row. The Dispatches view and the by-type drill-down share it.

// Columns of the Dispatches table, in display order.
const (
	dcInstance = iota
	dcType
	dcStatus
	dcCost
	dcShare
	dcTokens
	dcDuration
	dcPeak
	dcHit
	dcRewrites
	dcWrite
	dcWriteCost
	dcNet
	dcUnread
	dcEffort
	dcFlags
	dcCount
)

// dispatchColumns lists the columns with the priority at which each is dropped on
// a narrow terminal: the identity, status and cost stay.
func dispatchColumns() []table.Column {
	right := table.AlignRight
	cols := make([]table.Column, dcCount)
	cols[dcInstance] = table.Column{Title: "instance", Flex: true, MinWidth: 10}
	cols[dcType] = table.Column{Title: "type", Priority: 1, Flex: true, MinWidth: 8}
	cols[dcStatus] = table.Column{Title: "status"}
	cols[dcCost] = table.Column{Title: "cost", Align: right}
	cols[dcShare] = shareBarColumn(6)
	cols[dcTokens] = table.Column{Title: "tokens", Align: right, Priority: 3}
	cols[dcDuration] = table.Column{Title: "duration", Align: right, Priority: 3}
	cols[dcPeak] = table.Column{Title: "peak ctx", Align: right, Priority: 2}
	cols[dcHit] = table.Column{Title: "cache hit", Align: right, Priority: 3}
	cols[dcRewrites] = table.Column{Title: "rewrites", Align: right, Priority: 3}
	cols[dcWrite] = table.Column{Title: "cache write", Align: right, Priority: 4}
	cols[dcWriteCost] = table.Column{Title: "write cost", Align: right, Priority: 4}
	cols[dcNet] = table.Column{Title: "net cache", Align: right, Priority: 5}
	cols[dcUnread] = table.Column{Title: "unread cache writes", Align: right, Priority: 5}
	cols[dcEffort] = table.Column{Title: "effort", Priority: 3}
	cols[dcFlags] = table.Column{Title: "flags", Priority: 1}
	return cols
}

// newDispatchSortCycle offers the sorts of a dispatch table, cost first.
func newDispatchSortCycle() SortCycle {
	return NewSortCycle(SortOrder{SortCost, true}, SortOrder{SortTokens, true}, SortOrder{SortDuration, true},
		SortOrder{SortPeakContext, true}, SortOrder{SortName, false})
}

// dispatchSortColumn is the column that shows the sort key.
func dispatchSortColumn(key SortKey) int {
	switch key {
	case SortCost:
		return dcCost
	case SortTokens:
		return dcTokens
	case SortDuration:
		return dcDuration
	case SortPeakContext:
		return dcPeak
	case SortName:
		return dcInstance
	}
	return table.NoSort
}

// dispatchEntry is one dispatch with the agent type group it was listed under.
type dispatchEntry struct {
	d        domain.Dispatch
	typeName string
}

// typeLabel is the displayed name of an agent type.
func typeLabel(agentType string) string {
	if agentType == "" {
		return UnknownTypeLabel
	}
	return agentType
}

// dispatchEntries lists the dispatches of every agent type group.
func dispatchEntries(groups []domain.AgentTypeGroup) []dispatchEntry {
	var out []dispatchEntry
	for _, g := range groups {
		for _, d := range g.Dispatches {
			out = append(out, dispatchEntry{d: d, typeName: typeLabel(g.AgentType)})
		}
	}
	return out
}

// dispatchOrder returns the indexes of the entries in the order of the sort.
func dispatchOrder(entries []dispatchEntry, o SortOrder) []int {
	return sortedOrder(len(entries), o,
		func(i int) string { return entries[i].d.Invocation.Actor.Label() },
		func(key SortKey, i int) (int64, bool) {
			d := entries[i].d
			switch key {
			case SortCost:
				return moneyNanos(d.Totals.Money.Total)
			case SortTokens:
				return tokenSum(d.Totals.Tokens).Value()
			case SortDuration:
				dur, ok := d.Invocation.Duration()
				return int64(dur), ok
			case SortPeakContext:
				return d.PeakContext.Tokens.Value()
			}
			return 0, false
		})
}

// statusText is the outcome of an invocation: its status code, or the label of an
// open invocation or one that closed without a status.
func statusText(inv domain.Invocation) string {
	switch {
	case inv.State == domain.InvocationOpen:
		return OpenLabel
	case inv.StatusCode == "":
		return NoStatusLabel
	}
	return inv.StatusCode
}

// dispatchCells builds the cells of one dispatch row; runCost is the run total
// the share bar is measured against.
func dispatchCells(cur domain.CurrencyCode, e dispatchEntry, runCost domain.MoneyValue, theme tuicommon.Theme) []table.Cell {
	d := e.d
	inv := d.Invocation
	cells := make([]table.Cell, dcCount)
	cells[dcInstance] = table.Cell{Text: inv.Actor.Label(), Role: tuicommon.RoleDataActor}
	cells[dcType] = table.Cell{Text: e.typeName}
	cells[dcStatus] = table.Cell{Text: "[" + statusText(inv) + "]", Role: StatusRole(inv)}
	cells[dcCost] = moneyTotalCell(cur, d.Totals.Money)
	cells[dcShare] = shareBarCell(d.Totals.Money.Total, runCost, theme)
	cells[dcTokens] = tokensCell(tokenSum(d.Totals.Tokens))
	dur, ok := inv.Duration()
	cells[dcDuration] = durationCell(dur, ok)
	cells[dcPeak] = contextCell(d.PeakContext)
	setCacheCells(cur, cells, d.Cache)
	cells[dcEffort] = table.Cell{Text: effortSegment(d.Effort)}
	cells[dcFlags] = warningCell(strings.Join(dispatchFlags(d), "; "))
	return cells
}

// dispatchFlags are the incompleteness labels of a dispatch, in their fixed order.
func dispatchFlags(d domain.Dispatch) []string {
	var flags []string
	if d.MissingUsage {
		flags = append(flags, MissingUsageLabel)
	}
	if len(d.UnpricedModels) > 0 {
		flags = append(flags, UnpricedMarker+" models")
	}
	if a := FormatAssumedTier(d.Totals.AssumedTierCacheWrite, d.Totals.AssumedTiers); a != "" {
		flags = append(flags, a)
	}
	return flags
}

// orchestratorCells builds the orchestrator's row: its own cost, tokens, effort,
// peak context and cache figures; it has no type, status or duration.
func orchestratorCells(cur domain.CurrencyCode, run domain.RunReport, theme tuicommon.Theme) []table.Cell {
	o := run.Orchestrator
	cells := make([]table.Cell, dcCount)
	cells[dcInstance] = table.Cell{Text: o.Actor.Label(), Role: tuicommon.RoleDataLead}
	cells[dcCost] = moneyTotalCell(cur, o.Totals.Money)
	cells[dcShare] = shareBarCell(o.Totals.Money.Total, run.Totals.Money.Total, theme)
	cells[dcTokens] = tokensCell(tokenSum(o.Totals.Tokens))
	cells[dcDuration] = durationCell(0, false)
	cells[dcPeak] = contextCell(run.Context.Orchestrator.Peak)
	setCacheCells(cur, cells, run.OrchestratorCache)
	cells[dcEffort] = table.Cell{Text: effortSegment(o.Effort)}
	var flags []string
	if len(o.UnpricedModels) > 0 {
		flags = append(flags, UnpricedMarker+" models")
	}
	if a := FormatAssumedTier(o.Totals.AssumedTierCacheWrite, o.Totals.AssumedTiers); a != "" {
		flags = append(flags, a)
	}
	cells[dcFlags] = warningCell(strings.Join(flags, "; "))
	return cells
}

// setCacheCells fills the cache columns from the figures of one scope.
func setCacheCells(cur domain.CurrencyCode, cells []table.Cell, c domain.CacheFigures) {
	cells[dcHit] = ratioCell(c.HitRatio)
	cells[dcRewrites] = rewriteCountCell(c.Rewrites.Count())
	cells[dcWrite] = tokensCell(c.WriteTokens)
	cells[dcWriteCost] = moneyCell(cur, c.WriteCost, c.Complete)
	cells[dcNet] = netBenefitCell(cur, c)
	cells[dcUnread] = unreadCell(cur, c)
}

// netBenefitCell is the net cache benefit: the absent marker when it is not
// priced, and a leading minus in the bad role when the cache cost more than it saved.
func netBenefitCell(cur domain.CurrencyCode, c domain.CacheFigures) table.Cell {
	v := c.NetBenefit
	if v.State != domain.MoneyKnown {
		return mutedCell(AbsentMarker)
	}
	if v.Amount < 0 {
		return table.Cell{Text: "-" + FormatMoney(cur, domain.KnownMoney(-v.Amount)), Role: tuicommon.RoleSignalBad}
	}
	return moneyCell(cur, v, c.Complete)
}

// unreadCell is the tokens and cost of the cache writes of the last call that were
// never read back; each part shows the absent marker when it has no figure.
func unreadCell(cur domain.CurrencyCode, c domain.CacheFigures) table.Cell {
	cost := moneyCell(cur, c.UnreadWriteCost, c.Complete)
	return table.Cell{
		Text: FormatTokens(c.UnreadWriteTokens) + " " + cost.Text,
		Role: TokensRole(c.UnreadWriteTokens),
	}
}

// dispatchTableRows builds the rows in the order given; the orchestrator row, when
// asked for, leads and is not part of the order.
func dispatchTableRows(cur domain.CurrencyCode, run domain.RunReport, withOrchestrator bool, entries []dispatchEntry, order []int, theme tuicommon.Theme) [][]table.Cell {
	var rows [][]table.Cell
	if withOrchestrator {
		rows = append(rows, orchestratorCells(cur, run, theme))
	}
	for _, i := range order {
		rows = append(rows, dispatchCells(cur, entries[i], run.Totals.Money.Total, theme))
	}
	return rows
}

// dispatchTableLines draws the table of rows into lines. The window follows the
// cursor within rowBudget rows and the offset carries to the next render.
func dispatchTableLines(rows [][]table.Cell, cursor int, offset *int, o SortOrder, width, rowBudget int, st Styles) ([]string, Window) {
	w := ScrollWindow(*offset, cursor, len(rows), rowBudget)
	*offset = w.Offset
	t := table.Table{
		Columns: dispatchColumns(), Rows: rows, Cursor: cursor, Offset: w.Offset,
		SortColumn: dispatchSortColumn(o.Key), SortDesc: o.Desc,
	}
	return renderTableLines(t, width, w.Count, st), w
}
