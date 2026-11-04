package analysis_test

// Run-model tests for compaction counting: halves of one compaction are one
// entry in RunActivity.Compactions and Timeline.Compactions.
//
// Rules under test:
//   - Schema 1.5.0 phase: a "post" pairs with the nearest preceding unpaired
//     "pre" of the same stream; an unpaired half is one compaction; an event
//     without phase is one self-contained compaction. Streams never pair.
//   - Legacy rule (events without phase below 1.5.0): a legacy event P with no
//     tokens_after pairs with the next compaction event Q of the same stream
//     when Q is a legacy phase-less event carrying neither trigger nor
//     tokens_before. No time limit. Legacy events never pair with phased ones.
//   - A paired compaction takes At, Trigger and TokensBefore from the first
//     half, EndedAt and TokensAfter from the second.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// cpEvent builds a compaction event stamped with the given schema version and
// phase ("" = no phase).
func cpEvent(version string, phase domain.CompactionPhase, trigger string, before, after domain.TokenCount, at time.Time) domain.Event {
	ev := rmCompaction(trigger, before, after, at)
	ev.SchemaVersion = version
	ev.Compaction.Phase = phase
	return ev
}

func cpPre(trigger string, before int64, at time.Time) domain.Event {
	return cpEvent("1.5.0", domain.CompactionPhasePre, trigger, domain.Tokens(before), domain.TokenCount{}, at)
}

func cpPost(after int64, at time.Time) domain.Event {
	return cpEvent("1.5.0", domain.CompactionPhasePost, "", domain.TokenCount{}, domain.Tokens(after), at)
}

// cpLegacyPre is the first event of the legacy Claude Code pair.
func cpLegacyPre(version, trigger string, before int64, at time.Time) domain.Event {
	return cpEvent(version, domain.CompactionPhaseNone, trigger, domain.Tokens(before), domain.TokenCount{}, at)
}

// cpLegacyPost is the second event of the legacy Claude Code pair: tokens_after only.
func cpLegacyPost(version string, after int64, at time.Time) domain.Event {
	return cpEvent(version, domain.CompactionPhaseNone, "", domain.TokenCount{}, domain.Tokens(after), at)
}

func cpShapes(cs []domain.Compaction) []domain.CompactionShape {
	out := make([]domain.CompactionShape, len(cs))
	for i, c := range cs {
		out[i] = c.Shape
	}
	return out
}

func cpRequireShapes(t *testing.T, cs []domain.Compaction, want ...domain.CompactionShape) {
	t.Helper()
	got := cpShapes(cs)
	if len(got) != len(want) {
		t.Fatalf("got %d compactions with shapes %v, want %d with shapes %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("shapes = %v, want %v", got, want)
		}
	}
}

func cpOrchestrator(t *testing.T, events ...domain.Event) []domain.Compaction {
	t.Helper()
	return rmActivity(t, rmOrchestratorInput(events...)).Compactions
}

// --- schema 1.5.0 phase pairing -------------------------------------------

func TestCompactions_PhasePreThenPost_IsOnePairedCompactionCarryingBothHalves(t *testing.T) {
	cs := cpOrchestrator(t, cpPre("auto", 180000, rmAt(10)), cpPost(40000, rmAt(25)))

	cpRequireShapes(t, cs, domain.CompactionPaired)
	c := cs[0]
	if !c.At.Equal(rmAt(10)) || !c.EndedAt.Equal(rmAt(25)) {
		t.Errorf("At/EndedAt = %v / %v, want %v / %v", c.At, c.EndedAt, rmAt(10), rmAt(25))
	}
	if c.Trigger != "auto" || c.Actor.Kind != domain.ActorOrchestrator {
		t.Errorf("Trigger = %q, Actor = %+v, want auto on the orchestrator", c.Trigger, c.Actor)
	}
	assertTokenPresent(t, "TokensBefore", c.TokensBefore, 180000)
	assertTokenPresent(t, "TokensAfter", c.TokensAfter, 40000)
}

func TestCompactions_PairedFieldAbsentInItsHalf_StaysAbsent(t *testing.T) {
	pre := cpEvent("1.5.0", domain.CompactionPhasePre, "", domain.TokenCount{}, domain.TokenCount{}, rmAt(1))

	cs := cpOrchestrator(t, pre, cpPost(7, rmAt(2)))

	cpRequireShapes(t, cs, domain.CompactionPaired)
	assertTokenAbsent(t, "TokensBefore", cs[0].TokensBefore)
	assertTokenPresent(t, "TokensAfter", cs[0].TokensAfter, 7)
	if cs[0].Trigger != "" {
		t.Errorf("Trigger = %q, want empty", cs[0].Trigger)
	}
}

