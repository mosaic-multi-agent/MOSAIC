package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// effectiveSessions assigns every call of one actor, given by its session ids in
// time order, the session it belongs to. A call with an id is in that session; a
// call without one joins the nearest preceding call that has an id, or, when none
// precedes it, the nearest following one. When no call has an id all calls form
// one session (the empty id).
func effectiveSessions(ids []string) []string {
	out := make([]string, len(ids))
	current := ""
	first := -1
	for i, id := range ids {
		if id != "" {
			current = id
			if first < 0 {
				first = i
			}
		}
		out[i] = current
	}
	for i := 0; i < first; i++ {
		out[i] = ids[first]
	}
	return out
}

// callSessions is the effective session of each of an actor's context calls.
func callSessions(calls []domain.ContextCall) []string {
	ids := make([]string, len(calls))
	for i, c := range calls {
		ids[i] = c.SessionID
	}
	return effectiveSessions(ids)
}

// lastPerSession returns the last of the given usage records (in time order) of
// every effective session, ordered by position.
func lastPerSession(calls []domain.InvocationUsage) []domain.InvocationUsage {
	ids := make([]string, len(calls))
	for i, c := range calls {
		ids[i] = c.SessionID
	}
	lastIndex := make(map[string]int)
	for i, s := range effectiveSessions(ids) {
		lastIndex[s] = i
	}
	positions := make([]int, 0, len(lastIndex))
	for _, i := range lastIndex {
		positions = append(positions, i)
	}
	sort.Ints(positions)
	out := make([]domain.InvocationUsage, len(positions))
	for k, i := range positions {
		out[k] = calls[i]
	}
	return out
}
