package gantt

import (
	"strings"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chart"
	"mosaic-common/tui/kit/textwidth"
)

// ShadeSwatch returns one cell drawn exactly as s draws a shaded cell in the current glyph mode.
func ShadeSwatch(s Shade, theme tui.Theme) string {
	return paint(theme, s.Role, shadeGlyph(s))
}

// PointSwatch returns one cell drawn exactly as p draws on a lane.
func PointSwatch(p Point, theme tui.Theme) string {
	return paint(theme, p.Role, chart.MarkerGlyph(p.Kind))
}

// LegendEntry is one item of a legend line.
type LegendEntry struct {
	Swatch string // one styled cell, from ShadeSwatch / PointSwatch
	Label  string
}

// RenderLegend packs "swatch label" entries separated by two spaces into lines of at most
// width cells, wrapping only between entries; an entry wider than width is cut.
func RenderLegend(entries []LegendEntry, width int, theme tui.Theme) []string {
	if width <= 0 {
		return nil
	}
	const sep = "  "
	var lines []string
	var cur strings.Builder
	curWidth := 0
	for _, e := range entries {
		label := textwidth.Truncate(e.Label, max(width-2, 0))
		text := e.Swatch + " " + label
		w := 2 + textwidth.Of(label)
		if curWidth > 0 && curWidth+len(sep)+w > width {
			lines = append(lines, cur.String())
			cur.Reset()
			curWidth = 0
		}
		if curWidth > 0 {
			cur.WriteString(sep)
			curWidth += len(sep)
		}
		cur.WriteString(text)
		curWidth += w
	}
	if curWidth > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}
