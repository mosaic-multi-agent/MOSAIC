package cli_test

// Tests for the across-runs section of the report JSON (EncodeReport) and for
// the total JSON staying unchanged when the report carries across-runs figures.
//
// Rules under test:
//   - The top level carries an across_runs object with a runs list and a
//     by_agent_type list; both are arrays, never null.
//   - Each run item carries run id, provisional flag, totals, unpriced models and
//     the subagent figures; partial money, unpriced models and assumed-tier
//     tokens stay visible.
//   - Each agent-type item carries run counts, figures, the per-run ratios
//     ({numerator, denominator}, denominator 0 = undefined) and per-run money
//     statistics; an empty agent type is kept as "".
//   - The report schema_version stays "1"; the total JSON keeps its field set.

import (
	"bytes"
	"encoding/json"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

const arRunOne, arRunTwo = "20260101T000000Z-aaaa", "20260102T000000Z-bbbb"

func arKnown(nanos int64) domain.MoneyValue { return domain.KnownMoney(domain.Money(nanos)) }

// arAcrossRunsReport has two named runs (the second provisional and partial)
// and two agent types (a "coder" and an unknown one).
func arAcrossRunsReport() domain.Report {
	complete := domain.Totals{
		Tokens: domain.TokenUsage{Input: domain.Tokens(1000)},
		Money:  domain.CategoryMoney{Total: arKnown(5_000_000), Complete: true},
	}
	partial := domain.Totals{
		Tokens:                domain.TokenUsage{Input: domain.Tokens(300)},
		Money:                 domain.CategoryMoney{Total: arKnown(825_000), Complete: false},
		AssumedTierCacheWrite: 100,
	}
	group := domain.GroupFigures{Invocations: 3, Totals: complete}
	partialGroup := domain.GroupFigures{
		Invocations: 2, Totals: partial, UnpricedModels: []domain.PricingKey{{Model: "m-unpriced"}}, MissingUsage: 1, Open: 1,
	}
	report := vwReport(
		domain.RunReport{Run: domain.NamedRun(arRunOne), Totals: complete},
		domain.RunReport{Run: domain.NamedRun(arRunTwo), Provisional: true, Totals: partial},
	)
	report.AcrossRuns = domain.AcrossRuns{
		Runs: []domain.RunSummary{
			{Run: domain.NamedRun(arRunOne), Totals: complete, Subagents: group},
			{Run: domain.NamedRun(arRunTwo), Provisional: true, Totals: partial,
				UnpricedModels: []domain.PricingKey{{Model: "m-unpriced"}}, Subagents: partialGroup},
		},
		ByAgentType: []domain.AgentTypeAcrossRuns{
			{
				AgentType: "coder", Runs: 2, ProvisionalRuns: 1, Figures: partialGroup,
				PerRunInvocations: domain.Ratio{Numerator: 4, Denominator: 2},
				PerRunTokens:      domain.Ratio{Numerator: 7630, Denominator: 2},
				PerRunMoney: domain.MoneyStats{
					Basis: 1, Min: arKnown(3_500_000), Max: arKnown(3_500_000), Average: arKnown(3_500_000), Complete: false,
				},
			},
			{
				AgentType: "", Runs: 1, Figures: group,
				PerRunMoney: domain.MoneyStats{Min: domain.NoMoneyData(), Max: domain.NoMoneyData(), Average: domain.NoMoneyData()},
			},
		},
	}
	return report
}

func arAcrossJSON(t *testing.T, report domain.Report) map[string]any {
	t.Helper()
	return objectAt(t, decodeReportJSON(t, report), "across_runs")
}

func TestEncodeReport_CarriesTheAcrossRunsSection(t *testing.T) {
	across := arAcrossJSON(t, arAcrossRunsReport())

	assertKeySet(t, "across_runs", sortedKeys(across), []string{"runs", "by_agent_type"})
}

func TestEncodeReport_AcrossRuns_RunItemsCarryTotalsAndSubagentFigures(t *testing.T) {
	across := arAcrossJSON(t, arAcrossRunsReport())

	runs := vwList(t, across, "runs")

	if len(runs) != 2 {
		t.Fatalf("across_runs.runs has %d items, want 2", len(runs))
	}
	first, second := vwObj(t, runs[0]), vwObj(t, runs[1])
	assertKeySet(t, "run item", sortedKeys(first), []string{"run_id", "provisional", "totals", "unpriced_models", "subagents"})
	if first["run_id"] != arRunOne || first["provisional"] != false {
		t.Errorf("first run item = %v, want run %s, not provisional", first, arRunOne)
	}
	total := objectAt(t, objectAt(t, objectAt(t, first, "totals"), "money"), "total")
	if total["state"] != "known" || total["amount"] != "0.005" {
		t.Errorf("first run total = %v, want known 0.005", total)
	}
	if sub := objectAt(t, first, "subagents"); sub["invocations"] != float64(3) {
		t.Errorf("first run subagents = %v, want 3 invocations", sub)
	}
	if second["run_id"] != arRunTwo || second["provisional"] != true {
		t.Errorf("second run item = %v, want provisional run %s", second, arRunTwo)
	}
}

func TestEncodeReport_AcrossRuns_PartialRunKeepsItsLabels(t *testing.T) {
	across := arAcrossJSON(t, arAcrossRunsReport())

	second := vwObj(t, vwList(t, across, "runs")[1])

	totals := objectAt(t, second, "totals")
	if objectAt(t, totals, "money")["complete"] != false {
		t.Errorf("partial run money = %v, want complete false", totals["money"])
	}
	assertAssumedCount(t, "across_runs.runs[1].totals", totals, 100)
	if unpriced := vwList(t, second, "unpriced_models"); len(unpriced) != 1 || unpriced[0] != "m-unpriced" {
		t.Errorf("unpriced_models = %v, want [m-unpriced]", unpriced)
	}
	sub := objectAt(t, second, "subagents")
	if sub["open"] != float64(1) || sub["missing_usage"] != float64(1) {
		t.Errorf("subagents = %v, want 1 open and 1 missing usage", sub)
	}
}

func TestEncodeReport_AcrossRuns_AgentTypeItemCarriesRunCountsRatiosAndMoneyStats(t *testing.T) {
	across := arAcrossJSON(t, arAcrossRunsReport())

	types := vwList(t, across, "by_agent_type")

	if len(types) != 2 {
		t.Fatalf("by_agent_type has %d items, want 2", len(types))
	}
	coder := vwObj(t, types[0])
	assertKeySet(t, "agent type item", sortedKeys(coder), []string{
		"agent_type", "runs", "provisional_runs", "figures", "per_run_invocations", "per_run_tokens", "per_run_money",
	})
	if coder["agent_type"] != "coder" || coder["runs"] != float64(2) || coder["provisional_runs"] != float64(1) {
		t.Errorf("coder item = %v, want type coder over 2 runs, 1 provisional", coder)
	}
	vwAssertRatio(t, "coder", coder, "per_run_invocations", 4, 2)
	vwAssertRatio(t, "coder", coder, "per_run_tokens", 7630, 2)
	stats := objectAt(t, coder, "per_run_money")
	if stats["basis"] != float64(1) || stats["complete"] != false || objectAt(t, stats, "average")["amount"] != "0.0035" {
		t.Errorf("per_run_money = %v, want basis 1, partial, average 0.0035", stats)
	}
	if fig := objectAt(t, coder, "figures"); fig["invocations"] != float64(2) {
		t.Errorf("figures = %v, want 2 invocations", fig)
	}
}

func TestEncodeReport_AcrossRuns_UnknownTypeIsKeptAndUndefinedRatioHasZeroDenominator(t *testing.T) {
	across := arAcrossJSON(t, arAcrossRunsReport())

	unknown := vwObj(t, vwList(t, across, "by_agent_type")[1])

	if v, present := unknown["agent_type"]; !present || v != "" {
		t.Errorf("unknown type item agent_type = %v (present %v), want an empty string kept", v, present)
	}
	vwAssertRatio(t, "unknown type", unknown, "per_run_invocations", 0, 0)
	stats := objectAt(t, unknown, "per_run_money")
	if stats["basis"] != float64(0) || objectAt(t, stats, "min")["state"] != "no_data" {
		t.Errorf("per_run_money = %v, want basis 0 with no_data min", stats)
	}
}

func TestEncodeReport_AcrossRuns_WithoutNamedRuns_EncodesEmptyArraysNotNull(t *testing.T) {
	across := arAcrossJSON(t, vwReport())

	if got := vwList(t, across, "runs"); len(got) != 0 {
		t.Errorf("across_runs.runs = %v, want an empty array", got)
	}
	if got := vwList(t, across, "by_agent_type"); len(got) != 0 {
		t.Errorf("across_runs.by_agent_type = %v, want an empty array", got)
	}
}

func TestEncodeReport_AcrossRuns_SchemaVersionStaysOne(t *testing.T) {
	m := decodeReportJSON(t, arAcrossRunsReport())

	if m["schema_version"] != "1" {
		t.Errorf("schema_version = %v, want \"1\"", m["schema_version"])
	}
	if _, ok := m["across_runs"]; !ok {
		t.Errorf("report has no across_runs key; keys = %v", sortedKeys(m))
	}
}

func TestEncodeRunTotal_ReportWithAcrossRuns_KeepsExactFieldSet(t *testing.T) {
	report := arAcrossRunsReport()
	total, ok := app.RunTotalFor(report, arRunOne)
	if !ok {
		t.Fatalf("RunTotalFor: run %s not found", arRunOne)
	}

	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, total); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	assertKeySet(t, "top level", sortedKeys(m), []string{
		"schema_version", "run_id", "provisional", "currency", "tokens", "money", "complete",
	})
}
