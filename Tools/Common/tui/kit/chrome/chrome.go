// Package chrome renders screen chrome: badges, section headers, tab bars and help overlays.
package chrome

import (
	"strings"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

// rule returns the horizontal rule glyph for the current glyph mode.
func rule() string {
	if tui.CurrentRenderMode().Glyphs == tui.GlyphASCII {
		return "-"
	}
	return "─"
}

// Badge renders "[text]" in role, never wider than width.
func Badge(text string, role tui.StyleRole, width int, theme tui.Theme) string {
	if width < 3 {
		return ""
	}
	label := "[" + text + "]"
	if textwidth.Of(label) > width {
		label = "[" + textwidth.Truncate(text, width-2) + "]"
	}
	return theme.Style(role).Render(label)
}

// SectionHeader renders one line: title followed by a rule to width.
func SectionHeader(title string, width int, theme tui.Theme) string {
	if width <= 0 {
		return ""
	}
	if title == "" {
		return theme.Style(tui.RoleBorder).Render(strings.Repeat(rule(), width))
	}
	shown := textwidth.Truncate(title, width)
	left := width - textwidth.Of(shown)
	styled := theme.Style(tui.RoleTitle).Render(shown)
	if left <= 1 {
		return styled + strings.Repeat(" ", left)
	}
	return styled + " " + theme.Style(tui.RoleBorder).Render(strings.Repeat(rule(), left-1))
}
