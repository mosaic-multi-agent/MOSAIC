package gantt

import "time"

const day = 24 * time.Hour

// stepLadder lists the tick steps up to one day; larger steps are whole multiples of a day.
var stepLadder = []time.Duration{
	time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 30 * time.Second,
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute, 30 * time.Minute,
	time.Hour, 2 * time.Hour, 3 * time.Hour, 6 * time.Hour, 12 * time.Hour, day,
}

type tickLabel struct {
	col  int
	text string
}

// tickLabels lays the axis labels out on a plotWidth-cell plot area. Every label has the same
// length, labels do not overlap, and a label that would run past the plot area is left out.
func tickLabels(a Axis, plotWidth int) []tickLabel {
	loc := a.Start.Location()
	crossesDate := !sameDate(a.Start, a.End.In(loc))
	d, scaleWidth := a.End.Sub(a.Start), plotWidth
	if p, ok := a.collapsePlan(plotWidth); ok {
		d, scaleWidth = p.linear, p.cells
	}
	step, layout := chooseStep(d, scaleWidth, crossesDate)
	length := len(layout)
	cols, stretches := a.markerColumns(plotWidth)
	var labels []tickLabel
	usedEnd := 0
	for _, tick := range axisTicks(a, plotWidth, step) {
		col, _ := a.Column(tick, plotWidth)
		if col+length > plotWidth || tickBlocked(tick, col, length, usedEnd, cols, stretches) {
			continue
		}
		labels = append(labels, tickLabel{col: col, text: tick.In(loc).Format(layout)})
		usedEnd = col + length
	}
	if len(labels) == 0 && length <= plotWidth {
		labels = append(labels, tickLabel{col: 0, text: a.Start.Format(layout)})
	}
	return labels
}

func sameDate(x, y time.Time) bool {
	xy, xm, xd := x.Date()
	yy, ym, yd := y.Date()
	return xy == yy && xm == ym && xd == yd
}

func layoutFor(step time.Duration, crossesDate bool) string {
	layout := "15:04"
	if step < time.Minute {
		layout = "15:04:05"
	}
	if crossesDate {
		layout = "01-02 " + layout
	}
	return layout
}

// chooseStep returns the smallest step whose labels sit at least two blanks apart when d of
// time is laid out on plotWidth cells, with the time layout for that step.
func chooseStep(d time.Duration, plotWidth int, crossesDate bool) (time.Duration, string) {
	fits := func(step time.Duration) bool {
		need := int64(len(layoutFor(step, crossesDate)) + 2)
		return int64(step)*int64(plotWidth) >= need*int64(d)
	}
	for _, step := range stepLadder {
		if fits(step) {
			return step, layoutFor(step, crossesDate)
		}
	}
	step := 2 * day
	for !fits(step) {
		step += day
	}
	return step, layoutFor(step, crossesDate)
}

// ticks returns the tick times in [a.Start, a.End]: whole multiples of step after local
// midnight, or for steps over a day every n-th local midnight from the first one at or after Start.
func ticks(a Axis, step time.Duration) []time.Time {
	y, m, d := a.Start.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, a.Start.Location())
	var out []time.Time
	if step > day {
		next := midnight
		if next.Before(a.Start) {
			next = next.AddDate(0, 0, 1)
		}
		days := int(step / day)
		for ; !next.After(a.End); next = next.AddDate(0, 0, days) {
			out = append(out, next)
		}
		return out
	}
	elapsed := a.Start.Sub(midnight)
	k := elapsed / step
	if k*step < elapsed {
		k++
	}
	for t := midnight.Add(k * step); !t.After(a.End); t = t.Add(step) {
		out = append(out, t)
	}
	return out
}
