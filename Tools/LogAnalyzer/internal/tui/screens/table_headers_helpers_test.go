package screens_test

// Reads the header of a rendered table whose column titles may wrap onto several
// lines. The header is found by the last word of the first column's title (the
// bottom title line starts with it), the lines above it that belong to the table
// are the other title lines and the optional group line, and the lines below it
// up to the first line outside the table are its rows. Every column is the run of
// cells that header and rows occupy together, so a title is read back by joining
// the fragments of its column from the top line down.

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// gutterWidth is the cursor gutter drawn in front of every table line.
const gutterWidth = 2

// headerCol is one column of a rendered table.
type headerCol struct {
	title      string   // the title's words joined by single spaces, sort marker left out
	words      []string // the title's words
	titleLines int      // how many header lines carry a fragment of the title
	marked     bool     // the title carries the sort marker
	onBottom   bool     // the title's last word is on the bottom title line
	start, end int      // display columns [start, end) the column occupies
	cellWidth  int      // widest row cell of the column
}

// headerBlock is a rendered table: its columns, group line, header height and rows.
type headerBlock struct {
	cols        []headerCol
	group       string   // the group line, "" when the header has none
	groupLine   int      // index of the group line among the header lines
	headerLines int      // all header lines, the group line included
	hidden      bool     // the header shows a hidden-columns marker
	rows        []string // row lines, gutter included
}

var (
	hiddenColsRE   = regexp.MustCompile(`^\+\d+ \S$`)
	titleGapRE     = regexp.MustCompile(`\s{2,}`)
	trailingMarkRE = regexp.MustCompile(`\s[v^↑↓]$`)
)

// titleLineRE is what a title line may hold: words, the sort and hidden markers and
// the group rule, never the figures, glyphs and punctuation of the lines drawn
// above a table.
var titleLineRE = regexp.MustCompile(`^[A-Za-z0-9 ./*%+›↓↑^─]+$`)

// lastWordOf is the last space-separated word of s.
func lastWordOf(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}

// bottomHeaderLine is the index of the bottom title line of the table whose first
// column's title ends in anchorWord, or -1.
func bottomHeaderLine(lines []string, anchorWord string) int {
	for i, l := range lines {
		if !strings.HasPrefix(l, "  ") || strings.TrimSpace(l) == "" {
			continue
		}
		first := trailingMarkRE.ReplaceAllString(titleGapRE.Split(strings.TrimSpace(l), -1)[0], "")
		for _, alt := range strings.Split(anchorWord, "|") {
			if first == alt {
				return i
			}
		}
	}
	return -1
}

// runesOf drops the cursor gutter and splits the line into display cells.
func runesOf(line string) []rune {
	r := []rune(line)
	if len(r) < gutterWidth {
		return nil
	}
	return r[gutterWidth:]
}

// fragment is the trimmed text of cells [start, end) of the runes.
func fragment(r []rune, start, end int) string {
	if start >= len(r) {
		return ""
	}
	return strings.TrimSpace(string(r[start:min(end, len(r))]))
}

// occupiedRuns are the [start, end) runs of cells that any line occupies, where a
// single blank cell does not end a run (the gap between columns is wider).
func occupiedRuns(lines [][]rune) [][2]int {
	width := 0
	for _, l := range lines {
		width = max(width, len(l))
	}
	occ := make([]bool, width+2)
	for _, l := range lines {
		for i, c := range l {
			if c != ' ' {
				occ[i] = true
			}
		}
	}
	var runs [][2]int
	for i := 0; i < width; {
		if !occ[i] {
			i++
			continue
		}
		j := i
		for j < width && (occ[j] || occ[j+1]) {
			j++
		}
		runs = append(runs, [2]int{i, j})
		i = j
	}
	return runs
}

