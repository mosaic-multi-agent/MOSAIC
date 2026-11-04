package analysis_test

// Run-model tests for compactions, the unattributable bucket, run isolation,
// and the guarantee that the new events change nothing that exists today.

import (
	"reflect"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func TestRunModel_Compaction_KeepsActorTimestampTriggerAndTokenCounts(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{
			rmCompaction("auto", domain.Tokens(180000), domain.Tokens(40000), rmAt(100)),
		}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{
			rmCompaction("manual", domain.Tokens(0), domain.Tokens(0), rmAt(50)),
		}},
	}}

	act := rmActivity(t, in)

	if len(act.Compactions) != 2 {
		t.Fatalf("got %d compactions, want 2", len(act.Compactions))
	}
	agent, orch := act.Compactions[0], act.Compactions[1]
	if agent.Actor.Kind != domain.ActorAgentInstance || agent.Actor.Instance != "a#1" || !agent.At.Equal(rmAt(50)) || agent.Trigger != "manual" {
		t.Errorf("agent compaction = %+v, want a#1 manual at %v", agent, rmAt(50))
	}
	assertTokenPresent(t, "agent TokensBefore", agent.TokensBefore, 0)
	assertTokenPresent(t, "agent TokensAfter", agent.TokensAfter, 0)
	if orch.Actor.Kind != domain.ActorOrchestrator || !orch.At.Equal(rmAt(100)) || orch.Trigger != "auto" {
		t.Errorf("orchestrator compaction = %+v, want orchestrator auto at %v", orch, rmAt(100))
	}
	assertTokenPresent(t, "orchestrator TokensBefore", orch.TokensBefore, 180000)
	assertTokenPresent(t, "orchestrator TokensAfter", orch.TokensAfter, 40000)
}

func TestRunModel_Compaction_AbsentFieldsStayAbsent(t *testing.T) {
	in := rmOrchestratorInput(rmCompaction("", domain.TokenCount{}, domain.TokenCount{}, rmAt(1)))

	act := rmActivity(t, in)

	if len(act.Compactions) != 1 {
		t.Fatalf("got %d compactions, want 1", len(act.Compactions))
	}
	c := act.Compactions[0]
	if c.Trigger != "" {
		t.Errorf("Trigger = %q, want empty", c.Trigger)
	}
	assertTokenAbsent(t, "TokensBefore", c.TokensBefore)
	assertTokenAbsent(t, "TokensAfter", c.TokensAfter)
}

func TestRunModel_Compactions_AreSortedByTimeThenActorLabelThenFeedOrderWithZeroTimeLast(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{
			rmCompaction("m-zero", domain.Tokens(1), domain.TokenCount{}, time.Time{}),
			rmCompaction("m-late", domain.Tokens(2), domain.TokenCount{}, rmAt(30)),
			rmCompaction("m-tie-orch-first", domain.Tokens(3), domain.TokenCount{}, rmAt(20)),
			rmCompaction("m-tie-orch-second", domain.Tokens(4), domain.TokenCount{}, rmAt(20)),
		}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{
			rmCompaction("m-tie-agent", domain.Tokens(5), domain.TokenCount{}, rmAt(20)),
			rmCompaction("m-early", domain.Tokens(6), domain.TokenCount{}, rmAt(10)),
		}},
	}}

	act := rmActivity(t, in)

	var got []string
	for _, c := range act.Compactions {
		got = append(got, c.Trigger)
	}
	want := []string{"m-early", "m-tie-agent", "m-tie-orch-first", "m-tie-orch-second", "m-late", "m-zero"}
	if !rmEqualStrings(got, want) {
		t.Errorf("order = %v, want %v (time, then actor label a#1 < orchestrator, then feed order, zero time last)", got, want)
	}
}

func TestRunModel_CompactionOnlyAgentStream_AppearsAsOpenInvocation(t *testing.T) {
	in := oneAgentStream(testRunRef, "a#1", rmCompaction("auto", domain.Tokens(9), domain.Tokens(3), rmAt(1)))

	inv := rmInvocation(t, rmActivity(t, in), "a#1")

	if inv.State != domain.InvocationOpen {
		t.Errorf("State = %v, want InvocationOpen", inv.State)
	}
}

