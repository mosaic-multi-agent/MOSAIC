package analysis_test

// Tests that reasoning tokens never feed token sums behind ratios.
//
// Rules under test:
//   - The cache hit ratio and the across-runs per-run token average are
//     computed from the four billable categories only; reasoning is part of
//     output and adding it would double-count.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func ratioReport(t *testing.T, reasoning domain.TokenCount) domain.RunReport {
	t.Helper()
	usage := domain.TokenUsage{
		Input:     domain.Tokens(1000),
		CacheRead: domain.Tokens(3000),
		Output:    domain.Tokens(200),
		Reasoning: reasoning,
	}
	in := oneAgentStream(testRunRef, "coder#1",
		rmInvStart("coder#1", "coder", rmAt(1)),
		usageRecordEvent("coder#1", "r1", "model-a", usage),
	)
	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)
	return requireRunReport(t, report, testRunID)
}

func TestOverview_CacheHitRatio_IgnoresReasoningTokens(t *testing.T) {
	without := ratioReport(t, domain.AbsentTokens())
	with := ratioReport(t, domain.Tokens(150))

	assertRatio(t, "cache hit ratio without reasoning", without.Overview.CacheHitRatio, 3_000, 4_000)
	assertRatio(t, "cache hit ratio with reasoning", with.Overview.CacheHitRatio, 3_000, 4_000)
}

func TestAcrossRuns_PerRunTokens_IgnoreReasoningTokens(t *testing.T) {
	perRun := func(reasoning domain.TokenCount) domain.Ratio {
		in := oneAgentStream(testRunRef, "coder#1",
			rmInvStart("coder#1", "coder", rmAt(1)),
			usageRecordEvent("coder#1", "r1", "model-a", domain.TokenUsage{
				Input:     domain.Tokens(1000),
				CacheRead: domain.Tokens(3000),
				Output:    domain.Tokens(200),
				Reasoning: reasoning,
			}),
		)
		report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)
		for _, g := range report.AcrossRuns.ByAgentType {
			if g.AgentType == "coder" {
				return g.PerRunTokens
			}
		}
		t.Fatalf("no coder agent type in across-runs view: %+v", report.AcrossRuns.ByAgentType)
		return domain.Ratio{}
	}

	assertRatio(t, "per-run tokens without reasoning", perRun(domain.AbsentTokens()), 4_200, 1)
	assertRatio(t, "per-run tokens with reasoning", perRun(domain.Tokens(150)), 4_200, 1)
}
