package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"

	tuicommon "mosaic-common/tui"
)

const (
	selectHelpText   = "↑/k up  ↓/j down  enter select  esc cancel"
	selectScrollHint = "  pgup/pgdn scroll"
	minDetailRows    = 1
)

// wrapDetail splits text into screen lines of at most width columns. Every
// line break of text is kept (an empty line stays an empty line), lines are
// word-wrapped, and a word longer than width is broken.
func wrapDetail(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		out = append(out, wrapLine(line, width)...)
	}
	return out
}

func wrapLine(line string, width int) []string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	cur := ""
	for _, w := range words {
		for runewidth.StringWidth(w) > width {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			head, rest := splitAtWidth(w, width)
			out = append(out, head)
			w = rest
		}
		switch {
		case cur == "":
			cur = w
		case runewidth.StringWidth(cur)+1+runewidth.StringWidth(w) <= width:
			cur += " " + w
		default:
			out = append(out, cur)
			cur = w
		}
	}
	return append(out, cur)
}

// splitAtWidth cuts s after the longest prefix that fits width columns (at
// least one rune, so progress is always made).
func splitAtWidth(s string, width int) (string, string) {
	used := 0
	for i, r := range s {
		rw := runewidth.RuneWidth(r)
		if used+rw > width && i > 0 {
			return s[:i], s[i:]
		}
		used += rw
	}
	return s, ""
}

// lineCount is the number of screen lines a rendered block takes.
func lineCount(rendered string) int {
	return strings.Count(rendered, "\n") + 1
}

// optionRows estimates the screen lines the options (and the skip entry) take.
func (s *inlineSelectOne) optionRows() int {
	labelWidth := s.width - 2
	if labelWidth < 1 {
		labelWidth = 1
	}
	rows := 0
	for _, o := range s.q.Options {
		rows += len(wrapDetail(o.Label, labelWidth))
	}
	if s.q.AllowSkip {
		rows++
	}
	return rows
}

// detailRegionRows is how many detail lines fit beside the other parts of the
// overlay (title, prompt, options, help), never fewer than one.
func (s *inlineSelectOne) detailRegionRows(head string, helpLines int) int {
	rows := s.height - lineCount(head) - s.optionRows() - helpLines
	if rows < minDetailRows {
		rows = minDetailRows
	}
	return rows
}

// detailLayout wraps the detail and decides how much of it is shown. It
// returns the lines in view, the help text, and the region height.
func (s *inlineSelectOne) detailLayout(head string) (shown []string, help string, rows int) {
	lines := wrapDetail(s.q.Detail, s.width)
	help = selectHelpText
	rows = s.detailRegionRows(head, lineCount(s.renderHelp(help)))
	if len(lines) <= rows {
		s.detailOffset = 0
		return lines, help, rows
	}
	help += selectScrollHint
	rows = s.detailRegionRows(head, lineCount(s.renderHelp(help)))
	if max := len(lines) - rows; s.detailOffset > max {
		s.detailOffset = max
	}
	if s.detailOffset < 0 {
		s.detailOffset = 0
	}
	return lines[s.detailOffset : s.detailOffset+rows], help, rows
}

func (s *inlineSelectOne) renderHelp(text string) string {
	return s.styles.Style(tuicommon.RoleHelp).Width(s.width).Render(text)
}

// scrollDetail moves the detail region by one page; dir is -1 (up) or 1 (down).
func (s *inlineSelectOne) scrollDetail(dir int) {
	if s.q.Detail == "" {
		return
	}
	_, _, rows := s.detailLayout(s.headView())
	s.detailOffset += dir * rows
	if s.detailOffset < 0 {
		s.detailOffset = 0
	}
	s.detailLayout(s.headView()) // clamps the upper bound
}
