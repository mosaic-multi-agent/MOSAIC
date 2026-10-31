package analysis_test

// Tests for the intervals the Timeline shades: each actor's own waits on the user
// and the between-sessions intervals that exactly make up the Overview's figure.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func wtActor(id string) domain.ActorRef {
	return domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(id), Type: "coder"}
}

// wtWaitsOf returns the wait intervals of one actor, or nil and false.
func wtWaitsOf(split domain.WallTimeSplit, actor domain.ActorRef) ([]domain.TimeInterval, bool) {
	for _, w := range split.ActorWaits {
		if w.Actor.Kind == actor.Kind && w.Actor.Instance == actor.Instance {
			return w.Intervals, true
		}
	}
	return nil, false
}

func wtRequireWaitsOf(t *testing.T, split domain.WallTimeSplit, actor domain.ActorRef, want [][2]int) {
	t.Helper()
	got, ok := wtWaitsOf(split, actor)
	if !ok {
		t.Fatalf("no waits for %v among %d actors, want %v", actor.Label(), len(split.ActorWaits), want)
	}
	wtAssertIntervals(t, "waits of "+actor.Label(), got, want)
}

func wtTotal(ivs []domain.TimeInterval) time.Duration {
	var d time.Duration
	for _, iv := range ivs {
		d += iv.End.Sub(iv.Start)
	}
	return d
}

// wtUnion merges the intervals of every actor's waits into sorted, disjoint intervals.
func wtUnion(split domain.WallTimeSplit) []domain.TimeInterval {
	var all []domain.TimeInterval
	for _, w := range split.ActorWaits {
		all = append(all, w.Intervals...)
	}
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if all[j].Start.Before(all[i].Start) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
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

func TestActorWaits_OrchestratorWaitDuringASubagentsWork_BelongsToTheOrchestratorOnly(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 20, 30)}
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 60}}

	got := wtSplit(t, spec)

	wtRequireWaitsOf(t, got, domain.Orchestrator(), [][2]int{{20, 30}})
	if _, ok := wtWaitsOf(got, wtActor("a#1")); ok {
		t.Errorf("a#1 did not wait, but has wait intervals: %v", got.ActorWaits)
	}
}

func TestActorWaits_SubagentsOwnWait_BelongsToThatSubagentOnly(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{
		{id: "a#1", start: 10, end: 60, tools: []domain.ToolCall{wtTool("w1", ask, 20, 30)}},
		{id: "b#2", start: 15, end: 70},
	}

	got := wtSplit(t, spec)

	wtRequireWaitsOf(t, got, wtActor("a#1"), [][2]int{{20, 30}})
	if _, ok := wtWaitsOf(got, wtActor("b#2")); ok {
		t.Errorf("b#2 ran in parallel but did not wait; it must have no wait intervals: %v", got.ActorWaits)
	}
	if _, ok := wtWaitsOf(got, domain.Orchestrator()); ok {
		t.Errorf("the orchestrator did not wait; it must have no wait intervals: %v", got.ActorWaits)
	}
}

func TestActorWaits_AreOrchestratorFirstThenInvocationsByInstanceId(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w0", ask, 5, 8)}
	spec.agents = []wtAgent{
		{id: "z#9", start: 10, end: 90, tools: []domain.ToolCall{wtTool("w1", ask, 40, 45)}},
		{id: "a#1", start: 10, end: 90, tools: []domain.ToolCall{wtTool("w2", ask, 20, 25)}},
		{id: "m#5", start: 10, end: 90},
	}

	got := wtSplit(t, spec)

	var order []string
	for _, w := range got.ActorWaits {
		order = append(order, w.Actor.Label())
	}
	want := []string{domain.Orchestrator().Label(), "a#1", "z#9"}
	if len(order) != len(want) {
		t.Fatalf("actors with waits = %v, want %v (actors without waits are left out)", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("actor order = %v, want %v", order, want)
			break
		}
	}
}

func TestActorWaits_OverlappingWaitsOfTwoActors_KeepEachActorsOwnIntervals(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 10, 30)}
	spec.agents = []wtAgent{{id: "a#1", start: 5, end: 80, tools: []domain.ToolCall{wtTool("w2", ask, 20, 40)}}}

	got := wtSplit(t, spec)

	wtRequireWaitsOf(t, got, domain.Orchestrator(), [][2]int{{10, 30}})
	wtRequireWaitsOf(t, got, wtActor("a#1"), [][2]int{{20, 40}})
}

func TestActorWaits_OverlappingWaitsOfOneActor_AreMerged(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 30, 50), wtTool("w2", ask, 10, 35), wtTool("w3", ask, 70, 75)}

	got := wtSplit(t, spec)

	wtRequireWaitsOf(t, got, domain.Orchestrator(), [][2]int{{10, 50}, {70, 75}})
}

