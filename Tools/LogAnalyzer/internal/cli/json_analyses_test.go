package cli_test

// Tests for the further-analysis figures in the report JSON: outcome split,
// rework, cache and wall-time on the run object, plus peak context, rework and
// cache on agent-type groups and peak context, cache, repeat and likely-rework
// on dispatches.
//
// Rules under test:
//   - Rework objects label themselves: repeats include planned repeats and
//     likely rework is a heuristic.
//   - Absent token counts are JSON null; money uses the {"state": ...} shape;
//     a negative net benefit stays negative.
//   - Wall-time durations are integer milliseconds; span_ms is null without a
//     span and the four category durations are then 0; lists are arrays.
//   - Existing run fields keep their names.

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func jaGroup(invocations int, nanos int64) domain.GroupFigures {
	return domain.GroupFigures{Invocations: invocations, Totals: vwTotals(nanos, true)}
}

func jaCache() domain.CacheFigures {
	return domain.CacheFigures{
		HitRatio:          domain.Ratio{Numerator: 900, Denominator: 1000},
		WriteTokens:       domain.Tokens(4000),
		WriteCost:         vwKnown(25_000),
		NetBenefit:        vwKnown(-7_000),
		UnreadWriteTokens: domain.Tokens(1500),
		UnreadWriteCost:   vwKnown(9_000),
		Complete:          false,
	}
}

func jaEmptyCache() domain.CacheFigures {
	return domain.CacheFigures{
		HitRatio:        domain.Ratio{Numerator: 0, Denominator: 0},
		WriteCost:       domain.NoMoneyData(),
		NetBenefit:      domain.UnpricedMoney(),
		UnreadWriteCost: domain.NoMoneyData(),
	}
}

func jaRework() domain.ReworkFigures {
	return domain.ReworkFigures{Repeats: jaGroup(3, 600), LikelyRework: jaGroup(1, 200)}
}

func jaRun() domain.RunReport {
	run := vwRun()
	run.Overview.Outcomes = domain.OutcomeSplit{
		Success: jaGroup(4, 400), OtherStatus: jaGroup(2, 150), OpenOrNone: jaGroup(1, 50),
	}
	run.Rework = jaRework()
	run.Cache = jaCache()
	run.WallTime = domain.WallTimeSplit{
		Span:             domain.TimeInterval{Start: vwStart, End: vwStart.Add(100 * time.Second)},
		HasSpan:          true,
		Waiting:          10 * time.Second,
		Subagents:        50 * time.Second,
		OrchestratorOnly: 30 * time.Second,
		BetweenSessions:  10 * time.Second,
		CountedTools: []domain.CountedTool{
			{Tool: "AskUserQuestion", Harness: "claude-code", Calls: 2},
		},
		OpenWaits: []domain.OpenWait{
			{Actor: domain.Orchestrator(), At: vwStart.Add(time.Minute), Kind: domain.OpenWaitToolCall, Tool: "AskUserQuestion"},
			{Actor: domain.AgentInstance("coder#1"), Kind: domain.OpenWaitPermission},
			{Actor: domain.Orchestrator(), At: vwStart, Kind: domain.OpenWaitIdlePrompt},
		},
		MissingContactToolsHint: true,
		UnconfiguredTools:       []string{"mcp__x__ask"},
	}
	run.ByAgentType[0].PeakContext = domain.PeakContextStats{
		Basis: 2, Average: domain.Tokens(150_000), Max: jcFig(260_000, false),
	}
	run.ByAgentType[0].Cache = jaCache()
	run.ByAgentType[0].Rework = jaRework()
	run.ByAgentType[0].Dispatches[0].PeakContext = jcFig(120_000, true)
	run.ByAgentType[0].Dispatches[0].Cache = jaCache()
	run.ByAgentType[0].Dispatches[0].Repeat = true
	run.ByAgentType[0].Dispatches[0].LikelyRework = true
	return run
}

func jaRunObj(t *testing.T, run domain.RunReport) map[string]any {
	t.Helper()
	return firstRun(t, decodeReportJSON(t, vwReport(run)))
}

func jaAssertGroup(t *testing.T, name string, m map[string]any, key string, invocations float64) {
	t.Helper()
	g := objectAt(t, m, key)
	if g["invocations"] != invocations {
		t.Errorf("%s.%s.invocations = %v, want %v", name, key, g["invocations"], invocations)
	}
	if _, ok := g["totals"]; !ok {
		t.Errorf("%s.%s has no totals; keys = %v", name, key, sortedKeys(g))
	}
}

func jaAssertCache(t *testing.T, name string, c map[string]any) {
	t.Helper()
	vwAssertRatio(t, name, c, "hit_ratio", 900, 1000)
	if c["write_tokens"] != float64(4000) || c["unread_write_tokens"] != float64(1500) || c["complete"] != false {
		t.Errorf("%s = %v, want write 4000, unread 1500, complete false", name, c)
	}
	if objectAt(t, c, "write_cost")["state"] != "known" || objectAt(t, c, "unread_write_cost")["state"] != "known" {
		t.Errorf("%s costs = %v / %v, want known amounts", name, c["write_cost"], c["unread_write_cost"])
	}
	net := objectAt(t, c, "net_benefit")
	if amount, _ := net["amount"].(string); net["state"] != "known" || !strings.HasPrefix(amount, "-") {
		t.Errorf("%s.net_benefit = %v, want a known negative amount", name, net)
	}
}

