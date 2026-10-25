package cli_test

// Tests for the run-view breakdowns in the report JSON (EncodeReport) and for
// the unchanged total JSON (EncodeRunTotal).
//
// Rules under test:
//   - Every run object (named and unattributable) carries overview,
//     by_agent_type, by_status and tools; lists are arrays, never null.
//   - Ratios are {numerator, denominator} with denominator 0 meaning undefined;
//     durations are integer milliseconds or null; times are RFC3339 strings or
//     null; enums are lower-snake strings.
//   - Incompleteness is visible on the figures: unpriced models, missing usage,
//     open invocations, unpaired tool calls and partial money statistics.
//   - An empty agent type or tool name is encoded as "", never dropped.
//   - Tool figures hold counts, errors, unpaired counts and durations only.
//   - The total JSON keeps its field set whatever the report's views contain.

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

var vwStart = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

func vwKnown(nanos int64) domain.MoneyValue { return domain.KnownMoney(domain.Money(nanos)) }

func vwTotals(nanos int64, complete bool) domain.Totals {
	return domain.Totals{
		Tokens: domain.TokenUsage{Input: domain.Tokens(100)},
		Money:  domain.CategoryMoney{Total: vwKnown(nanos), Complete: complete},
	}
}

// vwRun builds a run with every view populated, including each incompleteness case.
func vwRun() domain.RunReport {
	closed := domain.Invocation{
		Actor:      domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"},
		State:      domain.InvocationClosed,
		StatusCode: "SUCCESS",
		Model:      "m-a",
		StartedAt:  vwStart,
		EndedAt:    vwStart.Add(90 * time.Second),
		EndEvents:  1,
		ToolCalls:  make([]domain.ToolCall, 3),
	}
	open := domain.Invocation{
		Actor: domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "x#2"},
		State: domain.InvocationOpen,
	}
	subagents := domain.GroupFigures{
		Invocations:    2,
		Totals:         vwTotals(300, false),
		UnpricedModels: []domain.PricingKey{{Model: "m-x"}},
		MissingUsage:   1,
		Open:           1,
	}
	return domain.RunReport{
		Run:          domain.NamedRun(cliTestRunID),
		Totals:       vwTotals(500, false),
		Orchestrator: domain.ActorReport{Actor: domain.Orchestrator(), Totals: vwTotals(200, true)},
		Overview: domain.RunOverview{
			Totals:            vwTotals(500, false),
			Orchestrator:      vwTotals(200, true),
			Subagents:         subagents,
			OrchestratorShare: domain.Ratio{Numerator: 200, Denominator: 500},
			SubagentShare:     domain.Ratio{Numerator: 300, Denominator: 500},
			ByModel: []domain.ModelFigures{
				{Model: "m-a", Priced: true, Requests: 3, Totals: vwTotals(500, true)},
				{Model: "", Priced: false, Requests: 1, Totals: vwTotals(0, false)},
			},
			CacheHitRatio: domain.Ratio{Numerator: 40, Denominator: 0},
		},
		ByAgentType: []domain.AgentTypeGroup{
			{
				AgentType: "coder",
				Figures:   domain.GroupFigures{Invocations: 1, Totals: vwTotals(300, true)},
				PerInvocation: domain.MoneyStats{
					Basis: 1, Min: vwKnown(300), Max: vwKnown(300), Average: vwKnown(300), Complete: true,
				},
				Dispatches: []domain.Dispatch{{
					Invocation: closed, Models: []domain.ModelID{"m-a"}, Totals: vwTotals(300, true),
				}},
			},
			{
				AgentType: "",
				Figures:   domain.GroupFigures{Invocations: 1, MissingUsage: 1, Open: 1},
				PerInvocation: domain.MoneyStats{
					Min: domain.NoMoneyData(), Max: domain.NoMoneyData(), Average: domain.NoMoneyData(),
				},
				Dispatches: []domain.Dispatch{{
					Invocation: open, MissingUsage: true, UnpricedModels: []domain.PricingKey{{Model: "m-x"}},
				}},
			},
		},
		ByStatus: domain.StatusBreakdown{
			Run: []domain.StatusGroup{
				{Kind: domain.StatusReported, Code: "SUCCESS", Figures: domain.GroupFigures{Invocations: 1}},
				{Kind: domain.StatusNone, Figures: domain.GroupFigures{Invocations: 2}},
				{Kind: domain.StatusOpen, Figures: domain.GroupFigures{Invocations: 3, Open: 3}},
			},
			ByAgentType: []domain.AgentTypeStatus{{
				AgentType: "",
				Groups:    []domain.StatusGroup{{Kind: domain.StatusOpen, Figures: domain.GroupFigures{Invocations: 1, Open: 1}}},
			}},
		},
		Tools: domain.ToolsBreakdown{
			Run: []domain.ToolFigures{
				{Tool: "Read", Calls: 3, Errors: 1, Unpaired: 1, Durations: domain.DurationStats{
					Count: 2, Total: 3500 * time.Millisecond, Min: time.Second, Max: 2500 * time.Millisecond, Average: 1750 * time.Millisecond,
				}},
				{Tool: "", Calls: 1, Unpaired: 1},
			},
			Orchestrator: []domain.ToolFigures{{Tool: "Grep", Calls: 1}},
			ByAgentType: []domain.AgentTypeTools{
				{AgentType: "", Tools: []domain.ToolFigures{{Tool: "", Calls: 1, Unpaired: 1}}},
			},
		},
	}
}

