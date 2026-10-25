package analysis_test

// Tests for the timeline view (RunReport.Timeline) over a scenario built in
// memory with explicit timestamps (seconds on the run-model clock).
//
// Rules under test:
//   - Entries are the subagent invocations ordered by start time, then instance
//     id; invocations without a usable start time come last, by instance id.
//   - Durations exist only for closed invocations with both timestamps and
//     end >= start; every other entry keeps its logged timestamps and shows no
//     duration, and nothing is guessed.
//   - Two invocations overlap when A.start < B.end && B.start < A.end; touching
//     intervals do not overlap. Entries without a duration have no overlaps.
//   - Open invocations (no end event) have no end, no duration, no overlaps.
//   - Compactions are carried in time order with their actor.
//
//	instance  type      start  end   status
//	a#1       coder     10     40    SUCCESS
//	h#8       coder     10     20    SUCCESS
//	d#4       reviewer  15     -     (open)
//	b#2       tester    20     50    BLOCKED
//	c#3       coder     40     60    SUCCESS
//	e#5       coder     50     12    SUCCESS  (end before start)
//	f#6       coder     zero   70    SUCCESS  (start timestamp unparseable)
//	g#7       -         -      -     (only token usage; no start, no end)
//
// Compactions: orchestrator auto at 25 (100000 -> 20000), a#1 manual at 30,
// orchestrator manual with an unparseable timestamp.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func tlInvocation(id domain.AgentInstanceID, typ string, start, end *int, status string) analysis.Stream {
	at := func(p *int) time.Time {
		if p == nil {
			return time.Time{}
		}
		return rmAt(*p)
	}
	events := []domain.Event{rmInvStart(id, typ, at(start))}
	if end != nil {
		events = append(events, rmInvEnd(id, status, "", at(end)))
	}
	return analysis.Stream{Ref: agentStream(testRunRef, id), Events: events}
}

func sec(n int) *int { return &n }

func tlScenarioInput() analysis.Input {
	a1 := tlInvocation("a#1", "coder", sec(10), sec(40), "SUCCESS")
	a1.Events = append(a1.Events, rmCompaction("manual", domain.Tokens(5000), domain.Tokens(900), rmAt(30)))
	return analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{
			rmCompaction("manual", domain.TokenCount{}, domain.TokenCount{}, time.Time{}),
			rmCompaction("auto", domain.Tokens(100000), domain.Tokens(20000), rmAt(25)),
		}},
		tlInvocation("c#3", "coder", sec(40), sec(60), "SUCCESS"),
		a1,
		tlInvocation("b#2", "tester", sec(20), sec(50), "BLOCKED"),
		tlInvocation("d#4", "reviewer", sec(15), nil, ""),
		tlInvocation("e#5", "coder", sec(50), sec(12), "SUCCESS"),
		{Ref: agentStream(testRunRef, "f#6"), Events: []domain.Event{
			rmInvStart("f#6", "coder", time.Time{}),
			rmInvEnd("f#6", "SUCCESS", "", rmAt(70)),
		}},
		{Ref: agentStream(testRunRef, "g#7"), Events: []domain.Event{rmUsage("g#7", "g-r1", 10)}},
		tlInvocation("h#8", "coder", sec(10), sec(20), "SUCCESS"),
	}}
}

func tlTimeline(t *testing.T) domain.Timeline {
	t.Helper()
	report := analysis.Analyze(tlScenarioInput(), viewsPricing(t), testSource(), nil)
	return requireRunReport(t, report, testRunID).Timeline
}

func tlEntryIDs(entries []domain.TimelineEntry) []string {
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = string(e.Invocation.Actor.Instance)
	}
	return ids
}

func tlEntry(t *testing.T, tl domain.Timeline, id domain.AgentInstanceID) domain.TimelineEntry {
	t.Helper()
	for _, e := range tl.Entries {
		if e.Invocation.Actor.Instance == id {
			return e
		}
	}
	t.Fatalf("no timeline entry for %q among %v", id, tlEntryIDs(tl.Entries))
	return domain.TimelineEntry{}
}

func tlOverlaps(e domain.TimelineEntry) []string {
	out := make([]string, len(e.Overlaps))
	for i, id := range e.Overlaps {
		out[i] = string(id)
	}
	return out
}

func TestTimeline_Entries_AreOrderedByStartThenInstanceWithUnstartedLast(t *testing.T) {
	tl := tlTimeline(t)

	got := tlEntryIDs(tl.Entries)

	want := []string{"a#1", "h#8", "d#4", "b#2", "c#3", "e#5", "f#6", "g#7"}
	if !rmEqualStrings(got, want) {
		t.Errorf("entry order = %v, want %v", got, want)
	}
}

