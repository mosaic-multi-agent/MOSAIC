package screens_test

// layout_helpers_test.go holds the shared driver and assertions of the layout tests of the
// multi-select screens: the rendered view must fit the terminal height whatever is selected,
// the list must scroll with its cursor row always rendered, and the content block (list beside
// detail pane) must not be taller than the rows need. Everything is asserted on the rendered
// View(), never on helper values of the screens. Nothing in this file is a test itself.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/tui/screens"
)

// layoutDriver is the part of every multi-select screen the layout tests use.
type layoutDriver interface {
	Update(tea.Msg) tea.Cmd
	View() string
	Resize(width, height int)
}

// layoutScreen describes one screen under test. build creates the screen at the given size
// with n rows and leaves it at the level that lists those rows; withProv hands it a catalog
// label for every row.
type layoutScreen struct {
	name  string
	title string
	// hasSummary reports that the screen shows a "Selected N" summary and a validation message.
	hasSummary bool
	// slack is the number of extra content lines the screen may need beyond its rows (for
	// example a category heading row).
	slack int
	// topMarks are the texts the first cursor stop may show (a row name or a heading).
	topMarks []string
	build    func(width, height, n int, withProv bool) layoutDriver
}

type layoutSize struct{ w, h int }

var layoutSizes = []layoutSize{{80, 12}, {40, 10}, {120, 40}}

const layoutManyRows = 30

func layoutRowName(i int) string { return fmt.Sprintf("Row-%02d", i+1) }
func layoutRowKey(i int) string  { return fmt.Sprintf("layout-key-%02d", i+1) }

// layoutDescription is a tall detail text, so a detail pane that is not cut to the list
// height would make the content block taller than the rows.
func layoutDescription() string {
	return strings.TrimSpace(strings.Repeat("detail line of the focused row\n", 15))
}

func layoutLabels(n int) screens.ProvenanceLabels {
	labels := screens.ProvenanceLabels{}
	for i := 0; i < n; i++ {
		labels[layoutRowKey(i)] = "catalog-x"
	}
	return labels
}

func layoutKey(d layoutDriver, k tea.KeyType) { d.Update(tea.KeyMsg{Type: k}) }

func layoutToggle(d layoutDriver) {
	d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
}

// layoutSelect selects the first k rows (the cursor ends on row k), then moves back to the top.
func layoutSelect(d layoutDriver, k, total int) {
	for i := 0; i < k; i++ {
		layoutToggle(d)
		layoutKey(d, tea.KeyDown)
	}
	for i := 0; i < total+2; i++ {
		layoutKey(d, tea.KeyUp)
	}
}

func isBorderLine(l string) bool {
	t := strings.TrimSpace(l)
	return t != "" && strings.Trim(t, "─") == ""
}

// listColumn returns, per line, the text left of the list/detail separator.
func listColumn(view string) []string {
	var out []string
	for _, l := range strings.Split(view, "\n") {
		if i := strings.Index(l, "│"); i >= 0 {
			l = l[:i]
		}
		out = append(out, l)
	}
	return out
}

// contentHeight is the number of lines between the first and the last border line.
func contentHeight(view string) int {
	first, last := -1, -1
	for i, l := range strings.Split(view, "\n") {
		if isBorderLine(l) {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 || last <= first {
		return -1
	}
	return last - first - 1
}

func assertLayoutFits(t *testing.T, view string, sc layoutScreen, size layoutSize, ctx string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > size.h {
		t.Errorf("%s: %s: view has %d lines on a terminal %d high; want at most %d\n%s",
			sc.name, ctx, len(lines), size.h, size.h, view)
	}
	if !strings.Contains(lines[0], sc.title) {
		t.Errorf("%s: %s: first line = %q; want the title %q", sc.name, ctx, lines[0], sc.title)
	}
}

// cursorLine returns the list-column line that carries the cursor glyph and how many there are.
func cursorLine(view string) (string, int) {
	var found string
	n := 0
	for _, l := range listColumn(view) {
		if strings.Contains(l, "▶") {
			found = l
			n++
		}
	}
	return found, n
}

func assertCursorVisible(t *testing.T, view string, sc layoutScreen, ctx string) string {
	t.Helper()
	line, n := cursorLine(view)
	if n != 1 {
		t.Errorf("%s: %s: %d cursor rows rendered; want exactly 1\n%s", sc.name, ctx, n, view)
	}
	return line
}
