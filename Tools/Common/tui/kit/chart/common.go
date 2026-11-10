package chart

import (
	"math"
	"strings"

	"mosaic-common/tui"
)

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

func ascii() bool { return tui.CurrentRenderMode().Glyphs == tui.GlyphASCII }

// roundHalfUp rounds x to the nearest integer, halves going up.
func roundHalfUp(x float64) int { return int(math.Floor(x + 0.5)) }

func spaces(n int) string { return strings.Repeat(" ", max(n, 0)) }