// titledRuns merges every run that has no title fragment into the run before it: a
// share cell holds its bar and its percent a few blanks apart, and both belong to
// the column titled above the bar.
func titledRuns(runs [][2]int, titleLines [][]rune) [][2]int {
	var out [][2]int
	for _, run := range runs {
		titled := false
		for _, tl := range titleLines {
			titled = titled || fragment(tl, run[0], run[1]) != ""
		}
		if !titled && len(out) > 0 {
			out[len(out)-1][1] = run[1]
			continue
		}
		out = append(out, run)
	}
	return out
}

// readHeader parses the table of the view whose first column's title ends in
// anchorWord. It fails the test when the view has no such table.
func readHeader(t *testing.T, view, anchorWord string) headerBlock {
	t.Helper()
	b, ok := parseHeader(view, anchorWord)
	if !ok {
		t.Fatalf("no table has a first column titled %q\n%s", anchorWord, stripStyling(view))
	}
	return b
}

// parseHeader is readHeader without the failure: ok is false when the view has no
// table with that first column.
func parseHeader(view, anchorWord string) (headerBlock, bool) {
	lines := viewLines(view)
	bottom := bottomHeaderLine(lines, anchorWord)
	if bottom < 0 {
		return headerBlock{}, false
	}
	top := bottom
	for top > 0 && strings.HasPrefix(lines[top-1], "  ") && titleLineRE.MatchString(lines[top-1]) {
		top--
	}
	var b headerBlock
	b.headerLines = bottom - top + 1
	var titleLines [][]rune
	for i := top; i <= bottom; i++ {
		if strings.ContainsRune(lines[i], '─') {
			b.group, b.groupLine = lines[i], i-top
			continue
		}
		titleLines = append(titleLines, runesOf(lines[i]))
	}
	var rows [][]rune
	for _, l := range lines[bottom+1:] {
		if l == "" || (l[0] != ' ' && l[0] != '>') || strings.TrimSpace(l) == "" {
			break
		}
		b.rows = append(b.rows, l)
		rows = append(rows, runesOf(l))
	}
	all := append(append([][]rune{}, titleLines...), rows...)
	for _, run := range titledRuns(occupiedRuns(all), titleLines) {
		col := headerCol{start: run[0], end: run[1]}
		var frags []string
		for _, tl := range titleLines {
			if f := fragment(tl, run[0], run[1]); f != "" {
				frags = append(frags, f)
			}
		}
		text := strings.Join(frags, " ")
		if hiddenColsRE.MatchString(text) {
			b.hidden = true
			continue
		}
		if trailingMarkRE.MatchString(text) {
			col.marked = true
			text = trailingMarkRE.ReplaceAllString(text, "")
		}
		col.title, col.words, col.titleLines = text, strings.Fields(text), len(frags)
		col.onBottom = fragment(titleLines[len(titleLines)-1], run[0], run[1]) != ""
		for _, r := range rows {
			col.cellWidth = max(col.cellWidth, utf8.RuneCountInString(fragment(r, run[0], run[1])))
		}
		b.cols = append(b.cols, col)
	}
	return b, true
}

// titles are the column titles in order.
func (b headerBlock) titles() []string {
	out := make([]string, len(b.cols))
	for i, c := range b.cols {
		out[i] = c.title
	}
	return out
}

// col is the column with the title, or false.
func (b headerBlock) col(title string) (headerCol, bool) {
	for _, c := range b.cols {
		if c.title == title {
			return c, true
		}
	}
	return headerCol{}, false
}

// longestWord is the width of the longest word of the title.
func (c headerCol) longestWord() int {
	n := 0
	for _, w := range c.words {
		n = max(n, utf8.RuneCountInString(w))
	}
	return n
}

// neededWidth is the narrowest the column can be: its widest cell, its longest title
// word, and its last title word with the sort marker when it is the sort column.
func (c headerCol) neededWidth() int {
	n := max(c.cellWidth, c.longestWord())
	if c.marked && len(c.words) > 0 {
		n = max(n, utf8.RuneCountInString(c.words[len(c.words)-1])+2)
	}
	return n
}
