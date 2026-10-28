package analysis

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

const (
	statusSuccess     = "SUCCESS"
	statusNeedsAction = "COMPLETED_NEEDS_ACTION"
)

// endedSuccessfully reports whether the dispatch closed with status SUCCESS,
// compared exactly as logged.
func endedSuccessfully(d domain.Dispatch) bool {
	return d.Invocation.State == domain.InvocationClosed && d.Invocation.StatusCode == statusSuccess
}

// startOrder returns the dispatch indexes ordered by start time ascending, then
// instance id; dispatches without a start time come last.
func startOrder(dispatches []domain.Dispatch) []int {
	order := make([]int, len(dispatches))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(x, y int) bool {
		a, b := dispatches[order[x]].Invocation, dispatches[order[y]].Invocation
		if a.StartedAt.IsZero() != b.StartedAt.IsZero() {
			return b.StartedAt.IsZero()
		}
		if !a.StartedAt.Equal(b.StartedAt) {
			return a.StartedAt.Before(b.StartedAt)
		}
		return a.Actor.Instance < b.Actor.Instance
	})
	return order
}

// markRework sets Repeat and LikelyRework on every dispatch. A repeat is a
// dispatch whose agent type has an earlier dispatch in start order. A repeat
// with a start time is likely rework when the previous dispatch of its type did
// not end SUCCESS, or when the most recent other dispatch that had ended by its
// start ended COMPLETED_NEEDS_ACTION.
func markRework(dispatches []domain.Dispatch) {
	previous := make(map[string]int)
	for _, i := range startOrder(dispatches) {
		typ := dispatches[i].Invocation.Actor.Type
		prev, seen := previous[typ]
		previous[typ] = i
		if !seen {
			continue
		}
		dispatches[i].Repeat = true
		started := dispatches[i].Invocation.StartedAt
		if started.IsZero() {
			continue
		}
		dispatches[i].LikelyRework = !endedSuccessfully(dispatches[prev]) ||
			recentlyNeededAction(dispatches, i, started)
	}
}

// recentlyNeededAction reports whether, among the dispatches other than self
// that ended at or before at, one with the latest end time ended
// COMPLETED_NEEDS_ACTION.
func recentlyNeededAction(dispatches []domain.Dispatch, self int, at time.Time) bool {
	var latest time.Time
	needed := false
	for j, d := range dispatches {
		ended := d.Invocation.EndedAt
		if j == self || ended.IsZero() || ended.After(at) {
			continue
		}
		switch {
		case latest.IsZero() || ended.After(latest):
			latest = ended
			needed = d.Invocation.StatusCode == statusNeedsAction
		case ended.Equal(latest):
			needed = needed || d.Invocation.StatusCode == statusNeedsAction
		}
	}
	return needed
}

// buildOutcomeSplit divides the dispatches by how each ended.
func buildOutcomeSplit(dispatches []domain.Dispatch) domain.OutcomeSplit {
	var success, other, none []domain.Dispatch
	for _, d := range dispatches {
		switch {
		case endedSuccessfully(d):
			success = append(success, d)
		case d.Invocation.State == domain.InvocationClosed && d.Invocation.StatusCode != "":
			other = append(other, d)
		default:
			none = append(none, d)
		}
	}
	return domain.OutcomeSplit{
		Success:     groupFigures(success),
		OtherStatus: groupFigures(other),
		OpenOrNone:  groupFigures(none),
	}
}

// buildReworkFigures sums the repeat and likely-rework dispatches of a scope.
func buildReworkFigures(dispatches []domain.Dispatch) domain.ReworkFigures {
	var repeats, likely []domain.Dispatch
	for _, d := range dispatches {
		if d.Repeat {
			repeats = append(repeats, d)
		}
		if d.LikelyRework {
			likely = append(likely, d)
		}
	}
	return domain.ReworkFigures{Repeats: groupFigures(repeats), LikelyRework: groupFigures(likely)}
}
