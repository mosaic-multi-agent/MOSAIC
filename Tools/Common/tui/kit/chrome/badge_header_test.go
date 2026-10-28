package chrome_test

// badge_header_test.go specifies the badge and the section header.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/chrome"
)

func TestBadge_ReadsAsBracketedTextWithColourOff(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := chrome.Badge("ok", tui.RoleSignalGood, 20, theme())

	if got != "[ok]" {
		t.Errorf("got %q, want %q", got, "[ok]")
	}
}

func TestBadge_ColourOnStylesTheLabelButKeepsText(t *testing.T) {
	useColourOn(t)

	bad := chrome.Badge("bad", tui.RoleSignalBad, 20, theme())
	good := chrome.Badge("bad", tui.RoleSignalGood, 20, theme())

	if stripANSI(bad) != "[bad]" {
		t.Errorf("got %q, want the text \"[bad]\" once escapes are removed", bad)
	}
	if pre := styleBefore(bad, "[bad]"); pre == "" || pre == "<text not found>" {
		t.Errorf("got %q, want the escape sequence to wrap the badge", bad)
	}
	if styleBefore(bad, "[bad]") == styleBefore(good, "[bad]") {
		t.Errorf("roles bad and good must style the badge differently: %q vs %q", bad, good)
	}
}

func TestBadge_NeverWiderThanWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, w := range []int{1, 2, 4, 6, 9} {
		got := chrome.Badge("long label", tui.RoleSignalWarning, w, theme())
		if got == "" {
			if w >= 4 {
				t.Errorf("width %d: got empty, want a bracketed (possibly truncated) label", w)
			}
			continue
		}
		assertBounds(t, got, w, 1)
	}
	if got := chrome.Badge("ok", tui.RoleSignalGood, 0, theme()); got != "" {
		t.Errorf("width 0: got %q, want empty", got)
	}
}

func TestBadge_WideRuneLabelIsMeasuredByDisplayWidth(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	got := chrome.Badge("日本語日本", tui.RoleSignalGood, 7, theme())
	assertBounds(t, got, 7, 1)
}

func TestSectionHeader_TitleThenRuleFillingTheWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)

	got := chrome.SectionHeader("Sizes", 30, theme())

	assertBounds(t, got, 30, 1)
	rest, ok := strings.CutPrefix(strings.TrimLeft(got, " "), "Sizes")
	if !ok {
		t.Fatalf("header %q does not start with the title", got)
	}
	if strings.Count(rest, "-") < 3 || strings.Trim(rest, " -") != "" {
		t.Errorf("after the title %q: want a rule of dashes", rest)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("header must be one line: %q", got)
	}
}

func TestSectionHeader_UnicodeModeUsesNonASCIIRule(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	got := chrome.SectionHeader("Sizes", 30, theme())
	assertBounds(t, got, 30, 1)
	if !strings.Contains(got, "Sizes") || strings.Contains(got, "---") {
		t.Errorf("got %q, want the title and a Unicode rule", got)
	}
}

func TestSectionHeader_LongTitleIsTruncatedToWidth(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	for _, w := range []int{1, 5, 12} {
		assertBounds(t, chrome.SectionHeader("A very long section title", w, theme()), w, 1)
	}
	if got := chrome.SectionHeader("Sizes", 0, theme()); got != "" {
		t.Errorf("width 0: got %q, want empty", got)
	}
}
