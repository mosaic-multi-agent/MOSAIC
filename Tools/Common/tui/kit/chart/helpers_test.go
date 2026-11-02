package chart_test

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"mosaic-common/tui"
)

var update = flag.Bool("update", false, "rewrite golden files from the current render")

func useMode(t *testing.T, g tui.GlyphMode) {
	t.Helper()
	t.Cleanup(tui.SetRenderMode(tui.RenderMode{Color: false, Glyphs: g}))
}

// useColourOn forces a true-colour profile with ASCII glyphs and restores everything afterwards.
func useColourOn(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	restore := tui.SetRenderMode(tui.RenderMode{Color: true, Glyphs: tui.GlyphASCII})
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		restore()
		lipgloss.SetColorProfile(prev)
	})
}

func theme() tui.Theme { return tui.PaletteTheme() }

var (
	ansiRE  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	tokenRE = regexp.MustCompile(`(?s)\x1b\[[0-9;]*m|.`)
)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// rolePrefix returns the escape sequences the theme emits before text rendered in role.
func rolePrefix(role tui.StyleRole) string {
	m := regexp.MustCompile(`^((?:\x1b\[[0-9;]*m)*)X`).FindStringSubmatch(theme().Style(role).Render("X"))
	if m == nil {
		return "<no prefix>"
	}
	return m[1]
}

// cellStyles returns, for every visible rune of out, the escape prefix that styles it.
func cellStyles(out string) []string {
	var styles []string
	cur := ""
	for _, tok := range tokenRE.FindAllString(out, -1) {
		switch {
		case tok == "\x1b[0m" || tok == "\x1b[m":
			cur = ""
		case strings.HasPrefix(tok, "\x1b["):
			cur += tok
		default:
			styles = append(styles, cur)
		}
	}
	return styles
}

// countStyled returns how many cells of out are styled with role.
func countStyled(out string, role tui.StyleRole) int {
	n := 0
	for _, s := range cellStyles(out) {
		if s == rolePrefix(role) {
			n++
		}
	}
	return n
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 0x7f {
			return false
		}
	}
	return true
}

func trimmedLines(out string) []string {
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// assertGolden compares out, ignoring trailing spaces, with testdata/<name>.golden.
func assertGolden(t *testing.T, name, out string) {
	t.Helper()
	got := strings.Join(trimmedLines(out), "\n") + "\n"
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if got != want {
		t.Errorf("render differs from %s\n--- got ---\n%s--- want ---\n%s", path, got, want)
	}
}
