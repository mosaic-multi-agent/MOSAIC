package analysis_test

// Run-model tests for invocations: which instances exist, how open and
// status-less invocations are told apart, and that nothing is inferred.

import (
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func TestRunModel_CompleteInvocations_HoldTypeStatusModelAndTimestamps(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: agentStream(testRunRef, "b#2"), Events: []domain.Event{
			rmInvStart("b#2", "reviewer", rmAt(10)),
			rmInvEnd("b#2", "BLOCKED", "model-b", rmAt(40)),
		}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{
			rmInvStart("a#1", "coder", rmAt(0)),
			rmInvEnd("a#1", "SUCCESS", "model-a", rmAt(90)),
		}},
	}}

	act := rmActivity(t, in)

	if len(act.Invocations) != 2 {
		t.Fatalf("got %d invocations, want 2", len(act.Invocations))
	}
	if act.Invocations[0].Actor.Instance != "a#1" || act.Invocations[1].Actor.Instance != "b#2" {
		t.Errorf("order = %q, %q; want sorted by instance id a#1, b#2",
			act.Invocations[0].Actor.Instance, act.Invocations[1].Actor.Instance)
	}
	a := rmInvocation(t, act, "a#1")
	if a.Actor.Kind != domain.ActorAgentInstance || a.Actor.Type != "coder" {
		t.Errorf("actor = %+v, want agent instance of type coder", a.Actor)
	}
	if a.State != domain.InvocationClosed || a.StatusCode != "SUCCESS" || a.Model != "model-a" || a.EndEvents != 1 {
		t.Errorf("invocation = %+v, want closed SUCCESS model-a with one end event", a)
	}
	if !a.StartedAt.Equal(rmAt(0)) || !a.EndedAt.Equal(rmAt(90)) {
		t.Errorf("timestamps = %v .. %v, want %v .. %v", a.StartedAt, a.EndedAt, rmAt(0), rmAt(90))
	}
	if d, ok := a.Duration(); !ok || d != 90*time.Second {
		t.Errorf("Duration() = (%v, %v), want (90s, true)", d, ok)
	}
	b := rmInvocation(t, act, "b#2")
	if b.StatusCode != "BLOCKED" || b.Actor.Type != "reviewer" {
		t.Errorf("second invocation = %+v, want BLOCKED reviewer", b)
	}
}

func TestRunModel_InvocationWithoutEnd_IsOpenWithNoEndAndNoStatus(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1", rmInvStart("a#1", "coder", rmAt(5)))

	inv := rmInvocation(t, rmActivity(t, in), "a#1")

	if inv.State != domain.InvocationOpen {
		t.Errorf("State = %v, want InvocationOpen", inv.State)
	}
	if inv.StatusCode != "" || inv.Model != "" || inv.EndEvents != 0 || !inv.EndedAt.IsZero() {
		t.Errorf("open invocation = %+v, want no status, model, end events or end time", inv)
	}
	if !inv.StartedAt.Equal(rmAt(5)) {
		t.Errorf("StartedAt = %v, want %v", inv.StartedAt, rmAt(5))
	}
	if _, ok := inv.Duration(); ok {
		t.Error("an open invocation must have no duration")
	}
}

func TestRunModel_EndWithoutStatusCode_IsClosedAndDistinctFromOpen(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: agentStream(testRunRef, "closed#1"), Events: []domain.Event{
			rmInvStart("closed#1", "coder", rmAt(0)), rmInvEnd("closed#1", "", "", rmAt(20)),
		}},
		{Ref: agentStream(testRunRef, "open#2"), Events: []domain.Event{
			rmInvStart("open#2", "coder", rmAt(0)),
		}},
	}}

	act := rmActivity(t, in)

	closed, open := rmInvocation(t, act, "closed#1"), rmInvocation(t, act, "open#2")
	if closed.State != domain.InvocationClosed || closed.StatusCode != "" || closed.EndEvents != 1 {
		t.Errorf("status-less end = %+v, want Closed, empty status, one end event", closed)
	}
	if !closed.EndedAt.Equal(rmAt(20)) {
		t.Errorf("EndedAt = %v, want %v (end time is kept even without a status code)", closed.EndedAt, rmAt(20))
	}
	if open.State != domain.InvocationOpen {
		t.Errorf("no end event State = %v, want InvocationOpen", open.State)
	}
}

func TestRunModel_ActorKnownOnlyFromInvocationEnd_AppearsWithoutStartOrType(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1", rmInvEnd("a#1", "SUCCESS", "model-a", rmAt(30)))

	inv := rmInvocation(t, rmActivity(t, in), "a#1")

	if inv.State != domain.InvocationClosed || inv.StatusCode != "SUCCESS" {
		t.Errorf("invocation = %+v, want Closed SUCCESS", inv)
	}
	if inv.Actor.Type != "" || !inv.StartedAt.IsZero() {
		t.Errorf("type = %q, StartedAt = %v; want both absent (nothing is inferred)", inv.Actor.Type, inv.StartedAt)
	}
	if _, ok := inv.Duration(); ok {
		t.Error("an invocation with no start timestamp must have no duration")
	}
}