func TestEncodeReport_Run_OutcomeSplitHasThreeGroups(t *testing.T) {
	split := objectAt(t, jaRunObj(t, jaRun()), "outcome_split")

	jaAssertGroup(t, "outcome_split", split, "success", 4)
	jaAssertGroup(t, "outcome_split", split, "other_status", 2)
	jaAssertGroup(t, "outcome_split", split, "open_or_no_status", 1)
}

func TestEncodeReport_Run_ReworkIsLabelledAsIncludingPlannedAndHeuristic(t *testing.T) {
	rework := objectAt(t, jaRunObj(t, jaRun()), "rework")

	jaAssertGroup(t, "rework", rework, "repeats", 3)
	jaAssertGroup(t, "rework", rework, "likely_rework", 1)
	if rework["repeats_include_planned"] != true || rework["likely_rework_heuristic"] != true {
		t.Errorf("rework = %v, want repeats_include_planned and likely_rework_heuristic true", rework)
	}
}

func TestEncodeReport_Run_CacheFiguresCarryRatioTokensAndMoney(t *testing.T) {
	jaAssertCache(t, "run cache", objectAt(t, jaRunObj(t, jaRun()), "cache"))
}

func TestEncodeReport_Cache_AbsentTokensAreNullAndMoneyKeepsItsState(t *testing.T) {
	run := jaRun()
	run.Cache = jaEmptyCache()

	c := objectAt(t, jaRunObj(t, run), "cache")

	for _, key := range []string{"write_tokens", "unread_write_tokens"} {
		if v, present := c[key]; !present || v != nil {
			t.Errorf("cache.%s = %v (present %v), want null", key, v, present)
		}
	}
	if objectAt(t, c, "net_benefit")["state"] != "unpriced" || objectAt(t, c, "write_cost")["state"] != "no_data" {
		t.Errorf("cache = %v, want net_benefit unpriced and write_cost no_data", c)
	}
	vwAssertRatio(t, "cache", c, "hit_ratio", 0, 0)
}

func TestEncodeReport_WallTime_CarriesSpanAndCategoryMilliseconds(t *testing.T) {
	wt := objectAt(t, jaRunObj(t, jaRun()), "wall_time")

	want := map[string]float64{
		"span_ms": 100_000, "waiting_ms": 10_000, "subagents_ms": 50_000,
		"orchestrator_only_ms": 30_000, "between_sessions_ms": 10_000,
	}
	for key, v := range want {
		if wt[key] != v {
			t.Errorf("wall_time.%s = %v, want %v", key, wt[key], v)
		}
	}
}

func TestEncodeReport_WallTime_WithoutSpanHasNullSpanAndZeroCategories(t *testing.T) {
	run := jaRun()
	run.WallTime = domain.WallTimeSplit{}

	wt := objectAt(t, jaRunObj(t, run), "wall_time")

	if v, present := wt["span_ms"]; !present || v != nil {
		t.Errorf("wall_time.span_ms = %v (present %v), want null", v, present)
	}
	for _, key := range []string{"waiting_ms", "subagents_ms", "orchestrator_only_ms", "between_sessions_ms"} {
		if wt[key] != float64(0) {
			t.Errorf("wall_time.%s = %v, want 0", key, wt[key])
		}
	}
	for _, key := range []string{"counted_tools", "open_waits", "unconfigured_tools"} {
		if list := vwList(t, wt, key); len(list) != 0 {
			t.Errorf("wall_time.%s = %v, want an empty array", key, list)
		}
	}
	if wt["missing_contact_tools_hint"] != false {
		t.Errorf("wall_time.missing_contact_tools_hint = %v, want false", wt["missing_contact_tools_hint"])
	}
}

func TestEncodeReport_WallTime_ListsCountedToolsAndHint(t *testing.T) {
	wt := objectAt(t, jaRunObj(t, jaRun()), "wall_time")

	tools := vwList(t, wt, "counted_tools")
	if len(tools) != 1 {
		t.Fatalf("counted_tools has %d entries, want 1", len(tools))
	}
	tool := vwObj(t, tools[0])
	if tool["tool"] != "AskUserQuestion" || tool["harness"] != "claude-code" || tool["calls"] != float64(2) {
		t.Errorf("counted tool = %v, want AskUserQuestion / claude-code / 2 calls", tool)
	}
	if wt["missing_contact_tools_hint"] != true {
		t.Errorf("missing_contact_tools_hint = %v, want true", wt["missing_contact_tools_hint"])
	}
	unconfigured := vwList(t, wt, "unconfigured_tools")
	if len(unconfigured) != 1 || unconfigured[0] != "mcp__x__ask" {
		t.Errorf("unconfigured_tools = %v, want [mcp__x__ask]", unconfigured)
	}
}