func TestCompactions_PairedTriggerIsFirstNonEmptyHalfPreFirst(t *testing.T) {
	post := cpEvent("1.5.0", domain.CompactionPhasePost, "manual", domain.TokenCount{}, domain.Tokens(5), rmAt(2))
	preWithTrigger := cpPre("auto", 9, rmAt(1))
	preWithout := cpPre("", 9, rmAt(1))

	withBoth := cpOrchestrator(t, preWithTrigger, post)
	onlyPost := cpOrchestrator(t, preWithout, post)

	if withBoth[0].Trigger != "auto" {
		t.Errorf("Trigger = %q, want the pre half's auto", withBoth[0].Trigger)
	}
	if onlyPost[0].Trigger != "manual" {
		t.Errorf("Trigger = %q, want the post half's manual when pre has none", onlyPost[0].Trigger)
	}
}

func TestCompactions_LonePre_IsOnePreOnlyCompaction(t *testing.T) {
	cs := cpOrchestrator(t, cpPre("auto", 100, rmAt(5)))

	cpRequireShapes(t, cs, domain.CompactionPreOnly)
	if !cs[0].At.Equal(rmAt(5)) || !cs[0].EndedAt.IsZero() {
		t.Errorf("At/EndedAt = %v / %v, want %v / zero", cs[0].At, cs[0].EndedAt, rmAt(5))
	}
	assertTokenPresent(t, "TokensBefore", cs[0].TokensBefore, 100)
	assertTokenAbsent(t, "TokensAfter", cs[0].TokensAfter)
}

func TestCompactions_LonePost_IsOnePostOnlyCompaction(t *testing.T) {
	cs := cpOrchestrator(t, cpPost(30, rmAt(9)))

	cpRequireShapes(t, cs, domain.CompactionPostOnly)
	if !cs[0].At.Equal(rmAt(9)) || !cs[0].EndedAt.IsZero() {
		t.Errorf("At/EndedAt = %v / %v, want %v / zero", cs[0].At, cs[0].EndedAt, rmAt(9))
	}
	assertTokenAbsent(t, "TokensBefore", cs[0].TokensBefore)
	assertTokenPresent(t, "TokensAfter", cs[0].TokensAfter, 30)
}

func TestCompactions_EventWithoutPhaseAtSchema150_IsOneSingleCompaction(t *testing.T) {
	ev := cpEvent("1.5.0", domain.CompactionPhaseNone, "manual", domain.Tokens(50), domain.Tokens(10), rmAt(3))

	cs := cpOrchestrator(t, ev)

	cpRequireShapes(t, cs, domain.CompactionSingle)
	assertTokenPresent(t, "TokensBefore", cs[0].TokensBefore, 50)
	assertTokenPresent(t, "TokensAfter", cs[0].TokensAfter, 10)
}

func TestCompactions_PhaseLessEventsAtSchema150_NeverUseTheLegacyPairing(t *testing.T) {
	cs := cpOrchestrator(t, cpLegacyPre("1.5.0", "auto", 100, rmAt(1)), cpLegacyPost("1.5.0", 20, rmAt(2)))

	cpRequireShapes(t, cs, domain.CompactionSingle, domain.CompactionSingle)
}

// Versions at or above 1.5.0 compare numerically per component, so these
// phase-less events must never use the legacy pairing.
func TestCompactions_PhaseLessEventsAtNumericallyNotBelow150_NeverUseTheLegacyPairing(t *testing.T) {
	for _, version := range []string{"1.10.0", "1.99999999999999999999.0"} {
		t.Run(version, func(t *testing.T) {
			cs := cpOrchestrator(t, cpLegacyPre(version, "auto", 100, rmAt(1)), cpLegacyPost(version, 20, rmAt(2)))

			cpRequireShapes(t, cs, domain.CompactionSingle, domain.CompactionSingle)
		})
	}
}

