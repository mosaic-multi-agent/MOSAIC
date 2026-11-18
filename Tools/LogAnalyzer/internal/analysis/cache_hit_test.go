package analysis_test

// Tests for the cache hit figure: summed cache reads (each capped at the
// previous call's context) over summed previous-call contexts, over the calls
// whose previous call is in the same session with no compaction and no model
// change between them. Driven through one dispatch; other scopes are in
// cache_hit_scopes_test.go.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestCacheHit_PerfectCaching_IsExactlyOneHundredPercent(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 100, 0),
		chCall("c2", 20, 150, 100),
		chCall("c3", 30, 200, 150),
	)

	// Each later call reads its previous call's whole context: 100/100 + 150/150.
	chAssertExact(t, "hit ratio", got, 250, 250)
}

func TestCacheHit_PartialReads_ShowTheShareOfThePreviousContextRead(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 1000, 0),
		chCall("c2", 20, 1200, 600),
	)

	chAssertExact(t, "hit ratio", got, 600, 1000)
}

func TestCacheHit_ReadsAbovePreviousContext_AreCappedSoTheFigureNeverExceedsOneHundredPercent(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 100, 0),
		chCall("c2", 20, 500, 400), // reads 400 of a previous context of 100
		chCall("c3", 30, 600, 200), // reads 200 of a previous context of 500
	)

	chAssertExact(t, "hit ratio", got, 100+200, 100+500)
	if got.Numerator > got.Denominator {
		t.Errorf("hit ratio %d/%d exceeds 100%%", got.Numerator, got.Denominator)
	}
}

func TestCacheHit_FirstCallOfASession_DoesNotCountAndNoPairSpansTwoSessions(t *testing.T) {
	got := chDispatchHit(t,
		chSession("s1", chCall("a1", 10, 100, 0)),
		chSession("s1", chCall("a2", 20, 150, 100)),
		chSession("s2", chCall("b1", 30, 400, 300)), // first of s2: no pair with a2
		chSession("s2", chCall("b2", 40, 450, 200)),
	)

	// Pairs: (a2, a1) = 100/100 and (b2, b1) = 200/400.
	chAssertExact(t, "hit ratio", got, 100+200, 100+400)
}

func TestCacheHit_OnlyCallsOfDifferentSessions_IsUndefined(t *testing.T) {
	got := chDispatchHit(t,
		chSession("s1", chCall("a1", 10, 100, 0)),
		chSession("s2", chCall("b1", 20, 400, 300)),
	)

	chAssertUndefined(t, "hit ratio", got)
}

func TestCacheHit_CallWithoutSessionId_TakesThePrecedingCallsSession(t *testing.T) {
	got := chDispatchHit(t,
		chSession("s1", chCall("a1", 10, 100, 0)),
		chCall("a2", 20, 150, 100), // no id: belongs to s1
		chSession("s2", chCall("b1", 30, 400, 300)),
	)

	chAssertExact(t, "hit ratio", got, 100, 100)
}

func TestCacheHit_LeadingCallWithoutSessionId_TakesTheFollowingCallsSession(t *testing.T) {
	got := chDispatchHit(t,
		chCall("a0", 10, 100, 0), // no id and nothing before it: belongs to s1
		chSession("s1", chCall("a1", 20, 150, 100)),
	)

	chAssertExact(t, "hit ratio", got, 100, 100)
}

func TestCacheHit_NoSessionIdAnywhere_IsOneSession(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 100, 0),
		chCall("c2", 20, 150, 50),
		chCall("c3", 30, 200, 150),
	)

	chAssertExact(t, "hit ratio", got, 50+150, 100+150)
}

func TestCacheHit_CompactionMarkBetweenTheCalls_ExcludesThePair(t *testing.T) {
	agent := chAgent("coder#1",
		chCall("c1", 10, 1000, 0),
		chCall("c2", 20, 1200, 900),
		chCall("c3", 30, 1300, 1200),
	)
	marks := []domain.Compaction{detLogged(chActor("coder#1"), 25)}

	run := chRun(t, nil, marks, agent)

	// Only (c2, c1) remains; (c3, c2) spans the logged compaction.
	chAssertExact(t, "hit ratio", rwDispatchOf(t, run, "coder#1").Cache.HitRatio, 900, 1000)
}

