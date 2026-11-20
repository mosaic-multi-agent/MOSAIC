package screens

// multiselect_layout.go is the shared height budget of Deploy's list screens. A screen renders a
// fixed frame (title, borders, summary, validation message, help) plus a content block of list
// rows beside a detail pane. The frame is measured from the strings the screen is about to
// render, so wrapped lines count, and the list is given the rest of the terminal height, but never
// more rows than it has. The detail pane is cut to the same height, so the content block is as
// tall as the list needs and the whole view fits the terminal.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"mosaic-deploy/internal/tui/widgets"
)

// minDetailRows is the least height given to a list beside a detail pane when the terminal has
// room: the pane shows a title and a few lines of text.
const minDetailRows = 3

// minDetailWidth is the narrowest detail pane.
const minDetailWidth = 10

// frameLines returns the number of terminal lines the rendered frame parts take. Empty parts are
// not rendered by the screens and count for nothing.
func frameLines(parts ...string) int {
	n := 0
	for _, p := range parts {
		if p != "" {
			n += lipgloss.Height(p)
		}
	}
	return n
}

// fitListHeight returns the height of a list with the given number of rows: the rows it needs,
// at least floor when the terminal has room, and never more than the terminal height left after
// the frame. A terminal too small for the frame still gets one row.
func fitListHeight(totalHeight, frame, rows, floor int) int {
	avail := totalHeight - frame
	if avail < 1 {
		avail = 1
	}
	h := rows
	if h < floor {
		h = floor
	}
	if h > avail {
		h = avail
	}
	return h
}

// splitListWidths returns the list and detail pane widths so that list, separator and pane add
// up to the terminal width, also on a narrow terminal.
func splitListWidths(totalWidth int) (listW, detailW int) {
	listW = uaListWidth(totalWidth)
	if listW+1+minDetailWidth > totalWidth {
		listW = totalWidth - 1 - minDetailWidth
		if listW < 1 {
			listW = 1
		}
	}
	detailW = totalWidth - listW - 1
	if detailW < minDetailWidth {
		detailW = minDetailWidth
	}
	return listW, detailW
}

// fitMultiSelect sizes a multi-select and its detail pane for the screen. frame holds the
// rendered parts of the screen outside the content block.
func fitMultiSelect(ms *widgets.MultiSelect, detail *widgets.DetailPane, width, height, rows int, frame ...string) {
	lw, dw := splitListWidths(width)
	budget := height - frameLines(frame...)
	lh := fitListHeight(height, frameLines(frame...), rows, minDetailRows)
	ms.Resize(lh, lw)
	// A row longer than the list column wraps onto further lines; show fewer rows until the
	// rendered rows fit. The cursor row stays visible because the widget keeps it in view.
	shown := lipgloss.Height(ms.View())
	for lh > 1 && shown > budget {
		lh--
		ms.Resize(lh, lw)
		shown = lipgloss.Height(ms.View())
	}
	detail.Resize(max(shown, min(minDetailRows, lh)), dw)
}

// fitList sizes a plain list that has no detail pane beside it.
func fitList(l *widgets.List, width, height, rows int, frame ...string) {
	lw, _ := splitListWidths(width)
	l.Resize(fitListHeight(height, frameLines(frame...), rows, 1), lw)
}

// oneLineSummary renders text in style on a single line of the given width, cutting it with an
// ellipsis when it does not fit, so a long selection never grows the frame.
func oneLineSummary(style lipgloss.Style, width int, text string) string {
	avail := width - style.GetHorizontalFrameSize()
	if avail < 1 {
		avail = 1
	}
	if lipgloss.Width(text) > avail {
		var sb strings.Builder
		for _, r := range text {
			if lipgloss.Width(sb.String()+string(r)) > avail-1 {
				break
			}
			sb.WriteRune(r)
		}
		text = sb.String() + "…"
	}
	return style.Width(width).Render(text)
}

// validationLine renders the transient validation message in style, or returns "" when there is
// none.
func validationLine(style lipgloss.Style, width int, msg string) string {
	if msg == "" {
		return ""
	}
	return style.Width(width).Render(msg)
}
