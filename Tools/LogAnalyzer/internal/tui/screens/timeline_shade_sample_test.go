package screens_test

// The shaded intervals of the sample run match the Overview's figures: the waits
// of all actors add up to the waiting time, and the between-sessions intervals
// total the between-sessions figure, which excludes subagent working time.

import (
	"sort"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func sumIntervals(ivs []domain.TimeInterval) time.Duration {
	var d time.Duration
	for _, iv := range ivs {
		d += iv.End.Sub(iv.Start)
	}
	return d
}

// unionIntervals merges intervals into sorted, disjoint ones.
func unionIntervals(in []domain.TimeInterval) []domain.TimeInterval {
	all := append([]domain.TimeInterval(nil), in...)
	sort.Slice(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })
	var out []domain.TimeInterval
	for _, iv := range all {
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

func TestSampleRun_BetweenSessionShadeTotalsTheOverviewFigure(t *testing.T) {
	wt := sampleRun(t).WallTime

	if got := sumIntervals(wt.BetweenSessionIntervals); got != wt.BetweenSessions || got.Round(time.Second) != 398*time.Second {
		t.Errorf("between-session intervals total %v, want the Overview's %v which rounds to 398s", got, wt.BetweenSessions)
	}
}

func TestSampleRun_BetweenSessionShadeNeverCoversAnInvocation(t *testing.T) {
	run := sampleRun(t)
	end := run.WallTime.Span.End

	for _, gap := range run.WallTime.BetweenSessionIntervals {
		for _, e := range run.Timeline.Entries {
			inv := e.Invocation
			if inv.StartedAt.IsZero() {
				continue
			}
			to := inv.EndedAt
			if inv.State == domain.InvocationOpen || to.IsZero() {
				to = end
			}
			if gap.Start.Before(to) && inv.StartedAt.Before(gap.End) {
				t.Errorf("between-session interval %v..%v overlaps %s working %v..%v",
					gap.Start, gap.End, inv.Actor.Label(), inv.StartedAt, to)
			}
		}
	}
}

func TestSampleRun_WaitsOfAllActorsMakeUpTheOverviewWaitingTime(t *testing.T) {
	wt := sampleRun(t).WallTime
	var all []domain.TimeInterval
	for _, w := range wt.ActorWaits {
		all = append(all, w.Intervals...)
	}

	union := unionIntervals(all)

	if len(wt.WaitIntervals) > 0 && len(wt.ActorWaits) == 0 {
		t.Fatalf("the run waited %v but no actor has wait intervals", wt.Waiting)
	}
	if got := sumIntervals(union); got != wt.Waiting {
		t.Errorf("the union of the per-actor waits totals %v, want the Overview's waiting time %v", got, wt.Waiting)
	}
	if len(union) != len(wt.WaitIntervals) {
		t.Fatalf("union has %d intervals, WaitIntervals %d", len(union), len(wt.WaitIntervals))
	}
	for i := range union {
		if !union[i].Start.Equal(wt.WaitIntervals[i].Start) || !union[i].End.Equal(wt.WaitIntervals[i].End) {
			t.Errorf("union[%d] = %v, want %v", i, union[i], wt.WaitIntervals[i])
		}
	}
}
