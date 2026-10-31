package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// addInterval records one wait and the actor it belongs to.
func (w *waitCollector) addInterval(actor domain.ActorRef, iv domain.TimeInterval) {
	w.intervals = append(w.intervals, iv)
	w.owners = append(w.owners, actor)
}

// actorWaits groups the waits by the actor that made them: merged, sorted and
// clipped to the axis; the orchestrator first, then invocations by instance id.
// Actors without waits inside the axis are left out.
func (w *waitCollector) actorWaits(axis domain.TimeInterval) []domain.ActorWaitIntervals {
	type key struct {
		kind     domain.ActorKind
		instance domain.AgentInstanceID
	}
	byActor := map[key][]domain.TimeInterval{}
	refs := map[key]domain.ActorRef{}
	for i, iv := range w.intervals {
		a := w.owners[i]
		k := key{a.Kind, a.Instance}
		if r, ok := refs[k]; !ok || r.Type == "" {
			refs[k] = a
		}
		byActor[k] = append(byActor[k], iv)
	}
	var out []domain.ActorWaitIntervals
	for k, ivs := range byActor {
		if merged := clipIntervals(mergeIntervals(ivs), axis); len(merged) > 0 {
			out = append(out, domain.ActorWaitIntervals{Actor: refs[k], Intervals: merged})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Actor, out[j].Actor
		if (a.Kind == domain.ActorAgentInstance) != (b.Kind == domain.ActorAgentInstance) {
			return a.Kind != domain.ActorAgentInstance
		}
		return a.Instance < b.Instance
	})
	return out
}

// betweenSessionIntervals returns the session gaps that remain once waits and
// subagent time are taken out; their total is the Overview's between-sessions figure.
func betweenSessionIntervals(gaps, waits, subagents []domain.TimeInterval, axis domain.TimeInterval) []domain.TimeInterval {
	return subtractIntervals(subtractIntervals(clipIntervals(mergeIntervals(gaps), axis), waits), subagents)
}

