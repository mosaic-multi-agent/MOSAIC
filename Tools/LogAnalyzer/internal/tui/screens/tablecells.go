package screens

import (
	"strings"
	"time"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/table"

	"mosaic-log-analyzer/internal/domain"
)

// Cell builders shared by the table views. They map domain figures to table
// cells and their colour roles; the kit table does the layout.

// shareBarWidth is the fixed width of a share-of-cost bar column.
const shareBarWidth = 10

// mutedCell is a cell of text with no figure behind it.
func mutedCell(text string) table.Cell {
	return table.Cell{Text: text, Role: tuicommon.RoleSignalMuted}
}

// moneyTotalCell is a cost total: a partial known total is marked, an unpriced
// or absent one never shows an amount.
func moneyTotalCell(c domain.CategoryMoney) table.Cell {
	return table.Cell{Text: formatMoneyTotal(c), Role: MoneyRole(c.Total, c.Complete)}
}

// moneyCell is a money figure; a known figure of an incomplete scope is marked partial.
func moneyCell(v domain.MoneyValue, complete bool) table.Cell {
	text := FormatMoney(v)
	if v.State == domain.MoneyKnown && !complete {
		text += " (partial)"
	}
	return table.Cell{Text: text, Role: MoneyRole(v, complete)}
}

// tokensCell is a token count, the absent marker when there is none.
func tokensCell(t domain.TokenCount) table.Cell {
	return table.Cell{Text: FormatTokens(t), Role: TokensRole(t)}
}

// ratioCell is a percentage, the absent marker when the ratio is undefined.
func ratioCell(r domain.Ratio) table.Cell {
	if r.Denominator <= 0 {
		return mutedCell(AbsentMarker)
	}
	return table.Cell{Text: FormatRatio(r)}
}

// contextCell is a context size coloured by its band.
func contextCell(f domain.ContextFigure) table.Cell {
	if !f.Tokens.IsPresent() {
		return mutedCell(AbsentMarker)
	}
	text := FormatTokens(f.Tokens)
	if f.Partial {
		text += " (partial)"
	}
	return table.Cell{Text: text, Role: BandRole(f.Band)}
}

// durationCell is a duration, the absent marker when there is none.
func durationCell(d time.Duration, ok bool) table.Cell {
	if !ok {
		return mutedCell(AbsentMarker)
	}
	return table.Cell{Text: FormatDuration(d, true), Role: tuicommon.RoleDataTime}
}

// warningCell is text that asks for a look, drawn in the warning role; empty text
// stays an empty cell.
func warningCell(text string) table.Cell {
	if text == "" {
		return table.Cell{}
	}
	return table.Cell{Text: text, Role: tuicommon.RoleSignalWarning}
}

// moneyNanos is the amount of a known money value; ok is false for an unpriced or
// absent one.
func moneyNanos(v domain.MoneyValue) (int64, bool) {
	if v.State != domain.MoneyKnown {
		return 0, false
	}
	return v.Amount.Nanos(), true
}

// shareBarCell is a bar of cost's share of total; an unpriced or absent cost
// draws an empty bar.
func shareBarCell(cost, total domain.MoneyValue, theme tuicommon.Theme) table.Cell {
	part, _ := moneyNanos(cost)
	whole, _ := moneyNanos(total)
	return table.Cell{Fill: func(width int) string {
		return chart.Bar(float64(part), float64(whole), width, tuicommon.RoleDataCost, theme)
	}}
}

// shareBarColumn is the column of a share bar.
func shareBarColumn(priority int) table.Column {
	return table.Column{Title: "share", Priority: priority, Width: shareBarWidth}
}

// tokenSum is the billable token total of a usage: input, cache read, cache write
// and output. Reasoning is part of output and never added.
func tokenSum(u domain.TokenUsage) domain.TokenCount {
	return domain.SumTokens(u.Input, u.CacheRead, u.CacheCreation, u.Output)
}

// badgeLabels renders each label as a bracketed badge, separated by spaces.
func badgeLabels(labels []string) string {
	badges := make([]string, len(labels))
	for i, l := range labels {
		badges[i] = "[" + l + "]"
	}
	return strings.Join(badges, " ")
}

// renderTableLines draws the table into body lines.
func renderTableLines(t table.Table, width, rows int, st Styles) []string {
	return strings.Split(table.Render(t, width, 1+max(rows, 0), st.Theme), "\n")
}
