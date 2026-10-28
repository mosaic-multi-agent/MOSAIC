package screens_test

// Helpers shared by the table-view tests (runs list, Dispatches, By agent type):
// width and truncation checks, share-bar weight, column alignment, role checks on
// styled output and a driver that walks a view's sort cycle.

import (
	"regexp"
	"strings"
	"testing"

	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"

	"mosaic-log-analyzer/internal/tui/screens"
)

// tableW is wide enough for every column of the table views to show in full.
const tableW = 240

// barGlyphWeight is the weight of one share-bar glyph in eighths of a cell, in
// the Unicode glyph mode the tests run in.
var barGlyphWeight = map[rune]int{
	'█': 8, '▉': 7, '▊': 6, '▋': 5, '▌': 4, '▍': 3, '▎': 2, '▏': 1,
}

// barWeight is the filled length of the share bar on a line, in eighths of a cell.
func barWeight(line string) int {
	n := 0
	for _, r := range stripStyling(line) {
		n += barGlyphWeight[r]
	}
	return n
}

// displayColumn is the display column at which needle starts on the line, or -1.
func displayColumn(line, needle string) int {
	i := strings.Index(line, needle)
	if i < 0 {
		return -1
	}
	return textwidth.Of(line[:i])
}

// assertNameShown fails unless the name appears intact on some line or as a prefix
// of at least minPrefix characters followed by an ellipsis.
func assertNameShown(t *testing.T, view, name string, minPrefix int) {
	t.Helper()
	plainView := stripStyling(view)
	if strings.Contains(plainView, name) {
		return
	}
	runes := []rune(name)
	for k := len(runes) - 1; k >= minPrefix; k-- {
		prefix := string(runes[:k])
		if strings.Contains(plainView, prefix+"…") || strings.Contains(plainView, prefix+"...") {
			return
		}
	}
	t.Errorf("%q is neither shown intact nor ellipsis-truncated (at least %d characters)\n%s", name, minPrefix, plainView)
}

// rawLineWith returns the first line of the styled view whose styling-free text
// contains needle; the line keeps its escape sequences.
func rawLineWith(t *testing.T, view, needle string) string {
	t.Helper()
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(stripStyling(l), needle) {
			return l
		}
	}
	t.Fatalf("no line contains %q\n%s", needle, stripStyling(view))
	return ""
}

// rolePrefix is the escape sequence the palette opens a role with. Colour must be
// on when it is called.
func rolePrefix(role tuicommon.StyleRole) string {
	s := tuicommon.PaletteTheme().Style(role).Render("X")
	return s[:strings.Index(s, "X")]
}

// assertDrawnInRole fails unless text on the raw line sits right after the opening
// of role, allowing a leading bracket or padding of the cell.
func assertDrawnInRole(t *testing.T, rawLine, text string, role tuicommon.StyleRole) {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(rolePrefix(role)) + `\[?\s*` + regexp.QuoteMeta(text))
	if !re.MatchString(rawLine) {
		t.Errorf("%q must be drawn in role %q: %q", text, role, rawLine)
	}
}

// headOf and tailOf are the first and last line of a view, or "" for an empty view,
// so a view that drew nothing fails its assertions instead of panicking the run.
func headOf(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func tailOf(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

// countOf is how many times needle occurs in the styling-free text.
func countOf(text, needle string) int { return strings.Count(stripStyling(text), needle) }

var sortLabelRE = regexp.MustCompile(`sort: ([a-z ]+?) (↓|↑|v|\^)`)

// activeSort reads the sort key and direction the view's header shows.
func activeSort(t *testing.T, view string) (key screens.SortKey, desc bool) {
	t.Helper()
	m := sortLabelRE.FindStringSubmatch(stripStyling(view))
	if m == nil {
		t.Fatalf("view shows no sort label\n%s", stripStyling(view))
	}
	return screens.SortKey(m[1]), m[2] == "↓" || m[2] == "v"
}

// assertAppearInOrder fails unless the needles appear on increasing lines when
// ascending is true, or on decreasing lines when it is false.
func assertAppearInOrder(t *testing.T, view string, needles []string, ascending bool) {
	t.Helper()
	prev := -1
	if !ascending {
		prev = 1 << 30
	}
	for _, n := range needles {
		i := lineIndex(stripStyling(view), n)
		if i < 0 {
			t.Errorf("%q is not listed\n%s", n, stripStyling(view))
			return
		}
		if (ascending && i <= prev) || (!ascending && i >= prev) {
			t.Errorf("rows are not in the order of the active sort (%q out of place, ascending=%v)\n%s",
				n, ascending, stripStyling(view))
			return
		}
		prev = i
	}
}

// walkSorts presses the sort key until the cycle returns to its first order. At
// every order whose key has an entry in ascending (the row labels from lowest to
// highest by that key), the rows must follow the order the header's direction
// marker states. It fails unless every key in want was offered, and returns the
// keys in the order they were offered.
func walkSorts(t *testing.T, s screens.Screen, width, height int, ascending map[screens.SortKey][]string,
	want ...screens.SortKey) []screens.SortKey {
	t.Helper()
	var seen []screens.SortKey
	wrapped := false
	first, _ := activeSort(t, s.View(width, height))
	for i := 0; i < 12; i++ {
		view := s.View(width, height)
		key, desc := activeSort(t, view)
		if i > 0 && key == first {
			wrapped = true
			break
		}
		seen = append(seen, key)
		if ids, ok := ascending[key]; ok {
			assertAppearInOrder(t, view, ids, !desc)
		}
		press(s, screens.KeySort)
	}
	if !wrapped {
		t.Errorf("the sort cycle never returns to %q; offered %v", first, seen)
	}
	for _, w := range want {
		found := false
		for _, k := range seen {
			found = found || k == w
		}
		if !found {
			t.Errorf("sort %q is never offered; offered %v", w, seen)
		}
	}
	return seen
}
