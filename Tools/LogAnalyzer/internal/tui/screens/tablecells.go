package screens

import (
	"strconv"
	"strings"
	"time"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/table"
	"mosaic-common/tui/kit/textwidth"

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
func moneyTotalCell(cur domain.CurrencyCode, c domain.CategoryMoney) table.Cell {
	return table.Cell{Text: formatMoneyTotal(cur, c), Role: MoneyRole(c.Total, c.Complete)}
}

// moneyCell is a money figure; a known figure of an incomplete scope is marked partial.
func moneyCell(cur domain.CurrencyCode, v domain.MoneyValue, complete bool) table.Cell {
	text := FormatMoney(cur, v)
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

// sharePercentWidth is the room of the percent text beside a share bar: "100%".
const sharePercentWidth = 4

// shareColumnWidth is the fixed width of a share column: the bar, a space and the percent.
const shareColumnWidth = shareBarWidth + 1 + sharePercentWidth

// sharePercentText is the whole percent part is of whole. A non-zero share under 1%
// reads "<1%", and a figure that is not known reads the absent marker.
func sharePercentText(part, whole int64, known bool) string {
	if !known {
		return AbsentMarker
	}
	if part <= 0 || whole <= 0 {
		return "0%"
	}
	if part*100 < whole {
		return "<1%"
	}
	return strconv.FormatInt(min((part*100+whole/2)/whole, 100), 10) + "%"
}

// shareCell is a bar of part's share of whole followed by its whole percent. A
// figure that is not known draws an empty bar and the absent marker.
func shareCell(part, whole int64, known bool, role tuicommon.StyleRole, theme tuicommon.Theme) table.Cell {
	text := sharePercentText(part, whole, known)
	return table.Cell{Fill: func(width int) string {
		barWidth := max(width-1-sharePercentWidth, 0)
		pct := textwidth.Pad(text, sharePercentWidth, true)
		if !known {
			pct = theme.Style(tuicommon.RoleSignalMuted).Render(pct)
		}
		if barWidth == 0 {
			return textwidth.Pad(pct, width, true)
		}
		return chart.Bar(float64(part), float64(whole), barWidth, role, theme) + " " + pct
	}}
}

// shareBarCell is a bar of cost's share of total with its percent; an unpriced
// or absent cost draws an empty bar and the absent marker.
func shareBarCell(cost, total domain.MoneyValue, theme tuicommon.Theme) table.Cell {
	part, ok := moneyNanos(cost)
	whole, _ := moneyNanos(total)
	return shareCell(part, whole, ok, tuicommon.RoleDataCost, theme)
}

// shareBarColumn is the column of a share bar.
func shareBarColumn(priority int) table.Column {
	return table.Column{Title: "share", Priority: priority, Width: shareColumnWidth}
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

// stackedTableLines draws tables stacked in one view with one shared column layout,
// so they line up and hide the same columns. Each section's heading line comes
// first. A section whose table has no columns is just its heading.
func stackedTableLines(sections []tableSection, width int, st Styles) []string {
	var tables []table.Table
	for i := range sections {
		sections[i].table.WrapTitles = true
		if len(sections[i].table.Columns) > 0 {
			tables = append(tables, sections[i].table)
		}
	}
	layout := table.NewSharedLayout(tables, width)
	var out []string
	for _, s := range sections {
		out = append(out, clip(st.Subtitle, width, s.title))
		if len(s.table.Columns) == 0 {
			continue
		}
		height := layout.HeaderHeight() + len(s.table.Rows)
		out = append(out, strings.Split(layout.Render(s.table, height, st.Theme), "\n")...)
	}
	return out
}

// tableSection is a heading and the table under it in a stacked view.
type tableSection struct {
	title string
	table table.Table
}

// renderTableLines draws the table into body lines: the header, whatever height
// its wrapped titles need, and rows data rows.
func renderTableLines(t table.Table, width, rows int, st Styles) []string {
	t.WrapTitles = true
	return strings.Split(table.Render(t, width, table.HeaderHeight(t, width)+max(rows, 0), st.Theme), "\n")
}

// extraHeaderLines is how many header lines the table needs beyond the one a plain
// title takes, so a screen can take them off its row budget.
func extraHeaderLines(t table.Table, width int) int {
	t.WrapTitles = true
	return table.HeaderHeight(t, width) - 1
}

// identityMinWidth is the narrowest an instance column is drawn: wide enough for
// "orchestrator" or the start of a long type name beside its #N.
const identityMinWidth = 16
