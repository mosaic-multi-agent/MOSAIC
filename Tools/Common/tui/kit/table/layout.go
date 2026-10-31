package table

import (
	"strconv"

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
		if t.WrapTitles {
			w = longestWord(titleWords(t, i))
		}
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

// markerText is the header marker announcing how many columns are hidden.
func markerText(hidden int) string {
	arrow := ">"
	if tui.CurrentRenderMode().Glyphs != tui.GlyphASCII {
		arrow = "›"
	}
	return "+" + strconv.Itoa(hidden) + " " + arrow
}

// markerWidth is the room the marker takes at the end of the header, gap included.
func markerWidth(hidden int) int {
	if hidden == 0 {
		return 0
	}
	return gapWidth + textwidth.Of(markerText(hidden))
}

// minWidths is how narrow each column can get: flex columns down to MinWidth, others not at all.
func minWidths(cols []Column, widths []int) []int {
	mins := make([]int, len(cols))
	for i, c := range cols {
		mins[i] = widths[i]
		if c.Flex && c.Width <= 0 {
			mins[i] = min(widths[i], max(c.MinWidth, 1))
		}
	}
	return mins
}

// Layout returns the indexes of the columns Render shows at width, in column order, and
// the number of hidden columns. Flex columns shrink before any column is dropped; columns
// then drop by priority (highest number first, rightmost among equals, priority 0 never).
func Layout(t Table, width int) (visible []int, hidden int) {
	visible, hidden, _ = layoutWidths(t, width)
	return visible, hidden
}

// layoutWidths is Layout plus the final column widths, shrunk to fit beside the marker.
func layoutWidths(t Table, width int) (visible []int, hidden int, widths []int) {
	return fit(t.Columns, naturalWidths(t), width)
}

// fit chooses the visible columns and shrinks the natural widths (modified in place) to fit width.
func fit(cols []Column, widths []int, width int) (visible []int, hidden int, _ []int) {
	visible, hidden = chooseColumns(cols, minWidths(cols, widths), width)
	shrinkFlex(cols, widths, visible, width-markerWidth(hidden))
	return visible, hidden, widths
}

// chooseColumns drops columns until the line fits at the given (minimum) widths beside the
// hidden-column marker: highest priority number first, rightmost among equals; priority 0 is
// never dropped.
func chooseColumns(cols []Column, widths []int, width int) (visible []int, hidden int) {
	visible = make([]int, len(cols))
	for i := range cols {
		visible[i] = i
	}
	for totalWidth(widths, visible)+markerWidth(hidden) > width {
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
		hidden++
	}
	return visible, hidden
}

// shrinkFlex narrows flex columns, down to their MinWidth, until the line fits width.
// Identity columns shrink only after every other flex column is at its MinWidth.
func shrinkFlex(cols []Column, widths []int, visible []int, width int) {
	for _, identity := range []bool{false, true} {
		deficit := totalWidth(widths, visible) - width
		shrinkPass(cols, widths, visible, deficit, identity)
	}
}

// shrinkPass narrows, left to right, the flex columns whose Identity flag equals identity.
func shrinkPass(cols []Column, widths []int, visible []int, deficit int, identity bool) {
	for _, i := range visible {
		if deficit <= 0 {
			return
		}
		if !cols[i].Flex || cols[i].Width > 0 || cols[i].Identity != identity {
			continue
		}
		floor := max(cols[i].MinWidth, 1)
		cut := min(deficit, max(widths[i]-floor, 0))
		widths[i] -= cut
		deficit -= cut
	}
}
