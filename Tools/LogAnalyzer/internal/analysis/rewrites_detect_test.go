package analysis_test

// Detection and classification of context rewrites (calls that wrote their
// context again instead of reading it from the cache).
//
// Rules under test, per actor, over its main calls in time order:
//   - The first call of each session is never a rewrite; the previous call is
//     the preceding main call of the same session; calls without a session id
//     form one session.
//   - A call is a rewrite when it wrote at least half of what the previous
//     request held and read less than half of it; a call without a cache-read
//     figure, with no time, or after a previous call without context is not analysed.
//   - Kind, first match wins: model changed (both name a model, differing), cold
//     after idle (nothing read), partial rewrite.
//   - The pause is measured between the two calls' start times.
//   - No label names a cause the logs do not prove.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// crHundredK is a first call whose context is exactly 100,000 tokens, all input.
func crHundredK(id string, sec int) domain.InvocationUsage {
	return crCall(id, sec, "", 100_000, 0, 0)
}

func TestRewrites_FirstCallOfASessionIsNeverARewrite(t *testing.T) {
	a := crOrch(t, crCall("r1", 0, "", 10, 0, 500_000))

	crRequireRewrites(t, a, 0)
}

func TestRewrites_ColdCallAfterAPauseIsColdAfterIdleWithTheTruePause(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crCall("r2", 600, "", 10, 0, 100_000),
	)

	crRequireRewrites(t, a, 1)
	r := a.Rewrites[0]
	crAssertKind(t, r, domain.RewriteColdIdle)
	crAssertPause(t, r, 10*time.Minute)
	if r.Call != 1 {
		t.Errorf("Call = %d, want 1 (index of the second call)", r.Call)
	}
	if got := r.Label(); got != "cold after 10 min idle" {
		t.Errorf("Label() = %q, want cold after 10 min idle", got)
	}
}

func TestRewrites_ColdPauseIsMeasuredFromThePreviousCallsStartInWholeMinutes(t *testing.T) {
	cases := []struct {
		pauseSec int
		label    string
	}{
		{30, "cold after <1 min idle"},
		{59, "cold after <1 min idle"},
		{60, "cold after 1 min idle"},
		{119, "cold after 1 min idle"},
		{3600, "cold after 60 min idle"},
	}
	for _, c := range cases {
		a := crOrch(t,
			crCall("r1", 100, "", 10, 0, 100_000),
			crCall("r2", 100+c.pauseSec, "", 10, 0, 100_000),
		)

		crRequireRewrites(t, a, 1)
		crAssertPause(t, a.Rewrites[0], time.Duration(c.pauseSec)*time.Second)
		if got := a.Rewrites[0].Label(); got != c.label {
			t.Errorf("pause %ds: Label() = %q, want %q", c.pauseSec, got, c.label)
		}
	}
}

func TestRewrites_CallThatReadSomeAndWroteALotIsAPartialRewrite(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crCall("r2", 30, "", 10, 30_000, 60_000),
	)

	crRequireRewrites(t, a, 1)
	crAssertKind(t, a.Rewrites[0], domain.RewritePartial)
	crAssertPause(t, a.Rewrites[0], 30*time.Second)
	if got := a.Rewrites[0].Label(); got != "partial rewrite" {
		t.Errorf("Label() = %q, want partial rewrite", got)
	}
}

func TestRewrites_ModelChangeWithAColdCallIsCountedOnceAsModelChanged(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crWithModel(crCall("r2", 900, "", 10, 0, 100_000), crModelB),
	)

	crRequireRewrites(t, a, 1)
	r := a.Rewrites[0]
	crAssertKind(t, r, domain.RewriteModelChanged)
	if r.PreviousModel != ctxModel {
		t.Errorf("PreviousModel = %q, want %q", r.PreviousModel, ctxModel)
	}
	crAssertPause(t, r, 15*time.Minute)
	if got := r.Label(); got != "model changed" {
		t.Errorf("Label() = %q, want model changed", got)
	}
}

func TestRewrites_ModelChangeWithAPartialRewriteIsModelChanged(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crWithModel(crCall("r2", 20, "", 10, 30_000, 60_000), crModelB),
	)

	crRequireRewrites(t, a, 1)
	crAssertKind(t, a.Rewrites[0], domain.RewriteModelChanged)
}

func TestRewrites_ModelChangeThatStillReadTheCacheIsNotARewrite(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crWithModel(crCall("r2", 20, "", 10, 90_000, 1_000), crModelB),
	)

	crRequireRewrites(t, a, 0)
}

func TestRewrites_ColdCallWhoseModelIsNotNamedBySomeCallIsNotModelChanged(t *testing.T) {
	prev := crWithModel(crCall("r1", 0, "", 10, 0, 100_000), "")
	a := crOrch(t, prev, crCall("r2", 300, "", 10, 0, 100_000))

	crRequireRewrites(t, a, 1)
	crAssertKind(t, a.Rewrites[0], domain.RewriteColdIdle)
}

func TestRewrites_WriteBoundaryIsHalfOfThePreviousContext(t *testing.T) {
	enough := crOrch(t, crHundredK("r1", 0), crCall("r2", 300, "", 0, 0, 50_000))
	short := crOrch(t, crHundredK("r1", 0), crCall("r2", 300, "", 0, 0, 49_999))

	crRequireRewrites(t, enough, 1)
	crRequireRewrites(t, short, 0)
}