func TestEncodeReport_WallTime_OpenWaitsKeepKindActorAndOptionalTime(t *testing.T) {
	wt := objectAt(t, jaRunObj(t, jaRun()), "wall_time")

	waits := vwList(t, wt, "open_waits")

	if len(waits) != 3 {
		t.Fatalf("open_waits has %d entries, want 3", len(waits))
	}
	first := vwObj(t, waits[0])
	if first["kind"] != "tool_call" || first["actor"] != "orchestrator" || first["tool"] != "AskUserQuestion" {
		t.Errorf("first open wait = %v, want a tool_call wait of orchestrator on AskUserQuestion", first)
	}
	at, _ := first["at"].(string)
	if got, err := time.Parse(time.RFC3339Nano, at); err != nil || !got.Equal(vwStart.Add(time.Minute)) {
		t.Errorf("first open wait at = %v, want %s", first["at"], vwStart.Add(time.Minute).Format(time.RFC3339Nano))
	}
	second := vwObj(t, waits[1])
	if second["kind"] != "permission_notification" || second["actor"] != "coder#1" {
		t.Errorf("second open wait = %v, want a permission_notification wait of coder#1", second)
	}
	if v, present := second["at"]; !present || v != nil {
		t.Errorf("second open wait at = %v (present %v), want null for a zero time", v, present)
	}
	if third := vwObj(t, waits[2]); third["kind"] != "idle_prompt" {
		t.Errorf("third open wait = %v, want kind idle_prompt", third)
	}
}

func TestEncodeReport_ByAgentType_CarriesPeakContextReworkAndCache(t *testing.T) {
	group := vwObj(t, vwList(t, jaRunObj(t, jaRun()), "by_agent_type")[0])

	peak := objectAt(t, group, "peak_context")
	if peak["basis"] != float64(2) || peak["average"] != float64(150_000) {
		t.Errorf("peak_context = %v, want basis 2 and average 150000", peak)
	}
	jcAssertFig(t, "peak_context", peak, "max", 260_000, "above_optimal", false)
	jaAssertGroup(t, "group rework", objectAt(t, group, "rework"), "repeats", 3)
	jaAssertCache(t, "group cache", objectAt(t, group, "cache"))
}

func TestEncodeReport_ByAgentType_PeakContextWithoutBasisIsNull(t *testing.T) {
	run := jaRun()
	run.ByAgentType[1].PeakContext = domain.PeakContextStats{}

	group := vwObj(t, vwList(t, jaRunObj(t, run), "by_agent_type")[1])

	peak := objectAt(t, group, "peak_context")
	if peak["basis"] != float64(0) {
		t.Errorf("peak_context.basis = %v, want 0", peak["basis"])
	}
	for _, key := range []string{"average", "max"} {
		if v, present := peak[key]; !present || v != nil {
			t.Errorf("peak_context.%s = %v (present %v), want null", key, v, present)
		}
	}
}

func TestEncodeReport_Dispatch_CarriesPeakContextCacheAndRepeatFlags(t *testing.T) {
	group := vwObj(t, vwList(t, jaRunObj(t, jaRun()), "by_agent_type")[0])
	d := vwObj(t, vwList(t, group, "dispatches")[0])

	jcAssertFig(t, "dispatch", d, "peak_context", 120_000, "optimal", true)
	jaAssertCache(t, "dispatch cache", objectAt(t, d, "cache"))
	if d["repeat"] != true || d["likely_rework"] != true {
		t.Errorf("dispatch = %v, want repeat and likely_rework true", d)
	}
}

func TestEncodeReport_Dispatch_AbsentPeakContextIsNullAndFlagsDefaultFalse(t *testing.T) {
	group := vwObj(t, vwList(t, jaRunObj(t, jaRun()), "by_agent_type")[1])
	d := vwObj(t, vwList(t, group, "dispatches")[0])

	if v, present := d["peak_context"]; !present || v != nil {
		t.Errorf("dispatch.peak_context = %v (present %v), want null", v, present)
	}
	if d["repeat"] != false || d["likely_rework"] != false {
		t.Errorf("dispatch = %v, want repeat and likely_rework false", d)
	}
}

func TestEncodeReport_Run_ExistingFieldsKeepTheirNamesNextToTheNewOnes(t *testing.T) {
	run := jaRunObj(t, jaRun())

	for _, key := range []string{
		"run_id", "provisional", "totals", "orchestrator", "agents", "unpriced_models",
		"overview", "by_agent_type", "by_status", "tools", "timeline",
		"outcome_split", "rework", "cache", "wall_time",
	} {
		if _, ok := run[key]; !ok {
			t.Errorf("run object has no %q key; keys = %v", key, sortedKeys(run))
		}
	}
	for _, key := range []string{"totals", "orchestrator", "subagents", "by_model", "cache_hit_ratio"} {
		if _, ok := objectAt(t, run, "overview")[key]; !ok {
			t.Errorf("overview lost %q", key)
		}
	}
}
