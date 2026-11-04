package cli_test

// Tests for the tool context growth fields of the report JSON.
//
// Rules under test:
//   - Every tool figures object (run, orchestrator, by_agent_type) carries a
//     context_growth object with samples, average_tokens, min_tokens,
//     max_tokens, negative and not_measurable; it is always present.
//   - average, min and max are explicit nulls when there are no samples.
//   - The change is additive: the previous fields of a tool keep their names
//     and values, and a growth-only tool is a new array element with calls 0.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

var growthKeys = []string{"samples", "average_tokens", "min_tokens", "max_tokens", "negative", "not_measurable"}

func vwWithGrowth() domain.RunReport {
	run := vwRun()
	run.Tools.Run[0].Growth = domain.ToolGrowth{
		Samples: 3, Average: domain.Tokens(1200), Min: domain.Tokens(800), Max: domain.Tokens(1900),
		Negative: 1, NotMeasurable: 2,
	}
	run.Tools.Orchestrator[0].Growth = domain.ToolGrowth{
		Samples: 1, Average: domain.Tokens(0), Min: domain.Tokens(0), Max: domain.Tokens(0),
	}
	run.Tools.ByAgentType[0].Tools[0].Growth = domain.ToolGrowth{Negative: 4}
	return run
}

func growthOf(t *testing.T, tool any) map[string]any {
	t.Helper()
	return objectAt(t, vwObj(t, tool), "context_growth")
}

func TestEncodeReport_ToolGrowth_CarriesTheFiguresOfARunLevelTool(t *testing.T) {
	tools := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwWithGrowth()))), "tools")

	g := growthOf(t, vwList(t, tools, "run")[0])

	want := map[string]float64{
		"samples": 3, "average_tokens": 1200, "min_tokens": 800, "max_tokens": 1900, "negative": 1, "not_measurable": 2,
	}
	for key, v := range want {
		if g[key] != v {
			t.Errorf("context_growth.%s = %v, want %v", key, g[key], v)
		}
	}
	assertKeySet(t, "context_growth", sortedKeys(g), append([]string(nil), growthKeys...))
}

func TestEncodeReport_ToolGrowth_ZeroSampleIsANumberNotNull(t *testing.T) {
	tools := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwWithGrowth()))), "tools")

	g := growthOf(t, vwList(t, tools, "orchestrator")[0])

	for _, key := range []string{"average_tokens", "min_tokens", "max_tokens"} {
		if g[key] != float64(0) {
			t.Errorf("context_growth.%s = %v, want the number 0 for a zero sample", key, g[key])
		}
	}
	if g["samples"] != float64(1) {
		t.Errorf("context_growth.samples = %v, want 1", g["samples"])
	}
}

func TestEncodeReport_ToolGrowth_WithoutSamples_HasExplicitNullFiguresAndKeepsCounts(t *testing.T) {
	tools := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwWithGrowth()))), "tools")
	perType := vwObj(t, vwList(t, tools, "by_agent_type")[0])

	g := growthOf(t, vwList(t, perType, "tools")[0])

	for _, key := range []string{"average_tokens", "min_tokens", "max_tokens"} {
		if v, present := g[key]; !present || v != nil {
			t.Errorf("context_growth.%s = %v (present %v), want an explicit null", key, v, present)
		}
	}
	if g["samples"] != float64(0) || g["negative"] != float64(4) || g["not_measurable"] != float64(0) {
		t.Errorf("context_growth = %v, want 0 samples and the 4 negative calls", g)
	}
}

func TestEncodeReport_ToolGrowth_IsPresentOnEveryToolEvenWithoutAnyGrowth(t *testing.T) {
	tools := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwRun()))), "tools")

	var all []any
	all = append(all, vwList(t, tools, "run")...)
	all = append(all, vwList(t, tools, "orchestrator")...)
	for _, byType := range vwList(t, tools, "by_agent_type") {
		all = append(all, vwList(t, vwObj(t, byType), "tools")...)
	}
	if len(all) == 0 {
		t.Fatal("no tool objects in the report")
	}
	for _, tool := range all {
		g := growthOf(t, tool)
		assertKeySet(t, "context_growth", sortedKeys(g), append([]string(nil), growthKeys...))
		if g["samples"] != float64(0) || g["average_tokens"] != nil || g["min_tokens"] != nil || g["max_tokens"] != nil {
			t.Errorf("context_growth = %v, want no samples and null figures", g)
		}
	}
}

func TestEncodeReport_ToolGrowth_LeavesTheExistingToolFieldsUnchanged(t *testing.T) {
	withGrowth := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwWithGrowth()))), "tools")
	without := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(vwRun()))), "tools")

	for _, level := range []string{"run", "orchestrator"} {
		a, b := vwList(t, withGrowth, level), vwList(t, without, level)
		if len(a) != len(b) {
			t.Fatalf("%s tools = %d, want %d: growth adds no element to a row that exists", level, len(a), len(b))
		}
		for i := range a {
			ta, tb := vwObj(t, a[i]), vwObj(t, b[i])
			for _, key := range []string{"tool", "calls", "errors", "unpaired"} {
				if ta[key] != tb[key] {
					t.Errorf("%s[%d].%s = %v, want %v", level, i, key, ta[key], tb[key])
				}
			}
			da, db := objectAt(t, ta, "durations"), objectAt(t, tb, "durations")
			assertKeySet(t, "durations", sortedKeys(da), sortedKeys(db))
			for key := range db {
				if da[key] != db[key] {
					t.Errorf("%s[%d].durations.%s = %v, want %v", level, i, key, da[key], db[key])
				}
			}
		}
	}
}

func TestEncodeReport_ToolGrowth_GrowthOnlyToolIsANewElementWithZeroCalls(t *testing.T) {
	run := vwRun()
	run.Tools.Run = append(run.Tools.Run, domain.ToolFigures{
		Tool:   "glob",
		Growth: domain.ToolGrowth{Samples: 1, Average: domain.Tokens(10), Min: domain.Tokens(10), Max: domain.Tokens(10)},
	})

	tools := objectAt(t, firstRun(t, decodeReportJSON(t, vwReport(run))), "tools")
	list := vwList(t, tools, "run")

	row := vwObj(t, list[len(list)-1])
	if row["tool"] != "glob" || row["calls"] != float64(0) {
		t.Fatalf("last run tool = %v, want glob with 0 calls", row)
	}
	if g := growthOf(t, row); g["samples"] != float64(1) || g["average_tokens"] != float64(10) {
		t.Errorf("glob context_growth = %v, want 1 sample averaging 10", g)
	}
}

func TestEncodeReport_ToolGrowth_ReportKeepsItsTopLevelAndRunFieldSets(t *testing.T) {
	plain := decodeReportJSON(t, vwReport(vwRun()))
	grown := decodeReportJSON(t, vwReport(vwWithGrowth()))

	assertKeySet(t, "report", sortedKeys(grown), sortedKeys(plain))
	assertKeySet(t, "run", sortedKeys(firstRun(t, grown)), sortedKeys(firstRun(t, plain)))
	assertKeySet(t, "tools", sortedKeys(objectAt(t, firstRun(t, grown), "tools")), []string{"run", "orchestrator", "by_agent_type"})
}