func TestCompactions_PostPairsWithNearestPrecedingUnpairedPre(t *testing.T) {
	cs := cpOrchestrator(t, cpPre("auto", 1, rmAt(1)), cpPre("manual", 2, rmAt(2)), cpPost(3, rmAt(3)))

	cpRequireShapes(t, cs, domain.CompactionPreOnly, domain.CompactionPaired)
	if cs[0].Trigger != "auto" || cs[1].Trigger != "manual" {
		t.Errorf("triggers = %q, %q; want the later pre (manual) paired with the post", cs[0].Trigger, cs[1].Trigger)
	}
	if !cs[1].At.Equal(rmAt(2)) || !cs[1].EndedAt.Equal(rmAt(3)) {
		t.Errorf("paired span = %v..%v, want %v..%v", cs[1].At, cs[1].EndedAt, rmAt(2), rmAt(3))
	}
}

func TestCompactions_SecondPostAfterAPair_IsPostOnly(t *testing.T) {
	cs := cpOrchestrator(t, cpPre("auto", 1, rmAt(1)), cpPost(2, rmAt(2)), cpPost(3, rmAt(3)))

	cpRequireShapes(t, cs, domain.CompactionPaired, domain.CompactionPostOnly)
}

func TestCompactions_PostBeforeAnyPre_IsPostOnlyAndTheLaterPreStaysPreOnly(t *testing.T) {
	cs := cpOrchestrator(t, cpPost(2, rmAt(1)), cpPre("auto", 1, rmAt(2)))

	cpRequireShapes(t, cs, domain.CompactionPostOnly, domain.CompactionPreOnly)
}

func TestCompactions_TwoSequentialPairs_AreTwoCompactions(t *testing.T) {
	cs := cpOrchestrator(t,
		cpPre("auto", 1, rmAt(1)), cpPost(2, rmAt(2)),
		cpPre("manual", 3, rmAt(10)), cpPost(4, rmAt(11)),
	)

	cpRequireShapes(t, cs, domain.CompactionPaired, domain.CompactionPaired)
	if !cs[0].At.Equal(rmAt(1)) || !cs[1].At.Equal(rmAt(10)) {
		t.Errorf("At = %v, %v; want %v, %v", cs[0].At, cs[1].At, rmAt(1), rmAt(10))
	}
}

func TestCompactions_PreAndPostInDifferentStreams_NeverPair(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{cpPre("auto", 1, rmAt(1))}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{cpPost(2, rmAt(2))}},
	}}

	cs := rmActivity(t, in).Compactions

	cpRequireShapes(t, cs, domain.CompactionPreOnly, domain.CompactionPostOnly)
	if cs[0].Actor.Kind != domain.ActorOrchestrator || cs[1].Actor.Instance != "a#1" {
		t.Errorf("actors = %+v, %+v; want orchestrator pre and a#1 post", cs[0].Actor, cs[1].Actor)
	}
}

func TestCompactions_PairsInTwoAgentStreams_AreOneCompactionEach(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{cpPre("auto", 1, rmAt(1)), cpPost(2, rmAt(4))}},
		{Ref: agentStream(testRunRef, "b#2"), Events: []domain.Event{cpPre("auto", 1, rmAt(2)), cpPost(2, rmAt(3))}},
	}}

	cs := rmActivity(t, in).Compactions

	cpRequireShapes(t, cs, domain.CompactionPaired, domain.CompactionPaired)
}

// --- legacy pairing (phase-less events below 1.5.0) ------------------------

func TestCompactions_LegacyClaudeCodePair_IsOnePairedCompaction(t *testing.T) {
	for _, version := range []string{"1.0.0", "1.2.0", "1.3.0", "1.4.0", "1.4.9", "1.04.0"} {
		t.Run(version, func(t *testing.T) {
			cs := cpOrchestrator(t, cpLegacyPre(version, "auto", 180000, rmAt(10)), cpLegacyPost(version, 40000, rmAt(12)))

			cpRequireShapes(t, cs, domain.CompactionPaired)
			c := cs[0]
			if !c.At.Equal(rmAt(10)) || !c.EndedAt.Equal(rmAt(12)) || c.Trigger != "auto" {
				t.Errorf("At/EndedAt/Trigger = %v / %v / %q, want %v / %v / auto", c.At, c.EndedAt, c.Trigger, rmAt(10), rmAt(12))
			}
			assertTokenPresent(t, "TokensBefore", c.TokensBefore, 180000)
			assertTokenPresent(t, "TokensAfter", c.TokensAfter, 40000)
		})
	}
}

