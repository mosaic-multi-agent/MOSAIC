package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// mergeIntervals returns the sorted union of ivs, joining overlapping or
// touching intervals and dropping empty ones.
func mergeIntervals(ivs []domain.TimeInterval) []domain.TimeInterval {
	sorted := make([]domain.TimeInterval, 0, len(ivs))
	for _, iv := range ivs {
		if iv.End.After(iv.Start) {
			sorted = append(sorted, iv)
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start.Before(sorted[j].Start) })
	var out []domain.TimeInterval
	for _, iv := range sorted {
		if n := len(out); n > 0 && !iv.Start.After(out[n-1].End) {
			if iv.End.After(out[n-1].End) {
				out[n-1].End = iv.End
			}
			continue
		}
		out = append(out, iv)
	}
	return out
}

// subtractIntervals returns a minus b; both must be merged and sorted.
func subtractIntervals(a, b []domain.TimeInterval) []domain.TimeInterval {
	var out []domain.TimeInterval
	for _, iv := range a {
		cur := iv.Start
		for _, cut := range b {
			if !cut.End.After(cur) || !cut.Start.Before(iv.End) {
				continue
			}
			if cut.Start.After(cur) {
				out = append(out, domain.TimeInterval{Start: cur, End: cut.Start})
			}
			cur = cut.End
		}
		if cur.Before(iv.End) {
			out = append(out, domain.TimeInterval{Start: cur, End: iv.End})
		}
	}
	return out
}

// clipIntervals restricts merged intervals to span.
func clipIntervals(ivs []domain.TimeInterval, span domain.TimeInterval) []domain.TimeInterval {
	var out []domain.TimeInterval
	for _, iv := range ivs {
		if iv.Start.Before(span.Start) {
			iv.Start = span.Start
		}
		if iv.End.After(span.End) {
			iv.End = span.End
		}
		if iv.End.After(iv.Start) {
			out = append(out, iv)
		}
	}
	return out
}

func totalIntervals(ivs []domain.TimeInterval) time.Duration {
	var total time.Duration
	for _, iv := range ivs {
		total += iv.End.Sub(iv.Start)
	}
	return total
}

// sessionGaps returns the gaps between a run's sessions, sorted. Sessions
// without a start are skipped; a session without an end leaves no gap after it.
func sessionGaps(sessions []domain.RunSession) []domain.TimeInterval {
	started := make([]domain.RunSession, 0, len(sessions))
	for _, s := range sessions {
		if !s.StartedAt.IsZero() {
			started = append(started, s)
		}
	}
	sort.SliceStable(started, func(i, j int) bool { return started[i].StartedAt.Before(started[j].StartedAt) })
	var gaps []domain.TimeInterval
	var covered time.Time
	seen, unbounded := false, false
	for _, s := range started {
		if seen && !unbounded && s.StartedAt.After(covered) {
			gaps = append(gaps, domain.TimeInterval{Start: covered, End: s.StartedAt})
		}
		seen = true
		switch {
		case s.EndedAt.IsZero():
			unbounded = true
		case s.EndedAt.After(covered):
			covered = s.EndedAt
		}
	}
	return gaps
}
