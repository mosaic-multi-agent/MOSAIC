package gantt

import (
	"fmt"
	"math/bits"
	"sort"
	"strings"
	"time"

	"mosaic-common/tui"
)

// FormatCollapseLength renders a stretch length with its two largest non-zero units
// among days, hours and minutes, seconds dropped: "3d 2h", "3d", "4h 5m", "2h", "45m".
func FormatCollapseLength(d time.Duration) string {
	days := int64(d / (24 * time.Hour))
	hours := int64(d % (24 * time.Hour) / time.Hour)
	minutes := int64(d % time.Hour / time.Minute)
	var parts []string
	for _, u := range []struct {
		n    int64
		unit string
	}{{days, "d"}, {hours, "h"}, {minutes, "m"}} {
		if u.n > 0 && len(parts) < 2 {
			parts = append(parts, fmt.Sprintf("%d%s", u.n, u.unit))
		}
	}
	if len(parts) == 0 {
		return "0m"
	}
	return strings.Join(parts, " ")
}

// collapsePlan is the collapsed layout of an axis on one plot width: the stretches that are cut
// out, the time that stays to scale and the cells it is scaled onto.
type collapsePlan struct {
	stretches []Collapse
	linear    time.Duration // time left on the axis once the stretches are cut out
	cells     int           // plot cells left for the linear time
}

// collapsePlan returns the collapsed layout for a plotWidth-cell plot area. ok is false when the
// axis has no usable stretch or the plot is too narrow to give every marker its width and every
// remaining segment a cell; the axis is then linear.
func (a Axis) collapsePlan(plotWidth int) (p collapsePlan, ok bool) {
	if len(a.Collapsed) == 0 || a.empty() {
		return p, false
	}
	prev, segments := a.Start, 1
	linear := a.End.Sub(a.Start)
	for _, c := range a.Collapsed {
		if !c.Start.After(a.Start) || !c.End.Before(a.End) || !c.End.After(c.Start) || c.Start.Before(prev) {
			continue
		}
		if c.Start.After(prev) && len(p.stretches) > 0 {
			segments++
		}
		p.stretches = append(p.stretches, c)
		linear -= c.End.Sub(c.Start)
		prev = c.End
	}
	if len(p.stretches) == 0 || linear <= 0 {
		return collapsePlan{}, false
	}
	segments++
	p.cells = plotWidth - CollapseMarkerWidth*len(p.stretches)
	p.linear = linear
	if p.cells < segments {
		return collapsePlan{}, false
	}
	return p, true
}

// scaled is the collapsed counterpart of Axis.scaled: floor of the cell position of t, with t
// clamped to the axis, and whether the position falls exactly on a cell edge. Time inside a
// stretch maps to the stretch's first marker column.
func (p collapsePlan) scaled(a Axis, t time.Time, plotWidth int) (q int, exact bool) {
	switch {
	case !t.After(a.Start):
		return 0, true
	case !t.Before(a.End):
		return plotWidth, true
	}
	x, markers := t.Sub(a.Start), 0
	for _, c := range p.stretches {
		if !t.After(c.Start) {
			break
		}
		if t.Before(c.End) {
			x -= t.Sub(c.Start)
			break
		}
		x -= c.End.Sub(c.Start)
		markers++
	}
	hi, lo := bits.Mul64(uint64(x), uint64(p.cells))
	quo, rem := bits.Div64(hi, lo, uint64(p.linear))
	return int(quo) + markers*CollapseMarkerWidth, rem == 0
}

// markerColumns returns the first plot column of every collapsed stretch, aligned with the
// returned stretches; both are nil on a linear axis.
func (a Axis) markerColumns(plotWidth int) (cols []int, stretches []Collapse) {
	p, ok := a.collapsePlan(plotWidth)
	if !ok {
		return nil, nil
	}
	cols = make([]int, len(p.stretches))
	for i, c := range p.stretches {
		cols[i], _ = a.Column(c.Start, plotWidth)
	}
	return cols, p.stretches
}

// breakGlyph is the glyph at both ends of a marker in the current render mode.
func breakGlyph() string {
	if tui.CurrentRenderMode().Glyphs == tui.GlyphASCII {
		return ":"
	}
	return "┊"
}

// markerText is the CollapseMarkerWidth cells of one marker on the axis row: a break glyph at
// each end around the centred length of the stretch.
func markerText(c Collapse) string {
	inner := CollapseMarkerWidth - 2
	label := FormatCollapseLength(c.End.Sub(c.Start))
	if len(label) > inner {
		label = label[:inner]
	}
	left := (inner - len(label)) / 2
	return breakGlyph() + spaces(left) + label + spaces(inner-left-len(label)) + breakGlyph()
}

// markerLayer draws the collapse markers of a into one layer of plotWidth cells, or returns nil
// on a linear axis. Marker cells are blank except for the break glyph at both ends.
func markerLayer(a Axis, plotWidth int) []cell {
	cols, _ := a.markerColumns(plotWidth)
	if cols == nil {
		return nil
	}
	cells := make([]cell, plotWidth)
	for _, col := range cols {
		for i := 0; i < CollapseMarkerWidth && col+i < plotWidth; i++ {
			c := cell{glyph: " ", set: true}
			if i == 0 || i == CollapseMarkerWidth-1 {
				c = cell{glyph: breakGlyph(), role: tui.RoleMuted, set: true}
			}
			cells[col+i] = c
		}
	}
	return cells
}

// tickBlocked reports whether a tick label at col of the given length may not be drawn: it
// names time inside a stretch, touches a marker or overlaps the previous label (ending at usedEnd).
func tickBlocked(tick time.Time, col, length, usedEnd int, cols []int, stretches []Collapse) bool {
	if cols == nil {
		return false
	}
	if col < usedEnd {
		return true
	}
	for i, c := range stretches {
		if tick.After(c.Start) && tick.Before(c.End) {
			return true
		}
		if col < cols[i]+CollapseMarkerWidth && col+length > cols[i] {
			return true
		}
	}
	return false
}

// axisTicks returns the tick times of a for step. On a collapsed axis the ticks are generated
// per remaining segment, so a long stretch costs nothing; steps over a day are always few.
func axisTicks(a Axis, plotWidth int, step time.Duration) []time.Time {
	p, ok := a.collapsePlan(plotWidth)
	if !ok || step > day {
		return ticks(a, step)
	}
	var out []time.Time
	from := a.Start
	for _, c := range append(append([]Collapse(nil), p.stretches...), Collapse{Start: a.End, End: a.End}) {
		out = append(out, ticks(Axis{Start: from, End: c.Start}, step)...)
		from = c.End
	}
	return out
}

// axisItems returns everything drawn on the axis row in column order: the tick labels and,
// on a collapsed axis, the stretch markers.
func axisItems(a Axis, plotWidth int) []tickLabel {
	labels := tickLabels(a, plotWidth)
	cols, stretches := a.markerColumns(plotWidth)
	for i, c := range stretches {
		labels = append(labels, tickLabel{col: cols[i], text: markerText(c)})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].col < labels[j].col })
	return labels
}
