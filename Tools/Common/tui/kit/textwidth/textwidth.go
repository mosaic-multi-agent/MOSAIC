// Package textwidth measures and fits text by terminal display cells.
package textwidth

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"mosaic-common/tui"
)

// Of returns the number of terminal display cells s occupies.
func Of(s string) int { return ansi.StringWidth(s) }

// ellipsis returns the truncation marker for the current glyph mode.
func ellipsis() string {
	if tui.CurrentRenderMode().Glyphs == tui.GlyphASCII {
		return "..."
	}
	return "…"
}

// Truncate returns s cut to fit width cells, ending with the ellipsis when cut.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if Of(s) <= width {
		return s
	}
	ell := ellipsis()
	ellWidth := Of(ell)
	if width < ellWidth {
		return strings.Repeat(".", width)
	}
	return ansi.Truncate(s, width-ellWidth, "") + ell
}

// Pad truncates s to width cells and pads it with spaces to exactly width cells.
func Pad(s string, width int, alignRight bool) string {
	if width <= 0 {
		return ""
	}
	t := Truncate(s, width)
	gap := strings.Repeat(" ", width-Of(t))
	if alignRight {
		return gap + t
	}
	return t + gap
}
