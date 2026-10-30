package screens_test

// Footer fitting: a key-hint footer that does not fit drops whole entries, the
// rightmost first (an `e` export entry last), leaves an ellipsis before the help
// entry, never cuts an entry mid-word and always keeps "? help". Frames and pages
// fit their footers the same way.

import (
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/tui/screens"
)

const (
	footerEllipsis = "…"
	footerHelp     = "? help"
)

// footerLine is the last line of a rendered view, styling-free and right-trimmed.
func footerLine(view string) string {
	lines := viewLines(view)
	return strings.TrimRight(lines[len(lines)-1], " ")
}

// footerEntries splits a footer into its entries (joined by two spaces).
func footerEntries(line string) []string { return strings.Split(line, "  ") }

func asciiGlyphs(t *testing.T) {
	t.Helper()
	restore := tuicommon.SetRenderMode(tuicommon.RenderMode{Color: false, Glyphs: tuicommon.GlyphASCII})
	t.Cleanup(restore)
}

func keyEntries(pairs ...string) []tuicommon.HelpEntry {
	out := make([]tuicommon.HelpEntry, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, tuicommon.HelpEntry{Key: pairs[i], Desc: pairs[i+1]})
	}
	return out
}

// footerRenderers render the footer of a frame and of a page with the same
// content; both must fit it identically.
type footerRenderer struct {
	name   string
	render func(t *testing.T, keys []tuicommon.HelpEntry, hint string, width int) string
}

func footerRenderers() []footerRenderer {
	return []footerRenderer{
		{"frame", func(t *testing.T, keys []tuicommon.HelpEntry, hint string, width int) string {
			colourOff(t)
			f := screens.Frame{Run: framedRun(), Active: screens.ScreenTools, Keys: keys, Hint: hint}
			return footerLine(screens.RenderFrame(f, nil, width, screens.FrameChromeLines, paletteStyles()))
		}},
		{"page", func(t *testing.T, keys []tuicommon.HelpEntry, hint string, width int) string {
			colourOff(t)
			p := screens.Page{Title: "T", Keys: keys, Hint: hint, Help: true}
			return footerLine(screens.RenderPage(p, nil, width, screens.PageChromeLines, paletteStyles()))
		}},
	}
}

func TestFooterFit_FittingLineIsShownUnchanged(t *testing.T) {
	keys := keyEntries("a", "one", "b", "two", "c", "three", "d", "four")
	for _, r := range footerRenderers() {
		t.Run(r.name, func(t *testing.T) {
			for _, width := range []int{37, 38, 60} {
				if got := r.render(t, keys, "", width); got != "a one  b two  c three  d four  ? help" {
					t.Errorf("width %d: footer = %q, want the whole line", width, got)
				}
			}
		})
	}
}

func TestFooterFit_DropsWholeEntriesRightmostFirstWithAnEllipsisBeforeHelp(t *testing.T) {
	keys := keyEntries("a", "one", "b", "two", "c", "three", "d", "four")
	cases := []struct {
		width int
		want  string
	}{
		{36, "a one  b two  c three  …  ? help"},
		{32, "a one  b two  c three  …  ? help"},
		{31, "a one  b two  …  ? help"},
		{23, "a one  b two  …  ? help"},
		{22, "a one  …  ? help"},
		{16, "a one  …  ? help"},
		{15, "…  ? help"},
		{9, "…  ? help"},
	}
	for _, r := range footerRenderers() {
		t.Run(r.name, func(t *testing.T) {
			for _, c := range cases {
				if got := r.render(t, keys, "", c.width); got != c.want {
					t.Errorf("width %d: footer = %q, want %q", c.width, got, c.want)
				}
			}
		})
	}
}

func TestFooterFit_ExportEntryIsDroppedAfterEveryOtherEntry(t *testing.T) {
	keys := keyEntries("a", "one", "e", "export", "b", "two", "c", "three")
	cases := []struct {
		width int
		want  string
	}{
		{39, "a one  e export  b two  c three  ? help"},
		{38, "a one  e export  b two  …  ? help"},
		{33, "a one  e export  b two  …  ? help"},
		{32, "a one  e export  …  ? help"},
		{26, "a one  e export  …  ? help"},
		{25, "e export  …  ? help"},
		{19, "e export  …  ? help"},
		{18, "…  ? help"},
		{9, "…  ? help"},
	}
	for _, r := range footerRenderers() {
		t.Run(r.name, func(t *testing.T) {
			for _, c := range cases {
				if got := r.render(t, keys, "", c.width); got != c.want {
					t.Errorf("width %d: footer = %q, want %q", c.width, got, c.want)
				}
			}
		})
	}
}

