package chrome

import (
	"strings"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

// Tab is one tab bar entry.
type Tab struct{ Key, Label string }

// tabText is the tab's text: the key, a space and the label.
func tabText(t Tab) string { return t.Key + " " + t.Label }

// tabCell is one tab as drawn: bracketed when active, padded with a space each side otherwise.
func tabCell(t Tab, active bool) string {
	if active {
		return "[" + tabText(t) + "]"
	}
	return " " + tabText(t) + " "
}

// TabBar renders one line with the active tab highlighted. When the tabs do not fit, the
// bar shows the active tab and as many neighbours as fit.
func TabBar(tabs []Tab, active, width int, theme tui.Theme) string {
	if len(tabs) == 0 || width <= 0 {
		return ""
	}
	cells := make([]string, len(tabs))
	for i, t := range tabs {
		cells[i] = tabCell(t, i == active)
	}
	first, last := 0, len(tabs)-1
	if widthOf(cells, first, last) > width {
		first, last = window(cells, min(max(active, 0), len(tabs)-1), width)
	}
	var b strings.Builder
	for i := first; i <= last; i++ {
		cell := cells[i]
		if first == last {
			cell = textwidth.Truncate(cell, width)
		}
		if i == active {
			cell = theme.Style(tui.RoleSelected).Render(cell)
		}
		b.WriteString(cell)
	}
	return b.String()
}

func widthOf(cells []string, first, last int) int {
	total := 0
	for i := first; i <= last; i++ {
		total += textwidth.Of(cells[i])
	}
	return total
}

// window grows a range from the anchor cell, left neighbour first, while it fits width.
func window(cells []string, anchor, width int) (first, last int) {
	first, last = anchor, anchor
	for {
		switch {
		case first > 0 && widthOf(cells, first-1, last) <= width:
			first--
		case last < len(cells)-1 && widthOf(cells, first, last+1) <= width:
			last++
		default:
			return first, last
		}
	}
}
