package cli_test

// Tests for reasoning_tokens in the report JSON (EncodeReport) and its absence
// from the total JSON (EncodeRunTotal).
//
// Rules under test:
//   - Every report token object carries "reasoning": a number when known
//     (including 0), null when not reported. The key is always present.
//   - Reasoning does not change any money figure.
//   - The total JSON is byte-identical with and without reasoning data and has
//     no reasoning key.

import (
	"bytes"
	"encoding/json"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

func withReasoning(t domain.Totals, reasoning domain.TokenCount) domain.Totals {
	t.Tokens.Output = domain.Tokens(140)
	t.Tokens.Reasoning = reasoning
	return t
}

// reasoningRun is vwRun with the same reasoning count on every token breakdown.
func reasoningRun(reasoning domain.TokenCount) domain.RunReport {
	run := vwRun()
	run.Totals = withReasoning(run.Totals, reasoning)
	run.Orchestrator.Totals = withReasoning(run.Orchestrator.Totals, reasoning)
	run.Agents = []domain.ActorReport{{
		Actor:  domain.AgentInstance("coder#1"),
		Totals: withReasoning(vwTotals(300, true), reasoning),
	}}
	run.Overview.Totals = withReasoning(run.Overview.Totals, reasoning)
	run.Overview.Orchestrator = withReasoning(run.Overview.Orchestrator, reasoning)
	run.Overview.Subagents.Totals = withReasoning(run.Overview.Subagents.Totals, reasoning)
	run.Overview.ByModel[0].Totals = withReasoning(run.Overview.ByModel[0].Totals, reasoning)
	run.ByAgentType[0].Figures.Totals = withReasoning(run.ByAgentType[0].Figures.Totals, reasoning)
	run.ByAgentType[0].Dispatches[0].Totals = withReasoning(run.ByAgentType[0].Dispatches[0].Totals, reasoning)
	return run
}

// totalsTokens returns the "tokens" object under m["totals"].
func totalsTokens(t *testing.T, where string, m map[string]any) map[string]any {
	t.Helper()
	totals, ok := m["totals"].(map[string]any)
	if !ok {
		t.Fatalf("%s has no totals object: %v", where, m)
	}
	tokens, ok := totals["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("%s.totals has no tokens object: %v", where, totals)
	}
	return tokens
}

func assertReasoning(t *testing.T, where string, tokens map[string]any, want any) {
	t.Helper()
	got, present := tokens["reasoning"]
	if !present {
		t.Errorf("%s.tokens has no reasoning key (want it present, %v)", where, want)
		return
	}
	if got != want {
		t.Errorf("%s.tokens.reasoning = %v, want %v", where, got, want)
	}
}

// tokenObjects collects every report token object that reasoningRun fills.
func tokenObjects(t *testing.T, report map[string]any) map[string]map[string]any {
	t.Helper()
	run := firstRun(t, report)
	ov := objectAt(t, run, "overview")
	agentType := vwObj(t, vwList(t, run, "by_agent_type")[0])
	dispatch := vwObj(t, vwList(t, agentType, "dispatches")[0])
	byModel := vwObj(t, vwList(t, ov, "by_model")[0])
	agent := vwObj(t, vwList(t, run, "agents")[0])
	return map[string]map[string]any{
		"all_runs":                     totalsTokens(t, "all_runs", map[string]any{"totals": report["all_runs"]}),
		"runs[0]":                      totalsTokens(t, "run", run),
		"runs[0].orchestrator":         totalsTokens(t, "orchestrator", objectAt(t, run, "orchestrator")),
		"runs[0].agents[0]":            totalsTokens(t, "agent", agent),
		"overview":                     totalsTokens(t, "overview", ov),
		"overview.orchestrator":        totalsTokens(t, "overview.orchestrator", map[string]any{"totals": ov["orchestrator"]}),
		"overview.subagents":           totalsTokens(t, "overview.subagents", objectAt(t, ov, "subagents")),
		"overview.by_model[0]":         totalsTokens(t, "by_model", byModel),
		"by_agent_type[0].figures":     totalsTokens(t, "figures", objectAt(t, agentType, "figures")),
		"by_agent_type[0].dispatch[0]": totalsTokens(t, "dispatch", dispatch),
	}
}

func TestEncodeReport_KnownReasoning_EveryTokenObjectCarriesTheCount(t *testing.T) {
	report := decodeReportJSON(t, reasoningReportJSONInput(domain.Tokens(40)))

	for where, tokens := range tokenObjects(t, report) {
		assertReasoning(t, where, tokens, float64(40))
	}
}

func TestEncodeReport_AbsentReasoning_EveryTokenObjectCarriesNull(t *testing.T) {
	report := decodeReportJSON(t, reasoningReportJSONInput(domain.AbsentTokens()))

	for where, tokens := range tokenObjects(t, report) {
		assertReasoning(t, where, tokens, nil)
	}
}

func TestEncodeReport_PresentZeroReasoning_EncodesZeroNotNull(t *testing.T) {
	report := decodeReportJSON(t, reasoningReportJSONInput(domain.Tokens(0)))

	for where, tokens := range tokenObjects(t, report) {
		assertReasoning(t, where, tokens, float64(0))
	}
}

func TestEncodeReport_AcrossRunsTotals_CarryReasoning(t *testing.T) {
	r := reasoningReportJSONInput(domain.Tokens(40))
	r.AcrossRuns = domain.AcrossRuns{Runs: []domain.RunSummary{{
		Run:    domain.NamedRun(cliTestRunID),
		Totals: withReasoning(vwTotals(500, true), domain.Tokens(40)),
	}}}

	report := decodeReportJSON(t, r)

	across := objectAt(t, report, "across_runs")
	summary := vwObj(t, vwList(t, across, "runs")[0])
	assertReasoning(t, "across_runs.runs[0]", totalsTokens(t, "summary", summary), float64(40))
}

func TestEncodeReport_Reasoning_DoesNotChangeMoney(t *testing.T) {
	with := decodeReportJSON(t, reasoningReportJSONInput(domain.Tokens(40)))
	without := decodeReportJSON(t, reasoningReportJSONInput(domain.AbsentTokens()))

	moneyOf := func(report map[string]any) []byte {
		run := firstRun(t, report)
		b, err := json.Marshal([]any{
			report["all_runs"].(map[string]any)["money"],
			run["totals"].(map[string]any)["money"],
			objectAt(t, run, "orchestrator")["totals"].(map[string]any)["money"],
		})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if a, b := moneyOf(with), moneyOf(without); !bytes.Equal(a, b) {
		t.Errorf("money differs with reasoning data:\nwith:    %s\nwithout: %s", a, b)
	}
	money := firstRun(t, with)["totals"].(map[string]any)["money"].(map[string]any)
	if _, has := money["reasoning"]; has {
		t.Error("money object carries a reasoning cell; reasoning must not be priced")
	}
}

func reasoningReportJSONInput(reasoning domain.TokenCount) domain.Report {
	r := vwReport(reasoningRun(reasoning))
	r.AllRuns = withReasoning(vwTotals(500, false), reasoning)
	return r
}

func encodeTotalBytes(t *testing.T, reasoning domain.TokenCount) []byte {
	t.Helper()
	totals := withReasoning(vwTotals(500, true), reasoning)
	report := domain.Report{Runs: []domain.RunReport{{Run: domain.NamedRun(cliTestRunID), Totals: totals}}}
	total, ok := app.RunTotalFor(report, cliTestRunID)
	if !ok {
		t.Fatalf("run %s not found", cliTestRunID)
	}
	var buf bytes.Buffer
	if err := cli.EncodeRunTotal(&buf, total); err != nil {
		t.Fatalf("EncodeRunTotal: %v", err)
	}
	return buf.Bytes()
}

func TestEncodeRunTotal_WithReasoning_ByteIdenticalToRunWithout(t *testing.T) {
	with := encodeTotalBytes(t, domain.Tokens(40))
	without := encodeTotalBytes(t, domain.AbsentTokens())

	if !bytes.Equal(with, without) {
		t.Errorf("total JSON changes with reasoning data:\nwith:    %s\nwithout: %s", with, without)
	}
}

func TestEncodeRunTotal_WithReasoning_TokensHaveExactlyTheFourBillableKeys(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(encodeTotalBytes(t, domain.Tokens(40)), &m); err != nil {
		t.Fatal(err)
	}

	tokens := objectAt(t, m, "tokens")

	assertKeySet(t, "tokens", sortedKeys(tokens), []string{"input", "cache_read", "cache_creation", "output"})
	if got, _ := tokens["output"].(float64); got != 140 {
		t.Errorf("tokens.output = %v, want 140 (output already includes reasoning)", tokens["output"])
	}
}