func vwReport(runs ...domain.RunReport) domain.Report {
	return domain.Report{Runs: runs, Currency: domain.Currency}
}

func vwList(t *testing.T, m map[string]any, key string) []any {
	t.Helper()
	list, ok := m[key].([]any)
	if !ok {
		t.Fatalf("key %q is %T (%v), want an array", key, m[key], m[key])
	}
	return list
}

func vwObj(t *testing.T, v any) map[string]any {
	t.Helper()
	o, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("value %v is %T, want an object", v, v)
	}
	return o
}

func vwAssertRatio(t *testing.T, name string, m map[string]any, key string, num, den float64) {
	t.Helper()
	r := vwObj(t, m[key])
	if r["numerator"] != num || r["denominator"] != den {
		t.Errorf("%s.%s = %v, want numerator %v denominator %v", name, key, r, num, den)
	}
}

func TestEncodeReport_RunCarriesTheFourBreakdowns(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(vwRun())))

	for _, key := range []string{"overview", "by_agent_type", "by_status", "tools"} {
		if _, ok := run[key]; !ok {
			t.Errorf("run object has no %q key; keys = %v", key, sortedKeys(run))
		}
	}
}

func TestEncodeReport_Overview_SharesRatiosAndModels(t *testing.T) {
	ov := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwRun()))), "overview")

	vwAssertRatio(t, "overview", ov, "orchestrator_share", 200, 500)
	vwAssertRatio(t, "overview", ov, "subagent_share", 300, 500)
	vwAssertRatio(t, "overview", ov, "cache_hit_ratio", 40, 0)
	for _, key := range []string{"totals", "orchestrator", "subagents", "by_model"} {
		if _, ok := ov[key]; !ok {
			t.Errorf("overview has no %q key; keys = %v", key, sortedKeys(ov))
		}
	}
	models := vwList(t, ov, "by_model")
	if len(models) != 2 {
		t.Fatalf("by_model has %d entries, want 2", len(models))
	}
	first, second := vwObj(t, models[0]), vwObj(t, models[1])
	if first["model"] != "m-a" || first["priced"] != true || first["requests"] != float64(3) {
		t.Errorf("first model = %v, want priced m-a with 3 requests", first)
	}
	if v, present := second["model"]; !present || v != "" || second["priced"] != false {
		t.Errorf("second model = %v, want an unpriced entry with an empty model id kept", second)
	}
}

func TestEncodeReport_Overview_GroupFiguresCarryIncompleteness(t *testing.T) {
	ov := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwRun()))), "overview")

	sub := objectAt(t, ov, "subagents")

	if sub["invocations"] != float64(2) || sub["missing_usage"] != float64(1) || sub["open"] != float64(1) {
		t.Errorf("subagents = %v, want 2 invocations, 1 missing usage, 1 open", sub)
	}
	unpriced := vwList(t, sub, "unpriced_models")
	if len(unpriced) != 1 || unpriced[0] != "m-x" {
		t.Errorf("subagents.unpriced_models = %v, want [m-x]", unpriced)
	}
	money := objectAt(t, objectAt(t, sub, "totals"), "money")
	if money["complete"] != false {
		t.Errorf("subagents totals money.complete = %v, want false", money["complete"])
	}
}

func TestEncodeReport_AssumedTierTokens_AppearOnViewTotalsOnlyWhenPositive(t *testing.T) {
	run := vwRun()
	run.Overview.Subagents.Totals.AssumedTierCacheWrite = 200

	ov := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(run))), "overview")

	assertAssumedCount(t, "overview.subagents.totals", objectAt(t, objectAt(t, ov, "subagents"), "totals"), 200)
	assertNoAssumedKey(t, "overview.totals", objectAt(t, ov, "totals"))
}

