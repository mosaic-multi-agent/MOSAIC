package tui

import (
	"github.com/charmbracelet/lipgloss"

	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/tui/screens"
)

// stylesFromTheme converts a tuicommon.Theme to a screens.Styles. The theme
// handed to screens also carries the palette's signal and data roles, whichever
// theme the model runs with, so every screen draws from the same set of roles.
func stylesFromTheme(t tuicommon.Theme) screens.Styles {
	return screens.Styles{
		Title:    t.Style(tuicommon.RoleTitle),
		Subtitle: t.Style(tuicommon.RoleSubtitle),
		Body:     t.Style(tuicommon.RoleBody),
		Muted:    t.Style(tuicommon.RoleMuted),
		Selected: t.Style(tuicommon.RoleSelected),
		Checked:  t.Style(tuicommon.RoleChecked),
		Success:  t.Style(tuicommon.RoleSuccess),
		Warning:  t.Style(tuicommon.RoleWarning),
		Error:    t.Style(tuicommon.RoleError),
		Help:     t.Style(tuicommon.RoleHelp),
		Border:   t.Style(tuicommon.RoleBorder),
		Theme:    withPaletteRoles(t),
	}
}

// withPaletteRoles returns the theme with every palette role the theme does not
// define itself added; the given theme is not modified.
func withPaletteRoles(t tuicommon.Theme) tuicommon.Theme {
	palette := tuicommon.PaletteTheme()
	styles := make(map[tuicommon.StyleRole]lipgloss.Style, len(palette.Styles)+len(t.Styles))
	for role, style := range palette.Styles {
		styles[role] = style
	}
	for role, style := range t.Styles {
		styles[role] = style
	}
	return tuicommon.Theme{Styles: styles}
}

// screenStyles is the screen styles of the model's theme, with the effective
// currency of its report so every screen shows money in it.
func (m Model) screenStyles() screens.Styles {
	st := stylesFromTheme(m.theme)
	st.Currency = m.report.Currency
	return st
}