func TestRewrites_ReadBoundaryIsHalfOfThePreviousContext(t *testing.T) {
	justUnder := crOrch(t, crHundredK("r1", 0), crCall("r2", 300, "", 0, 49_999, 60_000))
	half := crOrch(t, crHundredK("r1", 0), crCall("r2", 300, "", 0, 50_000, 60_000))

	crRequireRewrites(t, justUnder, 1)
	crAssertKind(t, justUnder.Rewrites[0], domain.RewritePartial)
	crRequireRewrites(t, half, 0)
}

func TestRewrites_ContextDropAfterACompactionWritesLittleAndIsNotARewrite(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 300_000, 0, 0),
		crCall("r2", 900, "", 20_000, 0, 5_000),
	)

	crRequireRewrites(t, a, 0)
}

func TestRewrites_CallWithoutACacheReadFigureIsNeverARewrite(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crNoCacheRead(crCall("r2", 900, "", 10, 0, 100_000)),
	)

	crRequireRewrites(t, a, 0)
}

func TestRewrites_CallWithoutATimeIsNotAnalysed(t *testing.T) {
	late := crCall("r2", 900, "", 10, 0, 100_000)
	late.At = time.Time{}
	a := crOrch(t, crCall("r1", 0, "", 10, 0, 100_000), late)

	crRequireRewrites(t, a, 0)
}

func TestRewrites_PreviousCallWithoutContextLeavesTheNextOneUnanalysed(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 0, 0, 0),
		crCall("r2", 900, "", 10, 0, 100_000),
	)

	crRequireRewrites(t, a, 0)
}

func TestRewrites_SessionsAreAnalysedApartAndEachFirstCallIsExcluded(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "s1", 10, 0, 100_000),
		crCall("r2", 100, "s2", 10, 0, 100_000), // first call of s2
		crCall("r3", 200, "s1", 10, 0, 100_000), // previous call is r1, not r2
		crCall("r4", 2000, "s2", 10, 0, 100_000),
	)

	crRequireRewrites(t, a, 2)
	if a.Rewrites[0].Call != 2 || a.Rewrites[1].Call != 3 {
		t.Errorf("rewritten calls = %d, %d, want 2 and 3", a.Rewrites[0].Call, a.Rewrites[1].Call)
	}
	crAssertPause(t, a.Rewrites[0], 200*time.Second)
	crAssertPause(t, a.Rewrites[1], 1900*time.Second)
}

func TestRewrites_CallsWithoutASessionIdFormOneSession(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crCall("r2", 600, "", 10, 0, 100_000),
		crCall("r3", 1200, "", 10, 0, 100_000),
	)

	crRequireRewrites(t, a, 2)
	crAssertPause(t, a.Rewrites[1], 10*time.Minute)
}

func TestRewrites_OtherKindsOfCallsAreNeitherRewritesNorThePreviousCall(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crWithKind(crCall("r2", 300, "", 10, 0, 100_000), domain.CallAuxiliary),
		crWithKind(crCall("r3", 400, "", 10, 0, 100_000), domain.CallCompaction),
		crCall("r4", 600, "", 10, 0, 100_000),
	)

	crRequireRewrites(t, a, 1)
	if a.Rewrites[0].Call != 3 {
		t.Errorf("rewritten call = %d, want 3 (the second main call)", a.Rewrites[0].Call)
	}
	crAssertPause(t, a.Rewrites[0], 10*time.Minute)
}

func TestRewrites_EachActorIsAnalysedOnItsOwnCalls(t *testing.T) {
	run := crPrice(t, ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			crCall("o1", 0, "", 10, 0, 100_000),
			crCall("o2", 30, "", 10, 90_000, 500),
		},
		agents: []ctxAgent{{id: "coder#1", typ: "coder", usage: []domain.InvocationUsage{
			crCall("a1", 0, "", 10, 0, 50_000),
			crCall("a2", 300, "", 10, 0, 50_000),
		}}},
	})

	crRequireRewrites(t, run.Context.Orchestrator, 0)
	agent := ctxRequireAgent(t, run, "coder#1")
	crRequireRewrites(t, agent, 1)
	crAssertKind(t, agent.Rewrites[0], domain.RewriteColdIdle)
	crAssertPause(t, agent.Rewrites[0], 5*time.Minute)
}

func TestRewrites_RewritesComeInTimeOrderAndOnlyProvenLabelsAppear(t *testing.T) {
	a := crOrch(t,
		crCall("r1", 0, "", 10, 0, 100_000),
		crCall("r2", 600, "", 10, 0, 100_000),
		crWithModel(crCall("r3", 700, "", 10, 0, 60_000), crModelB),
		crWithModel(crCall("r4", 800, "", 10, 30_000, 60_000), crModelB),
	)

	crRequireRewrites(t, a, 3)
	wantKinds := []domain.RewriteKind{domain.RewriteColdIdle, domain.RewriteModelChanged, domain.RewritePartial}
	for i, r := range a.Rewrites {
		crAssertKind(t, r, wantKinds[i])
		if !crAllowedLabel.MatchString(r.Label()) {
			t.Errorf("rewrite %d label = %q, want one of the proven causes", i, r.Label())
		}
		if i > 0 && r.Call <= a.Rewrites[i-1].Call {
			t.Errorf("rewrite %d is at call %d, not after call %d", i, r.Call, a.Rewrites[i-1].Call)
		}
	}
}
