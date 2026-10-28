package table

import (
	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

const (
	gutterWidth = 2
	gapWidth    = 2
)

// sortMarker returns the header marker for the sort direction in the current glyph mode.
func sortMarker(desc bool) string {
	ascii := tui.CurrentRenderMode().Glyphs == tui.GlyphASCII
	switch {
	case ascii && desc:
		return "v"
	case ascii:
		return "^"
	case desc:
		return "↓"
	default:
		return "↑"
	}
}

// headerText is the column title, followed by the sort marker on the sort column.
func headerText(t Table, i int) string {
	if i == t.SortColumn {
		return t.Columns[i].Title + " " + sortMarker(t.SortDesc)
	}
	return t.Columns[i].Title
}

// cellText is the text of row r in column i; missing cells are empty.
func cellText(row []Cell, i int) string {
	if i < len(row) {
		return row[i].Text
	}
	return ""
}

// naturalWidths sizes every column to its widest header or cell (or its fixed Width).
func naturalWidths(t Table) []int {
	widths := make([]int, len(t.Columns))
	for i, c := range t.Columns {
		if c.Width > 0 {
			widths[i] = c.Width
			continue
		}
		w := textwidth.Of(headerText(t, i))
		for _, row := range t.Rows {
			w = max(w, textwidth.Of(cellText(row, i)))
		}
		widths[i] = w
	}
	return widths
}

// totalWidth is the line width of the gutter plus the visible columns and their gaps.
func totalWidth(widths []int, visible []int) int {
	total := gutterWidth
	for n, i := range visible {
		if n > 0 {
			total += gapWidth
		}
		total += widths[i]
	}
	return total
}

// chooseColumns drops columns until the natural width fits: the highest priority number
// first, the rightmost among equals; priority 0 is never dropped.
func chooseColumns(cols []Column, widths []int, width int) []int {
	visible := make([]int, len(cols))
	for i := range cols {
		visible[i] = i
	}
	for totalWidth(widths, visible) > width {
		drop := -1
		for n, i := range visible {
			if cols[i].Priority > 0 && (drop < 0 || cols[i].Priority >= cols[visible[drop]].Priority) {
				drop = n
			}
		}
		if drop < 0 {
			break
		}
		visible = append(visible[:drop:drop], visible[drop+1:]...)
	}
	return visible
}

// shrinkFlex narrows flex columns, down to their MinWidth, until the line fits width.
func shrinkFlex(cols []Column, widths []int, visible []int, width int) {
	deficit := totalWidth(widths, visible) - width
	for _, i := range visible {
		if deficit <= 0 {
			return
		}
		if !cols[i].Flex || cols[i].Width > 0 {
			continue
		}
		floor := max(cols[i].MinWidth, 1)
		cut := min(deficit, max(widths[i]-floor, 0))
		widths[i] -= cut
		deficit -= cut
	}
}
