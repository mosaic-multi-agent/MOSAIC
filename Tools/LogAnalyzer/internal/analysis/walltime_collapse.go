package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// collapseLimit is the length a stretch must exceed to be drawn collapsed on the Timeline.
const collapseLimit = 10 * time.Minute

// collapsibleStretches chooses the stretches the Timeline draws collapsed: one actor's wait on
// the user, or one between-sessions interval, longer than collapseLimit, during which no
// invocation other than the waiting actor itself was open for a positive length. Each interval
// is taken alone, so touching intervals are never joined. Stretches that reach the span's start
// or end are left out. The result is sorted and non-overlapping, and is for drawing only.
func collapsibleStretches(actorWaits []domain.ActorWaitIntervals, between []domain.TimeInterval, invs []domain.Invocation, axis domain.TimeInterval) []domain.TimeInterval {
	var cands []domain.TimeInterval
	for _, aw := range actorWaits {
		for _, iv := range aw.Intervals {
			if !workedDuring(invs, axis.End, iv, aw.Actor) {
				cands = append(cands, iv)
			}
		}
	}
	for _, iv := range between {
		if !workedDuring(invs, axis.End, iv, domain.ActorRef{}) {
			cands = append(cands, iv)
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Start.Before(cands[j].Start) })
	var out []domain.TimeInterval
	for _, iv := range cands {
		if n := len(out); n > 0 && iv.Start.Before(out[n-1].End) {
			iv.Start = out[n-1].End
		}
		if iv.End.Sub(iv.Start) > collapseLimit && iv.Start.After(axis.Start) && iv.End.Before(axis.End) {
			out = append(out, iv)
		}
	}
	return out
}

// workedDuring reports whether an invocation other than self was open for a positive length
// within iv. Self is the waiting actor; the zero ActorRef (between sessions) matches nothing.
func workedDuring(invs []domain.Invocation, axisEnd time.Time, iv domain.TimeInterval, self domain.ActorRef) bool {
	for _, inv := range invs {
		if self.Kind == domain.ActorAgentInstance && inv.Actor.Kind == self.Kind && inv.Actor.Instance == self.Instance {
			continue
		}
		span, ok := invocationInterval(inv, axisEnd)
		if !ok {
			continue
		}
		start, end := span.Start, span.End
		if iv.Start.After(start) {
			start = iv.Start
		}
		if iv.End.Before(end) {
			end = iv.End
		}
		if end.After(start) {
			return true
		}
	}
	return false
}
