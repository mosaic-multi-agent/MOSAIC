package chrome_test

// tabbar_test.go specifies the tab bar: active tab marking that survives colour off,
// truncation on narrow widths, and exact renders at a wide width.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
)

func sampleTabs() []chrome.Tab {
	return []chrome.Tab{{Key: "1", Label: "Categories"}, {Key: "2", Label: "Overview"}, {Key: "3", Label: "By type"}}
}

func TestTabBar_ActiveTabIsBracketedAndOthersAreNot(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := chrome.TabBar(sampleTabs(), 1, 80, theme())

	if !strings.Contains(got, "[2 Overview]") {
		t.Errorf("got %q, want the active tab as \"[2 Overview]\"", got)
	}
	if strings.Count(got, "[") != 1 || strings.Count(got, "]") != 1 {
		t.Errorf("got %q, want exactly one bracketed tab", got)
	}
	for _, other := range []string{"1 Categories", "3 By type"} {
		if !strings.Contains(got, other) {
			t.Errorf("got %q, want inactive tab %q listed", got, other)
		}
	}
	if strings.Contains(got, "\n") {
		t.Errorf("tab bar must be one line: %q", got)
	}
}

func TestTabBar_ActiveMarkFollowsTheActiveIndex(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for i, want := range []string{"[1 Categories]", "[2 Overview]", "[3 By type]"} {
		if got := chrome.TabBar(sampleTabs(), i, 80, theme()); !strings.Contains(got, want) {
			t.Errorf("active %d: got %q, want %q", i, got, want)
		}
	}
}

func TestTabBar_ColourOnHighlightsTheActiveTab(t *testing.T) {
	useColourOn(t)

	first := chrome.TabBar(sampleTabs(), 0, 80, theme())
	second := chrome.TabBar(sampleTabs(), 1, 80, theme())

	if !strings.Contains(stripANSI(first), "[1 Categories]") {
		t.Errorf("got %q, want the active tab text intact", first)
	}
	if pre := styleBefore(first, "[1 Categories]"); pre == "" || pre == "<text not found>" {
		t.Errorf("got %q, want the escape sequence to wrap the active tab", first)
	}
	if pre := styleBefore(second, "[2 Overview]"); pre == "" || pre == "<text not found>" {
		t.Errorf("got %q, want the escape sequence to wrap the newly active tab", second)
	}
	if styleBefore(second, "1 Categories") == styleBefore(first, "[1 Categories]") {
		t.Errorf("a tab styled as active in one render must not carry that style once inactive: %q", second)
	}
}

func TestTabBar_NarrowWidthStaysWithinBoundsAndKeepsActiveTabVisible(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for active := range sampleTabs() {
		for _, w := range []int{40, 28, 20, 14, 8} {
			got := chrome.TabBar(sampleTabs(), active, w, theme())

			assertBounds(t, got, w, 1)
			if !strings.Contains(got, "["+string(rune('1'+active))) {
				t.Errorf("active %d width %d: %q lost the active tab", active, w, got)
			}
		}
	}
}

func TestTabBar_WideRuneLabelsAreMeasuredByDisplayWidth(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	tabs := []chrome.Tab{{Key: "1", Label: "日本語日本"}, {Key: "2", Label: "日本語"}}
	for _, w := range []int{30, 16, 10} {
		assertBounds(t, chrome.TabBar(tabs, 1, w, theme()), w, 1)
	}
}

func TestTabBar_EmptyAndOutOfRangeActiveDoNotPanic(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	_ = chrome.TabBar(nil, 0, 40, theme())
	got := chrome.TabBar(sampleTabs(), 9, 80, theme())
	if strings.Contains(got, "[") || !strings.Contains(got, "Overview") {
		t.Errorf("out-of-range active: got %q, want all tabs unhighlighted", got)
	}
}

func TestTabBar_Golden(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, tc := range []struct {
		name   string
		active int
	}{{"tabbar_first_active", 0}, {"tabbar_second_active", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			assertGolden(t, tc.name, chrome.TabBar(sampleTabs(), tc.active, 80, theme()))
		})
	}
}

func TestTabBar_ColourOnStylesOnlyTheActiveTab(t *testing.T) {
	useColourOn(t)

	got := chrome.TabBar(sampleTabs(), 1, 80, theme())

	i := strings.Index(got, "[2 Overview]")
	if i < 0 {
		t.Fatalf("got %q, want the active tab text intact", got)
	}
	head := trailingEscapes.ReplaceAllString(got[:i], "")
	tail := leadingEscapes.ReplaceAllString(got[i+len("[2 Overview]"):], "")
	if strings.Contains(head, "\x1b[") || strings.Contains(tail, "\x1b[") {
		t.Errorf("inactive tabs and separators must carry no escape sequence: %q", got)
	}
	if !strings.Contains(head, "1 Categories") || !strings.Contains(tail, "3 By type") {
		t.Errorf("got %q, want the inactive tabs listed around the active one", got)
	}
}
