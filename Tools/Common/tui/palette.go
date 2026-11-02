package tui

import "github.com/charmbracelet/lipgloss"

// Signal and data roles of the shared palette. Roles are named for kinds of value; mapping
// domain facts to roles is the job of each tool.
const (
	// signal roles
	RoleSignalGood    StyleRole = "signal-good"    // green
	RoleSignalWarning StyleRole = "signal-warning" // orange
	RoleSignalBad     StyleRole = "signal-bad"     // red
	RoleSignalMuted   StyleRole = "signal-muted"   // grey

	// data roles (kinds of value; one colour each, easily told apart)
	RoleDataCost   StyleRole = "data-cost"
	RoleDataTokens StyleRole = "data-tokens"
	RoleDataTime   StyleRole = "data-time"
	RoleDataActor  StyleRole = "data-actor" // every actor name
	RoleDataLead   StyleRole = "data-lead"  // the coordinating actor
	RoleDataTool   StyleRole = "data-tool"
	RoleDataModel  StyleRole = "data-model"

	// quiet background roles for flat shades (the style sets a background, not a foreground)
	RoleShadeNeutral StyleRole = "shade-neutral"
	RoleShadeCool    StyleRole = "shade-cool"
)

// paletteColours maps each new role to a colour chosen for dark backgrounds.
var paletteColours = map[StyleRole]string{
	RoleSignalGood:    "#2ecc71",
	RoleSignalWarning: "#ff8c1a",
	RoleSignalBad:     "#ff4d4d",
	RoleSignalMuted:   "#8a8a8a",

	RoleDataCost:   "#e5c07b",
	RoleDataTokens: "#61afef",
	RoleDataTime:   "#b48ead",
	RoleDataActor:  "#ff79c6",
	RoleDataLead:   "#56d4dd",
	RoleDataTool:   "#98c379",
	RoleDataModel:  "#d19a66",
}

// paletteBackgrounds maps each background role to a quiet colour for dark backgrounds.
var paletteBackgrounds = map[StyleRole]string{
	RoleShadeNeutral: "#3a3a3a",
	RoleShadeCool:    "#26384f",
}

// PaletteTheme returns DefaultTheme()'s roles with unchanged values plus every role above.
// DefaultTheme() itself is unchanged.
func PaletteTheme() Theme {
	theme := DefaultTheme()
	for role, code := range paletteColours {
		theme.Styles[role] = lipgloss.NewStyle().Foreground(lipgloss.Color(code))
	}
	for role, code := range paletteBackgrounds {
		theme.Styles[role] = lipgloss.NewStyle().Background(lipgloss.Color(code))
	}
	return theme
}