func TestCompactions_LegacySecondEventWithNoFields_PairsWithTheFirst(t *testing.T) {
	post := cpEvent("1.2.0", domain.CompactionPhaseNone, "", domain.TokenCount{}, domain.TokenCount{}, rmAt(2))

	cs := cpOrchestrator(t, cpLegacyPre("1.2.0", "auto", 100, rmAt(1)), post)

	cpRequireShapes(t, cs, domain.CompactionPaired)
	assertTokenPresent(t, "TokensBefore", cs[0].TokensBefore, 100)
	assertTokenAbsent(t, "TokensAfter", cs[0].TokensAfter)
}

func TestCompactions_LegacyPairHasNoTimeLimit(t *testing.T) {
	cs := cpOrchestrator(t, cpLegacyPre("1.2.0", "auto", 100, rmAt(0)), cpLegacyPost("1.2.0", 20, rmAt(7200)))

	cpRequireShapes(t, cs, domain.CompactionPaired)
}

func TestCompactions_LegacyFirstEventAlreadyHavingTokensAfter_DoesNotPair(t *testing.T) {
	complete := cpEvent("1.2.0", domain.CompactionPhaseNone, "auto", domain.Tokens(100), domain.Tokens(20), rmAt(1))

	cs := cpOrchestrator(t, complete, cpLegacyPost("1.2.0", 5, rmAt(2)))

	cpRequireShapes(t, cs, domain.CompactionSingle, domain.CompactionSingle)
}

func TestCompactions_LegacySecondEventCarryingTriggerOrTokensBefore_DoesNotPair(t *testing.T) {
	withTrigger := cpEvent("1.2.0", domain.CompactionPhaseNone, "manual", domain.TokenCount{}, domain.Tokens(5), rmAt(2))
	withBefore := cpEvent("1.2.0", domain.CompactionPhaseNone, "", domain.Tokens(50), domain.Tokens(5), rmAt(2))

	byTrigger := cpOrchestrator(t, cpLegacyPre("1.2.0", "auto", 100, rmAt(1)), withTrigger)
	byBefore := cpOrchestrator(t, cpLegacyPre("1.2.0", "auto", 100, rmAt(1)), withBefore)

	cpRequireShapes(t, byTrigger, domain.CompactionSingle, domain.CompactionSingle)
	cpRequireShapes(t, byBefore, domain.CompactionSingle, domain.CompactionSingle)
}

func TestCompactions_LegacyEventsOfTwoCompactions_PairEachFirstWithItsOwnNextEvent(t *testing.T) {
	cs := cpOrchestrator(t,
		cpLegacyPre("1.2.0", "auto", 100, rmAt(1)), cpLegacyPost("1.2.0", 20, rmAt(2)),
		cpLegacyPre("1.2.0", "manual", 300, rmAt(50)), cpLegacyPost("1.2.0", 60, rmAt(51)),
	)

	cpRequireShapes(t, cs, domain.CompactionPaired, domain.CompactionPaired)
	assertTokenPresent(t, "second TokensBefore", cs[1].TokensBefore, 300)
	assertTokenPresent(t, "second TokensAfter", cs[1].TokensAfter, 60)
}

func TestCompactions_LegacyFirstEventFollowedByAnotherFirst_OnlyTheSecondPairs(t *testing.T) {
	cs := cpOrchestrator(t,
		cpLegacyPre("1.2.0", "auto", 100, rmAt(1)),
		cpLegacyPre("1.2.0", "manual", 300, rmAt(2)),
		cpLegacyPost("1.2.0", 60, rmAt(3)),
	)

	cpRequireShapes(t, cs, domain.CompactionSingle, domain.CompactionPaired)
	if cs[0].Trigger != "auto" || cs[1].Trigger != "manual" {
		t.Errorf("triggers = %q, %q; want the lone auto first and the manual pair second", cs[0].Trigger, cs[1].Trigger)
	}
}

func TestCompactions_LegacyPairFollowedByAnotherSecondEvent_LeavesTheExtraAsSingle(t *testing.T) {
	cs := cpOrchestrator(t,
		cpLegacyPre("1.2.0", "auto", 100, rmAt(1)),
		cpLegacyPost("1.2.0", 20, rmAt(2)),
		cpEvent("1.2.0", domain.CompactionPhaseNone, "", domain.TokenCount{}, domain.TokenCount{}, rmAt(3)),
	)

	cpRequireShapes(t, cs, domain.CompactionPaired, domain.CompactionSingle)
}

