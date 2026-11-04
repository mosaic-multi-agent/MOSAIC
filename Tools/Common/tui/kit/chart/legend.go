package chart

import (
	"fmt"
	"strings"

	"mosaic-common/tui"
	"mosaic-common/tui/kit/textwidth"
)

const legendSeparator = "  "

const sepWidth = len(legendSeparator)

type legendEntry struct {
	swatch string
	role   tui.StyleRole
	text   string // label and share, without the swatch
	width  int    // cells of the whole entry, capped at the legend width
}

// Legend renders "label NN%" per segment, in segment order, wrapped between entries only.
// It returns at most height lines of exactly width cells; entries that do not fit are
// replaced by a "+N more" indicator on the last line.
func Legend(segs []Segment, width, height int, theme tui.Theme) string {
	if width <= 0 || height <= 0 || len(segs) == 0 {
		return ""
	}
	entries := legendEntries(segs, width)
	lines := packEntries(entries, width)
	var out []string
	if len(lines) <= height {
		for _, l := range lines {
			out = append(out, renderLegendLine(entries, l, "", width, theme))
		}
		return strings.Join(out, "\n")
	}
	first := 0
	for _, l := range lines[:height-1] {
		out = append(out, renderLegendLine(entries, l, "", width, theme))
		first += len(l)
	}
	out = append(out, overflowLine(entries, first, width, theme))
	return strings.Join(out, "\n")
}

func legendEntries(segs []Segment, width int) []legendEntry {
	total := 0.0
	for _, s := range segs {
		total += positive(s.Value)
	}
	entries := make([]legendEntry, len(segs))
	for i, s := range segs {
		text := sharePercent(positive(s.Value), total)
		if s.Label != "" {
			text = s.Label + " " + text
		}
		e := legendEntry{swatch: swatch(i), role: s.Role, text: text}
		e.width = min(textwidth.Of(e.swatch+" "+e.text), width)
		entries[i] = e
	}
	return entries
}

func sharePercent(v, total float64) string {
	if total <= 0 || v <= 0 {
		return "0%"
	}
	p := roundHalfUp(v / total * 100)
	if p == 0 {
		return "<1%"
	}
	return fmt.Sprintf("%d%%", p)
}

// packEntries greedily fills lines of at most width cells; each line holds entry indexes.
func packEntries(entries []legendEntry, width int) [][]int {
	var lines [][]int
	var cur []int
	used := 0
	for i, e := range entries {
		if len(cur) > 0 && used+sepWidth+e.width > width {
			lines = append(lines, cur)
			cur, used = nil, 0
		}
		if len(cur) > 0 {
			used += sepWidth
		}
		cur = append(cur, i)
		used += e.width
	}
	return append(lines, cur)
}

func moreText(n int) string { return fmt.Sprintf("+%d more", n) }

// overflowLine fills the last permitted line with as many entries from first on as leave
// room for the "+N more" indicator.
func overflowLine(entries []legendEntry, first, width int, theme tui.Theme) string {
	used := 0
	var picked []int
	for i := first; i < len(entries); i++ {
		sep := 0
		if len(picked) > 0 {
			sep = sepWidth
		}
		need := used + sep + entries[i].width
		if i < len(entries)-1 {
			need += sepWidth + textwidth.Of(moreText(len(entries)-i-1))
		}
		if need > width {
			break
		}
		used += sep + entries[i].width
		picked = append(picked, i)
	}
	remaining := len(entries) - first - len(picked)
	if remaining == 0 {
		return renderLegendLine(entries, picked, "", width, theme)
	}
	return renderLegendLine(entries, picked, moreText(remaining), width, theme)
}

// renderLegendLine draws the entries and an optional trailing indicator padded to width cells.
func renderLegendLine(entries []legendEntry, idx []int, indicator string, width int, theme tui.Theme) string {
	var b strings.Builder
	used := 0
	for k, i := range idx {
		if k > 0 {
			b.WriteString(legendSeparator)
			used += sepWidth
		}
		e := entries[i]
		plain := textwidth.Truncate(e.swatch+" "+e.text, width)
		if rest, ok := strings.CutPrefix(plain, e.swatch); ok {
			b.WriteString(paint(theme, e.role, e.swatch) + paint(theme, tui.RoleBody, rest))
		} else {
			b.WriteString(paint(theme, e.role, plain))
		}
		used += textwidth.Of(plain)
	}
	if indicator != "" {
		if len(idx) > 0 {
			b.WriteString(legendSeparator)
			used += sepWidth
		}
		text := textwidth.Truncate(indicator, width-used)
		b.WriteString(paint(theme, tui.RoleMuted, text))
		used += textwidth.Of(text)
	}
	b.WriteString(spaces(width - used))
	return b.String()
}
