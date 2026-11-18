package analysis_test

// Tests for unread cache writes across the sessions of one actor: the last call
// record of every session contributes its cache-write tokens and cost, so an
// actor with N sessions contributes N records, in every scope that sums them.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestUnreadCacheWrite_DispatchWithTwoSessions_SumsTheLastCallOfEach(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", sessJoin(
		sessAll("s1", ctxCall("c1", 1, 10, 0, 100, 0), ctxCall("c2", 2, 10, 0, 300, 0)),
		sessAll("s2", ctxCall("c3", 3, 10, 0, 50, 0), ctxCall("c4", 4, 10, 0, 250, 0)),
	)...))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 300+250)
	ccAssertMoneyNanos(t, "unread write cost", got.UnreadWriteCost, (300+250)*3750)
}

func TestUnreadCacheWrite_DispatchWithThreeSessions_CountsThreeRecords(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", sessJoin(
		sessAll("s1", ctxCall("c1", 1, 10, 0, 100, 0)),
		sessAll("s2", ctxCall("c2", 2, 10, 0, 200, 0)),
		sessAll("s3", ctxCall("c3", 3, 10, 0, 400, 0)),
	)...))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 100+200+400)
}

func TestUnreadCacheWrite_AgentTypeSumsEverySessionOfEveryMember(t *testing.T) {
	run := ccPriceRun(t, nil,
		ccDispatch("coder#1", "coder", sessJoin(
			sessAll("a", ctxCall("c1", 1, 10, 0, 900, 0), ctxCall("c2", 2, 10, 0, 250, 0)),
			sessAll("b", ctxCall("c3", 3, 10, 0, 40, 0)),
		)...),
		ccDispatch("coder#2", "coder", sessAll("x", ctxCall("c4", 1, 10, 0, 100, 0))...),
	)

	got := rwTypeGroup(t, run, "coder").Cache

	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 250+40+100)
	ccAssertMoneyNanos(t, "unread write cost", got.UnreadWriteCost, (250+40+100)*3750)
}

func TestUnreadCacheWrite_OrchestratorWithTwoSessions_CountsTheLastCallOfEach(t *testing.T) {
	orch := sessJoin(
		sessAll("s1", ctxCall("o1", 1, 10, 0, 500, 0), ctxCall("o2", 2, 10, 0, 70, 0)),
		sessAll("s2", ctxCall("o3", 3, 10, 0, 90, 0), ctxCall("o4", 4, 10, 0, 20, 0)),
	)

	run := ccPriceRun(t, orch,
		ccDispatch("coder#1", "coder", sessJoin(
			sessAll("a", ctxCall("c1", 1, 10, 0, 900, 0), ctxCall("c2", 2, 10, 0, 250, 0)),
			sessAll("b", ctxCall("c3", 3, 10, 0, 60, 0)),
		)...),
		ccDispatch("tester#1", "tester", ctxCall("c4", 1, 10, 0, 100, 0)),
	)

	ctxAssertTokens(t, "run unread write tokens", run.Cache.UnreadWriteTokens, (70+20)+(250+60)+100)
	ccAssertMoneyNanos(t, "run unread write cost", run.Cache.UnreadWriteCost, ((70+20)+(250+60)+100)*3750)
}

func TestUnreadCacheWrite_SingleNamedSession_CountsOnlyTheLastCall(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", sessAll("s1",
		ctxCall("c1", 1, 10, 0, 100, 0),
		ctxCall("c2", 2, 10, 0, 300, 0),
		ctxCall("c3", 3, 10, 0, 250, 0))...))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 250)
}

func TestUnreadCacheWrite_CallsWithoutIdJoinTheNeighbouringSession_NotASessionOfTheirOwn(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", sessJoin(
		[]domain.InvocationUsage{ctxCall("c1", 1, 10, 0, 100, 0)},
		sessAll("s1", ctxCall("c2", 2, 10, 0, 300, 0)),
		[]domain.InvocationUsage{ctxCall("c3", 3, 10, 0, 250, 0)},
	)...))

	got := rwDispatchOf(t, run, "coder#1").Cache

	// One session s1 holds all three calls: only c3 counts.
	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 250)
}

func TestUnreadCacheWrite_SessionWhoseLastCallWroteNothing_ContributesNothing(t *testing.T) {
	run := ccPriceRun(t, nil, ccDispatch("coder#1", "coder", sessJoin(
		sessAll("s1", ctxCall("c1", 1, 10, 0, 300, 0), ctxInputOnlyCall("c2", 2, 10, 5)),
		sessAll("s2", ctxCall("c3", 3, 10, 0, 40, 0)),
	)...))

	got := rwDispatchOf(t, run, "coder#1").Cache

	ctxAssertTokens(t, "unread write tokens", got.UnreadWriteTokens, 40)
}
