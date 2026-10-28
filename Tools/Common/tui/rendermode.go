package tui

import (
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// GlyphMode selects between Unicode block glyphs and plain ASCII glyphs.
type GlyphMode uint8

const (
	GlyphUnicode GlyphMode = iota
	GlyphASCII
)

// RenderMode is the process-wide colour and glyph mode read by widgets.
type RenderMode struct {
	Color  bool
	Glyphs GlyphMode
}

// GlyphEnvVar overrides glyph detection: "ascii" or "unicode" (case-insensitive).
const GlyphEnvVar = "MOSAIC_TUI_GLYPHS"

var (
	modeMu      sync.Mutex
	currentMode = RenderMode{Color: true, Glyphs: GlyphUnicode}
)

// DetectRenderMode derives the render mode from the environment and operating system.
// A non-empty NO_COLOR turns colour off. A valid glyph override wins; otherwise a Windows
// console that is not a modern terminal gets ASCII glyphs.
func DetectRenderMode(lookupEnv func(string) (string, bool), goos string) RenderMode {
	get := func(key string) string {
		v, _ := lookupEnv(key)
		return v
	}
	m := RenderMode{Color: get("NO_COLOR") == "", Glyphs: GlyphUnicode}
	switch strings.ToLower(get(GlyphEnvVar)) {
	case "ascii":
		m.Glyphs = GlyphASCII
	case "unicode":
		m.Glyphs = GlyphUnicode
	default:
		if goos == "windows" && get("WT_SESSION") == "" && get("TERM_PROGRAM") == "" {
			m.Glyphs = GlyphASCII
		}
	}
	return m
}

// DetectRenderModeFromProcess runs DetectRenderMode against the real environment and OS.
func DetectRenderModeFromProcess() RenderMode {
	return DetectRenderMode(os.LookupEnv, runtime.GOOS)
}

// SetRenderMode makes m the process-wide mode; the returned func restores the previous one.
// Colour off also forces the lipgloss colour profile to plain output; restore puts the
// previous profile back.
func SetRenderMode(m RenderMode) (restore func()) {
	modeMu.Lock()
	defer modeMu.Unlock()
	prevMode := currentMode
	prevProfile := lipgloss.ColorProfile()
	currentMode = m
	if !m.Color {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	return func() {
		modeMu.Lock()
		defer modeMu.Unlock()
		currentMode = prevMode
		lipgloss.SetColorProfile(prevProfile)
	}
}

// CurrentRenderMode returns the process-wide mode (default: colour on, Unicode glyphs).
func CurrentRenderMode() RenderMode {
	modeMu.Lock()
	defer modeMu.Unlock()
	return currentMode
}
