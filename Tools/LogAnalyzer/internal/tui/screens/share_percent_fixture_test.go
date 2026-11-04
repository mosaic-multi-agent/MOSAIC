package screens_test

// Fixtures of the share-percent tests. Every table holds five rows whose shares
// of the whole are a clean 50%, 30%, 19%, 0.6% and 0.4% (the last row is varied
// by the tests), so the percent text of a row can be told from a rounded or a
// differently measured one: 0.6% rounds to 1% but must read "<1%".

import (
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// shareWhole is the known cost, in cents, every fixture measures its shares against.
const shareWhole = 10_000

// shareCents are the costs of the five rows of a fixture, in cents.
var shareCents = [5]int64{5000, 3000, 1900, 60, 40}

// lastShare is the cost of the fifth row: its default, an unpriced cost or a
// known zero.
type lastShare struct{ cost domain.MoneyValue }

func defaultLast() lastShare  { return lastShare{cost: known(shareCents[4])} }
func unpricedLast() lastShare { return lastShare{cost: domain.UnpricedMoney()} }
func zeroCostLast() lastShare { return lastShare{cost: known(0)} }

// costOf is the cost of row i of a fixture.
func costOf(i int, last lastShare) domain.MoneyValue {
	if i == 4 {
		return last.cost
	}
	return known(shareCents[i])
}

func totalsOf(v domain.MoneyValue) domain.Totals {
	return domain.Totals{Money: domain.CategoryMoney{Total: v, Complete: true}}
}

// statusShareRun has five run-level status groups: dispatch counts 400, 250, 340, 6
// and 4 (40%, 25%, 34%, 0.6% and 0.4% of 1,000) and the costs of shareCents.
func statusShareRun(last lastShare) domain.RunReport {
	counts := [5]int{400, 250, 340, 6, 4}
	kinds := [5]domain.StatusKind{domain.StatusReported, domain.StatusReported, domain.StatusReported, domain.StatusNone, domain.StatusOpen}
	codes := [5]string{"SUCCESS", "BLOCKED", "WEIRD_CODE", "", ""}
	run := overviewRun()
	run.ByStatus = domain.StatusBreakdown{}
	for i := range counts {
		g := statusGroup(kinds[i], codes[i], counts[i], 0)
		g.Figures.Totals = totalsOf(costOf(i, last))
		run.ByStatus.Run = append(run.ByStatus.Run, g)
	}
	return run
}

// dispatchShareRun has the orchestrator at 50% of the run cost and the dispatches
// alpha#1, beta#2, gamma#3 and delta#4, each the only one of its type.
func dispatchShareRun(last lastShare) domain.RunReport {
	run := overviewRun()
	run.Totals.Money.Total = known(shareWhole)
	run.Overview.Totals.Money.Total = known(shareWhole)
	run.Orchestrator.Totals = totalsOf(known(shareCents[0]))
	run.ByAgentType = nil
	ids := []string{"alpha#1", "beta#2", "gamma#3", "delta#4"}
	for i, id := range ids {
		cost := costOf(i+1, last)
		d := closedDispatch(id, "SUCCESS", 0)
		d.Totals = totalsOf(cost)
		run.ByAgentType = append(run.ByAgentType, domain.AgentTypeGroup{
			AgentType:  id[:len(id)-2],
			Figures:    domain.GroupFigures{Invocations: 1, Totals: totalsOf(cost)},
			Dispatches: []domain.Dispatch{d},
		})
	}
	return run
}

// modelShareRun has the models model-a ... model-e.
func modelShareRun(last lastShare) domain.RunReport {
	run := overviewRun()
	run.Overview.Totals.Money.Total = known(shareWhole)
	run.Overview.ByModel = nil
	for i, name := range []string{"model-a", "model-b", "model-c", "model-d", "model-e"} {
		cost := costOf(i, last)
		run.Overview.ByModel = append(run.Overview.ByModel, domain.ModelFigures{
			Model: domain.ModelID(name), Priced: cost.State == domain.MoneyKnown, Requests: 1, Totals: totalsOf(cost),
		})
	}
	return run
}

// toolShareRun has the tools Alpha ... Epsilon with 500, 300, 190, 6 and 4 calls.
func toolShareRun() domain.RunReport {
	calls := [5]int{500, 300, 190, 6, 4}
	var tools []domain.ToolFigures
	for i, name := range []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"} {
		tools = append(tools, domain.ToolFigures{
			Tool: name, Calls: calls[i], Durations: durationStats(calls[i], time.Second, time.Millisecond, time.Millisecond, time.Millisecond),
		})
	}
	return runWithTools(tools...)
}

// acrossShareReport has five runs and four agent types; the run costs are the
// shares of shareCents, the type costs 70%, 29%, 0.6% and the last row's cost.
func acrossShareReport(last lastShare) domain.Report {
	report := acrossReport()
	report.Unattributable = nil
	report.AcrossRuns = domain.AcrossRuns{}
	runIDs := []string{
		"20260101T120000Z-aaaa", "20260102T120000Z-bbbb", "20260103T120000Z-cccc", "20260104T120000Z-dddd", "20260105T120000Z-eeee",
	}
	for i, id := range runIDs {
		report.AcrossRuns.Runs = append(report.AcrossRuns.Runs, domain.RunSummary{
			Run: domain.NamedRun(id), Totals: totalsOf(costOf(i, last)), Subagents: domain.GroupFigures{Invocations: 1},
		})
	}
	costs := [4]domain.MoneyValue{known(7000), known(2900), known(60), last.cost}
	for i, name := range []string{"coder", "tester", "scribe", "oiler"} {
		cost := costs[i]
		report.AcrossRuns.ByAgentType = append(report.AcrossRuns.ByAgentType, domain.AgentTypeAcrossRuns{
			AgentType: name, Runs: 1,
			Figures:           domain.GroupFigures{Invocations: 1, Totals: totalsOf(cost)},
			PerRunInvocations: domain.Ratio{Numerator: 1, Denominator: 1},
			PerRunMoney:       domain.MoneyStats{Basis: 1, Min: cost, Max: cost, Average: cost, Complete: true},
		})
	}
	return report
}
