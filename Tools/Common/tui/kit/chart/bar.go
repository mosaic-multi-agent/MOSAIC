// Package chart renders domain-free value charts: bars, stacked bars and sparklines.
package chart

import (
	"strings"

	"mosaic-common/tui"
)

// eighthBlocks holds the partial-cell glyphs for 1/8 to 7/8 of a cell.
var eighthBlocks = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// Bar renders value/max as a bar exactly width cells wide (padded with spaces).
// A non-positive value or maximum draws an empty bar; a value at or above max fills the width.
func Bar(value, max float64, width int, role tui.StyleRole, theme tui.Theme) string {
	if width <= 0 {
		return ""
	}
	if value <= 0 || max <= 0 {
		return spaces(width)
	}
	fraction := min(value/max, 1)
	var filled string
	used := 0
	if ascii() {
		used = roundHalfUp(fraction * float64(width))
		filled = strings.Repeat("#", used)
	} else {
		eighths := roundHalfUp(fraction * float64(width*8))
		full, rem := eighths/8, eighths%8
		filled = strings.Repeat("█", full)
		used = full
		if rem > 0 {
			filled += eighthBlocks[rem-1]
			used++
		}
	}
	return paint(theme, role, filled) + spaces(width-used)
}