func TestRunModel_UnattributableBucket_GetsItsOwnActivity(t *testing.T) {
	unknown := domain.UnattributableRun()
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{rmToolStart("n1", "Read", rmAt(1)), rmToolEnd("n1", domain.ToolCallSuccess, rmAt(2))}},
		{Ref: orchStream(unknown), Events: []domain.Event{
			rmToolStart("u1", "Bash", rmAt(1)), rmToolEnd("u1", domain.ToolCallError, rmAt(4)),
			rmCompaction("auto", domain.Tokens(5), domain.Tokens(2), rmAt(5)),
		}},
		{Ref: agentStream(unknown, "x#1"), Events: []domain.Event{
			rmInvStart("x#1", "coder", rmAt(0)), rmToolStart("ux", "Read", rmAt(1)),
		}},
	}}

	agg, _ := analysis.Aggregate(in)

	if agg.Unattributable == nil {
		t.Fatal("Unattributable bucket is nil")
	}
	un := agg.Unattributable.Activity
	if got := rmCallIDs(un.OrchestratorToolCalls); !rmEqualStrings(got, []string{"u1"}) {
		t.Errorf("unattributable orchestrator calls = %v, want [u1]", got)
	}
	if len(un.Compactions) != 1 {
		t.Errorf("unattributable compactions = %d, want 1", len(un.Compactions))
	}
	if got := rmCallIDs(rmInvocation(t, un, "x#1").ToolCalls); !rmEqualStrings(got, []string{"ux"}) {
		t.Errorf("x#1 calls = %v, want [ux]", got)
	}
	named := requireOneRun(t, agg).Activity
	if got := rmCallIDs(named.OrchestratorToolCalls); !rmEqualStrings(got, []string{"n1"}) {
		t.Errorf("named orchestrator calls = %v, want [n1]: the bucket is never folded into a named run", got)
	}
	if len(named.Compactions) != 0 || len(named.Invocations) != 0 {
		t.Errorf("named activity = %+v, want no compactions or invocations from the bucket", named)
	}
}

func TestRunModel_TwoRuns_KeepSeparateActivity(t *testing.T) {
	runB := domain.NamedRun("20260102T000000Z-beef")
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{rmToolStart("a", "Read", rmAt(1))}},
		{Ref: orchStream(runB), Events: []domain.Event{rmToolStart("b", "Read", rmAt(1))}},
	}}

	agg, _ := analysis.Aggregate(in)

	if len(agg.Runs) != 2 {
		t.Fatalf("got %d runs, want 2", len(agg.Runs))
	}
	for _, run := range agg.Runs {
		want := "a"
		if run.Run.ID == runB.ID {
			want = "b"
		}
		if got := rmCallIDs(run.Activity.OrchestratorToolCalls); !rmEqualStrings(got, []string{want}) {
			t.Errorf("run %s calls = %v, want [%s]", run.Run.ID, got, want)
		}
	}
}

func TestRunModel_NewEvents_DoNotChangeExistingAggregateOrRaiseHarnessFindings(t *testing.T) {
	base := []domain.Event{
		rmInvStart("a#1", "coder", rmAt(0)),
		rmUsage("a#1", "r1", 700),
		rmInvEnd("a#1", "SUCCESS", "model-a", rmAt(9)),
	}
	extra := []domain.Event{
		withHarness(rmToolStart("c1", "Read", rmAt(1)), "totally-unknown-harness"),
		withHarness(rmToolEnd("c1", domain.ToolCallSuccess, rmAt(2)), "totally-unknown-harness"),
		withHarness(rmCompaction("auto", domain.Tokens(5), domain.Tokens(2), rmAt(3)), "totally-unknown-harness"),
	}
	plain, plainFindings := analysis.Aggregate(oneAgentStream(testRunRef, "a#1", base...))
	mixed := append(append([]domain.Event{}, base[:2]...), extra...)
	mixed = append(mixed, base[2])

	withEvents, findings := analysis.Aggregate(oneAgentStream(testRunRef, "a#1", mixed...))

	if hasFindingKind(findings, domain.FindingUnrecognisedHarness) {
		t.Errorf("tool and compaction events must raise no unrecognised_harness finding: %v", findings)
	}
	if len(findings) != len(plainFindings) {
		t.Errorf("findings = %v, want the same as without the new events: %v", findings, plainFindings)
	}
	if len(requireOneRun(t, withEvents).Activity.Invocations) == 0 {
		t.Fatal("precondition: the run model must hold the invocation")
	}
	if !reflect.DeepEqual(stripActivity(plain), stripActivity(withEvents)) {
		t.Errorf("aggregate without activity differs:\n got %+v\nwant %+v", stripActivity(withEvents), stripActivity(plain))
	}
}

