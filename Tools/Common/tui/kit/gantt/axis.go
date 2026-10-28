package gantt

import (
	"math/bits"
	"time"

	"mosaic-common/tui"
)

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func (a Axis) empty() bool { return !a.End.After(a.Start) }

// Narrow returns the axis restricted to [from, to] intersected with a.
// A range disjoint from a gives an empty axis.
func (a Axis) Narrow(from, to time.Time) Axis {
	s, e := maxTime(a.Start, from), minTime(a.End, to)
	if e.Before(s) {
		return Axis{}
	}
	return Axis{Start: s, End: e}
}

// Column maps t to a cell index in [0, width) (clamped); ok is false when the axis is empty.
func (a Axis) Column(t time.Time, width int) (col int, ok bool) {
	if a.empty() || width <= 0 {
		return 0, false
	}
	q, _ := a.scaled(t, width)
	return min(q, width-1), true
}

// scaled returns floor((t - Start) / D * width), with t clamped to the axis, and whether the
// division was exact. The product is computed in 128 bits so long axes cannot overflow.
func (a Axis) scaled(t time.Time, width int) (q int, exact bool) {
	d := uint64(a.End.Sub(a.Start))
	off := t.Sub(a.Start)
	switch {
	case off <= 0:
		return 0, true
	case uint64(off) >= d:
		return width, true
	}
	hi, lo := bits.Mul64(uint64(off), uint64(width))
	quo, rem := bits.Div64(hi, lo, d)
	return int(quo), rem == 0
}

// coverage returns the cells [first, last] a closed span covers on a plotWidth-cell plot area.
// ok is false when the span lies outside the axis.
func (a Axis) coverage(start, end time.Time, plotWidth int) (first, last int, ok bool) {
	if end.Before(start) {
		end = start
	}
	if start.After(a.End) || end.Before(a.Start) {
		return 0, 0, false
	}
	s, e := maxTime(start, a.Start), minTime(end, a.End)
	first, _ = a.Column(s, plotWidth)
	if s.Equal(e) {
		return first, first, true
	}
	q, exact := a.scaled(e, plotWidth)
	last = q
	if exact {
		last = q - 1
	}
	return first, min(max(last, first), plotWidth-1), true
}

// RenderAxis renders one line of tick labels for the plot area.
// The line is exactly width cells: labelWidth+1 blanks, then the plot area.
func RenderAxis(a Axis, labelWidth, width int, theme tui.Theme) string {
	if width <= 0 {
		return ""
	}
	offset := max(labelWidth, 0) + 1
	plotWidth := width - offset
	if plotWidth < 1 || a.empty() {
		return spaces(width)
	}
	labels := tickLabels(a, plotWidth)
	line, used := spaces(offset), 0
	for _, l := range labels {
		line += spaces(l.col-used) + paint(theme, tui.RoleMuted, l.text)
		used = l.col + len(l.text)
	}
	return line + spaces(plotWidth-used)
}