func TestCacheHit_DetectedCompactionBetweenTheCalls_ExcludesThePair(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 1000, 0),
		chCall("c2", 20, 500, 400), // context fell by half: a detected compaction
		chCall("c3", 30, 600, 500),
	)

	// (c2, c1) is excluded although c2 read 400; only (c3, c2) counts.
	chAssertExact(t, "hit ratio", got, 500, 500)
}

func TestCacheHit_CompactionCallBetweenTheCalls_ExcludesThePair(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 1000, 0),
		crWithKind(chCall("k1", 20, 1000, 0), domain.CallCompaction),
		chCall("c2", 30, 300, 250),
		chCall("c3", 40, 350, 300),
	)

	// (c2, c1) spans the compaction call; the compaction call itself is no pair member.
	chAssertExact(t, "hit ratio", got, 300, 300)
}

func TestCacheHit_ModelChangeBetweenTheCalls_ExcludesThePair(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 100, 0),
		crWithModel(chCall("c2", 20, 150, 0), crModelB),
		crWithModel(chCall("c3", 30, 200, 150), crModelB),
	)

	// (c2, c1) changes model; (c3, c2) keeps it.
	chAssertExact(t, "hit ratio", got, 150, 150)
}

func TestCacheHit_ColdReadAfterAPause_CountsAsAMiss(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 1000, 0),
		chCall("c2", 10_000, 1100, 0), // cache expired during the pause
		chCall("c3", 10_010, 1200, 1100),
	)

	// The cold pair adds 0 over 1000; it is not excluded.
	chAssertExact(t, "hit ratio", got, 0+1100, 1000+1100)
}

func TestCacheHit_AuxiliaryCallBetweenMainCalls_IsNeitherCallNorPrevious(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 100, 0),
		crWithKind(chCall("x1", 20, 5000, 4000), domain.CallAuxiliary),
		chCall("c2", 30, 150, 100),
	)

	chAssertExact(t, "hit ratio", got, 100, 100)
}

func TestCacheHit_PartialPreviousContext_ExcludesThePair(t *testing.T) {
	got := chDispatchHit(t,
		chNoCacheWrite(chCall("c1", 10, 1000, 0)),
		chCall("c2", 20, 1100, 1000),
		chCall("c3", 30, 1200, 1100),
	)

	// (c2, c1) has a partial previous context; (c3, c2) counts.
	chAssertExact(t, "hit ratio", got, 1100, 1100)
}

func TestCacheHit_PartialCurrentContext_StillCountsWhenItsCacheReadIsPresent(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 1000, 0),
		chNoCacheWrite(chCall("c2", 20, 1100, 900)),
	)

	chAssertExact(t, "hit ratio", got, 900, 1000)
}

func TestCacheHit_CallWithoutCacheReadFigure_IsNotACurrentCall(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 1000, 0),
		crNoCacheRead(chCall("c2", 20, 1100, 0)),
	)

	chAssertUndefined(t, "hit ratio", got)
}

func TestCacheHit_UntimedCall_IsNotACurrentCall(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 1000, 0),
		chUntimed(chCall("c2", 0, 1100, 1000)),
	)

	chAssertUndefined(t, "hit ratio", got)
}

func TestCacheHit_UntimedCall_IsNotAPreviousCall(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 1000, 0),
		chUntimed(chCall("c2", 0, 1100, 1000)),
		chCall("c3", 30, 1200, 500),
	)

	// c3 pairs with the timed c1 (previous context 1000), not with the untimed c2 (1100).
	chAssertExact(t, "hit ratio", got, 500, 1000)
}

func TestCacheHit_UnknownModelOnOneSide_IsNotAModelChange(t *testing.T) {
	got := chDispatchHit(t,
		chCall("c1", 10, 100, 0),
		crWithModel(chCall("c2", 20, 150, 100), ""),
	)

	// A model change needs both models known and different, so the pair counts.
	chAssertExact(t, "hit ratio", got, 100, 100)
}

func TestCacheHit_SingleCall_IsUndefinedNotZero(t *testing.T) {
	got := chDispatchHit(t, chCall("c1", 10, 1000, 900))

	chAssertUndefined(t, "hit ratio", got)
}

func TestCacheHit_DispatchWithoutCalls_IsUndefined(t *testing.T) {
	chAssertUndefined(t, "hit ratio", chDispatchHit(t))
}
