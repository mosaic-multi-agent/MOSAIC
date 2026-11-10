package screens_test

// Fixture for the Dispatches view and the by-type drill-down: one orchestrator
// and six dispatches whose figures are all distinct, so a figure found on a row
// can only have come from that dispatch. Dispatches are listed under their agent
// type in ByAgentType and as actor reports in Agents, as the analysis delivers
// them.

import (
	"time"

	"mosaic-log-analyzer/internal/domain"
)

const (
	// longInstance has 26 characters and longType has 23: the longest names a
	// dispatch row has to hold.
	longInstance = "senior-code-reviewer#00042"
	longType     = "security-audit-reviewer"
)

var dispatchEpoch = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// dispatchOf is a closed dispatch of the given type with its status and cost.
func dispatchOf(id, agentType, status string, cents int64) domain.Dispatch {
	return domain.Dispatch{
		Invocation: domain.Invocation{
			Actor:      domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: domain.AgentInstanceID(id), Type: agentType},
			State:      domain.InvocationClosed,
			StatusCode: status,
			EndEvents:  1,
		},
		Totals: moneyTotals(cents),
	}
}

func withSeconds(d domain.Dispatch, secs int) domain.Dispatch {
	d.Invocation.StartedAt = dispatchEpoch
	d.Invocation.EndedAt = dispatchEpoch.Add(time.Duration(secs) * time.Second)
	return d
}

// withInputTokens gives the dispatch a token mix whose categories sum to n:
// input 40%, cache read 30%, cache write 20% and output 10% (n must be a multiple
// of 10), plus a reasoning part of half the output that is never added to the
// sum. A cell showing only one category, or the sum with reasoning, shows a
// different number.
func withInputTokens(d domain.Dispatch, n int64) domain.Dispatch {
	d.Totals.Tokens = domain.TokenUsage{
		Input:         domain.Tokens(n * 4 / 10),
		CacheRead:     domain.Tokens(n * 3 / 10),
		CacheCreation: domain.Tokens(n * 2 / 10),
		Output:        domain.Tokens(n / 10),
		Reasoning:     domain.Tokens(n / 20),
	}
	return d
}

func withPeak(d domain.Dispatch, tokens int64) domain.Dispatch {
	d.PeakContext = domain.ContextFigure{Tokens: domain.Tokens(tokens), Band: domain.BandOf(tokens)}
	return d
}

// cacheOf builds cache figures: hit ratio num/den, cache writes, net benefit and
// the unread cache writes of the last call.
func cacheOf(num, den, writeTokens, writeCents, netCents, unreadTokens, unreadCents int64) domain.CacheFigures {
	return domain.CacheFigures{
		HitRatio:          domain.Ratio{Numerator: num, Denominator: den},
		WriteTokens:       domain.Tokens(writeTokens),
		WriteCost:         known(writeCents),
		NetBenefit:        known(netCents),
		UnreadWriteTokens: domain.Tokens(unreadTokens),
		UnreadWriteCost:   known(unreadCents),
		Complete:          true,
	}
}

// The six dispatches. Costs, peaks, durations and input tokens rank differently
// so each sort order gives its own sequence:
//
//	        cost   peak     seconds  input tokens
//	d1      3.17   120,000  10       1,000
//	d2      2.28   380,000  50       5,000
//	d3      1.39   260,000  30       9,000
//	d4      0.54   150,000  20       2,000
//	d5      0.35   40,000   40       3,000   (partial cost, no status)
//	d6      unpriced, open, no peak, no duration
func dispatchD1() domain.Dispatch {
	d := withInputTokens(withSeconds(withPeak(dispatchOf("builder#1", "builder", "SUCCESS", 317), 120_000), 10), 1_000)
	d.Cache = cacheOf(3, 4, 12_345, 77, 111, 4_567, 29)
	return d
}

func dispatchD2() domain.Dispatch {
	d := withInputTokens(withSeconds(withPeak(dispatchOf(longInstance, longType, "BLOCKED", 228), 380_000), 50), 5_000)
	d.Cache = cacheOf(1, 8, 98_765, 66, 46, 55_555, 33)
	return d
}

func dispatchD3() domain.Dispatch {
	d := withInputTokens(withSeconds(withPeak(dispatchOf("reviewer#1", "reviewer", "PARTIALLY_DONE", 139), 260_000), 30), 9_000)
	d.Cache = cacheOf(2, 3, 3_210, 19, 12, 1_234, 8)
	return d
}

