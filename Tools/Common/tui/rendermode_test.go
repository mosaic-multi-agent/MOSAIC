package tui_test

// rendermode_test.go specifies colour and glyph mode handling: NO_COLOR, the glyph override,
// platform detection, and the process-wide mode that tests can force and restore.

import (
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"
)

type detectCase struct {
	name string
	vars map[string]string
	goos string
	want tuicommon.RenderMode
}

func envOf(vars map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	}
}

func runDetectCases(t *testing.T, cases []detectCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tuicommon.DetectRenderMode(envOf(tc.vars), tc.goos); got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func mode(color bool, g tuicommon.GlyphMode) tuicommon.RenderMode {
	return tuicommon.RenderMode{Color: color, Glyphs: g}
}

func TestDetectRenderMode_NoColor(t *testing.T) {
	runDetectCases(t, []detectCase{
		{"unset", map[string]string{}, "linux", mode(true, tuicommon.GlyphUnicode)},
		{"non-empty", map[string]string{"NO_COLOR": "1"}, "linux", mode(false, tuicommon.GlyphUnicode)},
		{"any non-empty value", map[string]string{"NO_COLOR": "false"}, "linux", mode(false, tuicommon.GlyphUnicode)},
		{"set but empty", map[string]string{"NO_COLOR": ""}, "linux", mode(true, tuicommon.GlyphUnicode)},
	})
}

func TestDetectRenderMode_GlyphOverride(t *testing.T) {
	runDetectCases(t, []detectCase{
		{"ascii on linux", map[string]string{tuicommon.GlyphEnvVar: "ascii"}, "linux", mode(true, tuicommon.GlyphASCII)},
		{"ASCII uppercase", map[string]string{tuicommon.GlyphEnvVar: "ASCII"}, "linux", mode(true, tuicommon.GlyphASCII)},
		{"unicode beats windows detection", map[string]string{tuicommon.GlyphEnvVar: "unicode"}, "windows", mode(true, tuicommon.GlyphUnicode)},
		{"Unicode mixed case", map[string]string{tuicommon.GlyphEnvVar: "Unicode"}, "windows", mode(true, tuicommon.GlyphUnicode)},
		{"ascii beats modern windows terminal", map[string]string{tuicommon.GlyphEnvVar: "ascii", "WT_SESSION": "x"}, "windows", mode(true, tuicommon.GlyphASCII)},
	})
}

func TestDetectRenderMode_GlyphDetectionWithoutOverride(t *testing.T) {
	runDetectCases(t, []detectCase{
		{"legacy windows console", map[string]string{}, "windows", mode(true, tuicommon.GlyphASCII)},
		{"windows terminal", map[string]string{"WT_SESSION": "abc"}, "windows", mode(true, tuicommon.GlyphUnicode)},
		{"windows with TERM_PROGRAM", map[string]string{"TERM_PROGRAM": "vscode"}, "windows", mode(true, tuicommon.GlyphUnicode)},
		{"linux", map[string]string{}, "linux", mode(true, tuicommon.GlyphUnicode)},
		{"darwin", map[string]string{}, "darwin", mode(true, tuicommon.GlyphUnicode)},
		{"invalid override falls back (windows)", map[string]string{tuicommon.GlyphEnvVar: "bogus"}, "windows", mode(true, tuicommon.GlyphASCII)},
		{"invalid override falls back (linux)", map[string]string{tuicommon.GlyphEnvVar: "bogus"}, "linux", mode(true, tuicommon.GlyphUnicode)},
		{"empty override falls back", map[string]string{tuicommon.GlyphEnvVar: ""}, "windows", mode(true, tuicommon.GlyphASCII)},
	})
}

func TestDetectRenderMode_ColourAndGlyphsAreIndependent(t *testing.T) {
	runDetectCases(t, []detectCase{
		{"no colour and ascii", map[string]string{"NO_COLOR": "1", tuicommon.GlyphEnvVar: "ascii"}, "linux", mode(false, tuicommon.GlyphASCII)},
		{"no colour on legacy windows", map[string]string{"NO_COLOR": "1"}, "windows", mode(false, tuicommon.GlyphASCII)},
	})
}

func TestDetectRenderModeFromProcess_ReadsRealEnvironment(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv(tuicommon.GlyphEnvVar, "ascii")

	got := tuicommon.DetectRenderModeFromProcess()

	if want := mode(false, tuicommon.GlyphASCII); got != want {
		t.Errorf("process mode = %+v, want %+v", got, want)
	}
}

func TestNoColourEnvironment_RendersWithoutEscapesWhenApplied(t *testing.T) {
	forceTrueColor(t)
	theme := tuicommon.PaletteTheme()
	if probe := theme.Style(tuicommon.RoleSignalBad).Render("x"); !strings.Contains(probe, "\x1b[") {
		t.Fatalf("precondition: forced colour profile should emit escapes, got %q", probe)
	}
	t.Setenv("NO_COLOR", "1")

	restore := tuicommon.SetRenderMode(tuicommon.DetectRenderModeFromProcess())
	defer restore()

	for _, role := range allNewRoles() {
		if got := theme.Style(role).Render("text"); got != "text" {
			t.Errorf("role %q with NO_COLOR set rendered %q, want plain text", role, got)
		}
	}
}

// Depends on package initialisation leaving the default mode untouched; if init ever applies
// environment detection, this test must set its own environment first.
func TestCurrentRenderMode_DefaultIsColourAndUnicode(t *testing.T) {
	got := tuicommon.CurrentRenderMode()
	want := mode(true, tuicommon.GlyphUnicode)
	if got != want {
		t.Errorf("default mode = %+v, want %+v", got, want)
	}
}

func TestSetRenderMode_ForcesEveryModeAndRestores(t *testing.T) {
	modes := []tuicommon.RenderMode{
		mode(false, tuicommon.GlyphASCII),
		mode(false, tuicommon.GlyphUnicode),
		mode(true, tuicommon.GlyphASCII),
		mode(true, tuicommon.GlyphUnicode),
	}
	before := tuicommon.CurrentRenderMode()
	for _, m := range modes {
		restore := tuicommon.SetRenderMode(m)
		if got := tuicommon.CurrentRenderMode(); got != m {
			t.Errorf("after Set(%+v) current = %+v", m, got)
		}
		restore()
		if got := tuicommon.CurrentRenderMode(); got != before {
			t.Errorf("after restore current = %+v, want %+v", got, before)
		}
	}
}

func TestSetRenderMode_NestedRestoreUnwindsInOrder(t *testing.T) {
	outer := mode(false, tuicommon.GlyphASCII)
	inner := mode(true, tuicommon.GlyphASCII)
	base := tuicommon.CurrentRenderMode()

	restoreOuter := tuicommon.SetRenderMode(outer)
	restoreInner := tuicommon.SetRenderMode(inner)
	restoreInner()
	if got := tuicommon.CurrentRenderMode(); got != outer {
		t.Errorf("after inner restore current = %+v, want %+v", got, outer)
	}
	restoreOuter()
	if got := tuicommon.CurrentRenderMode(); got != base {
		t.Errorf("after outer restore current = %+v, want %+v", got, base)
	}
}
