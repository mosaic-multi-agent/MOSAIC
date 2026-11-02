package table

import "mosaic-common/tui"

// SharedLayout is one column layout computed for tables stacked in one view.
type SharedLayout struct {
	width int
	cols  int
	shape shape
}

// NewSharedLayout lays out tables that share one column set (same len(Columns) and
// titles): widths are the widest natural width of each column across all tables,
// and the hidden columns are chosen once for all of them.
func NewSharedLayout(tables []Table, width int) SharedLayout {
	l := SharedLayout{width: width, shape: shape{titleH: 1}}
	if len(tables) == 0 {
		return l
	}
	l.cols = len(tables[0].Columns)
	var members []Table
	natural := make([]int, l.cols)
	for _, t := range tables {
		if len(t.Columns) != l.cols {
			continue
		}
		members = append(members, t)
		for i, w := range naturalWidths(t) {
			natural[i] = max(natural[i], w)
		}
	}
	visible, hidden, widths := fit(tables[0].Columns, natural, width)
	l.shape = newShape(members, visible, hidden, widths)
	return l
}

// Render draws t with the shared widths and visible columns. A table whose column count
// differs from the layout is drawn with Render at the layout's width instead.
func (l SharedLayout) Render(t Table, height int, theme tui.Theme) string {
	if len(t.Columns) != l.cols || l.width <= 0 || height <= 0 {
		return Render(t, l.width, height, theme)
	}
	return draw(t, l.shape, l.width, height, theme)
}

// HeaderHeight is the header height every table of the layout draws.
func (l SharedLayout) HeaderHeight() int { return l.shape.headerHeight() }

// BodyRows is max(height - l.HeaderHeight(), 0).
func (l SharedLayout) BodyRows(height int) int { return max(height-l.HeaderHeight(), 0) }
