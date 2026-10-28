package chrome_test

// helpoverlay_test.go specifies the help overlay: every entry listed when it fits, a "more"
// line and offset scrolling when it does not, and width/height bounds at every size.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
)

func numberedGroup(title string, from, to int) chrome.HelpGroup {
	g := chrome.HelpGroup{Title: title}
	for i := from; i < to; i++ {
		g.Entries = append(g.Entries, tui.HelpEntry{Key: fmt.Sprintf("k%02d", i), Desc: fmt.Sprintf("does thing %02d", i)})
	}
	return g
}

func smallGroups() []chrome.HelpGroup {
	return []chrome.HelpGroup{numberedGroup("Views", 0, 3), numberedGroup("Export", 3, 5)}
}

func bigGroups() []chrome.HelpGroup {
	return []chrome.HelpGroup{numberedGroup("Views", 0, 10), numberedGroup("Export", 10, 20)}
}

func TestHelpOverlay_ListsTitleGroupsAndEveryEntryWhenItFits(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := chrome.HelpOverlay("Help", smallGroups(), 0, 60, 30, theme())

	assertBounds(t, got, 60, 30)
	wants := []string{"Help", "Views", "Export"}
	for i := 0; i < 5; i++ {
		wants = append(wants, fmt.Sprintf("k%02d", i), fmt.Sprintf("does thing %02d", i))
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("overlay lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(strings.ToLower(got), "more") {
		t.Errorf("a fitting overlay must not show a more line:\n%s", got)
	}
}

func TestHelpOverlay_KeyAndDescriptionShareALine(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	got := chrome.HelpOverlay("Help", smallGroups(), 0, 60, 30, theme())
	found := false
	for _, l := range splitLines(got) {
		if strings.Contains(l, "k01") && strings.Contains(l, "does thing 01") {
			found = true
		}
	}
	if !found {
		t.Errorf("no line pairs the key with its description:\n%s", got)
	}
}

func TestHelpOverlay_TallContentIsCutWithAMoreLine(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := chrome.HelpOverlay("Help", bigGroups(), 0, 60, 8, theme())

	assertBounds(t, got, 60, 8)
	lines := splitLines(got)
	if !strings.Contains(strings.ToLower(lines[len(lines)-1]), "more") {
		t.Errorf("last line %q, want a more line", lines[len(lines)-1])
	}
	if strings.Contains(got, "does thing 19") {
		t.Errorf("entries beyond the height were drawn:\n%s", got)
	}
	if !strings.Contains(got, "does thing 00") {
		t.Errorf("offset 0 must show the first entry:\n%s", got)
	}
}

func TestHelpOverlay_OffsetScrollsToLaterEntries(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	top := chrome.HelpOverlay("Help", bigGroups(), 0, 60, 8, theme())

	scrolled := chrome.HelpOverlay("Help", bigGroups(), 5, 60, 8, theme())
	bottom := chrome.HelpOverlay("Help", bigGroups(), 1000, 60, 8, theme())

	assertBounds(t, scrolled, 60, 8)
	assertBounds(t, bottom, 60, 8)
	if scrolled == top {
		t.Errorf("offset 5 rendered the same as offset 0:\n%s", scrolled)
	}
	if !strings.Contains(bottom, "does thing 19") {
		t.Errorf("an offset past the end must clamp so the last entry shows:\n%s", bottom)
	}
	if strings.Contains(bottom, "does thing 00") {
		t.Errorf("clamped bottom view still shows the first entry:\n%s", bottom)
	}
}

func TestHelpOverlay_EveryEntryIsReachableByScrolling(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	seen := map[string]bool{}
	for offset := 0; offset < 60; offset++ {
		out := chrome.HelpOverlay("Help", bigGroups(), offset, 60, 6, theme())
		for i := 0; i < 20; i++ {
			if strings.Contains(out, fmt.Sprintf("does thing %02d", i)) {
				seen[fmt.Sprint(i)] = true
			}
		}
	}
	if len(seen) != 20 {
		t.Errorf("scrolling reached %d of 20 entries", len(seen))
	}
}

func TestHelpOverlay_NegativeOffsetBehavesAsZero(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	a := chrome.HelpOverlay("Help", bigGroups(), -4, 60, 8, theme())
	b := chrome.HelpOverlay("Help", bigGroups(), 0, 60, 8, theme())
	if a == "" || a != b {
		t.Errorf("negative offset differs from offset 0:\n%s\n---\n%s", a, b)
	}
}

func TestHelpOverlay_NeverExceedsWidthOrHeightAtAnySize(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	groups := append(bigGroups(), chrome.HelpGroup{Title: "Long", Entries: []tui.HelpEntry{
		{Key: "ctrl+shift+alt+x", Desc: strings.Repeat("a very long description ", 6)},
	}})
	for w := 1; w <= 70; w += 7 {
		for _, h := range []int{1, 2, 3, 6, 12, 40} {
			for _, off := range []int{0, 3, 100} {
				assertBounds(t, chrome.HelpOverlay("Help", groups, off, w, h, theme()), w, h)
			}
		}
	}
}

func TestHelpOverlay_WideRuneEntriesAreMeasuredByDisplayWidth(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	groups := []chrome.HelpGroup{{Title: "\u65e5\u672c", Entries: []tui.HelpEntry{
		{Key: "\u65e5", Desc: strings.Repeat("\u8a9e", 20)},
	}}}
	for _, w := range []int{40, 15, 9} {
		assertBounds(t, chrome.HelpOverlay("Help", groups, 0, w, 10, theme()), w, 10)
	}
}

func TestHelpOverlay_NoGroupsStillRendersWithinBounds(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	got := chrome.HelpOverlay("Help", nil, 0, 40, 5, theme())
	assertBounds(t, got, 40, 5)
	if !strings.Contains(got, "Help") {
		t.Errorf("got %q, want the title", got)
	}
}