func TestEncodeReport_ByAgentType_GroupsStatsAndDispatches(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(vwRun())))

	groups := vwList(t, run, "by_agent_type")

	if len(groups) != 2 {
		t.Fatalf("by_agent_type has %d groups, want 2", len(groups))
	}
	coder := vwObj(t, groups[0])
	stats := objectAt(t, coder, "per_invocation")
	if coder["agent_type"] != "coder" || stats["basis"] != float64(1) || stats["complete"] != true {
		t.Errorf("coder group = %v, want agent_type coder with complete stats over 1 invocation", coder)
	}
	for _, key := range []string{"min", "max", "average"} {
		if objectAt(t, stats, key)["state"] != "known" {
			t.Errorf("per_invocation.%s = %v, want a known amount", key, stats[key])
		}
	}
	d := vwObj(t, vwList(t, coder, "dispatches")[0])
	if d["actor"] != "coder#1" || d["agent_type"] != "coder" || d["state"] != "closed" || d["status_code"] != "SUCCESS" {
		t.Errorf("dispatch = %v, want closed coder#1 with status SUCCESS", d)
	}
	if d["duration_ms"] != float64(90_000) || d["tool_calls"] != float64(3) || d["end_events"] != float64(1) || d["model"] != "m-a" {
		t.Errorf("dispatch = %v, want duration_ms 90000, 3 tool calls, 1 end event, model m-a", d)
	}
	started, _ := d["started_at"].(string)
	if got, err := time.Parse(time.RFC3339Nano, started); err != nil || !got.Equal(vwStart) {
		t.Errorf("started_at = %v, want %s as RFC3339", d["started_at"], vwStart.Format(time.RFC3339Nano))
	}
}

func TestEncodeReport_ByAgentType_UnknownTypeAndOpenDispatchAreExplicit(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(vwRun())))

	unknown := vwObj(t, vwList(t, run, "by_agent_type")[1])
	d := vwObj(t, vwList(t, unknown, "dispatches")[0])

	if v, present := unknown["agent_type"]; !present || v != "" {
		t.Errorf("unknown group agent_type = %v (present %v), want an empty string kept", v, present)
	}
	stats := objectAt(t, unknown, "per_invocation")
	if stats["basis"] != float64(0) || stats["complete"] != false || objectAt(t, stats, "min")["state"] != "no_data" {
		t.Errorf("unknown group stats = %v, want basis 0, incomplete, no-data amounts", stats)
	}
	if d["state"] != "open" || d["missing_usage"] != true {
		t.Errorf("dispatch = %v, want open with missing usage", d)
	}
	for _, key := range []string{"started_at", "ended_at", "duration_ms"} {
		if v, present := d[key]; !present || v != nil {
			t.Errorf("dispatch.%s = %v (present %v), want an explicit null", key, v, present)
		}
	}
	if unpriced := vwList(t, d, "unpriced_models"); len(unpriced) != 1 || unpriced[0] != "m-x" {
		t.Errorf("dispatch.unpriced_models = %v, want [m-x]", unpriced)
	}
}

func TestEncodeReport_ByStatus_KindsAreSeparateAndCodeOnlyWhenReported(t *testing.T) {
	status := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwRun()))), "by_status")

	groups := vwList(t, status, "run")

	if len(groups) != 3 {
		t.Fatalf("by_status.run has %d groups, want 3", len(groups))
	}
	reported, none, open := vwObj(t, groups[0]), vwObj(t, groups[1]), vwObj(t, groups[2])
	if reported["kind"] != "reported" || reported["code"] != "SUCCESS" {
		t.Errorf("first group = %v, want reported SUCCESS", reported)
	}
	if none["kind"] != "none" || open["kind"] != "open" {
		t.Errorf("groups = %v / %v, want kinds none and open as separate groups", none, open)
	}
	for _, g := range []map[string]any{none, open} {
		if _, present := g["code"]; present {
			t.Errorf("group %v carries a code although no status was reported", g)
		}
	}
	if objectAt(t, open, "figures")["open"] != float64(3) {
		t.Errorf("open group figures = %v, want open 3", open["figures"])
	}
	perType := vwObj(t, vwList(t, status, "by_agent_type")[0])
	if v, present := perType["agent_type"]; !present || v != "" || len(vwList(t, perType, "groups")) != 1 {
		t.Errorf("by_agent_type[0] = %v, want the unknown type with one group", perType)
	}
}

