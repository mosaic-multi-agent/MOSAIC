package table_test

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
	"mosaic-common/tui/kit/table"
)

var update = flag.Bool("update", false, "rewrite golden files from the current render")

func useMode(t *testing.T, g tui.GlyphMode) {
	t.Helper()
	t.Cleanup(tui.SetRenderMode(tui.RenderMode{Color: false, Glyphs: g}))
}

// useColourOn forces a true-colour profile for the test and restores everything afterwards.
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

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes every escape sequence.
func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// styleBefore returns the escape sequences that directly precede the first occurrence of text.
func styleBefore(s, text string) string {
	m := regexp.MustCompile("((?:\x1b\\[[0-9;]*m)*)" + regexp.QuoteMeta(text)).FindStringSubmatch(s)
	if m == nil {
		return "<text not found>"
	}
	return m[1]
}

func theme() tui.Theme { return tui.PaletteTheme() }

func cells(texts ...string) []table.Cell {
	out := make([]table.Cell, len(texts))
	for i, s := range texts {
		out[i] = table.Cell{Text: s}
	}
	return out
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// render draws tb; an empty render yields placeholder lines so indexing tests fail by assertion.
func render(tb table.Table, width, height int) []string {
	lines := splitLines(table.Render(tb, width, height, theme()))
	if len(lines) == 0 {
		return []string{"<no output>", "<no output>", "<no output>", "<no output>"}
	}
	return lines
}

// plain builds a table with no cursor and no sort marker.
func plain(cols []table.Column, rows ...[]table.Cell) table.Table {
	return table.Table{Columns: cols, Rows: rows, Cursor: table.NoCursor, SortColumn: table.NoSort}
}

func assertBounds(t *testing.T, lines []string, width, height int) {
	t.Helper()
	if len(lines) == 0 {
		t.Fatalf("render produced no lines")
	}
	if len(lines) > height {
		t.Errorf("%d lines exceed height %d", len(lines), height)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > width {
			t.Errorf("line %d is %d cells wide, limit %d: %q", i, w, width, l)
		}
	}
}

func trimRight(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(l, " ")
	}
	return out
}

// assertGolden compares the render, ignoring trailing spaces, with testdata/<name>.golden.
func assertGolden(t *testing.T, name string, lines []string) {
	t.Helper()
	got := strings.Join(trimRight(lines), "\n") + "\n"
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
