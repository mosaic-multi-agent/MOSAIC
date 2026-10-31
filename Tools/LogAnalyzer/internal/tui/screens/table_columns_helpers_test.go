package screens_test

// Helpers that read the column structure and the share cells of a rendered table
// view: where each header title starts, which tables a stacked view holds, which
// rows a table has and the percent texts a row shows.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// colSeg is one column title of a header line and the display column it starts at.
type colSeg struct {
	start int
	title string
}

var (
	titleRE        = regexp.MustCompile(`\S+(?: \S+)*`)
	hiddenSegRE    = regexp.MustCompile(`^\+\d+ \S$`)
	hiddenTailRE   = regexp.MustCompile(`\+\d+ \S$`)
	sharePercentRE = regexp.MustCompile(`(?:^|[^0-9.])(<1%|[0-9]+%)`)
)

// headerSegments are the column titles of a header line, the hidden-columns marker
// left out. A sort arrow stays part of its title.
func headerSegments(line string) []colSeg {
	var out []colSeg
	for _, loc := range titleRE.FindAllStringIndex(line, -1) {
		text := line[loc[0]:loc[1]]
		if hiddenSegRE.MatchString(text) {
			continue
		}
		out = append(out, colSeg{start: utf8.RuneCountInString(line[:loc[0]]), title: text})
	}
	return out
}

// hiddenMarkerOf is the hidden-columns marker at the end of a header line, or "".
func hiddenMarkerOf(line string) string {
	return hiddenTailRE.FindString(strings.TrimRight(line, " "))
}

// stackedTable is one table of a view that stacks several: the label above it, its
// header lines (the bottom title line in header, the title lines above it in upper)
// and its row lines.
type stackedTable struct {
	label  string
	header string
	upper  []string
	rows   []string
}

// stackedTables splits a view of stacked tables, one label line then the header lines
// then the title lines (the last starts with the anchor title) then rows per table, into its tables. The first lines up to the rule under the tab
// bar and the last line (the footer) are not part of any table.
func stackedTables(t *testing.T, view, anchor string) []stackedTable {
	t.Helper()
	lines := viewLines(view)
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	rule := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "──") {
			rule = i
			break
		}
	}
	if rule < 0 || len(lines) < rule+3 {
		t.Fatalf("the view has no rule under the tab bar or no body\n%s", stripStyling(view))
	}
	var tables []stackedTable
	for _, l := range lines[rule+1 : len(lines)-1] {
		switch {
		case strings.TrimSpace(l) == "":
		case l[0] != ' ' && l[0] != '>':
			tables = append(tables, stackedTable{label: l})
		case len(tables) == 0:
			t.Fatalf("a table line %q comes before any table label", l)
		case tables[len(tables)-1].header == "" && !isBottomTitleLine(l, anchor):
			tables[len(tables)-1].upper = append(tables[len(tables)-1].upper, l)
		case tables[len(tables)-1].header == "":
			tables[len(tables)-1].header = l
		default:
			tables[len(tables)-1].rows = append(tables[len(tables)-1].rows, l)
		}
	}
	return tables
}

// columnLayout describes where the columns of a header start and which ones are
// hidden, so two tables with the same layout give the same text.
func columnLayout(header string) string {
	return fmt.Sprintf("%v hidden=%q", headerSegments(header), hiddenMarkerOf(header))
}

// tableRows are the row lines of the table whose first column title is anchor (a sort
// arrow is ignored), up to the first line that is not indented.
func tableRows(t *testing.T, view, anchor string) []string {
	t.Helper()
	lines := viewLines(view)
	for i, l := range lines {
		segs := headerSegments(l)
		if len(segs) == 0 || sortMarkerRE.ReplaceAllString(segs[0].title, "") != anchor {
			continue
		}
		var rows []string
		for _, r := range lines[i+1:] {
			if strings.TrimSpace(r) == "" || (r[0] != ' ' && r[0] != '>') {
				break
			}
			rows = append(rows, r)
		}
		return rows
	}
	t.Fatalf("no table has %q as its first column\n%s", anchor, stripStyling(view))
	return nil
}

// firstCell is the first cell text of a row line, without the cursor gutter.
func firstCell(row string) string {
	f := strings.Fields(strings.TrimLeft(row, "> "))
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// firstCells are the first cell texts of the rows.
func firstCells(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = firstCell(r)
	}
	return out
}

// identityShown reports whether some cell shows id whole or cut in the middle with
// its head and its tail (the part from the last "#") both still readable.
func identityShown(cells []string, id string) bool {
	tail := id[strings.LastIndex(id, "#"):]
	for _, c := range cells {
		if c == id {
			return true
		}
		for _, ell := range []string{"…", "..."} {
			if head, rest, ok := strings.Cut(c, ell); ok && strings.HasPrefix(id, head) && rest == tail {
				return true
			}
		}
	}
	return false
}

// sharePercents are the percent texts ("40%", "<1%") a row shows in order; ratios with
// a decimal part such as "67.9%" are not percents of a share bar.
func sharePercents(row string) []string {
	var out []string
	for _, m := range sharePercentRE.FindAllStringSubmatch(row, -1) {
		out = append(out, m[1])
	}
	return out
}

// markedTitle is the full column title that carries the sort arrow in the header whose
// first column is anchor, or "" when no title carries it.
func markedTitle(t *testing.T, view, anchor string) string {
	t.Helper()
	for _, c := range readHeader(t, view, anchor).cols {
		if c.marked {
			return c.title
		}
	}
	return ""
}

// isBottomTitleLine reports whether the line is the bottom title line of a table whose
// first column is titled anchor: its first title (a sort arrow ignored) is the anchor.
func isBottomTitleLine(line, anchor string) bool {
	segs := headerSegments(line)
	return len(segs) > 0 && sortMarkerRE.ReplaceAllString(segs[0].title, "") == anchor
}

// fullLayout is the column layout of every header line of the table, top line first.
func fullLayout(tb stackedTable) string {
	parts := make([]string, 0, len(tb.upper)+1)
	for _, l := range tb.upper {
		parts = append(parts, columnLayout(l))
	}
	return strings.Join(append(parts, columnLayout(tb.header)), "\n")
}
