package tui

// Helpers for tests that look at the colour a message is drawn in, and at how far
// a rendered view reaches. Colour is forced on (true colour) only for tests that
// ask for it, and always restored.

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

// colourOn forces colour output for the duration of the test.
func colourOn(t *testing.T) {
	t.Helper()
	restore := tuicommon.SetRenderMode(tuicommon.RenderMode{Color: true, Glyphs: tuicommon.GlyphUnicode})
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(prev)
		restore()
	})
}

// colourOff switches the render mode to colour off for the duration of the test.
func colourOff(t *testing.T) {
	t.Helper()
	restore := tuicommon.SetRenderMode(tuicommon.RenderMode{Color: false, Glyphs: tuicommon.GlyphUnicode})
	t.Cleanup(restore)
}

// rolePrefix is the escape sequence the palette opens a role with. Colour must be
// on when it is called.
func rolePrefix(role tuicommon.StyleRole) string {
	s := tuicommon.PaletteTheme().Style(role).Render("X")
	return s[:strings.Index(s, "X")]
}

// rawLineWith returns the first line of the styled view whose styling-free text
// contains needle; the line keeps its escape sequences.
func rawLineWith(t *testing.T, view, needle string) string {
	t.Helper()
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(escapeRE.ReplaceAllString(l, ""), needle) {
			return l
		}
	}
	t.Fatalf("no line contains %q\n%s", needle, escapeRE.ReplaceAllString(view, ""))
	return ""
}

// assertDrawnInRole fails unless text on the raw line sits after the opening of role.
func assertDrawnInRole(t *testing.T, rawLine, text string, role tuicommon.StyleRole) {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(rolePrefix(role)) + `\[?\s*` + regexp.QuoteMeta(text))
	if !re.MatchString(rawLine) {
		t.Errorf("%q must be drawn in role %q: %q", text, role, rawLine)
	}
}

// assertViewWithin fails when the view is taller or wider than the terminal.
func assertViewWithin(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(escapeRE.ReplaceAllString(view, ""), "\n")
	if view == "" {
		lines = nil
	}
	if len(lines) > height {
		t.Errorf("view has %d lines on a %d-line terminal\n%s", len(lines), height, strings.Join(lines, "\n"))
	}
	for i, l := range lines {
		if w := textwidth.Of(l); w > width {
			t.Errorf("line %d is %d cells wide on a %d-column terminal: %q", i+1, w, width, l)
		}
	}
}
