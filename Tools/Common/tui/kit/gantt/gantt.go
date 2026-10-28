package gantt

import (
	"strings"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/textwidth"
)

// cell is what one plot cell shows after layering.
type cell struct {
	glyph string
	role  tui.StyleRole
	set   bool
	kind  int // point kind, -1 when the cell holds no point
}

// RenderLane renders one lane: its label, a space, then the plot area.
// Cells are layered point over interval over shade; a width at or below labelWidth shows only
// the cut label.
func RenderLane(a Axis, lane Lane, shades []Shade, labelWidth, width int, selected bool, theme tui.Theme) string {
	if width <= 0 {
		return ""
	}
	labelWidth = max(labelWidth, 0)
	labelRole := lane.LabelRole
	if selected {
		labelRole = tui.RoleSelected
	}
	if width <= labelWidth {
		return paint(theme, labelRole, textwidth.Pad(lane.Label, width, false))
	}
	line := paint(theme, labelRole, textwidth.Pad(lane.Label, labelWidth, false)) + " "
	plotWidth := width - labelWidth - 1
	if plotWidth == 0 {
		return line
	}
	return line + plotCells(a, lane, shades, plotWidth, theme)
}

func plotCells(a Axis, lane Lane, shades []Shade, plotWidth int, theme tui.Theme) string {
	if _, ok := a.Column(a.Start, plotWidth); !ok {
		return spaces(plotWidth)
	}
	shade, bar, open := "░", "█", ">"
	if tui.CurrentRenderMode().Glyphs == tui.GlyphASCII {
		shade, bar = ".", "="
	}
	shadeCells := make([]cell, plotWidth)
	for _, s := range shades {
		if first, last, ok := a.coverage(s.Start, s.End, plotWidth); ok {
			fill(shadeCells, first, last, cell{glyph: shade, role: s.Role, set: true})
		}
	}
	barCells := make([]cell, plotWidth)
	for _, iv := range lane.Intervals {
		if iv.Open {
			if iv.Start.After(a.End) {
				continue
			}
			first, _ := a.Column(maxTime(iv.Start, a.Start), plotWidth)
			fill(barCells, first, plotWidth-1, cell{glyph: bar, role: iv.Role, set: true})
			barCells[plotWidth-1].glyph = open
		} else if first, last, ok := a.coverage(iv.Start, iv.End, plotWidth); ok {
			fill(barCells, first, last, cell{glyph: bar, role: iv.Role, set: true})
		}
	}
	pointCells := make([]cell, plotWidth)
	for i := range pointCells {
		pointCells[i].kind = -1
	}
	for _, p := range lane.Points {
		if p.At.Before(a.Start) || p.At.After(a.End) {
			continue
		}
		col, _ := a.Column(p.At, plotWidth)
		if int(p.Kind) > pointCells[col].kind {
			pointCells[col] = cell{glyph: chart.MarkerGlyph(p.Kind), role: p.Role, set: true, kind: int(p.Kind)}
		}
	}
	var b strings.Builder
	for i := 0; i < plotWidth; i++ {
		switch c := pickCell(pointCells[i], barCells[i], shadeCells[i]); {
		case c.set:
			b.WriteString(paint(theme, c.role, c.glyph))
		default:
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// pickCell returns the topmost set cell of point, interval and shade layers.
func pickCell(point, bar, shade cell) cell {
	switch {
	case point.set:
		return point
	case bar.set:
		return bar
	}
	return shade
}

func fill(cells []cell, first, last int, c cell) {
	for i := first; i <= last; i++ {
		cells[i] = c
	}
}

// paint renders text in role's style; an empty role uses the body style and empty text stays empty.
func paint(theme tui.Theme, role tui.StyleRole, text string) string {
	if text == "" {
		return ""
	}
	if role == "" {
		role = tui.RoleBody
	}
	return theme.Style(role).Render(text)
}

func spaces(n int) string { return strings.Repeat(" ", max(n, 0)) }
