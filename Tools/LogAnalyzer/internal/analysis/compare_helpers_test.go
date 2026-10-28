package analysis_test

// Builders and assertions for the run comparison tests. Runs are priced from
// aggregates (input-only dispatches of N tokens cost N*3000 nanodollars and
// have a peak context of N tokens), or built by hand to look like runs loaded
// from a digest.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

const (
	cmpRunA = "20261028T100000Z-aaaa"
	cmpRunB = "20261028T110000Z-bbbb"
)

// cmpOK is a closed SUCCESS dispatch of the given type with input-only tokens.
func cmpOK(id domain.AgentInstanceID, typ string, tokens int64) rwDispatch {
	return rwClosed(id, typ, "SUCCESS", 0, 10, tokens)
}

// cmpCompare prices runs a and b together and compares them.
func cmpCompare(t *testing.T, a, b rwRun) domain.RunComparison {
	t.Helper()
	a.id, b.id = cmpRunA, cmpRunB
	report := rwPriceReport(t, rwAggregate(a, b))
	return analysis.CompareRuns(requireRunReport(t, report, cmpRunA), requireRunReport(t, report, cmpRunB))
}

func cmpRow(t *testing.T, c domain.RunComparison, typ string) domain.ComparisonRow {
	t.Helper()
	for _, r := range c.Rows {
		if r.AgentType == typ {
			return r
		}
	}
	t.Fatalf("no comparison row for type %q among %d rows", typ, len(c.Rows))
	return domain.ComparisonRow{}
}

func cmpRowOrder(c domain.RunComparison) []string {
	order := make([]string, 0, len(c.Rows))
	for _, r := range c.Rows {
		order = append(order, r.AgentType)
	}
	return order
}

// cmpAssertQty checks a quantity present on both sides: values, delta and direction.
func cmpAssertQty(t *testing.T, name string, q domain.ComparedQuantity, a, b int64, dir domain.ChangeDirection) {
	t.Helper()
	if !q.HasA || !q.HasB {
		t.Errorf("%s: HasA=%v HasB=%v, want both present", name, q.HasA, q.HasB)
		return
	}
	if q.A != a || q.B != b || q.Delta != b-a {
		t.Errorf("%s: A=%d B=%d delta=%d, want A=%d B=%d delta=%d", name, q.A, q.B, q.Delta, a, b, b-a)
	}
	if q.Direction != dir {
		t.Errorf("%s: direction = %v, want %v", name, q.Direction, dir)
	}
	if a == 0 {
		if q.HasPercent {
			t.Errorf("%s: has a percent change from a zero base", name)
		}
		return
	}
	if !q.HasPercent {
		t.Errorf("%s: no percent change, want %d/%d", name, b-a, a)
		return
	}
	rwRatioIs(t, name+" percent", q.Percent, b-a, a)
}

// cmpAssertCost checks a cost known on both sides: values, delta, percent and direction.
func cmpAssertCost(t *testing.T, name string, m domain.ComparedMoney, a, b int64, dir domain.ChangeDirection) {
	t.Helper()
	assertMoneyKnown(t, name+" A", m.A, a)
	assertMoneyKnown(t, name+" B", m.B, b)
	assertMoneyKnown(t, name+" delta", m.Delta, b-a)
	if m.Direction != dir {
		t.Errorf("%s: direction = %v, want %v", name, m.Direction, dir)
	}
	if !m.Complete {
		t.Errorf("%s: marked incomplete for fully priced runs", name)
	}
	if a == 0 {
		if m.HasPercent {
			t.Errorf("%s: has a percent change from a zero base", name)
		}
		return
	}
	if !m.HasPercent {
		t.Errorf("%s: no percent change, want %d/%d", name, b-a, a)
		return
	}
	rwRatioIs(t, name+" percent", m.Percent, b-a, a)
}

// cmpAssertNoChangeFigure checks a quantity with no usable change: no delta
// direction and no percent.
func cmpAssertNoChangeFigure(t *testing.T, name string, q domain.ComparedQuantity) {
	t.Helper()
	if q.HasPercent {
		t.Errorf("%s: has a percent change, want none", name)
	}
	if q.Direction != domain.ChangeUnknown {
		t.Errorf("%s: direction = %v, want unknown", name, q.Direction)
	}
}

// cmpDigestRun builds a run the way a digest load leaves it: totals and the
// per-type groups with their dispatches, and nothing else. peak < 0 means the
// dispatch has no peak context.
func cmpDigestRun(id string, typ string, tokens, nanos, peak int64) domain.RunReport {
	totals := domain.Totals{
		Tokens: domain.TokenUsage{Input: domain.Tokens(tokens)},
		Money: domain.CategoryMoney{
			Total: domain.KnownMoney(domain.Money(nanos)), Complete: true,
		},
	}
	dispatch := domain.Dispatch{Totals: totals}
	stats := domain.PeakContextStats{}
	if peak >= 0 {
		fig := domain.ContextFigure{Tokens: domain.Tokens(peak)}
		dispatch.PeakContext = fig
		stats = domain.PeakContextStats{Basis: 1, Average: domain.Tokens(peak), Max: fig}
	}
	return domain.RunReport{
		Run:      domain.NamedRun(id),
		Totals:   totals,
		Overview: domain.RunOverview{Subagents: domain.GroupFigures{Invocations: 1, Totals: totals}},
		ByAgentType: []domain.AgentTypeGroup{{
			AgentType:   typ,
			Figures:     domain.GroupFigures{Invocations: 1, Totals: totals},
			Dispatches:  []domain.Dispatch{dispatch},
			PeakContext: stats,
		}},
	}
}
