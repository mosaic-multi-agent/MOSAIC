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
	// Identity marks a flex column that names the row (for example an instance id).
	// It shrinks only after every other visible flex column is at its MinWidth, and an
	// identity cell that does not fit is cut in the middle so its tail stays visible.
	Identity bool
	// Group is a label shown on an extra header line above this column and the adjacent
	// visible columns with the same Group. "" = no group.
	Group string
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
	// WrapTitles lets every column title wrap at its spaces onto more header lines.
	WrapTitles bool
}

// BodyRows is the number of rows Render shows for height.
func BodyRows(height int) int { return max(height-1, 0) }

// Render returns at most height lines, none wider than width cells.
func Render(t Table, width, height int, theme tui.Theme) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	visible, hidden, widths := layoutWidths(t, width)
	return draw(t, newShape([]Table{t}, visible, hidden, widths), width, height, theme)
}

// draw renders t in the given shape. When height is smaller than the header, the bottom
// header lines are kept and no rows are shown.
func draw(t Table, s shape, width, height int, theme tui.Theme) string {
	lines := headerLines(t, s, theme)
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	body := max(height-s.headerHeight(), 0)
	start := min(max(t.Offset, 0), max(len(t.Rows)-1, 0))
	for r := start; r < len(t.Rows) && r < start+body; r++ {
		lines = append(lines, rowLine(t.Rows[r], r == t.Cursor, t.Columns, s.widths, s.visible, theme))
	}
	for i, line := range lines {
		lines[i] = cutLine(line, width)
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
	text := cell.Text
	if col.Identity {
		text = cutIdentity(text, width)
	}
	return theme.Style(role).Render(textwidth.Pad(text, width, col.Align == AlignRight))
}

// cutIdentity shortens text to width cells by cutting its middle, keeping the tail from the
// last "#" (for example "#12"). Text without a "#" is cut at the end as usual.
func cutIdentity(text string, width int) string {
	k := strings.LastIndex(text, "#")
	if textwidth.Of(text) <= width || k < 0 {
		return text
	}
	ell := "..."
	if tui.CurrentRenderMode().Glyphs != tui.GlyphASCII {
		ell = "…"
	}
	tail := text[k:]
	room := width - textwidth.Of(ell) - textwidth.Of(tail)
	if room < 1 {
		return text
	}
	return ansi.Truncate(text, room, "") + ell + tail
}
