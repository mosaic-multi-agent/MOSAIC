package analysis_test

// Tests that the cache hit figure follows the same definition in every scope:
// dispatch, agent type, orchestrator, run and the run overview. A scope's figure
// sums the qualifying pairs of every actor in it.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// chScopesRun is a run of an orchestrator and three dispatches (two coders and
// one tester), each with one qualifying pair:
//
//	orchestrator  200/200
//	coder#1       500/1000
//	coder#2       400/400
//	tester#3      50/200
func chScopesRun(t *testing.T) domain.RunReport {
	t.Helper()
	orch := []domain.InvocationUsage{chCall("o1", 10, 200, 0), chCall("o2", 20, 300, 200)}
	tester := ctxAgent{id: "tester#3", typ: "tester", usage: []domain.InvocationUsage{
		chCall("t1", 10, 200, 0), chCall("t2", 20, 250, 50),
	}}
	return chRun(t, orch, nil,
		chAgent("coder#1", chCall("c1", 10, 1000, 0), chCall("c2", 20, 1100, 500)),
		chAgent("coder#2", chCall("d1", 10, 400, 0), chCall("d2", 20, 450, 400)),
		tester,
	)
}

func TestCacheHit_DispatchScope_CoversOnlyThatInvocation(t *testing.T) {
	run := chScopesRun(t)

	chAssertExact(t, "coder#1", rwDispatchOf(t, run, "coder#1").Cache.HitRatio, 500, 1000)
	chAssertExact(t, "coder#2", rwDispatchOf(t, run, "coder#2").Cache.HitRatio, 400, 400)
}

func TestCacheHit_AgentTypeScope_SumsPairsOverItsDispatchesNotAveragesOfRatios(t *testing.T) {
	run := chScopesRun(t)

	chAssertExact(t, "coder", rwTypeGroup(t, run, "coder").Cache.HitRatio, 500+400, 1000+400)
	chAssertExact(t, "tester", rwTypeGroup(t, run, "tester").Cache.HitRatio, 50, 200)
}

func TestCacheHit_OrchestratorScope_CoversOnlyTheOrchestrator(t *testing.T) {
	run := chScopesRun(t)

	chAssertExact(t, "orchestrator", run.OrchestratorCache.HitRatio, 200, 200)
}

func TestCacheHit_RunScope_SumsOrchestratorAndEveryInvocation(t *testing.T) {
	run := chScopesRun(t)

	chAssertExact(t, "run", run.Cache.HitRatio, 200+500+400+50, 200+1000+400+200)
}

func TestCacheHit_Overview_EqualsTheRunScopeFigure(t *testing.T) {
	run := chScopesRun(t)

	chAssertExact(t, "overview", run.Overview.CacheHitRatio, 200+500+400+50, 200+1000+400+200)
	if run.Overview.CacheHitRatio != run.Cache.HitRatio {
		t.Errorf("overview = %+v, run scope = %+v, want equal", run.Overview.CacheHitRatio, run.Cache.HitRatio)
	}
}

func TestCacheHit_SingleSessionActors_AreIncludedLikeAnyOther(t *testing.T) {
	run := chRun(t, nil, nil,
		chAgent("coder#1", sessAll("only", chCall("c1", 10, 100, 0), chCall("c2", 20, 150, 100))...))

	chAssertExact(t, "dispatch", rwDispatchOf(t, run, "coder#1").Cache.HitRatio, 100, 100)
	chAssertExact(t, "run", run.Cache.HitRatio, 100, 100)
}

func TestCacheHit_OrchestratorCompactionMark_ExcludesOnlyItsOwnPair(t *testing.T) {
	orch := []domain.InvocationUsage{
		chCall("o1", 10, 1000, 0), chCall("o2", 20, 1100, 1000), chCall("o3", 30, 300, 200),
	}
	marks := []domain.Compaction{detLogged(domain.Orchestrator(), 25)}

	run := chRun(t, orch, marks,
		chAgent("coder#1", chCall("c1", 10, 100, 0), chCall("c2", 20, 150, 100)))

	chAssertExact(t, "orchestrator", run.OrchestratorCache.HitRatio, 1000, 1000)
	chAssertExact(t, "run", run.Cache.HitRatio, 1000+100, 1000+100)
}

func TestCacheHit_OrchestratorSessionSwitch_FormsNoPair(t *testing.T) {
	orch := []domain.InvocationUsage{
		chSession("s1", chCall("o1", 10, 100, 0)),
		chSession("s2", chCall("o2", 20, 150, 100)),
		chSession("s2", chCall("o3", 30, 200, 150)),
	}

	run := chRun(t, orch, nil)

	// o2 starts a new session, so only (o3, o2) counts.
	chAssertExact(t, "orchestrator", run.OrchestratorCache.HitRatio, 150, 150)
}

func TestCacheHit_ExclusionsAndCapApplyPerActorBeforeSumming(t *testing.T) {
	run := chRun(t, nil, nil,
		chAgent("coder#1",
			chCall("a1", 10, 100, 0),
			chCall("a2", 20, 500, 400)), // reads 400 of previous 100: capped to 100
		chAgent("coder#2",
			chCall("b1", 10, 200, 0),
			crWithModel(chCall("b2", 20, 250, 200), crModelB), // model change: excluded
			crWithModel(chCall("b3", 30, 300, 100), crModelB))) // reads 100 of previous 250

	chAssertExact(t, "agent type", rwTypeGroup(t, run, "coder").Cache.HitRatio, 100+100, 100+250)
	chAssertExact(t, "run", run.Cache.HitRatio, 100+100, 100+250)
}

func TestCacheHit_NoQualifyingPairInTheRun_IsUndefinedInEveryScope(t *testing.T) {
	run := chRun(t, []domain.InvocationUsage{chCall("o1", 10, 100, 50)}, nil,
		chAgent("coder#1", chCall("c1", 10, 100, 50)))

	chAssertUndefined(t, "dispatch", rwDispatchOf(t, run, "coder#1").Cache.HitRatio)
	chAssertUndefined(t, "agent type", rwTypeGroup(t, run, "coder").Cache.HitRatio)
	chAssertUndefined(t, "orchestrator", run.OrchestratorCache.HitRatio)
	chAssertUndefined(t, "run", run.Cache.HitRatio)
	chAssertUndefined(t, "overview", run.Overview.CacheHitRatio)
}

func TestCacheHit_UnattributedUsage_DoesNotTakePartInTheOverview(t *testing.T) {
	spec := ctxRunSpec{agents: []ctxAgent{chAgent("coder#1", chCall("c1", 10, 100, 0), chCall("c2", 20, 150, 100))}}
	agg := ctxAggregate(spec)
	agg.Runs[0].Unattributed = &domain.ActorAggregate{
		Invocations: []domain.InvocationUsage{chCall("u1", 10, 5000, 0), chCall("u2", 20, 5100, 0)},
	}

	run := ctxPrice(t, agg, domain.DefaultAnalysisSettings())

	// The unattributed calls would add 0/5000; they have no actor and no session.
	chAssertExact(t, "overview", run.Overview.CacheHitRatio, 100, 100)
	chAssertExact(t, "run", run.Cache.HitRatio, 100, 100)
}
