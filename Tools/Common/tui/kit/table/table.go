// Package table renders a width- and height-bounded table of coloured cells.
package table

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

// Align is a column's horizontal alignment.
type Align uint8

const (
	AlignLeft Align = iota
	AlignRight
)

// Column describes one table column.
type Column struct {
	Title    string
	Align    Align
	Priority int
	Flex     bool
	MinWidth int
	Width    int
}

// Cell is one table cell.
type Cell struct {
	Text string
	Role tui.StyleRole
	Fill func(width int) string
}

const (
	NoCursor = -1
	NoSort   = -1
)

// Table is the data and view state Render draws.
type Table struct {
	Columns    []Column
	Rows       [][]Cell
	Cursor     int
	Offset     int
	SortColumn int
	SortDesc   bool
}

// BodyRows is the number of rows Render shows for height.
func BodyRows(height int) int { return max(height-1, 0) }

// Render returns at most height lines, none wider than width cells.
func Render(t Table, width, height int, theme tui.Theme) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	widths := naturalWidths(t)
	visible := chooseColumns(t.Columns, widths, width)
	shrinkFlex(t.Columns, widths, visible, width)

	lines := []string{cutLine(headerLine(t, widths, visible, theme), width)}
	body := BodyRows(height)
	start := min(max(t.Offset, 0), max(len(t.Rows)-1, 0))
	for r := start; r < len(t.Rows) && r < start+body; r++ {
		line := rowLine(t.Rows[r], r == t.Cursor, t.Columns, widths, visible, theme)
		lines = append(lines, cutLine(line, width))
	}
	return strings.Join(lines, "\n")
}

// cutLine hard-cuts a styled line to width cells, the last resort for very narrow widths.
func cutLine(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "")
}

func headerLine(t Table, widths []int, visible []int, theme tui.Theme) string {
	style := theme.Style(tui.RoleTitle)
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", gutterWidth))
	for n, i := range visible {
		if n > 0 {
			b.WriteString(strings.Repeat(" ", gapWidth))
		}
		text := textwidth.Pad(headerText(t, i), widths[i], t.Columns[i].Align == AlignRight)
		b.WriteString(style.Render(text))
	}
	return b.String()
}

func rowLine(row []Cell, cursor bool, cols []Column, widths []int, visible []int, theme tui.Theme) string {
	var b strings.Builder
	gutter := strings.Repeat(" ", gutterWidth)
	if cursor {
		gutter = theme.Style(tui.RoleSelected).Render("> ")
	}
	b.WriteString(gutter)
	for n, i := range visible {
		if n > 0 {
			b.WriteString(strings.Repeat(" ", gapWidth))
		}
		b.WriteString(renderCell(row, i, cols[i], widths[i], cursor, theme))
	}
	return b.String()
}

func renderCell(row []Cell, i int, col Column, width int, cursor bool, theme tui.Theme) string {
	var cell Cell
	if i < len(row) {
		cell = row[i]
	}
	if cell.Fill != nil {
		raw := cell.Fill(width)
		return raw + strings.Repeat(" ", max(width-lipgloss.Width(raw), 0))
	}
	role := cell.Role
	if cursor {
		role = tui.RoleSelected
	}
	return theme.Style(role).Render(textwidth.Pad(cell.Text, width, col.Align == AlignRight))
}
