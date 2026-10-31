package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	tuicommon "mosaic-common/tui"
	"mosaic-common/interaction"
	"mosaic-common/tui/widgets"
)

const (
	multilineChromeLines = 6 // title, detail gap, label, hint and slack around the text area
	multilineMinRows     = 3
	multilineDefaultCols = 80
	multilineDefaultRows = 8
)

// multilineSize is the column and row count of the multi-line entry for a
// terminal of the given size.
func multilineSize(q interaction.TextQuestion, width, height int) (cols, rows int) {
	cols, rows = width, height-multilineChromeLines-lineCount(q.Detail)
	if cols < 1 {
		cols = multilineDefaultCols
	}
	if height < 1 {
		rows = multilineDefaultRows
	}
	if rows < multilineMinRows {
		rows = multilineMinRows
	}
	return cols, rows
}

// newMultilineArea builds the multi-line entry for a text question that asks
// for one: pre-filled with the default, validated on submit, focused.
func newMultilineArea(q interaction.TextQuestion, styles tuicommon.Theme, width, height int) *widgets.TextArea {
	cols, rows := multilineSize(q, width, height)
	area := widgets.NewTextArea(q.Prompt, cols, rows, widgets.TextAreaStyles{
		Label:  styles.Style(tuicommon.RoleBody),
		Input:  lipgloss.NewStyle(),
		ErrMsg: styles.Style(tuicommon.RoleError),
		Help:   styles.Style(tuicommon.RoleHelp),
	})
	if q.Default != "" {
		area.SetValue(q.Default)
	}
	if q.Validate != nil {
		area.SetValidate(q.Validate)
	}
	area.Focus()
	return area
}

// multilineView renders title, detail and the multi-line entry.
func (t *inlineText) multilineView() string {
	var sb strings.Builder
	sb.WriteString(t.styles.Style(tuicommon.RoleTitle).Width(t.width).Render(t.q.Title))
	sb.WriteByte('\n')
	if t.q.Detail != "" {
		sb.WriteString(t.styles.Style(tuicommon.RoleMuted).Width(t.width).Render(t.q.Detail))
		sb.WriteByte('\n')
	}
	// The widget's key hint line is not wrapped; wrap it to the overlay width.
	body := t.area.View()
	if t.width > 0 {
		body = lipgloss.NewStyle().Width(t.width).Render(body)
	}
	sb.WriteString(body)
	return sb.String()
}
