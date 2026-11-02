package screens_test

// Fixture for the by-agent-type view. The group figures are set directly and
// are deliberately independent of the dispatch lists, so each test verifies
// that the screen renders the analysed figures rather than recomputing them.

import (
	"fmt"

	"mosaic-log-analyzer/internal/domain"
)

func moneyTotals(cents int64) domain.Totals {
	return domain.Totals{Money: domain.CategoryMoney{Total: known(cents), Complete: true}}
}

func closedDispatch(id, status string, cents int64) domain.Dispatch {
	return domain.Dispatch{
		Invocation: domain.Invocation{
			Actor:      domain.AgentInstance(domain.AgentInstanceID(id)),
			State:      domain.InvocationClosed,
			StatusCode: status,
			EndEvents:  1,
		},
		Totals: moneyTotals(cents),
	}
}

func stats(basis int, min, max, avg int64) domain.MoneyStats {
	return domain.MoneyStats{Basis: basis, Min: known(min), Max: known(max), Average: known(avg), Complete: true}
}

// agentTypesRun has three groups in contract order: builder, reviewer and the
// unknown type last.
func agentTypesRun() domain.RunReport {
	run := overviewRun()
	run.ByAgentType = []domain.AgentTypeGroup{
		{
			AgentType:     "builder",
			Figures:       domain.GroupFigures{Invocations: 12, Totals: moneyTotals(6025)},
			PerInvocation: stats(12, 137, 964, 502),
			Dispatches: []domain.Dispatch{
				closedDispatch("builder#1", "SUCCESS", 221),
				closedDispatch("builder#2", "SUCCESS", 343),
			},
		},
		{
			AgentType:     "reviewer",
			Figures:       domain.GroupFigures{Invocations: 15, Totals: moneyTotals(2050)},
			PerInvocation: stats(15, 45, 388, 136),
			Dispatches: []domain.Dispatch{
				closedDispatch("reviewer#1", "SUCCESS", 456),
				closedDispatch("reviewer#2", "SUCCESS", 567),
			},
		},
		{
			AgentType:     "",
			Figures:       domain.GroupFigures{Invocations: 21, Totals: moneyTotals(777)},
			PerInvocation: stats(21, 777, 777, 777),
			Dispatches:    []domain.Dispatch{closedDispatch("mystery#1", "SUCCESS", 777)},
		},
	}
	return run
}

// manyTypesRun has n groups type-01 ... each with one dispatch type-NN#1. The
// first group costs the most, so the default cost order lists them as numbered.
func manyTypesRun(n int) domain.RunReport {
	run := overviewRun()
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("type-%02d", i)
		cents := int64(n - i + 1)
		run.ByAgentType = append(run.ByAgentType, domain.AgentTypeGroup{
			AgentType:     name,
			Figures:       domain.GroupFigures{Invocations: 1, Totals: moneyTotals(cents)},
			PerInvocation: stats(1, cents, cents, cents),
			Dispatches:    []domain.Dispatch{closedDispatch(name+"#1", "SUCCESS", cents)},
		})
	}
	return run
}

// manyDispatchesRun has one type "builder" with n dispatches builder#01 ... The
// first dispatch costs the most, so the default cost order lists them as numbered.
func manyDispatchesRun(n int) domain.RunReport {
	run := overviewRun()
	g := domain.AgentTypeGroup{
		AgentType:     "builder",
		Figures:       domain.GroupFigures{Invocations: n, Totals: moneyTotals(100)},
		PerInvocation: stats(n, 1, 1, 1),
	}
	for i := 1; i <= n; i++ {
		g.Dispatches = append(g.Dispatches, closedDispatch(fmt.Sprintf("builder#%02d", i), "SUCCESS", int64(n-i+1)))
	}
	run.ByAgentType = []domain.AgentTypeGroup{g}
	return run
}
