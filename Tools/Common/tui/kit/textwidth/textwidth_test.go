package textwidth_test

// textwidth_test.go specifies display-cell measurement and ellipsis truncation, including
// wide runes, ANSI sequences and the ASCII glyph mode.

import (
	"strings"
	"testing"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

func useMode(t *testing.T, g tui.GlyphMode) {
	t.Helper()
	t.Cleanup(tui.SetRenderMode(tui.RenderMode{Color: false, Glyphs: g}))
}

func TestOf_CountsDisplayCells(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "abc", 3},
		{"wide runes count two", "\u65e5\u672c", 4},
		{"mixed", "a\u65e5b", 4},
		{"ansi sequences count zero", "\x1b[31mabc\x1b[0m", 3},
		{"precomposed accent", "\u00e9", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := textwidth.Of(tc.in); got != tc.want {
				t.Errorf("Of(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruncate_UnicodeMode(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"fits unchanged", "abcdef", 6, "abcdef"},
		{"shorter unchanged", "abc", 10, "abc"},
		{"cut ends with ellipsis", "abcdefghij", 6, "abcde\u2026"},
		{"wide runes measured by cells", "\u65e5\u672c\u8a9e", 5, "\u65e5\u672c\u2026"},
		{"zero width", "abc", 0, ""},
		{"negative width", "abc", -3, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := textwidth.Truncate(tc.in, tc.width); got != tc.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
		})
	}
}

func TestTruncate_ASCIIMode(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	if got := textwidth.Truncate("abcdefghij", 6); got != "abc..." {
		t.Errorf("got %q, want %q", got, "abc...")
	}
	if got := textwidth.Truncate("\u65e5\u672c\u8a9e", 5); got != "\u65e5..." {
		t.Errorf("wide runes: got %q, want %q", got, "\u65e5...")
	}
	if got := textwidth.Truncate("abcdef", 6); got != "abcdef" {
		t.Errorf("fitting text changed: %q", got)
	}
}

func TestTruncate_NeverSplitsAWideRuneOrExceedsWidth(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	in := "\u65e5\u672c\u8a9e\u65e5\u672c\u8a9e"
	for w := 1; w <= 13; w++ {
		got := textwidth.Truncate(in, w)
		if textwidth.Of(got) > w {
			t.Errorf("width %d: %q is %d cells", w, got, textwidth.Of(got))
		}
		if got != strings.ToValidUTF8(got, "?") {
			t.Errorf("width %d: invalid UTF-8 %q", w, got)
		}
	}
	if got := textwidth.Truncate(in, 4); !strings.HasPrefix(got, "\u65e5") || !strings.HasSuffix(got, "\u2026") {
		t.Errorf("width 4: got %q, want one wide rune then the ellipsis", got)
	}
}

func TestTruncate_AsciiModeStaysSevenBit(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	got := textwidth.Truncate("abcdefghijklmnop", 8)
	if got == "" || strings.ContainsFunc(got, func(r rune) bool { return r > 127 }) {
		t.Errorf("got %q, want non-empty 7-bit text", got)
	}
}

func TestTruncate_WidthSmallerThanEllipsisStillFits(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	got := textwidth.Truncate("abcdef", 2)
	if textwidth.Of(got) > 2 {
		t.Errorf("got %q (%d cells), want at most 2", got, textwidth.Of(got))
	}
}

func TestPad(t *testing.T) {
	useMode(t, tui.GlyphUnicode)
	cases := []struct {
		name  string
		in    string
		width int
		right bool
		want  string
	}{
		{"left aligned", "ab", 5, false, "ab   "},
		{"right aligned", "ab", 5, true, "   ab"},
		{"exact width", "abcde", 5, false, "abcde"},
		{"wide rune padded by cells", "\u65e5", 5, false, "\u65e5   "},
		{"truncates then pads exactly", "abcdefgh", 5, false, "abcd\u2026"},
		{"right aligned truncated", "abcdefgh", 5, true, "abcd\u2026"},
		{"zero width", "ab", 0, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := textwidth.Pad(tc.in, tc.width, tc.right); got != tc.want {
				t.Errorf("Pad(%q, %d, %v) = %q, want %q", tc.in, tc.width, tc.right, got, tc.want)
			}
		})
	}
}

func TestPad_ASCIIModeEllipsis(t *testing.T) {
	useMode(t, tui.GlyphASCII)
	if got := textwidth.Pad("abcdefgh", 6, false); got != "abc..." {
		t.Errorf("got %q, want %q", got, "abc...")
	}
}
