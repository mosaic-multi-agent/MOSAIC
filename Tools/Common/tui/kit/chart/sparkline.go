package chart

import (
	"strings"

	"mosaic-common/tui"
)

// MarkerKind selects a marker glyph.
type MarkerKind uint8

const (
	MarkerPrimary MarkerKind = iota
	MarkerSecondary
	MarkerTertiary
)

var (
	unicodeLevels = []rune("▁▂▃▄▅▆▇█")
	asciiLevels   = []rune("_-=#")
)

// Sparkline renders values as one line of width cells. A series longer than width is bucketed
// by maximum; a shorter one is left aligned. roles[i] colours point i (missing or empty roles
// use the body style); a bucket takes the role of its first maximum point. A marker in
// [0, len(values)) replaces the glyph of the column holding that point.
func Sparkline(values []float64, roles []tui.StyleRole, marker, width int, theme tui.Theme) string {
	if width <= 0 {
		return ""
	}
	if len(values) == 0 {
		return spaces(width)
	}
	cols := sparkColumns(values, width)
	top := 0.0
	for _, c := range cols {
		top = max(top, c.value)
	}
	levels, markGlyph := unicodeLevels, "┃"
	if ascii() {
		levels, markGlyph = asciiLevels, "|"
	}
	markCol := -1
	if marker >= 0 && marker < len(values) {
		markCol = columnOf(marker, len(values), width)
	}
	var b strings.Builder
	for j, c := range cols {
		glyph := string(levels[levelOf(c.value, top, len(levels))])
		if j == markCol {
			glyph = markGlyph
		}
		role := tui.StyleRole("")
		if c.point < len(roles) {
			role = roles[c.point]
		}
		b.WriteString(paint(theme, role, glyph))
	}
	b.WriteString(spaces(width - len(cols)))
	return b.String()
}

type sparkColumn struct {
	value float64 // maximum of the column's points, negatives as zero
	point int     // index of the first point holding that maximum
}

// columnOf maps point index i of n points to its column in a line of width cells.
func columnOf(i, n, width int) int {
	if n <= width {
		return i
	}
	return i * width / n
}

func sparkColumns(values []float64, width int) []sparkColumn {
	n := len(values)
	count := min(n, width)
	cols := make([]sparkColumn, count)
	seen := make([]bool, count)
	for i, v := range values {
		v = positive(v)
		j := columnOf(i, n, width)
		if !seen[j] || v > cols[j].value {
			cols[j] = sparkColumn{value: v, point: i}
			seen[j] = true
		}
	}
	return cols
}

// levelOf scales v within [0, top] onto one of count glyph levels.
func levelOf(v, top float64, count int) int {
	if top <= 0 {
		return 0
	}
	return roundHalfUp(v / top * float64(count-1))
}
