package chart

import (
	"strings"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

// RenderCurve renders exactly height lines, each at most width cells.
// A height below 2 gives "". The scale is [0, largest value]; guide bands colour the plot-area
// rows whose value they contain, and markers fill the full height of the column holding their point.
func RenderCurve(c Curve, width, height int, theme tui.Theme) string {
	if height < 2 {
		return ""
	}
	if width <= 0 {
		return strings.Repeat("\n", height-1)
	}
	top := curveTop(c.Values)
	gutter, plotWidth := curveGutter(c, top, width)
	if plotWidth < 1 {
		return strings.Join(repeatLine(spaces(width), height), "\n")
	}
	pointRows, markers := curveColumns(c, top, plotWidth, height)
	pointGlyph := "•"
	if ascii() {
		pointGlyph = "o"
	}
	lines := make([]string, 0, height)
	for r := height - 1; r >= 0; r-- {
		role, inBand := guideRole(c.Guides, float64(r)*top/float64(height-1))
		var b strings.Builder
		b.WriteString(gutterText(gutter, c, top, r, height))
		for col := 0; col < plotWidth; col++ {
			glyph := " "
			switch {
			case markers[col] != "":
				glyph = markers[col]
			case pointRows[col] == r:
				glyph = pointGlyph
			}
			switch {
			case inBand:
				b.WriteString(paint(theme, role, glyph))
			case glyph != " ":
				b.WriteString(paint(theme, tui.RoleBody, glyph))
			default:
				b.WriteString(glyph)
			}
		}
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

func repeatLine(line string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = line
	}
	return out
}

// curveTop is the largest value, with negatives counting as zero.
func curveTop(values []float64) float64 {
	top := 0.0
	for _, v := range values {
		top = max(top, v)
	}
	return top
}

// curveGutter returns the gutter width (0 without YLabel) and the remaining plot width.
func curveGutter(c Curve, top float64, width int) (gutter, plotWidth int) {
	if c.YLabel == nil {
		return 0, width
	}
	gutter = max(textwidth.Of(c.YLabel(top)), textwidth.Of(c.YLabel(0))) + 1
	return gutter, width - gutter
}

// gutterText is the left gutter of row r: the top and zero labels on the end rows, blanks between.
func gutterText(gutter int, c Curve, top float64, r, height int) string {
	switch {
	case gutter == 0:
		return ""
	case r == height-1:
		return textwidth.Pad(c.YLabel(top), gutter-1, true) + " "
	case r == 0:
		return textwidth.Pad(c.YLabel(0), gutter-1, true) + " "
	}
	return spaces(gutter)
}

// guideRole returns the role of the first guide whose range contains v.
func guideRole(guides []Guide, v float64) (tui.StyleRole, bool) {
	for _, g := range guides {
		if v >= g.From && (g.To <= g.From || v < g.To) {
			return g.Role, true
		}
	}
	return "", false
}

// curveColumns places the series on a plotWidth-column grid. pointRows[col] is the row of the
// point drawn in col (-1 for none); markers[col] is the marker glyph filling col ("" for none).
func curveColumns(c Curve, top float64, plotWidth, height int) (pointRows []int, markers []string) {
	n := len(c.Values)
	colOf := make([]int, n)
	for i := range colOf {
		colOf[i] = curveColumnOf(i, n, plotWidth)
	}
	colMax := make([]float64, plotWidth)
	pointRows = make([]int, plotWidth)
	for i := range pointRows {
		pointRows[i] = -1
	}
	for i, v := range c.Values {
		v = max(v, 0)
		col := colOf[i]
		if pointRows[col] < 0 || v > colMax[col] {
			colMax[col] = v
			pointRows[col] = 0
			if top > 0 {
				pointRows[col] = roundHalfUp(v * float64(height-1) / top)
			}
		}
	}
	markers = make([]string, plotWidth)
	kinds := make([]int, plotWidth)
	for i := range kinds {
		kinds[i] = -1
	}
	for _, m := range c.Markers {
		if m.Index < 0 || m.Index >= n {
			continue
		}
		col := colOf[m.Index]
		if int(m.Kind) > kinds[col] {
			kinds[col] = int(m.Kind)
			markers[col] = MarkerGlyph(m.Kind)
		}
	}
	return pointRows, markers
}

// curveColumnOf maps point i of n to its column: points are spread across the width when they
// fit, otherwise column c holds the points [c*n/width, (c+1)*n/width).
func curveColumnOf(i, n, width int) int {
	if n <= width {
		if n == 1 {
			return 0
		}
		return i * (width - 1) / (n - 1)
	}
	col := i * width / n
	for col > 0 && col*n/width > i {
		col--
	}
	for col < width-1 && (col+1)*n/width <= i {
		col++
	}
	return col
}