func TestCompactions_LegacyEventsInDifferentStreams_NeverPair(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{cpLegacyPre("1.2.0", "auto", 100, rmAt(1))}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{cpLegacyPost("1.2.0", 20, rmAt(2))}},
	}}

	cs := rmActivity(t, in).Compactions

	cpRequireShapes(t, cs, domain.CompactionSingle, domain.CompactionSingle)
}

func TestCompactions_LegacyEventsNeverPairWithPhasedEvents(t *testing.T) {
	legacyThenPost := cpOrchestrator(t, cpLegacyPre("1.4.0", "auto", 100, rmAt(1)), cpPost(20, rmAt(2)))
	preThenLegacy := cpOrchestrator(t, cpPre("auto", 100, rmAt(1)), cpLegacyPost("1.4.0", 20, rmAt(2)))

	cpRequireShapes(t, legacyThenPost, domain.CompactionSingle, domain.CompactionPostOnly)
	cpRequireShapes(t, preThenLegacy, domain.CompactionPreOnly, domain.CompactionSingle)
}

func TestCompactions_LegacySecondEventStampedAt150_DoesNotPairWithALegacyFirst(t *testing.T) {
	cs := cpOrchestrator(t, cpLegacyPre("1.4.0", "auto", 100, rmAt(1)), cpLegacyPost("1.5.0", 20, rmAt(2)))

	cpRequireShapes(t, cs, domain.CompactionSingle, domain.CompactionSingle)
}

func cpFieldless(version string, sec int) domain.Event {
	return cpEvent(version, domain.CompactionPhaseNone, "", domain.TokenCount{}, domain.TokenCount{}, rmAt(sec))
}

func TestCompactions_ThreeFieldlessLegacyEvents_PairingIsGreedyLeftToRight(t *testing.T) {
	cs := cpOrchestrator(t, cpFieldless("1.2.0", 1), cpFieldless("1.2.0", 2), cpFieldless("1.2.0", 3))

	cpRequireShapes(t, cs, domain.CompactionPaired, domain.CompactionSingle)
	if !cs[0].At.Equal(rmAt(1)) || !cs[0].EndedAt.Equal(rmAt(2)) || !cs[1].At.Equal(rmAt(3)) {
		t.Errorf("compactions = %+v, want the pair 1..2 then the single at 3", cs)
	}
	if cs[0].Trigger != "" {
		t.Errorf("paired Trigger = %q, want empty", cs[0].Trigger)
	}
	assertTokenAbsent(t, "TokensBefore", cs[0].TokensBefore)
	assertTokenAbsent(t, "TokensAfter", cs[0].TokensAfter)
}

func TestCompactions_FourFieldlessLegacyEvents_ArePairedTwice(t *testing.T) {
	cs := cpOrchestrator(t, cpFieldless("1.2.0", 1), cpFieldless("1.2.0", 2), cpFieldless("1.2.0", 3), cpFieldless("1.2.0", 4))

	cpRequireShapes(t, cs, domain.CompactionPaired, domain.CompactionPaired)
}

func TestCompactions_EventsWithEmptyOrUnparseableSchemaVersion_AreNeverLegacy(t *testing.T) {
	for _, version := range []string{"", "abc", "1.4", "2.0.0"} {
		t.Run("version="+version, func(t *testing.T) {
			cs := cpOrchestrator(t, cpLegacyPre(version, "auto", 100, rmAt(1)), cpLegacyPost(version, 20, rmAt(2)))

			cpRequireShapes(t, cs, domain.CompactionSingle, domain.CompactionSingle)
		})
	}
}

func TestCompactions_PhasedEventWithEmptyVersion_FollowsThePhaseRule(t *testing.T) {
	pre := cpEvent("", domain.CompactionPhasePre, "auto", domain.Tokens(1), domain.TokenCount{}, rmAt(1))
	post := cpEvent("", domain.CompactionPhasePost, "", domain.TokenCount{}, domain.Tokens(2), rmAt(2))

	cs := cpOrchestrator(t, pre, post)

	cpRequireShapes(t, cs, domain.CompactionPaired)
}

func TestCompactions_PerActorCount_IsTheNumberOfEntriesOfThatActor(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{cpPre("auto", 1, rmAt(1)), cpPost(2, rmAt(2)), cpPre("auto", 3, rmAt(5))}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{cpPre("manual", 1, rmAt(3)), cpPost(2, rmAt(4))}},
	}}

	cs := rmActivity(t, in).Compactions

	orch, agent := 0, 0
	for _, c := range cs {
		switch {
		case c.Actor.Kind == domain.ActorOrchestrator:
			orch++
		case c.Actor.Instance == "a#1":
			agent++
		}
	}
	if len(cs) != 3 || orch != 2 || agent != 1 {
		t.Errorf("entries = %d (orchestrator %d, a#1 %d), want 3 (2, 1)", len(cs), orch, agent)
	}
}

