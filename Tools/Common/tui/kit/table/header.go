package table

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

// shape is everything that decides how a table's lines are drawn: the visible columns, their
// widths, and how tall the header is.
type shape struct {
	visible []int
	hidden  int
	widths  []int
	titleH  int  // title lines, at least 1
	group   bool // a group-label line sits above the titles
}

func (s shape) headerHeight() int {
	if s.group {
		return s.titleH + 1
	}
	return s.titleH
}

// HeaderHeight is the number of header lines Render draws for t at width:
// the tallest wrapped title, plus one when any visible column has a Group.
func HeaderHeight(t Table, width int) int {
	if width <= 0 {
		return 1
	}
	visible, hidden, widths := layoutWidths(t, width)
	return newShape([]Table{t}, visible, hidden, widths).headerHeight()
}

// BodyRowsFor is max(height - HeaderHeight(t, width), 0).
func BodyRowsFor(t Table, width, height int) int {
	return max(height-HeaderHeight(t, width), 0)
}

// newShape measures the header the tables need at the given visible columns and widths.
func newShape(tables []Table, visible []int, hidden int, widths []int) shape {
	s := shape{visible: visible, hidden: hidden, widths: widths, titleH: 1}
	for _, t := range tables {
		for _, i := range visible {
			s.titleH = max(s.titleH, len(titleLines(t, i, widths[i])))
			if t.Columns[i].Group != "" {
				s.group = true
			}
		}
	}
	return s
}

// titleWords are the words a title wraps between; the sort marker stays on the last word.
func titleWords(t Table, i int) []string {
	words := strings.Fields(t.Columns[i].Title)
	if i != t.SortColumn {
		return words
	}
	marker := sortMarker(t.SortDesc)
	if len(words) == 0 {
		return []string{marker}
	}
	words[len(words)-1] += " " + marker
	return words
}

// titleLines is the title of column i for a column of the given width: wrapped when the table
// asks for it, otherwise a single line.
func titleLines(t Table, i, width int) []string {
	if !t.WrapTitles {
		return []string{headerText(t, i)}
	}
	return wrapWords(titleWords(t, i), width)
}

// longestWord is the widest word, the narrowest a wrapped title can get.
func longestWord(words []string) int {
	w := 0
	for _, word := range words {
		w = max(w, textwidth.Of(word))
	}
	return w
}

// wrapWords fills lines greedily up to width cells; a word wider than width is cut every width cells.
func wrapWords(words []string, width int) []string {
	width = max(width, 1)
	var lines []string
	cur := ""
	for _, word := range words {
		for textwidth.Of(word) > width {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			lines = append(lines, ansi.Truncate(word, width, ""))
			word = ansi.TruncateLeft(word, width, "")
		}
		switch {
		case cur == "":
			cur = word
		case textwidth.Of(cur)+1+textwidth.Of(word) <= width:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" || len(lines) == 0 {
		lines = append(lines, cur)
	}
	return lines
}

// headerLines draws the group line (when the shape has one) and the title lines, titles aligned to the bottom.
func headerLines(t Table, s shape, theme tui.Theme) []string {
	style := theme.Style(tui.RoleTitle)
	perCol := make(map[int][]string, len(s.visible))
	for _, i := range s.visible {
		perCol[i] = titleLines(t, i, s.widths[i])
	}
	var out []string
	if s.group {
		out = append(out, groupLine(t, s, style.Render))
	}
	for line := 0; line < s.titleH; line++ {
		var b strings.Builder
		b.WriteString(strings.Repeat(" ", gutterWidth))
		for n, i := range s.visible {
			if n > 0 {
				b.WriteString(strings.Repeat(" ", gapWidth))
			}
			lines := perCol[i]
			text := ""
			if k := line - (s.titleH - len(lines)); k >= 0 {
				text = lines[k]
			}
			b.WriteString(style.Render(textwidth.Pad(text, s.widths[i], t.Columns[i].Align == AlignRight)))
		}
		if s.hidden > 0 && line == s.titleH-1 {
			b.WriteString(strings.Repeat(" ", gapWidth))
			b.WriteString(style.Render(markerText(s.hidden)))
		}
		out = append(out, b.String())
	}
	return out
}

// groupLine draws one label over each run of adjacent visible columns sharing a Group.
func groupLine(t Table, s shape, styleFn func(...string) string) string {
	var b strings.Builder
	pos := 0
	put := func(n int, text string) {
		b.WriteString(strings.Repeat(" ", n))
		b.WriteString(text)
	}
	col := gutterWidth
	starts := make([]int, len(s.visible))
	for n, i := range s.visible {
		starts[n] = col
		col += s.widths[i] + gapWidth
	}
	for n := 0; n < len(s.visible); {
		label := t.Columns[s.visible[n]].Group
		end := n
		for label != "" && end+1 < len(s.visible) && t.Columns[s.visible[end+1]].Group == label {
			end++
		}
		if label != "" {
			stop := starts[end] + s.widths[s.visible[end]]
			put(starts[n]-pos, styleFn(groupSpan(label, stop-starts[n])))
			pos = stop
		}
		n = end + 1
	}
	return b.String()
}

// groupSpan centres label in width cells between rules; a label that does not fit is cut with an ellipsis.
func groupSpan(label string, width int) string {
	if textwidth.Of(label) > width {
		return textwidth.Pad(label, width, false)
	}
	rule := "-"
	if tui.CurrentRenderMode().Glyphs != tui.GlyphASCII {
		rule = "─"
	}
	free := width - textwidth.Of(label)
	left := free / 2
	return strings.Repeat(rule, left) + label + strings.Repeat(rule, free-left)
}
