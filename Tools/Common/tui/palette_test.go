package tui_test

// palette_test.go specifies the shared palette: every signal and data role resolves to a
// coloured style, the existing roles keep their values, signal and data roles are easy to tell
// apart, and with colour off a rendered role is plain text.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	tuicommon "mosaic-common/tui"
)

var signalRoles = []tuicommon.StyleRole{
	tuicommon.RoleSignalGood, tuicommon.RoleSignalWarning,
	tuicommon.RoleSignalBad, tuicommon.RoleSignalMuted,
}

var dataRoles = []tuicommon.StyleRole{
	tuicommon.RoleDataCost, tuicommon.RoleDataTokens, tuicommon.RoleDataTime,
	tuicommon.RoleDataActor, tuicommon.RoleDataLead, tuicommon.RoleDataTool,
	tuicommon.RoleDataModel,
}

var existingRoles = []tuicommon.StyleRole{
	tuicommon.RoleTitle, tuicommon.RoleSubtitle, tuicommon.RoleBody, tuicommon.RoleMuted,
	tuicommon.RoleSelected, tuicommon.RoleChecked, tuicommon.RoleSuccess, tuicommon.RoleWarning,
	tuicommon.RoleError, tuicommon.RoleHelp, tuicommon.RoleBorder,
}

// forceTrueColor makes lipgloss emit colour escapes for the duration of the test, so that a
// plain result can only come from the render mode.
func forceTrueColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func newPaletteStyle(t *testing.T, role tuicommon.StyleRole) lipgloss.Style {
	t.Helper()
	style, ok := tuicommon.PaletteTheme().Styles[role]
	if !ok {
		t.Fatalf("PaletteTheme has no style for role %q", role)
	}
	return style
}

func colourKey(c lipgloss.TerminalColor) string {
	r, g, b, a := c.RGBA()
	return fmt.Sprintf("%d,%d,%d,%d", r, g, b, a)
}

func allNewRoles() []tuicommon.StyleRole {
	return append(append([]tuicommon.StyleRole{}, signalRoles...), dataRoles...)
}

func TestPaletteTheme_NewRoles_ResolveToColouredStyle(t *testing.T) {
	for _, role := range allNewRoles() {
		t.Run(string(role), func(t *testing.T) {
			style := newPaletteStyle(t, role)
			if _, none := style.GetForeground().(lipgloss.NoColor); none {
				t.Errorf("role %q has no foreground colour", role)
			}
		})
	}
}

func TestPaletteTheme_SignalRoles_AreDistinct(t *testing.T) {
	forceTrueColor(t)
	seen := map[string]tuicommon.StyleRole{}
	for _, role := range signalRoles {
		key := colourKey(newPaletteStyle(t, role).GetForeground())
		if other, dup := seen[key]; dup {
			t.Errorf("signal roles %q and %q share colour %s", role, other, key)
		}
		seen[key] = role
	}
}

// rgb8 returns the 8-bit channels of a foreground colour.
func rgb8(c lipgloss.TerminalColor) (int, int, int) {
	r, g, b, _ := c.RGBA()
	return int(r >> 8), int(g >> 8), int(b >> 8)
}