func TestFooterFit_ScrollHintIsDroppedLikeAnyOtherEntryButBeforeExport(t *testing.T) {
	for _, r := range footerRenderers() {
		t.Run(r.name, func(t *testing.T) {
			if got := r.render(t, keyEntries("a", "one"), "5-9 of 30", 23); got != "5-9 of 30  …  ? help" {
				t.Errorf("the key goes before the scroll hint: footer = %q", got)
			}
			if got := r.render(t, keyEntries("e", "export"), "5-9 of 30", 26); got != "e export  …  ? help" {
				t.Errorf("the scroll hint goes before the export entry: footer = %q", got)
			}
		})
	}
}

func TestFooterFit_NeverCutsAnEntryAndKeepsHelpAtEveryWidth(t *testing.T) {
	keys := keyEntries("a", "one", "e", "export", "b", "two", "c", "three")
	whole := map[string]bool{"a one": true, "e export": true, "b two": true, "c three": true,
		footerEllipsis: true, footerHelp: true}
	for _, r := range footerRenderers() {
		t.Run(r.name, func(t *testing.T) {
			for width := 9; width <= 60; width++ {
				line := r.render(t, keys, "", width)
				entries := footerEntries(line)
				if textwidth.Of(line) > width {
					t.Errorf("width %d: footer %q is %d cells wide", width, line, textwidth.Of(line))
				}
				for _, e := range entries {
					if !whole[e] {
						t.Errorf("width %d: footer %q holds the cut entry %q", width, line, e)
					}
				}
				if entries[len(entries)-1] != footerHelp {
					t.Errorf("width %d: footer %q must end with %q", width, line, footerHelp)
				}
				if width >= 19 && !strings.Contains(line, "e export") {
					t.Errorf("width %d: footer %q must keep the export entry", width, line)
				}
			}
		})
	}
}

func TestFooterFit_HelpEntryAloneSurvivesVeryNarrowWidths(t *testing.T) {
	keys := keyEntries("a", "one", "e", "export")
	for _, r := range footerRenderers() {
		t.Run(r.name, func(t *testing.T) {
			for width := 1; width <= 8; width++ {
				line := r.render(t, keys, "5-9 of 30", width)
				if textwidth.Of(line) > width {
					t.Errorf("width %d: footer %q is too wide", width, line)
				}
				if width >= len(footerHelp) && !strings.Contains(line, footerHelp) {
					t.Errorf("width %d: footer %q must keep %q", width, line, footerHelp)
				}
				if strings.Contains(line, "one") || strings.Contains(line, "5-9") {
					t.Errorf("width %d: footer %q must have dropped the other entries", width, line)
				}
			}
		})
	}
}

func TestFooterFit_FrameAndPageFootersAreIdentical(t *testing.T) {
	keys := keyEntries("↑/↓", "scroll", "s", "sort", "e", "export", "esc", "back")
	rs := footerRenderers()
	for width := 1; width <= 70; width++ {
		frame, page := rs[0].render(t, keys, "5-9 of 30", width), rs[1].render(t, keys, "5-9 of 30", width)
		if frame != page {
			t.Errorf("width %d: frame footer %q differs from page footer %q", width, frame, page)
		}
	}
}

func TestFooterFit_AsciiGlyphModeUsesThreeDotsAsTheEllipsis(t *testing.T) {
	keys := keyEntries("a", "one", "b", "two", "c", "three", "d", "four")
	for _, r := range footerRenderers() {
		t.Run(r.name, func(t *testing.T) {
			asciiGlyphs(t)
			f := screens.Frame{Run: framedRun(), Active: screens.ScreenTools, Keys: keys}
			p := screens.Page{Title: "T", Keys: keys, Help: true}
			var got string
			if r.name == "frame" {
				got = footerLine(screens.RenderFrame(f, nil, 35, screens.FrameChromeLines, paletteStyles()))
			} else {
				got = footerLine(screens.RenderPage(p, nil, 35, screens.PageChromeLines, paletteStyles()))
			}
			if want := "a one  b two  c three  ...  ? help"; got != want {
				t.Errorf("footer = %q, want %q", got, want)
			}
		})
	}
}

func TestFooterFit_PageWithoutHelpEntryEndsWithTheEllipsis(t *testing.T) {
	colourOff(t)
	keys := keyEntries("a", "one", "b", "two", "c", "three", "d", "four")
	render := func(width int) string {
		p := screens.Page{Title: "T", Keys: keys}
		return footerLine(screens.RenderPage(p, nil, width, screens.PageChromeLines, paletteStyles()))
	}

	if got := render(29); got != "a one  b two  c three  d four" {
		t.Errorf("a fitting footer is unchanged: %q", got)
	}
	if got := render(28); got != "a one  b two  c three  …" {
		t.Errorf("width 28: footer = %q, want the last entry dropped and an ellipsis at the end", got)
	}
	if got := render(23); got != "a one  b two  …" {
		t.Errorf("width 23: footer = %q, want two entries dropped", got)
	}
}