func TestTimeline_Entries_ExcludeTheOrchestrator(t *testing.T) {
	tl := tlTimeline(t)

	for _, e := range tl.Entries {
		if e.Invocation.Actor.Kind != domain.ActorAgentInstance {
			t.Errorf("entry %+v is not an agent instance", e.Invocation.Actor)
		}
	}
	if len(tl.Entries) != 8 {
		t.Errorf("entries = %d, want 8 subagent invocations", len(tl.Entries))
	}
}

func TestTimeline_Durations_AreEndMinusStartForClosedInvocations(t *testing.T) {
	tl := tlTimeline(t)

	cases := map[domain.AgentInstanceID]time.Duration{
		"a#1": 30 * time.Second, "b#2": 30 * time.Second, "c#3": 20 * time.Second, "h#8": 10 * time.Second,
	}
	for id, want := range cases {
		e := tlEntry(t, tl, id)
		if !e.HasDuration || e.Duration != want {
			t.Errorf("%s: duration = %v (has %v), want %v", id, e.Duration, e.HasDuration, want)
		}
	}
}

func TestTimeline_ParallelInvocations_ListEachOtherAsOverlaps(t *testing.T) {
	tl := tlTimeline(t)

	cases := map[domain.AgentInstanceID][]string{
		"a#1": {"b#2", "h#8"},
		"b#2": {"a#1", "c#3"},
		"c#3": {"b#2"},
		"h#8": {"a#1"},
	}
	for id, want := range cases {
		if got := tlOverlaps(tlEntry(t, tl, id)); !rmEqualStrings(got, want) {
			t.Errorf("%s overlaps = %v, want %v", id, got, want)
		}
	}
}

func TestTimeline_TouchingIntervals_DoNotOverlap(t *testing.T) {
	tl := tlTimeline(t)

	// a#1 ends at 40 when c#3 starts; h#8 ends at 20 when b#2 starts.
	for _, c := range []struct {
		id    domain.AgentInstanceID
		other string
	}{{"a#1", "c#3"}, {"c#3", "a#1"}, {"h#8", "b#2"}, {"b#2", "h#8"}} {
		for _, o := range tlOverlaps(tlEntry(t, tl, c.id)) {
			if o == c.other {
				t.Errorf("%s must not overlap %s: the intervals only touch", c.id, c.other)
			}
		}
	}
}

func TestTimeline_OpenInvocation_HasNoEndNoDurationAndNoOverlaps(t *testing.T) {
	tl := tlTimeline(t)

	e := tlEntry(t, tl, "d#4")

	if e.Invocation.State != domain.InvocationOpen {
		t.Errorf("state = %v, want open", e.Invocation.State)
	}
	if !e.Invocation.EndedAt.IsZero() || e.HasDuration || e.Duration != 0 {
		t.Errorf("open entry has end %v, duration %v (has %v), want none", e.Invocation.EndedAt, e.Duration, e.HasDuration)
	}
	if len(e.Overlaps) != 0 {
		t.Errorf("open entry overlaps = %v, want none: it runs 15..? and must not be given an end", tlOverlaps(e))
	}
	if !e.Invocation.StartedAt.Equal(rmAt(15)) {
		t.Errorf("open entry start = %v, want the logged %v", e.Invocation.StartedAt, rmAt(15))
	}
}

func TestTimeline_OpenInvocation_IsNotListedAsAnOverlapOfOthers(t *testing.T) {
	tl := tlTimeline(t)

	if len(tl.Entries) == 0 {
		t.Fatal("timeline has no entries, so no overlap list can be checked")
	}
	for _, e := range tl.Entries {
		for _, id := range e.Overlaps {
			if id == "d#4" {
				t.Errorf("%s lists open d#4 as an overlap although d#4 has no end", e.Invocation.Actor.Instance)
			}
		}
	}
}

func TestTimeline_EndBeforeStart_KeepsTimestampsWithoutDurationOrOverlap(t *testing.T) {
	tl := tlTimeline(t)

	e := tlEntry(t, tl, "e#5")

	if !e.Invocation.StartedAt.Equal(rmAt(50)) || !e.Invocation.EndedAt.Equal(rmAt(12)) {
		t.Errorf("e#5 timestamps = %v / %v, want the logged %v / %v", e.Invocation.StartedAt, e.Invocation.EndedAt, rmAt(50), rmAt(12))
	}
	if e.HasDuration || e.Duration != 0 || len(e.Overlaps) != 0 {
		t.Errorf("e#5 = duration %v (has %v), overlaps %v, want none", e.Duration, e.HasDuration, tlOverlaps(e))
	}
}

