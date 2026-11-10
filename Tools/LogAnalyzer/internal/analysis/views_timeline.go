package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// invocationInterval is the span an invocation occupies on the timeline: its
// logged start to its logged end, or to the axis end while it is open. ok is
// false when the invocation has no start or no usable end.
func invocationInterval(inv domain.Invocation, axisEnd time.Time) (domain.TimeInterval, bool) {
	if inv.StartedAt.IsZero() {
		return domain.TimeInterval{}, false
	}
	if inv.State == domain.InvocationOpen {
		return domain.TimeInterval{Start: inv.StartedAt, End: axisEnd}, true
	}
	if _, ok := inv.Duration(); !ok {
		return domain.TimeInterval{}, false
	}
	return domain.TimeInterval{Start: inv.StartedAt, End: inv.EndedAt}, true
}

// buildTimeline places the run's subagent invocations on a timeline using only
// logged timestamps: ordered by start, with unstarted invocations last. An open
// invocation runs to the axis end, like its Gantt bar, for the overlaps.
func buildTimeline(run domain.RunAggregate) domain.Timeline {
	act := run.Activity
	axis, hasAxis := runAxis(run)
	entries := make([]domain.TimelineEntry, 0, len(act.Invocations))
	for _, inv := range act.Invocations {
		e := domain.TimelineEntry{Invocation: inv}
		e.Duration, e.HasDuration = inv.Duration()
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].Invocation, entries[j].Invocation
		if a.StartedAt.IsZero() != b.StartedAt.IsZero() {
			return !a.StartedAt.IsZero()
		}
		if !a.StartedAt.Equal(b.StartedAt) {
			return a.StartedAt.Before(b.StartedAt)
		}
		return a.Actor.Instance < b.Actor.Instance
	})
	intervals := make([]domain.TimeInterval, len(entries))
	usable := make([]bool, len(entries))
	for i := range entries {
		intervals[i], usable[i] = invocationInterval(entries[i].Invocation, axis.End)
	}
	for i := range entries {
		if !usable[i] {
			continue
		}
		for j := range entries {
			if i == j || !usable[j] {
				continue
			}
			if intervals[i].Start.Before(intervals[j].End) && intervals[j].Start.Before(intervals[i].End) {
				entries[i].Overlaps = append(entries[i].Overlaps, entries[j].Invocation.Actor.Instance)
			}
		}
		sort.Slice(entries[i].Overlaps, func(x, y int) bool { return entries[i].Overlaps[x] < entries[i].Overlaps[y] })
	}
	return domain.Timeline{Entries: entries, Compactions: act.Compactions, Axis: axis, HasAxis: hasAxis}
}