func TestEncodeReport_Tools_CountsErrorsUnpairedAndDurationsInMilliseconds(t *testing.T) {
	tools := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwRun()))), "tools")

	runTools := vwList(t, tools, "run")

	read := vwObj(t, runTools[0])
	if read["tool"] != "Read" || read["calls"] != float64(3) || read["errors"] != float64(1) || read["unpaired"] != float64(1) {
		t.Errorf("Read = %v, want 3 calls, 1 error, 1 unpaired", read)
	}
	d := objectAt(t, read, "durations")
	if d["count"] != float64(2) || d["total_ms"] != float64(3500) || d["min_ms"] != float64(1000) ||
		d["max_ms"] != float64(2500) || d["average_ms"] != float64(1750) {
		t.Errorf("Read durations = %v, want count 2, total 3500, min 1000, max 2500, average 1750 ms", d)
	}
}

func TestEncodeReport_Tools_UnpairedCallHasNullDurationsAndNoOtherAttribution(t *testing.T) {
	tools := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwRun()))), "tools")

	unnamed := vwObj(t, vwList(t, tools, "run")[1])

	if v, present := unnamed["tool"]; !present || v != "" || unnamed["unpaired"] != float64(1) {
		t.Errorf("unnamed tool = %v, want an empty tool name kept and 1 unpaired call", unnamed)
	}
	d := objectAt(t, unnamed, "durations")
	for _, key := range []string{"total_ms", "min_ms", "max_ms", "average_ms"} {
		if v, present := d[key]; !present || v != nil {
			t.Errorf("durations.%s = %v (present %v), want an explicit null", key, v, present)
		}
	}
	assertKeySet(t, "tool figures", sortedKeys(unnamed), []string{"tool", "calls", "errors", "unpaired", "durations"})
	if len(vwList(t, tools, "orchestrator")) != 1 {
		t.Errorf("tools.orchestrator = %v, want the one orchestrator tool", tools["orchestrator"])
	}
	perType := vwObj(t, vwList(t, tools, "by_agent_type")[0])
	if v, present := perType["agent_type"]; !present || v != "" || len(vwList(t, perType, "tools")) != 1 {
		t.Errorf("tools.by_agent_type[0] = %v, want the unknown type with one tool", perType)
	}
}

func TestEncodeReport_RunWithoutViewData_EncodesEmptyArraysNotNull(t *testing.T) {
	run := firstRun(t, decodeReportJSON(t, vwReport(domain.RunReport{Run: domain.NamedRun(cliTestRunID)})))

	if got := vwList(t, run, "by_agent_type"); len(got) != 0 {
		t.Errorf("by_agent_type = %v, want an empty array", got)
	}
	status, tools, ov := objectAt(t, run, "by_status"), objectAt(t, run, "tools"), objectAt(t, run, "overview")
	for name, list := range map[string][]any{
		"by_status.run":               vwList(t, status, "run"),
		"by_status.by_agent_type":     vwList(t, status, "by_agent_type"),
		"tools.run":                   vwList(t, tools, "run"),
		"tools.orchestrator":          vwList(t, tools, "orchestrator"),
		"tools.by_agent_type":         vwList(t, tools, "by_agent_type"),
		"overview.by_model":           vwList(t, ov, "by_model"),
		"overview.subagents.unpriced": vwList(t, objectAt(t, ov, "subagents"), "unpriced_models"),
	} {
		if len(list) != 0 {
			t.Errorf("%s = %v, want an empty array", name, list)
		}
	}
}

func TestEncodeReport_UnattributableRunCarriesTheBreakdownsToo(t *testing.T) {
	unattributable := vwRun()
	unattributable.Run = domain.UnattributableRun()
	report := vwReport()
	report.Unattributable = &unattributable

	run := objectAt(t, decodeReportJSON(t, report), "unattributable")

	for _, key := range []string{"overview", "by_agent_type", "by_status", "tools"} {
		if _, ok := run[key]; !ok {
			t.Errorf("unattributable object has no %q key; keys = %v", key, sortedKeys(run))
		}
	}
}

func TestEncodeReport_SchemaVersionStaysOne(t *testing.T) {
	m := decodeReportJSON(t, vwReport(vwRun()))

	if m["schema_version"] != "1" {
		t.Errorf("schema_version = %v, want \"1\"", m["schema_version"])
	}
	if _, ok := firstRun(t, m)["overview"]; !ok {
		t.Error("run has no overview, so the report does not carry the breakdowns")
	}
}

func TestEncodeRunTotal_ReportWithPopulatedViews_KeepsExactFieldSet(t *testing.T) {
	run := vwRun()
	run.Totals = vwTotals(500, true)
	total, ok := app.RunTotalFor(vwReport(run), cliTestRunID)
	if !ok {
		t.Fatalf("RunTotalFor: run %s not found", cliTestRunID)
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
