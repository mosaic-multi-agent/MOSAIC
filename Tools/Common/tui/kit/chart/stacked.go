package chart

import (
	"sort"
	"strings"

	"mosaic-common/tui"
)

// Segment is one share of a stacked bar.
type Segment struct {
	Label string
	Value float64 // >= 0; negatives treated as 0
	Role  tui.StyleRole
}

var (
	unicodeSwatches = []string{"█", "▓", "▒", "░"}
	asciiSwatches   = []string{"#", "=", "+", ":"}
)

// swatch returns the fill glyph of the i-th segment; glyphs repeat after four segments.
func swatch(i int) string {
	set := unicodeSwatches
	if ascii() {
		set = asciiSwatches
	}
	return set[i%len(set)]
}

func positive(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

// StackedBar renders segments as one bar exactly width cells wide.
func StackedBar(segs []Segment, width int, theme tui.Theme) string {
	if width <= 0 {
		return ""
	}
	cells := segmentCells(segs, width)
	if cells == nil {
		return spaces(width)
	}
	var b strings.Builder
	for i, s := range segs {
		b.WriteString(paint(theme, s.Role, strings.Repeat(swatch(i), cells[i])))
	}
	return b.String()
}

// segmentCells splits width cells among segments by largest remainder, then gives every
// positive segment at least one cell taken from the largest segment. It returns nil when no
// segment has a positive value.
func segmentCells(segs []Segment, width int) []int {
	total := 0.0
	for _, s := range segs {
		total += positive(s.Value)
	}
	if total <= 0 {
		return nil
	}
	cells := make([]int, len(segs))
	remainders := make([]float64, len(segs))
	assigned := 0
	for i, s := range segs {
		quota := positive(s.Value) * float64(width) / total
		cells[i] = int(quota)
		remainders[i] = quota - float64(cells[i])
		assigned += cells[i]
	}
	order := make([]int, len(segs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return remainders[order[a]] > remainders[order[b]] })
	for k := 0; assigned < width && k < len(order); k++ {
		cells[order[k]]++
		assigned++
	}
	ensureVisible(segs, cells)
	return cells
}

// ensureVisible gives each positive segment without cells one cell from the largest segment
// (ties to the earliest) while that segment keeps at least one.
func ensureVisible(segs []Segment, cells []int) {
	for i, s := range segs {
		if positive(s.Value) <= 0 || cells[i] > 0 {
			continue
		}
		donor := -1
		for j, c := range cells {
			if donor < 0 || c > cells[donor] {
				donor = j
			}
		}
		if donor < 0 || cells[donor] <= 1 {
			return
		}
		cells[donor]--
		cells[i]++
	}
}
