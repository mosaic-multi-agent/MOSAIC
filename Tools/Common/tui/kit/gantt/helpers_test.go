package gantt_test

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/gantt"
)

var update = flag.Bool("update", false, "rewrite golden files from the current render")

var bothModes = []struct {
	name string
	mode tui.GlyphMode
}{{"unicode", tui.GlyphUnicode}, {"ascii", tui.GlyphASCII}}

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

func isASCII(s string) bool {
	for _, r := range s {
		if r > 0x7f {
			return false
		}
	}
	return true
}

// assertGolden compares out, ignoring trailing spaces, with testdata/<name>.golden.
func assertGolden(t *testing.T, name, out string) {
	t.Helper()
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	got := strings.Join(lines, "\n") + "\n"
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

// base is the start of every test axis; times are offsets in seconds from it.
var base = time.Date(2026, 1, 2, 14, 0, 0, 0, time.UTC)

func at(sec float64) time.Time { return base.Add(time.Duration(sec * float64(time.Second))) }

// axisOf returns an axis from base spanning sec seconds.
func axisOf(sec float64) gantt.Axis { return gantt.Axis{Start: base, End: at(sec)} }

func span(from, to float64) gantt.Interval {
	return gantt.Interval{Start: at(from), End: at(to)}
}

func roled(iv gantt.Interval, role tui.StyleRole) gantt.Interval {
	iv.Role = role
	return iv
}

// plotOf returns the plot-area cells of a lane line rendered at lineW cells. A line that is too
// short is padded with U+FFFD cells, so a missing render fails the assertions instead of
// panicking on an index.
func plotOf(line string, labelWidth int) []rune {
	r := []rune(line)
	plot := []rune{}
	if len(r) > labelWidth+1 {
		plot = r[labelWidth+1:]
	}
	for len(plot) < lineW-labelWidth-1 {
		plot = append(plot, '�')
	}
	return plot
}

// styleAt returns the style prefix of visible cell i of out, or a marker when out is too short.
func styleAt(out string, i int) string {
	s := cellStyles(out)
	if i >= len(s) {
		return "<missing cell>"
	}
	return s[i]
}

// occupied lists the indices of the non-space cells. A padding cell (a missing render) is listed
// as -1, so a render that is too short never matches an expected list of indices.
func occupied(plot []rune) []int {
	var out []int
	for i, r := range plot {
		switch r {
		case ' ':
		case '�':
			out = append(out, -1)
		default:
			out = append(out, i)
		}
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func runeCount(s string) int { return len([]rune(s)) }