// dispatchD4 has cache figures but no unread cache writes: its last call carried
// no cache-write figure.
func dispatchD4() domain.Dispatch {
	d := withInputTokens(withSeconds(withPeak(dispatchOf("builder#2", "builder", "SUCCESS", 54), 150_000), 20), 2_000)
	d.Cache = cacheOf(1, 2, 7_777, 11, 5, 0, 0)
	d.Cache.UnreadWriteTokens = domain.AbsentTokens()
	d.Cache.UnreadWriteCost = domain.NoMoneyData()
	return d
}

func dispatchD5() domain.Dispatch {
	d := withInputTokens(withSeconds(withPeak(dispatchOf("builder#3", "builder", "", 35), 40_000), 40), 3_000)
	d.Totals.Money.Complete = false
	return d
}

func dispatchD6() domain.Dispatch {
	d := dispatchOf("builder#4", "builder", "", 0)
	d.Invocation.State = domain.InvocationOpen
	d.Invocation.EndEvents = 0
	d.Totals = domain.Totals{Money: domain.CategoryMoney{Total: domain.UnpricedMoney()}}
	d.UnpricedModels = []domain.PricingKey{{Model: "model-x"}}
	return d
}

// dispatchRun is a run with the orchestrator ($4.00, peak 210,000, cache figures
// in OrchestratorCache with 2,222 unread cache-write tokens) and the six
// dispatches. The orchestrator's context calls carry cache-write values that
// differ from OrchestratorCache, so a row derived from them is told apart.
func dispatchRun() domain.RunReport {
	run := overviewRun()
	run.Totals.Money.Total = known(1080)
	run.Overview.Totals.Money.Total = known(1080)
	run.Cache = domain.CacheFigures{UnreadWriteTokens: domain.Tokens(99_999), Complete: true}
	run.OrchestratorCache = cacheOf(7, 10, 31_313, 41, 17, 2_222, 14)
	run.Context.Orchestrator = domain.ActorContext{
		Actor: domain.Orchestrator(),
		Calls: []domain.ContextCall{
			{RecordID: "o1", Usage: domain.TokenUsage{CacheCreation: domain.Tokens(1_111)}},
			{RecordID: "o2", Usage: domain.TokenUsage{CacheCreation: domain.Tokens(6_543)}},
		},
		Peak: domain.ContextFigure{Tokens: domain.Tokens(210_000), Band: domain.BandOptimal},
	}
	groups := []struct {
		name  string
		cents int64
		ds    []domain.Dispatch
	}{
		{"builder", 1337, []domain.Dispatch{dispatchD1(), dispatchD4(), dispatchD5(), dispatchD6()}},
		{longType, 228, []domain.Dispatch{dispatchD2()}},
		{"reviewer", 139, []domain.Dispatch{dispatchD3()}},
	}
	run.Agents = nil
	run.ByAgentType = nil
	for _, g := range groups {
		run.ByAgentType = append(run.ByAgentType, domain.AgentTypeGroup{
			AgentType:     g.name,
			Figures:       domain.GroupFigures{Invocations: len(g.ds), Totals: moneyTotals(g.cents)},
			PerInvocation: stats(len(g.ds), 1, 1, 1),
			Dispatches:    g.ds,
		})
		for _, d := range g.ds {
			run.Agents = append(run.Agents, domain.ActorReport{
				Actor: d.Invocation.Actor, Totals: d.Totals, Effort: d.Effort, UnpricedModels: d.UnpricedModels,
			})
		}
	}
	return run
}

// dispatchIDs lists the six instance ids in the order d1 ... d6.
func dispatchIDs() []string {
	return []string{"builder#1", longInstance, "reviewer#1", "builder#2", "builder#3", "builder#4"}
}

// withDispatchCount returns a run with n numbered builder dispatches, each one in
// Agents and in ByAgentType, for scrolling tests.
func withDispatchCount(n int) domain.RunReport {
	run := overviewRun()
	g := domain.AgentTypeGroup{AgentType: "builder", Figures: domain.GroupFigures{Invocations: n, Totals: moneyTotals(100)}, PerInvocation: stats(n, 1, 1, 1)}
	for i := 1; i <= n; i++ {
		d := dispatchOf("builder#"+twoDigits(i), "builder", "SUCCESS", int64(n-i+1)) // the first costs the most
		g.Dispatches = append(g.Dispatches, d)
		run.Agents = append(run.Agents, domain.ActorReport{Actor: d.Invocation.Actor, Totals: d.Totals})
	}
	run.ByAgentType = []domain.AgentTypeGroup{g}
	return run
}

func twoDigits(i int) string {
	return string([]byte{byte('0' + i/10), byte('0' + i%10)})
}
