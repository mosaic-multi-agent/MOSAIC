package analysis_test

// Shared scenario for the across-runs tests: three named runs plus an
// unattributable bucket, built in memory. Rates per 1M tokens for model "m-a"
// come from viewsPricing: input 1.00, cache write 5m 1.25, output 5.00, i.e.
// 1000 / 1250 / 5000 nanodollars per token. Model "m-unpriced" has no entry
// unless a test adds one at the same rates.
//
//	run   ended  actor         type    model       usage                         end
//	aaaa  yes    orchestrator  -       m-a         in 1000                       -
//	aaaa         coder#1       coder   m-a         in 2000 out 100               SUCCESS
//	aaaa         coder#2       coder   m-a         in 1000                       SUCCESS
//	aaaa         tester#3      tester  m-a         in 500                        SUCCESS
//	aaaa         coder#4       coder   -           no usage record at all        SUCCESS
//	bbbb  yes    orchestrator  -       m-a         in 200                        -
//	bbbb         coder#1       coder   m-a         in 4000 out 200               SUCCESS
//	bbbb         x#2           (none)  m-a         in 100                        SUCCESS
//	cccc  no     coder#1       coder   m-unpriced  in 300 out 30                 SUCCESS
//	cccc         tester#2      tester  m-a         in 700 cc 100 (no split)      open
//	-     -      u#1 (unattributable) ghost m-a    in 99999                      SUCCESS
//
// Money in nanodollars per run: aaaa 5,000,000 (subagents 4,000,000); bbbb
// 5,300,000 (subagents 5,100,000); cccc 825,000 known and partial because coder#1
// is unpriced (the tester's 100 cache-write tokens are priced at the assumed
// 5m tier: 125,000 of it).

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const (
	arRunA = "20260101T000000Z-aaaa"
	arRunB = "20260102T000000Z-bbbb"
	arRunC = "20260103T000000Z-cccc"
)

// arAgent builds one subagent stream with a start (when typ is set), one usage
// record and, when closed, an end event.
func arAgent(run domain.RunRef, id domain.AgentInstanceID, typ string, model domain.ModelID, usage domain.TokenUsage, closed bool) analysis.Stream {
	var events []domain.Event
	if typ != "" {
		events = append(events, rmInvStart(id, typ, rmAt(1)))
	}
	events = append(events, usageRecordEvent(id, "rec-"+string(id), model, usage))
	if closed {
		events = append(events, rmInvEnd(id, "SUCCESS", model, rmAt(9)))
	}
	return analysis.Stream{Ref: agentStream(run, id), Events: events}
}

// arOrchestrator builds an orchestrator stream with one usage record and a run_end.
func arOrchestrator(run domain.RunRef, input int64) analysis.Stream {
	return analysis.Stream{Ref: orchStream(run), Events: []domain.Event{
		usageRecordEvent("", "orch-rec", viewsModel, viewsUsage(input, 0, 0, 0)),
		runEndEvent(),
	}}
}

// arSilentAgent builds a closed subagent stream that never reported any usage.
func arSilentAgent(run domain.RunRef, id domain.AgentInstanceID, typ string) analysis.Stream {
	return analysis.Stream{Ref: agentStream(run, id), Events: []domain.Event{
		rmInvStart(id, typ, rmAt(1)),
		rmInvEnd(id, "SUCCESS", "", rmAt(9)),
	}}
}

func arInput() analysis.Input {
	a, b, c := domain.NamedRun(arRunA), domain.NamedRun(arRunB), domain.NamedRun(arRunC)
	return analysis.Input{Streams: []analysis.Stream{
		// Streams deliberately not in run order: the report orders runs itself.
		arAgent(c, "coder#1", "coder", viewsUnpriced, viewsUsage(300, 0, 0, 30), true),
		arAgent(c, "tester#2", "tester", viewsModel, viewsUsage(700, 0, 100, 0), false),
		arOrchestrator(b, 200),
		arAgent(b, "coder#1", "coder", viewsModel, viewsUsage(4000, 0, 0, 200), true),
		arAgent(b, "x#2", "", viewsModel, viewsUsage(100, 0, 0, 0), true),
		arOrchestrator(a, 1000),
		arAgent(a, "coder#1", "coder", viewsModel, viewsUsage(2000, 0, 0, 100), true),
		arAgent(a, "coder#2", "coder", viewsModel, viewsUsage(1000, 0, 0, 0), true),
		arAgent(a, "tester#3", "tester", viewsModel, viewsUsage(500, 0, 0, 0), true),
		arSilentAgent(a, "coder#4", "coder"),
		arAgent(domain.UnattributableRun(), "u#1", "ghost", viewsModel, viewsUsage(99999, 0, 0, 0), true),
	}}
}

// arPricingBoth prices "m-unpriced" at the same rates as "m-a".
func arPricingBoth(t *testing.T) domain.PricingTable {
	t.Helper()
	base := viewsPricing(t)
	entry, _ := base.Lookup(viewsModel)
	entry.Model = viewsUnpriced
	return base.With(entry)
}

func arReport(t *testing.T, table domain.PricingTable) domain.Report {
	t.Helper()
	return analysis.Analyze(arInput(), table, domain.DefaultAnalysisSettings(), testSource(), nil)
}

func arSummary(t *testing.T, ac domain.AcrossRuns, id string) domain.RunSummary {
	t.Helper()
	for _, s := range ac.Runs {
		if s.Run.ID == id {
			return s
		}
	}
	t.Fatalf("across-runs has no summary for run %s among %d", id, len(ac.Runs))
	return domain.RunSummary{}
}

func arType(t *testing.T, ac domain.AcrossRuns, agentType string) domain.AgentTypeAcrossRuns {
	t.Helper()
	for _, g := range ac.ByAgentType {
		if g.AgentType == agentType {
			return g
		}
	}
	t.Fatalf("across-runs has no agent type %q among %d", agentType, len(ac.ByAgentType))
	return domain.AgentTypeAcrossRuns{}
}

func assertMoneyStats(t *testing.T, name string, got domain.MoneyStats, basis int, min, max, avg int64, complete bool) {
	t.Helper()
	if got.Basis != basis || got.Complete != complete {
		t.Errorf("%s: basis = %d, complete = %v, want %d and %v", name, got.Basis, got.Complete, basis, complete)
	}
	assertMoneyKnown(t, name+" min", got.Min, min)
	assertMoneyKnown(t, name+" max", got.Max, max)
	assertMoneyKnown(t, name+" average", got.Average, avg)
}
