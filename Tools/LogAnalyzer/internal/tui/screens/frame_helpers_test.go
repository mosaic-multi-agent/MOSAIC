package screens_test

// Shared helpers for the frame, help overlay and role tests: content checks that
// ignore styling and spacing, and the list of per-run views with their tab text.

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// stripStyling removes terminal escape sequences from a rendered view.
func stripStyling(s string) string { return ansiRE.ReplaceAllString(s, "") }

// colourOff switches the shared render mode to colour off (Unicode glyphs, as the
// markers in the views use) for the duration of the test.
func colourOff(t *testing.T) {
	t.Helper()
	restore := tuicommon.SetRenderMode(tuicommon.RenderMode{Color: false, Glyphs: tuicommon.GlyphUnicode})
	t.Cleanup(restore)
}

// paletteStyles are the styles of the real application: the palette theme included.
func paletteStyles() screens.Styles { return screens.Styles{Theme: tuicommon.PaletteTheme()} }

// viewLines splits a rendered view into styling-free lines; an empty view has none.
func viewLines(view string) []string {
	if view == "" {
		return nil
	}
	return strings.Split(stripStyling(view), "\n")
}

// framedLines returns the styling-free lines of a framed view and stops the test
// when the view is too short to hold the frame's chrome.
func framedLines(t *testing.T, view string) []string {
	t.Helper()
	lines := viewLines(view)
	if len(lines) < screens.FrameChromeLines {
		t.Fatalf("framed view has %d lines, want at least %d\n%s", len(lines), screens.FrameChromeLines, view)
	}
	return lines
}

// assertWithinBounds fails when the view is taller or wider than the terminal.
func assertWithinBounds(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := viewLines(view)
	if len(lines) > height {
		t.Errorf("view has %d lines on a %d-line terminal\n%s", len(lines), height, view)
	}
	for i, l := range lines {
		if w := textwidth.Of(l); w > width {
			t.Errorf("line %d is %d cells wide on a %d-column terminal: %q", i+1, w, width, l)
		}
	}
}

// overlayLines are the lines between the header and tab bar at the top and the
// help line at the bottom of a framed view.
func overlayLines(view string) []string {
	lines := viewLines(view)
	if len(lines) < 3 {
		return nil
	}
	return lines[2 : len(lines)-1]
}

// openOverlay presses the help key and renders the screen at the given size.
func openOverlay(s screens.Screen, width, height int) string {
	return press(s, screens.KeyHelp).View(width, height)
}

// frameView is a per-run view with the tab text that marks it as current.
type frameView struct {
	id       screens.ScreenID
	key      string
	tab      string // "key label" as shown in the tab bar
	otherKey string // a view key that requests a different view
	run      domain.RunReport
	make     func(run domain.RunReport, w, h int, st screens.Styles) screens.RunView
}

func allFrameViews() []frameView {
	return []frameView{
		{screens.ScreenRunDetail, screens.KeyViewDispatches, "1 Dispatches", screens.KeyViewOverview, overviewRun(),
			func(r domain.RunReport, w, h int, st screens.Styles) screens.RunView {
				return screens.NewRunDetailScreen(r, w, h, st)
			}},
		{screens.ScreenOverview, screens.KeyViewOverview, "2 Overview", screens.KeyViewDispatches, manyModelsRun(40),
			func(r domain.RunReport, w, h int, st screens.Styles) screens.RunView {
				return screens.NewOverviewScreen(r, w, h, st)
			}},
		{screens.ScreenAgentTypes, screens.KeyViewAgentTypes, "3 By type", screens.KeyViewDispatches, agentTypesRun(),
			func(r domain.RunReport, w, h int, st screens.Styles) screens.RunView {
				return screens.NewAgentTypesScreen(r, w, h, st)
			}},
		{screens.ScreenStatus, screens.KeyViewStatus, "4 By status", screens.KeyViewDispatches, statusRun(),
			func(r domain.RunReport, w, h int, st screens.Styles) screens.RunView {
				return screens.NewStatusScreen(r, w, h, st)
			}},
		{screens.ScreenTools, screens.KeyViewTools, "5 Tools", screens.KeyViewDispatches, toolsRun(),
			func(r domain.RunReport, w, h int, st screens.Styles) screens.RunView {
				return screens.NewToolsScreen(r, w, h, st)
			}},
		{screens.ScreenTimeline, screens.KeyViewTimeline, "6 Timeline", screens.KeyViewDispatches, timelineRun(),
			func(r domain.RunReport, w, h int, st screens.Styles) screens.RunView {
				return screens.NewTimelineScreen(r, w, h, st)
			}},
	}
}

// qualitySetter is the capability every per-run view gains: showing the findings
// of its run in the header.
type qualitySetter interface{ SetRunQuality(screens.RunQuality) }

// framedRun is a run with a known total, three invocations and a 90 second span,
// so every header badge has a value. The invocation count is 3 whether it is read
// from the overview's subagent figures or from the timeline entries.
func framedRun() domain.RunReport {
	run := overviewRun()
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	run.Timeline = domain.Timeline{
		Axis:    domain.TimeInterval{Start: start, End: start.Add(90 * time.Second)},
		HasAxis: true,
	}
	for i := 0; i < 3; i++ {
		run.Timeline.Entries = append(run.Timeline.Entries, domain.TimelineEntry{})
	}
	return run
}

const framedRunSpan = 90 * time.Second

// assertOverlayListsViewKeys opens the help overlay of a per-run view and checks
// that it lists every given view key and the way back, one entry per line.
func assertOverlayListsViewKeys(t *testing.T, s screens.Screen, keys ...string) {
	t.Helper()
	overlay := strings.Join(overlayLines(openOverlay(s, wideW, legendOverlayH)), "\n")
	for _, k := range append(keys, "esc") {
		if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(k) + `\s+\S`).MatchString(overlay) {
			t.Errorf("help overlay must list key %q\n%s", k, overlay)
		}
	}
}