// --- counts reach the run report and the timeline --------------------------

func TestCompactions_Order_FollowsTheFirstHalfAcrossStreams(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{cpPre("auto", 1, rmAt(10)), cpPost(2, rmAt(30))}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{cpPre("manual", 1, rmAt(20)), cpPost(2, rmAt(21))}},
	}}

	cs := rmActivity(t, in).Compactions

	cpRequireShapes(t, cs, domain.CompactionPaired, domain.CompactionPaired)
	if cs[0].Actor.Kind != domain.ActorOrchestrator || cs[1].Actor.Instance != "a#1" {
		t.Errorf("actors = %+v, %+v; want the pair starting at 10 (orchestrator) before the pair starting at 20 (a#1)", cs[0].Actor, cs[1].Actor)
	}
}

func TestCompactions_TimelineHasOneEntryPerCompaction(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{
			cpPre("auto", 1, rmAt(10)), cpPost(2, rmAt(12)), // one pair
			cpLegacyPre("1.2.0", "manual", 9, rmAt(20)), cpLegacyPost("1.2.0", 3, rmAt(21)), // one legacy pair
			cpPre("auto", 5, rmAt(30)), // one lone pre
		}},
		tlInvocation("a#1", "coder", sec(1), sec(50), "SUCCESS"),
	}}
	in.Streams[1].Events = append(in.Streams[1].Events, cpPost(4, rmAt(40))) // one lone post

	report := analysis.Analyze(in, viewsPricing(t), domain.DefaultAnalysisSettings(), testSource(), nil)

	tl := requireRunReport(t, report, testRunID).Timeline
	cpRequireShapes(t, tl.Compactions, domain.CompactionPaired, domain.CompactionPaired, domain.CompactionPreOnly, domain.CompactionPostOnly)
	if !tl.Compactions[0].EndedAt.Equal(rmAt(12)) {
		t.Errorf("first compaction EndedAt = %v, want %v", tl.Compactions[0].EndedAt, rmAt(12))
	}
}

func TestCompactions_ZeroAtCompaction_IsCountedOnceAndSortsLast(t *testing.T) {
	cs := cpOrchestrator(t,
		cpEvent("1.5.0", domain.CompactionPhaseNone, "auto", domain.TokenCount{}, domain.TokenCount{}, time.Time{}),
		cpEvent("1.5.0", domain.CompactionPhaseNone, "manual", domain.TokenCount{}, domain.TokenCount{}, rmAt(5)),
	)

	cpRequireShapes(t, cs, domain.CompactionSingle, domain.CompactionSingle)
	if cs[0].Trigger != "manual" || !cs[1].At.IsZero() {
		t.Errorf("compactions = %+v, want the timestamped one first and the zero-At one last", cs)
	}
}

func TestCompactions_AggregateIsDeterministicAcrossRepeatedRuns(t *testing.T) {
	events := []domain.Event{cpPre("auto", 1, rmAt(1)), cpPre("manual", 2, rmAt(2)), cpPost(3, rmAt(3)), cpLegacyPre("1.2.0", "x", 1, rmAt(4))}

	first := cpOrchestrator(t, events...)
	for i := 0; i < 10; i++ {
		again := cpOrchestrator(t, events...)
		if len(again) != len(first) {
			t.Fatalf("run %d: %d compactions, want %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j].Shape != first[j].Shape || !again[j].At.Equal(first[j].At) || again[j].Trigger != first[j].Trigger {
				t.Fatalf("run %d: compaction %d differs: %+v vs %+v", i, j, again[j], first[j])
			}
		}
	}
}

func TestCompactions_IncrementalAggregator_PairsLikeAggregate(t *testing.T) {
	events := []domain.Event{cpPre("auto", 1, rmAt(1)), cpPost(2, rmAt(2)), cpPre("manual", 3, rmAt(9))}
	agg := analysis.NewAggregator()
	for _, ev := range events {
		agg.Add(orchStream(testRunRef), ev)
	}

	result, _ := agg.Result()

	cpRequireShapes(t, requireOneRun(t, result).Activity.Compactions, domain.CompactionPaired, domain.CompactionPreOnly)
}
