package analysis_test

// Shared builders for the rework, cache and comparison tests. Aggregates are
// built directly so every dispatch's type, status, timing and usage is explicit.
//
// Prices come from flatModelPricing: input 3000 nanodollars per token, cache
// read 300, cache write 3750 (assumed 5m tier) or 6000 (1h tier), output 15000.
// An input-only dispatch of N tokens therefore costs N*3000 nanodollars.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const (
	rwNoTime        = -1 // a start or end second meaning "no timestamp"
	rwUnpricedModel = domain.ModelID("model-unpriced")
)

func rwSecs(sec int) time.Time {
	if sec == rwNoTime {
		return time.Time{}
	}
	return ctxAt(sec)
}

// rwDispatch describes one subagent dispatch of a test run.
type rwDispatch struct {
	id     domain.AgentInstanceID
	typ    string
	state  domain.InvocationState
	status string
	start  int // seconds from the base time, or rwNoTime
	end    int // seconds from the base time, or rwNoTime
	usage  []domain.InvocationUsage
}

// rwRun describes one run of a test aggregate.
type rwRun struct {
	id          string
	provisional bool
	orch        []domain.InvocationUsage
	dispatches  []rwDispatch
}

// rwInputUsage is one per-call record of input-only tokens (cost N*3000 nanos).
func rwInputUsage(id domain.AgentInstanceID, tokens int64) []domain.InvocationUsage {
	return []domain.InvocationUsage{ctxInputOnlyCall("rec-"+string(id), 0, tokens, 0)}
}

// rwUnpricedUsage is one per-call record of input-only tokens of a model that
// has no pricing entry.
func rwUnpricedUsage(id domain.AgentInstanceID, tokens int64) []domain.InvocationUsage {
	u := rwInputUsage(id, tokens)
	u[0].Model = rwUnpricedModel
	return u
}

// rwClosed is a closed dispatch with the given status ("" = closed, no status).
func rwClosed(id domain.AgentInstanceID, typ, status string, start, end int, tokens int64) rwDispatch {
	return rwDispatch{
		id: id, typ: typ, state: domain.InvocationClosed, status: status,
		start: start, end: end, usage: rwInputUsage(id, tokens),
	}
}

// rwOpen is a dispatch with no invocation_end.
func rwOpen(id domain.AgentInstanceID, typ string, start int, tokens int64) rwDispatch {
	return rwDispatch{
		id: id, typ: typ, state: domain.InvocationOpen,
		start: start, end: rwNoTime, usage: rwInputUsage(id, tokens),
	}
}

func rwAggregate(runs ...rwRun) domain.Aggregate {
	var agg domain.Aggregate
	for _, spec := range runs {
		run := domain.RunAggregate{
			Run:          domain.NamedRun(spec.id),
			Provisional:  spec.provisional,
			Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: spec.orch},
			Facts:        domain.RunFacts{Harnesses: []domain.Harness{domain.HarnessClaudeCode}},
		}
		for _, d := range spec.dispatches {
			actor := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: d.id, Type: d.typ}
			run.Agents = append(run.Agents, domain.ActorAggregate{Actor: actor, Invocations: d.usage})
			run.Activity.Invocations = append(run.Activity.Invocations, domain.Invocation{
				Actor:      actor,
				State:      d.state,
				StatusCode: d.status,
				StartedAt:  rwSecs(d.start),
				EndedAt:    rwSecs(d.end),
				Usage:      d.usage,
			})
		}
		agg.Runs = append(agg.Runs, run)
	}
	return agg
}

// rwPriceReport prices an aggregate with only ctxModel priced.
func rwPriceReport(t *testing.T, agg domain.Aggregate) domain.Report {
	t.Helper()
	table := domain.NewPricingTable([]domain.ModelPricing{flatModelPricing(t, ctxModel)})
	report, _ := analysis.Price(agg, table, domain.DefaultAnalysisSettings(), testSource())
	return report
}

// rwPrice prices an aggregate and returns the named run's report.
func rwPrice(t *testing.T, agg domain.Aggregate, runID string) domain.RunReport {
	t.Helper()
	return requireRunReport(t, rwPriceReport(t, agg), runID)
}

// rwPriceRun prices one run made of the given dispatches (no orchestrator usage).
func rwPriceRun(t *testing.T, dispatches ...rwDispatch) domain.RunReport {
	t.Helper()
	return rwPrice(t, rwAggregate(rwRun{id: ctxRunID, dispatches: dispatches}), ctxRunID)
}

func rwTypeGroup(t *testing.T, run domain.RunReport, typ string) domain.AgentTypeGroup {
	t.Helper()
	for _, g := range run.ByAgentType {
		if g.AgentType == typ {
			return g
		}
	}
	t.Fatalf("no agent type group %q among %d groups", typ, len(run.ByAgentType))
	return domain.AgentTypeGroup{}
}

func rwDispatchOf(t *testing.T, run domain.RunReport, id domain.AgentInstanceID) domain.Dispatch {
	t.Helper()
	for _, g := range run.ByAgentType {
		for _, d := range g.Dispatches {
			if d.Invocation.Actor.Instance == id {
				return d
			}
		}
	}
	t.Fatalf("no dispatch %s in the report", id)
	return domain.Dispatch{}
}

// rwAssertGroup checks a group's member count and, when wantNanos >= 0, that its
// money total is Known with that amount.
func rwAssertGroup(t *testing.T, name string, g domain.GroupFigures, wantMembers int, wantNanos int64) {
	t.Helper()
	if g.Invocations != wantMembers {
		t.Errorf("%s: invocations = %d, want %d", name, g.Invocations, wantMembers)
	}
	if wantNanos >= 0 {
		assertMoneyKnown(t, name+" cost", g.Totals.Money.Total, wantNanos)
	}
}

// rwAssertEmptyGroup checks a group has no members and no invented zero cost.
func rwAssertEmptyGroup(t *testing.T, name string, g domain.GroupFigures) {
	t.Helper()
	if g.Invocations != 0 {
		t.Errorf("%s: invocations = %d, want 0", name, g.Invocations)
	}
	if g.Totals.Money.Total.State == domain.MoneyKnown {
		t.Errorf("%s: cost is Known (%d), want not Known for an empty group", name, g.Totals.Money.Total.Amount.Nanos())
	}
}

// rwRatioIs checks a ratio equals num/den by cross-multiplication, so a reduced
// form is accepted.
func rwRatioIs(t *testing.T, name string, got domain.Ratio, num, den int64) {
	t.Helper()
	if got.Denominator <= 0 {
		t.Errorf("%s: ratio %d/%d is undefined, want %d/%d", name, got.Numerator, got.Denominator, num, den)
		return
	}
	if got.Numerator*den != num*got.Denominator {
		t.Errorf("%s: ratio = %d/%d, want %d/%d", name, got.Numerator, got.Denominator, num, den)
	}
}
