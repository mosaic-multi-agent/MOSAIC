package tui

// The drawing of the question overlays and the error screen. They compose the
// kit table and text helpers: widths, scrolling and colours come from the kit and
// the theme roles, and nothing is drawn past the terminal.

import (
	"strings"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/table"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	selectHelp = "↑/↓ navigate  enter select  esc cancel  ctrl+c quit"
	textHelp   = "enter confirm  esc cancel  ctrl+c quit"
	errorHint  = "Press any key to quit."

	// minOptionRows is how many option rows the select overlay keeps by cutting
	// its prompt on a short terminal.
	minOptionRows = 3
)

// styledLines draws each line in the role, cut to width.
func styledLines(lines []string, role tuicommon.StyleRole, width int, theme tuicommon.Theme) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = theme.Style(role).Render(textwidth.Truncate(l, width))
	}
	return out
}

// wrapped wraps text to width and draws it in role.
func wrapped(text string, role tuicommon.StyleRole, width int, theme tuicommon.Theme) []string {
	return styledLines(screens.WrapText(text, width), role, width, theme)
}

// fitHeight cuts lines to the terminal height.
func fitHeight(lines []string, height int) string {
	return strings.Join(lines[:min(len(lines), max(height, 0))], "\n")
}

// optionColumns are the columns of the option list; the description column only
// exists when some option has a description.
func (o *overlaySelectOne) optionColumns() []table.Column {
	cols := []table.Column{{Title: "option", Flex: true, MinWidth: 8}}
	for _, opt := range o.q.Options {
		if opt.Description != "" {
			return append(cols, table.Column{Title: "description", Priority: 1})
		}
	}
	return cols
}

// optionRows are the table rows of the options followed by the skip entry.
func (o *overlaySelectOne) optionRows(columns int) [][]table.Cell {
	var rows [][]table.Cell
	for _, opt := range o.q.Options {
		role := tuicommon.RoleBody
		if opt.Disabled {
			role = tuicommon.RoleSignalMuted
		}
		row := []table.Cell{{Text: opt.Label, Role: role}}
		if columns > 1 {
			row = append(row, table.Cell{Text: opt.Description, Role: tuicommon.RoleSignalMuted})
		}
		rows = append(rows, row)
	}
	if o.q.AllowSkip {
		rows = append(rows, []table.Cell{{Text: "Skip"}})
	}
	return rows
}

// view renders the select overlay within width and height; the option list
// scrolls so the cursor stays visible and the key help stays at the bottom.
func (o *overlaySelectOne) view(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	n := o.totalOptions()
	head := styledLines([]string{o.q.Title}, tuicommon.RoleTitle, width, o.theme)
	if o.q.Prompt != "" {
		head = append(head, wrapped(o.q.Prompt, tuicommon.RoleBody, width, o.theme)...)
	}
	head = head[:min(len(head), max(height-1-min(1+n, 1+minOptionRows), 1))]
	help := styledLines([]string{selectHelp}, tuicommon.RoleHelp, width, o.theme)

	lines := head
	if rows := height - len(head) - 1 - 1; rows >= 0 && n > 0 {
		w := screens.ScrollWindow(o.offset, o.cursor, n, rows)
		o.offset = w.Offset
		cols := o.optionColumns()
		t := table.Table{Columns: cols, Rows: o.optionRows(len(cols)), Cursor: o.cursor, Offset: w.Offset, SortColumn: table.NoSort}
		lines = append(lines, strings.Split(table.Render(t, width, 1+w.Count, o.theme), "\n")...)
	}
	return fitHeight(append(lines, help...), height)
}

// view renders the text-input overlay within width and height. On a short
// terminal the title, the input and the key help stay; the validation message,
// the prompt and the detail follow in that order as room allows.
func (o *overlayText) view(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	room := height - 3
	take := func(lines []string) []string {
		n := min(len(lines), max(room, 0))
		room -= n
		return lines[:n]
	}
	var message, prompt, detail []string
	if msg := o.input.ErrMsg(); msg != "" {
		message = take(wrapped(msg, tuicommon.RoleSignalBad, width, o.theme))
	}
	if o.q.Prompt != "" {
		prompt = take(wrapped(o.q.Prompt, tuicommon.RoleBody, width, o.theme))
	}
	if o.q.Detail != "" {
		detail = take(wrapped(o.q.Detail, tuicommon.RoleSignalMuted, width, o.theme))
	}

	lines := styledLines([]string{o.q.Title}, tuicommon.RoleTitle, width, o.theme)
	lines = append(lines, prompt...)
	lines = append(lines, detail...)
	lines = append(lines, textwidth.Truncate(o.input.View(), width))
	lines = append(lines, message...)
	return fitHeight(append(lines, styledLines([]string{textHelp}, tuicommon.RoleHelp, width, o.theme)...), height)
}

// viewError renders the error screen: the title, the message in the bad signal
// role and the key hint. On a short terminal the message is cut first.
func (m Model) viewError() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	theme := stylesFromTheme(m.theme).Theme
	message := screens.WrapText(m.errMsg, m.width)
	message = message[:min(len(message), max(m.height-3, 0))]

	lines := styledLines([]string{"Log Analyzer — Error"}, tuicommon.RoleTitle, m.width, theme)
	lines = append(lines, styledLines(message, tuicommon.RoleSignalBad, m.width, theme)...)
	if m.height >= 3 {
		lines = append(lines, "")
	}
	return fitHeight(append(lines, styledLines([]string{errorHint}, tuicommon.RoleHelp, m.width, theme)...), m.height)
}