func TestRunModel_AggregateTwice_IsDeterministic(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: orchStream(testRunRef), Events: []domain.Event{
			rmToolStart("o1", "Read", rmAt(1)), rmCompaction("auto", domain.Tokens(5), domain.Tokens(2), rmAt(2)),
		}},
		{Ref: agentStream(testRunRef, "b#2"), Events: []domain.Event{rmInvStart("b#2", "t", rmAt(0)), rmToolStart("x", "Bash", rmAt(1))}},
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{rmInvStart("a#1", "t", rmAt(0)), rmToolEnd("y", "", rmAt(1))}},
	}}

	first, _ := analysis.Aggregate(in)
	firstAct := requireOneRun(t, first).Activity
	if len(firstAct.Invocations) != 2 || len(firstAct.OrchestratorToolCalls) != 1 || len(firstAct.Compactions) != 1 {
		t.Fatalf("precondition: model must hold 2 invocations, 1 orchestrator call, 1 compaction; got %d, %d, %d",
			len(firstAct.Invocations), len(firstAct.OrchestratorToolCalls), len(firstAct.Compactions))
	}
	if firstAct.Invocations[0].Actor.Instance != "a#1" || firstAct.Invocations[1].Actor.Instance != "b#2" {
		t.Errorf("invocations not sorted by instance id: %v, %v",
			firstAct.Invocations[0].Actor.Instance, firstAct.Invocations[1].Actor.Instance)
	}

	for i := 0; i < 20; i++ {
		again, _ := analysis.Aggregate(in)
		if !reflect.DeepEqual(firstAct, requireOneRun(t, again).Activity) {
			t.Fatalf("aggregate run %d produced different activity", i+2)
		}
	}

	permuted := analysis.Input{Streams: []analysis.Stream{in.Streams[2], in.Streams[0], in.Streams[1]}}
	perm, _ := analysis.Aggregate(permuted)
	if !reflect.DeepEqual(firstAct.Invocations, requireOneRun(t, perm).Activity.Invocations) {
		t.Error("stream order changed the sorted invocations")
	}
}

func TestRunModel_IncrementalAggregator_MatchesAggregateAndResultIsRepeatable(t *testing.T) {
	events := []domain.Event{
		rmInvStart("a#1", "coder", rmAt(0)),
		rmToolStart("c1", "Read", rmAt(1)),
		rmToolEnd("c1", domain.ToolCallSuccess, rmAt(2)),
		rmCompaction("auto", domain.Tokens(5), domain.Tokens(2), rmAt(3)),
	}
	want, _ := analysis.Aggregate(oneAgentStream(testRunRef, "a#1", events...))
	agg := analysis.NewAggregator()
	for _, ev := range events {
		agg.Add(agentStream(testRunRef, "a#1"), ev)
	}

	first, _ := agg.Result()
	second, _ := agg.Result()

	if len(requireOneRun(t, want).Activity.Invocations) != 1 {
		t.Fatal("precondition: Aggregate must produce the invocation")
	}
	if !reflect.DeepEqual(requireOneRun(t, first).Activity, requireOneRun(t, want).Activity) {
		t.Error("incremental activity differs from Aggregate")
	}
	if !reflect.DeepEqual(requireOneRun(t, second).Activity, requireOneRun(t, first).Activity) {
		t.Error("a second Result call changed the activity")
	}
}

// withHarness returns ev with its harness set to h.
func withHarness(ev domain.Event, h string) domain.Event {
	ev.Harness = domain.Harness(h)
	return ev
}

// stripActivity returns the aggregate with every run model cleared, so only
// the pre-existing outputs are compared.
func stripActivity(a domain.Aggregate) domain.Aggregate {
	runs := make([]domain.RunAggregate, len(a.Runs))
	for i, r := range a.Runs {
		r.Activity = domain.RunActivity{}
		r.Facts = domain.RunFacts{}
		runs[i] = r
	}
	a.Runs = runs
	if a.Unattributable != nil {
		u := *a.Unattributable
		u.Activity = domain.RunActivity{}
		u.Facts = domain.RunFacts{}
		a.Unattributable = &u
	}
	return a
}