func TestPaletteTheme_SignalRoles_HaveExpectedHues(t *testing.T) {
	forceTrueColor(t)
	fg := func(role tuicommon.StyleRole) (int, int, int) {
		return rgb8(newPaletteStyle(t, role).GetForeground())
	}

	gr, gg, gb := fg(tuicommon.RoleSignalGood)
	if gg <= gr || gg <= gb {
		t.Errorf("good should be green-dominant, got rgb(%d,%d,%d)", gr, gg, gb)
	}
	br, bg, bb := fg(tuicommon.RoleSignalBad)
	if br <= bg+60 || br <= bb+60 {
		t.Errorf("bad should be red-dominant, got rgb(%d,%d,%d)", br, bg, bb)
	}
	wr, wg, wb := fg(tuicommon.RoleSignalWarning)
	if wr <= wb+60 || wg <= wb || wg >= wr {
		t.Errorf("warning should be orange (R high, G mid, B low), got rgb(%d,%d,%d)", wr, wg, wb)
	}
	mr, mg, mb := fg(tuicommon.RoleSignalMuted)
	if abs(mr-mg) > 30 || abs(mg-mb) > 30 {
		t.Errorf("muted should be grey (R~G~B), got rgb(%d,%d,%d)", mr, mg, mb)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestPaletteTheme_DataRoles_AreDistinct(t *testing.T) {
	forceTrueColor(t)
	seen := map[string]tuicommon.StyleRole{}
	for _, role := range dataRoles {
		key := colourKey(newPaletteStyle(t, role).GetForeground())
		if other, dup := seen[key]; dup {
			t.Errorf("data roles %q and %q share colour %s", role, other, key)
		}
		seen[key] = role
	}
}

func TestPaletteTheme_ExistingRoles_KeepDefaultThemeValues(t *testing.T) {
	forceTrueColor(t)
	def := tuicommon.DefaultTheme()
	pal := tuicommon.PaletteTheme()
	for _, role := range existingRoles {
		t.Run(string(role), func(t *testing.T) {
			want := def.Styles[role]
			got, ok := pal.Styles[role]
			if !ok {
				t.Fatalf("PaletteTheme lost role %q", role)
			}
			if colourKey(got.GetForeground()) != colourKey(want.GetForeground()) {
				t.Errorf("foreground of %q changed", role)
			}
			if got.GetBold() != want.GetBold() {
				t.Errorf("bold of %q changed", role)
			}
		})
	}
}

// Regression coverage: DefaultTheme keeps exactly its 11 roles and values.
func TestDefaultTheme_Unchanged(t *testing.T) {
	forceTrueColor(t)
	def := tuicommon.DefaultTheme()
	if len(def.Styles) != len(existingRoles) {
		t.Fatalf("DefaultTheme has %d roles, want %d", len(def.Styles), len(existingRoles))
	}
	wantFg := map[tuicommon.StyleRole]string{
		tuicommon.RoleTitle: "12", tuicommon.RoleSubtitle: "14", tuicommon.RoleBody: "15",
		tuicommon.RoleMuted: "8", tuicommon.RoleSelected: "10", tuicommon.RoleChecked: "10",
		tuicommon.RoleSuccess: "10", tuicommon.RoleWarning: "11", tuicommon.RoleError: "9",
		tuicommon.RoleHelp: "14", tuicommon.RoleBorder: "8",
	}
	for role, code := range wantFg {
		got := def.Styles[role].GetForeground()
		if colourKey(got) != colourKey(lipgloss.Color(code)) {
			t.Errorf("DefaultTheme foreground of %q changed", role)
		}
	}
}

func TestPaletteTheme_ColourOff_RendersPlainText(t *testing.T) {
	forceTrueColor(t)
	theme := tuicommon.PaletteTheme()
	probe := theme.Style(tuicommon.RoleDataCost).Render("x")
	if !strings.Contains(probe, "\x1b[") {
		t.Fatalf("precondition: forced colour profile should emit escapes, got %q", probe)
	}
	restore := tuicommon.SetRenderMode(tuicommon.RenderMode{Color: false, Glyphs: tuicommon.GlyphUnicode})
	defer restore()

	all := append(allNewRoles(), existingRoles...)
	for _, role := range all {
		if got := theme.Style(role).Render("text"); got != "text" {
			t.Errorf("role %q with colour off rendered %q, want plain text", role, got)
		}
	}
}

func TestPaletteTheme_ColourRestoredAfterRestore(t *testing.T) {
	forceTrueColor(t)
	theme := tuicommon.PaletteTheme()
	restore := tuicommon.SetRenderMode(tuicommon.RenderMode{Color: false})
	restore()

	if got := theme.Style(tuicommon.RoleSignalBad).Render("x"); !strings.Contains(got, "\x1b[") {
		t.Errorf("colour not restored after restore(); got %q", got)
	}
}
