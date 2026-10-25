package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// buildTimeline places the run's subagent invocations on a timeline using only
// logged timestamps: ordered by start, with unstarted invocations last.
// Overlaps are computed between invocations that have a usable interval.
func buildTimeline(act domain.RunActivity) domain.Timeline {
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
	for i := range entries {
		if !entries[i].HasDuration {
			continue
		}
		a := entries[i].Invocation
		for j := range entries {
			if i == j || !entries[j].HasDuration {
				continue
			}
			b := entries[j].Invocation
			if a.StartedAt.Before(b.EndedAt) && b.StartedAt.Before(a.EndedAt) {
				entries[i].Overlaps = append(entries[i].Overlaps, b.Actor.Instance)
			}
		}
		sort.Slice(entries[i].Overlaps, func(x, y int) bool { return entries[i].Overlaps[x] < entries[i].Overlaps[y] })
	}
	return domain.Timeline{Entries: entries, Compactions: act.Compactions}
}
