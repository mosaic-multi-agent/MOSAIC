package screens_test

// Fixture for the across-runs view. Money is given in whole cents so every
// rendered amount is distinct and can be found on its row.

import (
	"fmt"
	"strings"

	"mosaic-log-analyzer/internal/domain"
)

const (
	acrossRunA = "20260101T120000Z-aaaa"
	acrossRunB = "20260102T120000Z-bbbb"
	acrossRunC = "20260103T120000Z-cccc"
)

func acrossTotals(cents int64, complete bool) domain.Totals {
	return domain.Totals{Money: domain.CategoryMoney{Total: known(cents), Complete: complete}}
}

// acrossReport has three named runs and a separate unattributable bucket.
//
//	run aaaa  $10.00 complete
//	run bbbb  $6.00 complete
//	run cccc  $3.21 partial, provisional, unpriced model, assumed-tier cache writes
//	unattributable $99.99 (must never appear in the view)
//
// Agent types: "coder" over 47 invocations in 3 runs ($16.00 partial, $5.33 per run, one without usage), "tester"
// over 2 runs ($2.40, $1.20 per run, one open invocation, assumed-tier cache
// writes) and the unknown type over 1 run ($0.77, $0.77 per run).
func acrossReport() domain.Report {
	partial := acrossTotals(321, false)
	partial.AssumedTierCacheWrite = 1234
	coder := domain.GroupFigures{
		Invocations: 47, Totals: acrossTotals(1600, false), UnpricedModels: []domain.PricingKey{{Model: "m-unpriced"}}, MissingUsage: 1,
	}
	tester := domain.GroupFigures{Invocations: 2, Totals: acrossTotals(240, true), Open: 1}
	tester.Totals.AssumedTierCacheWrite = 99
	unknown := domain.GroupFigures{Invocations: 1, Totals: acrossTotals(77, true)}
	stats := func(cents int64, complete bool) domain.MoneyStats {
		return domain.MoneyStats{
			Basis: 1, Min: known(cents), Max: known(cents), Average: known(cents), Complete: complete,
		}
	}
	unattributable := domain.RunReport{Run: domain.UnattributableRun(), Totals: acrossTotals(9999, true)}

	return domain.Report{
		Currency:       domain.Currency,
		Quality:        domain.NewQualitySummary(nil),
		Unattributable: &unattributable,
		AcrossRuns: domain.AcrossRuns{
			Runs: []domain.RunSummary{
				{Run: domain.NamedRun(acrossRunA), Totals: acrossTotals(1000, true), Subagents: domain.GroupFigures{Invocations: 3}},
				{Run: domain.NamedRun(acrossRunB), Totals: acrossTotals(600, true), Subagents: domain.GroupFigures{Invocations: 2}},
				{Run: domain.NamedRun(acrossRunC), Provisional: true, Totals: partial,
					UnpricedModels: []domain.PricingKey{{Model: "m-unpriced"}}, Subagents: domain.GroupFigures{Invocations: 2, Open: 1}},
			},
			ByAgentType: []domain.AgentTypeAcrossRuns{
				{AgentType: "coder", Runs: 3, ProvisionalRuns: 1, Figures: coder,
					PerRunInvocations: domain.Ratio{Numerator: 47, Denominator: 3}, PerRunMoney: stats(533, false),
					PerRunTokens: domain.Ratio{Numerator: 123456, Denominator: 3}},
				{AgentType: "tester", Runs: 2, ProvisionalRuns: 1, Figures: tester,
					PerRunInvocations: domain.Ratio{Numerator: 2, Denominator: 2}, PerRunMoney: stats(120, true)},
				{AgentType: "", Runs: 1, Figures: unknown,
					PerRunInvocations: domain.Ratio{Numerator: 1, Denominator: 1}, PerRunMoney: stats(77, true)},
			},
		},
	}
}

// acrossCompleteReport has two fully priced, finished runs and one agent type,
// so no incompleteness label may be shown.
func acrossCompleteReport() domain.Report {
	report := acrossReport()
	report.Unattributable = nil
	report.AcrossRuns = domain.AcrossRuns{
		Runs: report.AcrossRuns.Runs[:2],
		ByAgentType: []domain.AgentTypeAcrossRuns{{
			AgentType: "coder", Runs: 2,
			Figures:           domain.GroupFigures{Invocations: 5, Totals: acrossTotals(1600, true)},
			PerRunInvocations: domain.Ratio{Numerator: 5, Denominator: 2},
			PerRunMoney: domain.MoneyStats{
				Basis: 2, Min: known(600), Max: known(1000), Average: known(800), Complete: true,
			},
		}},
	}
	return report
}

// Per-run averages of the fixture's coder type: 47 invocations over 3 runs and
// 123456 tokens over 3 runs.
const (
	coderPerRunInvocations = "15.7"
	coderPerRunTokens      = "41152"
)

// withoutThousands removes thousands separators so a count is found whichever
// way the table groups its digits.
func withoutThousands(s string) string { return strings.ReplaceAll(stripStyling(s), ",", "") }

// manyAcrossRunsReport has n fully priced runs named run-01 ... and no agent types.
func manyAcrossRunsReport(n int) domain.Report {
	report := domain.Report{Currency: domain.Currency, Quality: domain.NewQualitySummary(nil)}
	for i := 1; i <= n; i++ {
		report.AcrossRuns.Runs = append(report.AcrossRuns.Runs, domain.RunSummary{
			Run: domain.NamedRun(fmt.Sprintf("run-%02d", i)), Totals: acrossTotals(int64(i), true),
		})
	}
	return report
}