func TestActorWaits_UnionOfAllActorsEqualsTheOverviewWaitingIntervalsAndTotal(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 10, 30), wtTool("w4", ask, 85, 90)}
	spec.agents = []wtAgent{
		{id: "a#1", start: 5, end: 80, tools: []domain.ToolCall{wtTool("w2", ask, 20, 40)}},
		{id: "b#2", start: 50, end: 95, tools: []domain.ToolCall{wtTool("w3", ask, 60, 70)}},
	}

	got := wtSplit(t, spec)

	union := wtUnion(got)
	if len(union) == 0 {
		t.Fatalf("no per-actor waits, want the waits of three actors")
	}
	wtAssertIntervals(t, "union of the per-actor waits", union, [][2]int{{10, 40}, {60, 70}, {85, 90}})
	wtAssertIntervals(t, "WaitIntervals", got.WaitIntervals, [][2]int{{10, 40}, {60, 70}, {85, 90}})
	if wtTotal(union) != got.Waiting {
		t.Errorf("union totals %v, want the Overview waiting time %v", wtTotal(union), got.Waiting)
	}
}

func TestActorWaits_PairedNotification_BelongsToTheActorOfItsEarliestTimedNotification(t *testing.T) {
	cases := []struct {
		name       string
		first      domain.ActorRef
		second     domain.ActorRef
		wantActor  domain.ActorRef
		otherActor domain.ActorRef
	}{
		{"subagent asks first", wtActor("a#1"), domain.Orchestrator(), wtActor("a#1"), domain.Orchestrator()},
		{"orchestrator asks first", domain.Orchestrator(), wtActor("a#1"), domain.Orchestrator(), wtActor("a#1")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := newTestWallSpec(0, 100)
			spec.agents = []wtAgent{{id: "a#1", start: 10, end: 80}}
			spec.notifs = []domain.Notification{
				wtNotification(tc.second, 40, "permission_resolved", "n1"),
				wtNotification(tc.first, 25, "permission_prompt", "n1"),
			}

			got := wtSplit(t, spec)

			wtRequireWaitsOf(t, got, tc.wantActor, [][2]int{{25, 40}})
			if _, ok := wtWaitsOf(got, tc.otherActor); ok {
				t.Errorf("%s did not ask; it must have no wait intervals: %v", tc.otherActor.Label(), got.ActorWaits)
			}
		})
	}
}

func TestActorWaits_WithoutWaits_IsEmpty(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 60}}

	got := wtSplit(t, spec)

	if len(got.ActorWaits) != 0 {
		t.Errorf("ActorWaits = %v, want none for a run without waits", got.ActorWaits)
	}
}

func TestBetweenSessionIntervals_AreTheSessionGapWithoutWaitsAndSubagentTime(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 30), wtSession("s2", 50, 100)}
	spec.agents = []wtAgent{{id: "a#1", start: 35, end: 45}}
	spec.orchTools = []domain.ToolCall{wtTool("w1", ask, 47, 60)}

	got := wtSplit(t, spec)

	// The gap 30..50 minus the subagent 35..45 and the wait 47..60 leaves 30..35 and 45..47.
	wtAssertIntervals(t, "BetweenSessionIntervals", got.BetweenSessionIntervals, [][2]int{{30, 35}, {45, 47}})
	if total := wtTotal(got.BetweenSessionIntervals); total != got.BetweenSessions || total != 7*time.Second {
		t.Errorf("between-session intervals total %v, want %v and 7s", total, got.BetweenSessions)
	}
}

func TestBetweenSessionIntervals_NeverOverlapSubagentWorkingTime(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s1", 0, 20), wtSession("s2", 80, 100)}
	spec.agents = []wtAgent{{id: "a#1", start: 10, end: 90}}

	got := wtSplit(t, spec)

	if len(got.BetweenSessionIntervals) != 0 || got.BetweenSessions != 0 {
		t.Errorf("a subagent ran through the whole gap: intervals %v, between sessions %v, want none and 0",
			got.BetweenSessionIntervals, got.BetweenSessions)
	}
	if len(got.SessionGaps) == 0 {
		t.Errorf("SessionGaps must keep the raw list of gaps")
	}
}

func TestBetweenSessionIntervals_AreSortedWhenSessionsAreLoggedOutOfOrder(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.sessions = []domain.RunSession{wtSession("s3", 70, 100), wtSession("s1", 0, 20), wtSession("s2", 40, 50)}

	got := wtSplit(t, spec)

	wtAssertIntervals(t, "BetweenSessionIntervals", got.BetweenSessionIntervals, [][2]int{{20, 40}, {50, 70}})
	if total := wtTotal(got.BetweenSessionIntervals); total != got.BetweenSessions {
		t.Errorf("intervals total %v, want BetweenSessions %v", total, got.BetweenSessions)
	}
}

func TestBetweenSessionIntervals_WithoutSessionGaps_IsEmpty(t *testing.T) {
	got := wtSplit(t, newTestWallSpec(0, 100))

	if len(got.BetweenSessionIntervals) != 0 {
		t.Errorf("BetweenSessionIntervals = %v, want none", got.BetweenSessionIntervals)
	}
}
