package screens_test

// The Context drill-down marks every rewrite call with "rewrite: <label>" and
// leaves the other calls unmarked.

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// rewriteActor has four calls and three rewrites: cold after 12 minutes (call 1),
// a model change (call 2) and a partial rewrite (call 3).
func rewriteActor() domain.ActorContext {
	calls := []domain.ContextCall{
		ctxCallOf(10, 100_000, 5, 11, "model-w"),
		ctxCallOf(730, 100_000, 5, 12, "model-w"),
		ctxCallOf(800, 100_000, 5, 13, "model-v"),
		ctxCallOf(830, 100_000, 5, 14, "model-v"),
	}
	return domain.ActorContext{
		Actor: ctxAgent("rewr#1", "rewr"), Calls: calls,
		First: ctxFig(100_000, false), Peak: ctxFig(100_000, false), Final: ctxFig(100_000, false),
		Rewrites: []domain.ContextRewrite{
			{Call: 1, Kind: domain.RewriteColdIdle, Pause: 12 * time.Minute, Tokens: 60_000, Cost: known(40), ExtraCost: known(36)},
			{Call: 2, Kind: domain.RewriteModelChanged, Pause: 70 * time.Second, PreviousModel: "model-w", Tokens: 60_000, Cost: known(40), ExtraCost: known(36)},
			{Call: 3, Kind: domain.RewritePartial, Pause: 30 * time.Second, Tokens: 60_000, Cost: known(40), ExtraCost: known(36)},
		},
	}
}

func rewriteRun() domain.RunReport {
	run := contextRun()
	run.Context.Invocations = append(run.Context.Invocations, rewriteActor())
	return run
}

func rewriteDrillView(t *testing.T) string {
	t.Helper()
	return stripStyling(ctxDrill(t, rewriteRun(), "rewr#1", tableW, tallH).View(tableW, tallH))
}

func TestContextDrill_MarksEachRewriteCallWithItsLabel(t *testing.T) {
	view := rewriteDrillView(t)

	assertContains(t, ctxCallRow(t, view, 730), "rewrite: cold after 12 min idle")
	assertContains(t, ctxCallRow(t, view, 800), "rewrite: model changed")
	assertContains(t, ctxCallRow(t, view, 830), "rewrite: partial rewrite")
}

func TestContextDrill_CallsThatAreNotRewritesCarryNoRewriteMark(t *testing.T) {
	view := rewriteDrillView(t)

	assertNotContains(t, strings.ToLower(ctxCallRow(t, view, 10)), "rewrite")
}

func TestContextDrill_RewriteMarksUseOnlyProvenCauses(t *testing.T) {
	view := strings.ToLower(rewriteDrillView(t))

	assertNotContains(t, view, "expired")
}

func TestContextDrill_ActorWithoutRewritesShowsNoRewriteMark(t *testing.T) {
	view := strings.ToLower(stripStyling(ctxDrill(t, rewriteRun(), "coder#1", tableW, tallH).View(tableW, tallH)))

	assertNotContains(t, view, "rewrite:")
}
