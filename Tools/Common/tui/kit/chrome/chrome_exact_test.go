package chrome_test

// chrome_exact_test.go pins the exact text of the section header, the help overlay and the
// badge (colour off), including the scroll boundaries, and holds their golden renders.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
)

// trimmedLines splits a render and drops the trailing padding of every line.
func trimmedLines(out string) []string {
	lines := splitLines(out)
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

func TestSectionHeader_ExactLayoutAscii(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	cases := []struct {
		name  string
		title string
		width int
		want  string
	}{
		{"title then rule", "Sizes", 30, "Sizes " + strings.Repeat("-", 24)},
		{"rule of one cell", "Sizes", 7, "Sizes -"},
		{"empty title is all rule", "", 5, "-----"},
		{"no room for a rule pads the title", "Sizes", 6, "Sizes "},
		{"title cut when wider than width", "Sizes", 4, "S..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := chrome.SectionHeader(tc.title, tc.width, theme()); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSectionHeader_ExactLayoutUnicode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)

	got := chrome.SectionHeader("Sizes", 30, theme())

	if want := "Sizes " + strings.Repeat("─", 24); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSectionHeader_Golden(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode tui.GlyphMode
	}{{"header_ascii", tui.GlyphASCII}, {"header_unicode", tui.GlyphUnicode}} {
		t.Run(tc.name, func(t *testing.T) {
			useMode(t, tc.mode)
			assertGolden(t, tc.name, chrome.SectionHeader("Sizes", 30, theme()))
		})
	}
}

func TestBadge_Golden(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	assertGolden(t, "badge_ok", chrome.Badge("ok", tui.RoleSignalGood, 20, theme()))
}

func TestHelpOverlay_ExactLayoutWhenItFits(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := trimmedLines(chrome.HelpOverlay("Help", smallGroups(), 0, 60, 30, theme()))

	want := []string{
		"Help", "", "  Views",
		"    k00  does thing 00", "    k01  does thing 01", "    k02  does thing 02",
		"", "  Export",
		"    k03  does thing 03", "    k04  does thing 04",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHelpOverlay_EveryLineIsPaddedToExactlyTheWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	lines := splitLines(chrome.HelpOverlay("Help", smallGroups(), 0, 60, 30, theme()))
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(lines))
	}
	for _, line := range lines {
		if len(line) != 60 {
			t.Errorf("line %q is %d cells, want exactly 60", line, len(line))
		}
	}
}

func TestHelpOverlay_DescriptionsAlignAcrossGroupsAndAnUntitledGroupHasNoTitleLine(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	groups := []chrome.HelpGroup{{Entries: []tui.HelpEntry{
		{Key: "q", Desc: "quit"}, {Key: "ctrl+c", Desc: "force quit"},
	}}}

	got := trimmedLines(chrome.HelpOverlay("Keys", groups, 0, 40, 10, theme()))

	want := []string{"Keys", "", "    q       quit", "    ctrl+c  force quit"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHelpOverlay_CutShowsTheMoreLineWithLineNumbers(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := trimmedLines(chrome.HelpOverlay("Help", bigGroups(), 0, 60, 8, theme()))

	want := []string{
		"Help", "", "  Views",
		"    k00  does thing 00", "    k01  does thing 01", "    k02  does thing 02", "    k03  does thing 03",
		"-- more: 1-7 of 25 --",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHelpOverlay_BottomOfScrollStillShowsTheMoreLine(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := trimmedLines(chrome.HelpOverlay("Help", bigGroups(), 1000, 60, 8, theme()))

	if len(got) != 8 || got[0] != "    k13  does thing 13" || got[6] != "    k19  does thing 19" ||
		got[7] != "-- more: 19-25 of 25 --" {
		t.Errorf("got:\n%s", strings.Join(got, "\n"))
	}
}

func TestHelpOverlay_MoreLineBoundaryAtContentHeight(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	exact := splitLines(chrome.HelpOverlay("Help", smallGroups(), 0, 60, 10, theme()))
	oneShort := trimmedLines(chrome.HelpOverlay("Help", smallGroups(), 0, 60, 9, theme()))

	if len(exact) != 10 || strings.Contains(strings.Join(exact, "\n"), "more") {
		t.Errorf("content of 10 lines at height 10: want all 10 lines and no more line, got:\n%s", strings.Join(exact, "\n"))
	}
	if len(oneShort) != 9 || oneShort[8] != "-- more: 1-8 of 10 --" {
		t.Errorf("content of 10 lines at height 9: got:\n%s", strings.Join(oneShort, "\n"))
	}
}

func TestHelpOverlay_FittingContentIgnoresOffset(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	a := chrome.HelpOverlay("Help", smallGroups(), 7, 60, 30, theme())
	b := chrome.HelpOverlay("Help", smallGroups(), 0, 60, 30, theme())
	if a == "" || a != b {
		t.Errorf("offset must be ignored when everything fits:\n%s\n---\n%s", a, b)
	}
}

func TestHelpOverlay_HeightOneShowsOnlyTheLineCount(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := trimmedLines(chrome.HelpOverlay("Help", bigGroups(), 0, 60, 1, theme()))

	if len(got) != 1 || got[0] != "-- more: 25 lines --" {
		t.Errorf("got %q, want the single line \"-- more: 25 lines --\"", got)
	}
}

func TestHelpOverlay_NonPositiveSizeGivesEmpty(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, sz := range [][2]int{{0, 10}, {10, 0}, {-1, 5}} {
		if got := chrome.HelpOverlay("Help", smallGroups(), 0, sz[0], sz[1], theme()); got != "" {
			t.Errorf("width %d height %d: got %q, want empty", sz[0], sz[1], got)
		}
	}
}

func TestHelpOverlay_Golden(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	t.Run("overlay_fits", func(t *testing.T) {
		assertGolden(t, "overlay_fits", chrome.HelpOverlay("Help", smallGroups(), 0, 60, 30, theme()))
	})
	t.Run("overlay_cut", func(t *testing.T) {
		assertGolden(t, "overlay_cut", chrome.HelpOverlay("Help", bigGroups(), 0, 60, 8, theme()))
	})
	t.Run("overlay_cut_scrolled", func(t *testing.T) {
		assertGolden(t, "overlay_cut_scrolled", chrome.HelpOverlay("Help", bigGroups(), 1000, 60, 8, theme()))
	})
}
