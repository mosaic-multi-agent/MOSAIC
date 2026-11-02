package chrome

import (
	"fmt"
	"strings"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

// HelpGroup is a titled list of key entries.
type HelpGroup struct {
	Title   string
	Entries []tui.HelpEntry
}

// helpLine is one overlay content line: a styled part and the remaining plain part.
type helpLine struct {
	head     string
	headRole tui.StyleRole
	tail     string
}

// HelpOverlay lists every entry of every group within width x height. Content taller than
// height is cut to a window starting at offset, with a final line showing the position.
func HelpOverlay(title string, groups []HelpGroup, offset, width, height int, theme tui.Theme) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	content := helpContent(title, groups)
	total := len(content)
	var shown []string
	if total <= height {
		shown = renderHelpLines(content, width, theme)
	} else if height == 1 {
		shown = []string{moreLine(fmt.Sprintf("-- more: %d lines --", total), width)}
	} else {
		body := height - 1
		start := min(max(offset, 0), total-body)
		shown = renderHelpLines(content[start:start+body], width, theme)
		more := fmt.Sprintf("-- more: %d-%d of %d --", start+1, start+body, total)
		shown = append(shown, moreLine(more, width))
	}
	return strings.Join(shown, "\n")
}

func moreLine(text string, width int) string {
	return textwidth.Pad(text, width, false)
}

// helpContent lays out the title, then each group with its entries aligned on one key column.
func helpContent(title string, groups []HelpGroup) []helpLine {
	keyWidth := 0
	for _, g := range groups {
		for _, e := range g.Entries {
			keyWidth = max(keyWidth, textwidth.Of(e.Key))
		}
	}
	lines := []helpLine{{head: title, headRole: tui.RoleTitle}, {}}
	for n, g := range groups {
		if n > 0 {
			lines = append(lines, helpLine{})
		}
		if g.Title != "" {
			lines = append(lines, helpLine{head: "  " + g.Title, headRole: tui.RoleSubtitle})
		}
		for _, e := range g.Entries {
			lines = append(lines, helpLine{
				head:     "    " + textwidth.Pad(e.Key, keyWidth, false),
				headRole: tui.RoleHelp,
				tail:     "  " + e.Desc,
			})
		}
	}
	return lines
}

// renderHelpLines styles each line and pads it to exactly width cells.
func renderHelpLines(lines []helpLine, width int, theme tui.Theme) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		head := textwidth.Truncate(l.head, width)
		tail := textwidth.Truncate(l.tail, width-textwidth.Of(head))
		pad := strings.Repeat(" ", width-textwidth.Of(head)-textwidth.Of(tail))
		out[i] = theme.Style(l.headRole).Render(head) + tail + pad
	}
	return out
}
