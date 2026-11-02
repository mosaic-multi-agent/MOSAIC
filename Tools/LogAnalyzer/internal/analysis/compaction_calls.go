package analysis

import (
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// applyCompactionCalls attributes an actor's compaction calls (OTel calls of
// kind compaction) to its compactions. Marks must already be in time order.
//
// A detected drop preceded by compaction calls since the actor's previous
// compaction becomes a harness compaction carrying their count and cost; a
// later match to a logged compaction clears the Harness flag again. A
// compaction call that no compaction of the actor follows is an unapplied
// attempt; a call without a time is never attributed and always unapplied.
func applyCompactionCalls(a *domain.ActorContext) {
	var all, unapplied []domain.MoneyValue
	for i := range a.Calls {
		c := &a.Calls[i]
		if c.Kind != domain.CallCompaction {
			continue
		}
		all = append(all, c.Cost)
		if !followedByCompaction(a.Compactions, c.At) {
			c.Unapplied = true
			unapplied = append(unapplied, c.Cost)
		}
	}
	a.CompactionCalls.Count = len(all)
	a.CompactionCalls.UnappliedCount = len(unapplied)
	a.CompactionCalls.Cost, _ = domain.SumMoney(all...)
	a.CompactionCalls.UnappliedCost, _ = domain.SumMoney(unapplied...)
	attributeToDrops(a)
}

// followedByCompaction reports whether a timed compaction mark lies at or
// after t. An unknown t is never followed.
func followedByCompaction(marks []domain.CompactionMark, t time.Time) bool {
	if t.IsZero() {
		return false
	}
	for _, m := range marks {
		if !m.At.IsZero() && !m.At.Before(t) {
			return true
		}
	}
	return false
}

// attributeToDrops gives each detected drop the compaction calls made after
// the previous timed compaction and before the drop.
func attributeToDrops(a *domain.ActorContext) {
	var prevAt time.Time
	for i := range a.Compactions {
		m := &a.Compactions[i]
		if m.At.IsZero() {
			continue
		}
		if m.Kind == domain.CompactionDetected {
			var costs []domain.MoneyValue
			for _, c := range a.Calls {
				if c.Kind != domain.CallCompaction || c.At.IsZero() ||
					!c.At.Before(m.At) || (!prevAt.IsZero() && !c.At.After(prevAt)) {
					continue
				}
				costs = append(costs, c.Cost)
			}
			if len(costs) > 0 {
				m.Harness = true
				m.CallCount = len(costs)
				m.CallCost, _ = domain.SumMoney(costs...)
			}
		}
		prevAt = m.At
	}
}