func TestRunModel_ActorKnownOnlyFromUsageRecord_AppearsAsOpenWithUsage(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1", rmUsage("a#1", "r1", 700), rmUsage("a#1", "r2", 300))

	agg, _ := analysis.Aggregate(in)

	run := requireOneRun(t, agg)
	inv := rmInvocation(t, run.Activity, "a#1")
	if inv.State != domain.InvocationOpen {
		t.Errorf("State = %v, want InvocationOpen (no invocation_end observed)", inv.State)
	}
	if inv.Actor.Type != "" || !inv.StartedAt.IsZero() {
		t.Errorf("type = %q, StartedAt = %v; want both absent", inv.Actor.Type, inv.StartedAt)
	}
	if len(inv.Usage) != 2 {
		t.Errorf("got %d usage entries, want 2", len(inv.Usage))
	}
}

func TestRunModel_InvocationUsage_IsIdenticalToActorAggregateEntries(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		rmInvStart("a#1", "coder", rmAt(0)),
		rmUsage("a#1", "r1", 700),
		rmUsage("a#1", "r2", 300),
		rmUsage("a#1", "r1", 750), // supersedes r1
		rmInvEnd("a#1", "SUCCESS", "model-a", rmAt(9)))

	agg, _ := analysis.Aggregate(in)

	run := requireOneRun(t, agg)
	agent := requireOneAgent(t, run)
	inv := rmInvocation(t, run.Activity, "a#1")
	if len(agent.Invocations) != 2 {
		t.Fatalf("precondition: %d aggregate entries, want 2", len(agent.Invocations))
	}
	if !reflect.DeepEqual(inv.Usage, agent.Invocations) {
		t.Errorf("Usage = %+v, want the actor aggregate entries %+v", inv.Usage, agent.Invocations)
	}
}

func TestRunModel_InvocationWithoutUsage_HasNilUsage(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1", rmInvStart("a#1", "coder", rmAt(0)))

	inv := rmInvocation(t, rmActivity(t, in), "a#1")

	if inv.Usage != nil {
		t.Errorf("Usage = %+v, want nil when the instance logged no usage", inv.Usage)
	}
}

func TestRunModel_SeveralEnds_LastEndWinsAndAllAreCounted(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		rmInvStart("a#1", "coder", rmAt(0)),
		rmInvEnd("a#1", "PARTIALLY_DONE", "model-a", rmAt(10)),
		rmInvEnd("a#1", "SUCCESS", "model-b", rmAt(50)))

	inv := rmInvocation(t, rmActivity(t, in), "a#1")

	if inv.EndEvents != 2 || inv.StatusCode != "SUCCESS" || inv.Model != "model-b" || !inv.EndedAt.Equal(rmAt(50)) {
		t.Errorf("invocation = %+v, want 2 end events and the last end's SUCCESS / model-b / %v", inv, rmAt(50))
	}
}

func TestRunModel_LastEndWithoutStatus_DoesNotKeepEarlierStatus(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		rmInvEnd("a#1", "SUCCESS", "model-a", rmAt(10)),
		rmInvEnd("a#1", "", "", rmAt(20)))

	inv := rmInvocation(t, rmActivity(t, in), "a#1")

	if inv.StatusCode != "" || inv.Model != "" {
		t.Errorf("status = %q, model = %q; want the last end's absence kept, not filled from an earlier end",
			inv.StatusCode, inv.Model)
	}
}

func TestRunModel_SeveralStarts_FirstStartTimestampIsKept(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		rmInvStart("a#1", "coder", rmAt(5)),
		rmInvStart("a#1", "coder", rmAt(8)),
		rmInvEnd("a#1", "SUCCESS", "", rmAt(60)))

	inv := rmInvocation(t, rmActivity(t, in), "a#1")

	if !inv.StartedAt.Equal(rmAt(5)) {
		t.Errorf("StartedAt = %v, want the first start %v", inv.StartedAt, rmAt(5))
	}
}

func TestRunModel_UnparseableTimestamps_StayZeroAndGiveNoDuration(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		rmInvStart("a#1", "coder", time.Time{}),
		rmInvEnd("a#1", "SUCCESS", "", rmAt(60)))

	inv := rmInvocation(t, rmActivity(t, in), "a#1")

	if !inv.StartedAt.IsZero() {
		t.Errorf("StartedAt = %v, want zero", inv.StartedAt)
	}
	if _, ok := inv.Duration(); ok {
		t.Error("Duration must be unavailable when a timestamp is missing")
	}
}

func TestRunModel_AgentTypeFromStart_MatchesTypeOnActorAggregate(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1",
		rmInvStart("a#1", "coder", rmAt(0)), rmUsage("a#1", "r1", 5))

	agg, _ := analysis.Aggregate(in)

	run := requireOneRun(t, agg)
	if got, want := rmInvocation(t, run.Activity, "a#1").Actor.Type, requireOneAgent(t, run).Actor.Type; got != want || got != "coder" {
		t.Errorf("invocation type = %q, aggregate type = %q; want both coder", got, want)
	}
}

func TestRunModel_OrchestratorInvocationEvents_AreIgnored(t *testing.T) {
	in := rmOrchestratorInput(
		rmInvStart("ghost#1", "coder", rmAt(0)),
		rmInvEnd("ghost#1", "SUCCESS", "model-a", rmAt(5)))

	act := rmActivity(t, in)

	if len(act.Invocations) != 0 {
		t.Errorf("got %d invocations, want 0: orchestrator invocation_* events stay ignored", len(act.Invocations))
	}
}

func TestRunModel_RunWithoutAnyActivity_HasEmptyActivity(t *testing.T) {
	act := rmActivity(t, rmOrchestratorInput(runEndEvent()))

	if len(act.Invocations) != 0 || len(act.OrchestratorToolCalls) != 0 || len(act.Compactions) != 0 {
		t.Errorf("activity = %+v, want empty", act)
	}
}
