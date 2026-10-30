package analysis_test

// Builders for the context rewrite tests. Calls come from ctxCall (input, cache
// read, cache write, output). Prices per token in nanodollars, for model-a and
// model-b alike: input 3000, cache read 300, cache write 3750 (5m tier, the
// default) or 6000 (1h tier). Model "model-unpriced" has no pricing entry.

import (
	"regexp"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const crModelB = domain.ModelID("model-b")

// crAllowedLabel is every label a rewrite may carry: no other cause exists.
var crAllowedLabel = regexp.MustCompile(`^(model changed|cold after (<1|[0-9]+) min idle|partial rewrite)$`)

// crCall is a main call of the given session with 10 output tokens.
func crCall(id string, sec int, session string, in, read, write int64) domain.InvocationUsage {
	u := ctxCall(id, sec, in, read, write, 10)
	u.SessionID = session
	return u
}

func crWithModel(u domain.InvocationUsage, m domain.ModelID) domain.InvocationUsage {
	u.Model = m
	return u
}

func crWithKind(u domain.InvocationUsage, k domain.CallKind) domain.InvocationUsage {
	u.Kind = k
	return u
}

// crNoCacheRead drops the cache-read figure of the call.
func crNoCacheRead(u domain.InvocationUsage) domain.InvocationUsage {
	u.Usage.CacheRead = domain.AbsentTokens()
	return u
}

func crTable(t *testing.T) domain.PricingTable {
	t.Helper()
	return domain.NewPricingTable([]domain.ModelPricing{
		flatModelPricing(t, ctxModel), flatModelPricing(t, crModelB),
	})
}

// crPrice prices the spec's run with both models priced.
func crPrice(t *testing.T, spec ctxRunSpec) domain.RunReport {
	t.Helper()
	report, _ := analysis.Price(ctxAggregate(spec), crTable(t), domain.DefaultAnalysisSettings(), testSource())
	if len(report.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(report.Runs))
	}
	return report.Runs[0]
}

// crOrch prices a run with only orchestrator calls and returns the orchestrator's context.
func crOrch(t *testing.T, calls ...domain.InvocationUsage) domain.ActorContext {
	t.Helper()
	return crPrice(t, ctxRunSpec{orchUsage: calls}).Context.Orchestrator
}

// crRequireRewrites fails unless the actor has exactly n rewrites.
func crRequireRewrites(t *testing.T, a domain.ActorContext, n int) {
	t.Helper()
	if len(a.Rewrites) != n {
		t.Fatalf("rewrites = %d (%+v), want %d", len(a.Rewrites), a.Rewrites, n)
	}
}

func crAssertPause(t *testing.T, r domain.ContextRewrite, want time.Duration) {
	t.Helper()
	if r.Pause != want {
		t.Errorf("pause = %v, want %v", r.Pause, want)
	}
}

func crAssertKind(t *testing.T, r domain.ContextRewrite, want domain.RewriteKind) {
	t.Helper()
	if r.Kind != want {
		t.Errorf("kind = %q (%s), want %q", r.Kind.Key(), r.Label(), want.Key())
	}
}
