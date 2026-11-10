package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// CompareRuns compares run a with run b. Pure. It reads only figures a run
// carries after a digest load (totals, per-type groups and their dispatches), so
// a figure a run lacks stays absent on that side.
func CompareRuns(a, b domain.RunReport) domain.RunComparison {
	groupsA, groupsB := groupsByType(a), groupsByType(b)
	var rows []domain.ComparisonRow
	for _, typ := range unionTypes(groupsA, groupsB) {
		ga, inA := groupsA[typ]
		gb, inB := groupsB[typ]
		row := compareSides(sideOfGroup(ga, inA), sideOfGroup(gb, inB))
		row.AgentType = typ
		rows = append(rows, row)
	}
	sortComparisonRows(rows)
	return domain.RunComparison{
		A: a.Run, B: b.Run,
		ProvisionalA: a.Provisional, ProvisionalB: b.Provisional,
		Rows:   rows,
		Totals: compareSides(sideOfRun(a), sideOfRun(b)),
	}
}

// compareSide is one run's figures for one row.
type compareSide struct {
	present     bool
	invocations int
	tokens      domain.TokenCount
	money       domain.MoneyValue
	complete    bool
	avgPeak     domain.TokenCount
}

func sideOfGroup(g domain.AgentTypeGroup, present bool) compareSide {
	if !present {
		return compareSide{}
	}
	t := g.Figures.Totals
	return compareSide{
		present:     true,
		invocations: g.Figures.Invocations,
		tokens:      sumAllTokens(t.Tokens),
		money:       t.Money.Total,
		complete:    t.Money.Complete,
		avgPeak:     g.PeakContext.Average,
	}
}

func sideOfRun(r domain.RunReport) compareSide {
	var all []domain.Dispatch
	for _, g := range r.ByAgentType {
		all = append(all, g.Dispatches...)
	}
	return compareSide{
		present:     true,
		invocations: r.Overview.Subagents.Invocations,
		tokens:      sumAllTokens(r.Totals.Tokens),
		money:       r.Totals.Money.Total,
		complete:    r.Totals.Money.Complete,
		avgPeak:     peakContextStats(all).Average,
	}
}

// sumAllTokens adds input, cache read, cache write and output; absent when none is present.
func sumAllTokens(u domain.TokenUsage) domain.TokenCount {
	return domain.SumTokens(u.Input, u.CacheRead, u.CacheCreation, u.Output)
}

func groupsByType(r domain.RunReport) map[string]domain.AgentTypeGroup {
	out := make(map[string]domain.AgentTypeGroup, len(r.ByAgentType))
	for _, g := range r.ByAgentType {
		out[g.AgentType] = g
	}
	return out
}

func unionTypes(a, b map[string]domain.AgentTypeGroup) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var types []string
	for _, m := range []map[string]domain.AgentTypeGroup{a, b} {
		for typ := range m {
			if !seen[typ] {
				seen[typ] = true
				types = append(types, typ)
			}
		}
	}
	sort.Strings(types)
	return types
}

// compareSides builds one comparison row from both runs' figures.
func compareSides(a, b compareSide) domain.ComparisonRow {
	row := domain.ComparisonRow{
		Invocations:    compareQuantity(domain.Tokens(int64(a.invocations)), a.present, domain.Tokens(int64(b.invocations)), b.present),
		Tokens:         compareQuantity(a.tokens, a.present, b.tokens, b.present),
		Cost:           compareMoney(a, b),
		AvgPeakContext: compareQuantity(a.avgPeak, a.present, b.avgPeak, b.present),
	}
	switch {
	case a.present && !b.present:
		row.Presence = domain.OnlyInA
	case !a.present && b.present:
		row.Presence = domain.OnlyInB
	}
	return row
}

// compareQuantity compares two counts; a side is absent when its row is absent or
// its count is absent.
func compareQuantity(a domain.TokenCount, aRow bool, b domain.TokenCount, bRow bool) domain.ComparedQuantity {
	var q domain.ComparedQuantity
	q.A, q.HasA = a.Value()
	q.HasA = q.HasA && aRow
	q.B, q.HasB = b.Value()
	q.HasB = q.HasB && bRow
	if !q.HasA {
		q.A = 0
	}
	if !q.HasB {
		q.B = 0
	}
	if q.HasA && q.HasB {
		q.Delta = q.B - q.A
		q.Direction = direction(q.Delta)
		if q.A != 0 {
			q.Percent = domain.Ratio{Numerator: q.Delta, Denominator: q.A}
			q.HasPercent = true
		}
	}
	return q
}

func compareMoney(a, b compareSide) domain.ComparedMoney {
	m := domain.ComparedMoney{A: domain.NoMoneyData(), B: domain.NoMoneyData(), Delta: domain.NoMoneyData()}
	if a.present {
		m.A = a.money
	}
	if b.present {
		m.B = b.money
	}
	m.Complete = (!a.present || a.complete) && (!b.present || b.complete)
	if m.A.State != domain.MoneyKnown || m.B.State != domain.MoneyKnown {
		return m
	}
	delta := m.B.Amount.Nanos() - m.A.Amount.Nanos()
	m.Delta = domain.KnownMoney(domain.Money(delta))
	if !m.Complete {
		// A partial total is a lower bound: its change has no percent or direction.
		return m
	}
	m.Direction = direction(delta)
	if base := m.A.Amount.Nanos(); base != 0 {
		m.Percent = domain.Ratio{Numerator: delta, Denominator: base}
		m.HasPercent = true
	}
	return m
}

func direction(delta int64) domain.ChangeDirection {
	switch {
	case delta > 0:
		return domain.ChangeIncrease
	case delta < 0:
		return domain.ChangeDecrease
	}
	return domain.ChangeUnchanged
}

// sortComparisonRows orders rows by absolute cost change, largest first; rows
// without a known delta follow, by the cost of the side that has one, largest
// first. Ties order by type name.
func sortComparisonRows(rows []domain.ComparisonRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		aKnown := a.Cost.Delta.State == domain.MoneyKnown
		bKnown := b.Cost.Delta.State == domain.MoneyKnown
		if aKnown != bKnown {
			return aKnown
		}
		ka, kb := rowSortKey(a, aKnown), rowSortKey(b, bKnown)
		if ka != kb {
			return ka > kb
		}
		return a.AgentType < b.AgentType
	})
}

func rowSortKey(r domain.ComparisonRow, known bool) int64 {
	if known {
		d := r.Cost.Delta.Amount.Nanos()
		if d < 0 {
			d = -d
		}
		return d
	}
	var best int64
	for _, v := range []domain.MoneyValue{r.Cost.A, r.Cost.B} {
		if v.State == domain.MoneyKnown && v.Amount.Nanos() > best {
			best = v.Amount.Nanos()
		}
	}
	return best
}