func TestTimeline_UnparseableStart_StaysVisibleWithoutGuessedStartOrDuration(t *testing.T) {
	tl := tlTimeline(t)

	e := tlEntry(t, tl, "f#6")

	if !e.Invocation.StartedAt.IsZero() {
		t.Errorf("f#6 start = %v, want zero (not guessed)", e.Invocation.StartedAt)
	}
	if !e.Invocation.EndedAt.Equal(rmAt(70)) {
		t.Errorf("f#6 end = %v, want the logged %v", e.Invocation.EndedAt, rmAt(70))
	}
	if e.HasDuration || len(e.Overlaps) != 0 {
		t.Errorf("f#6 = has duration %v, overlaps %v, want neither", e.HasDuration, tlOverlaps(e))
	}
}

func TestTimeline_InvocationWithoutStartEvent_StaysVisibleAsOpenAndUntimed(t *testing.T) {
	tl := tlTimeline(t)

	e := tlEntry(t, tl, "g#7")

	if !e.Invocation.StartedAt.IsZero() || !e.Invocation.EndedAt.IsZero() || e.HasDuration {
		t.Errorf("g#7 = start %v, end %v, has duration %v, want no times and no duration",
			e.Invocation.StartedAt, e.Invocation.EndedAt, e.HasDuration)
	}
	if e.Invocation.State != domain.InvocationOpen {
		t.Errorf("g#7 state = %v, want open", e.Invocation.State)
	}
}

func TestTimeline_Entry_CarriesTheInvocationAsAggregated(t *testing.T) {
	tl := tlTimeline(t)

	e := tlEntry(t, tl, "b#2")

	if e.Invocation.Actor.Type != "tester" || e.Invocation.StatusCode != "BLOCKED" || e.Invocation.State != domain.InvocationClosed {
		t.Errorf("b#2 invocation = %+v, want closed tester with status BLOCKED", e.Invocation)
	}
}

func TestTimeline_Compactions_AreInTimeOrderWithTheirActor(t *testing.T) {
	tl := tlTimeline(t)

	if len(tl.Compactions) != 3 {
		t.Fatalf("compactions = %d, want 3: %+v", len(tl.Compactions), tl.Compactions)
	}
	first, second, third := tl.Compactions[0], tl.Compactions[1], tl.Compactions[2]
	if first.Actor.Kind != domain.ActorOrchestrator || first.Trigger != "auto" || !first.At.Equal(rmAt(25)) {
		t.Errorf("first compaction = %+v, want orchestrator auto at %v", first, rmAt(25))
	}
	if first.TokensBefore != domain.Tokens(100000) || first.TokensAfter != domain.Tokens(20000) {
		t.Errorf("first compaction tokens = %v -> %v, want 100000 -> 20000", first.TokensBefore, first.TokensAfter)
	}
	if second.Actor.Instance != "a#1" || second.Trigger != "manual" || !second.At.Equal(rmAt(30)) {
		t.Errorf("second compaction = %+v, want a#1 manual at %v", second, rmAt(30))
	}
	if third.Actor.Kind != domain.ActorOrchestrator || !third.At.IsZero() {
		t.Errorf("third compaction = %+v, want the orchestrator compaction with the unparseable time last", third)
	}
}

func TestTimeline_RunWithoutInvocationsOrCompactions_IsEmpty(t *testing.T) {
	in := rmOrchestratorInput(rmToolStart("o1", "Read", rmAt(1)), rmToolEnd("o1", domain.ToolCallSuccess, rmAt(2)))

	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	tl := requireRunReport(t, report, testRunID).Timeline
	if len(tl.Entries) != 0 || len(tl.Compactions) != 0 {
		t.Errorf("timeline = %+v, want no entries and no compactions", tl)
	}
}

func TestTimeline_CompactionsOnly_AreShownWithoutEntries(t *testing.T) {
	in := rmOrchestratorInput(rmCompaction("auto", domain.Tokens(10), domain.Tokens(5), rmAt(3)))

	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	tl := requireRunReport(t, report, testRunID).Timeline
	if len(tl.Entries) != 0 || len(tl.Compactions) != 1 || tl.Compactions[0].Trigger != "auto" {
		t.Errorf("timeline = %+v, want one auto compaction and no entries", tl)
	}
}

func TestTimeline_RepricingDoesNotChangeTheTimeline(t *testing.T) {
	cheap := analysis.Analyze(tlScenarioInput(), viewsPricing(t), testSource(), nil)
	unpriced := analysis.Analyze(tlScenarioInput(), domain.NewPricingTable(nil), testSource(), nil)

	a := requireRunReport(t, cheap, testRunID).Timeline
	b := requireRunReport(t, unpriced, testRunID).Timeline

	if !rmEqualStrings(tlEntryIDs(a.Entries), tlEntryIDs(b.Entries)) || len(a.Entries) == 0 {
		t.Errorf("entries %v vs %v, want the same non-empty timeline whatever the pricing", tlEntryIDs(a.Entries), tlEntryIDs(b.Entries))
	}
}
