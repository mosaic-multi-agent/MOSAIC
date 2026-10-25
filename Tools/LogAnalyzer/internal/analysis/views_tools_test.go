package analysis_test

// Tests for the tools view (RunReport.Tools) over the shared scenario in
// views_scenario_test.go.
//
// Rules under test:
//   - Exact figures only: call count, error count, unpaired count and durations
//     per tool, for the orchestrator, per agent type and for the whole run.
//   - Durations come only from paired start/end timestamps; an unpaired call is
//     counted and counted as unpaired but contributes no duration.
//   - Calls that overlap in time (parallel) each keep their own duration.
//   - Tool lists are sorted by name with the unnamed tool last.

import (
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func requireTool(t *testing.T, tools []domain.ToolFigures, name string) domain.ToolFigures {
	t.Helper()
	for _, f := range tools {
		if f.Tool == name {
			return f
		}
	}
	t.Fatalf("no tool %q among %+v", name, tools)
	return domain.ToolFigures{}
}

func toolNames(tools []domain.ToolFigures) []string {
	names := make([]string, len(tools))
	for i, f := range tools {
		names[i] = f.Tool
	}
	return names
}

func assertDurations(t *testing.T, name string, got domain.DurationStats, count int, total, min, max, avg time.Duration) {
	t.Helper()
	want := domain.DurationStats{Count: count, Total: total, Min: min, Max: max, Average: avg}
	if got != want {
		t.Errorf("%s durations = %+v, want %+v", name, got, want)
	}
}

func TestTools_Run_CountsErrorsAndDurationsAcrossOrchestratorAndSubagents(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	read := requireTool(t, run.Tools.Run, "Read")

	if read.Calls != 4 || read.Errors != 1 || read.Unpaired != 1 {
		t.Errorf("Read = %d calls, %d errors, %d unpaired, want 4, 1, 1", read.Calls, read.Errors, read.Unpaired)
	}
	// Paired Read calls: orchestrator 4s, p1 1s, p2 2s; the unpaired tester call has none.
	assertDurations(t, "Read", read.Durations, 3, 7*time.Second, time.Second, 4*time.Second, 2333333333*time.Nanosecond)
}

func TestTools_Run_ListIsSortedWithUnnamedToolLast(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	if got := toolNames(run.Tools.Run); !rmEqualStrings(got, []string{"Bash", "Grep", "Read", ""}) {
		t.Errorf("run tools = %q, want Bash, Grep, Read, unnamed last", got)
	}
}

func TestTools_UnpairedCalls_AreCountedWithoutDuration(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	grep := requireTool(t, run.Tools.Run, "Grep")
	unnamed := requireTool(t, run.Tools.Run, "")

	if grep.Calls != 1 || grep.Unpaired != 1 {
		t.Errorf("Grep = %d calls, %d unpaired, want 1 and 1", grep.Calls, grep.Unpaired)
	}
	assertDurations(t, "Grep", grep.Durations, 0, 0, 0, 0, 0)
	if unnamed.Calls != 2 || unnamed.Unpaired != 2 || unnamed.Durations.Count != 0 {
		t.Errorf("unnamed = %+v, want 2 unpaired end-only calls with no duration", unnamed)
	}
}

func TestTools_Orchestrator_ListsOnlyOrchestratorCalls(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	orch := run.Tools.Orchestrator

	if got := toolNames(orch); !rmEqualStrings(got, []string{"Grep", "Read", ""}) {
		t.Fatalf("orchestrator tools = %q, want Grep, Read, unnamed", got)
	}
	read := requireTool(t, orch, "Read")
	if read.Calls != 1 || read.Errors != 0 {
		t.Errorf("orchestrator Read = %+v, want 1 call and no errors", read)
	}
	assertDurations(t, "orchestrator Read", read.Durations, 1, 4*time.Second, 4*time.Second, 4*time.Second, 4*time.Second)
}

func TestTools_PerAgentType_ParallelCallsKeepTheirOwnDurations(t *testing.T) {
	run := viewsScenarioRun(t, viewsPricing(t))

	var coder, tester *domain.AgentTypeTools
	for i := range run.Tools.ByAgentType {
		switch run.Tools.ByAgentType[i].AgentType {
		case "coder":
			coder = &run.Tools.ByAgentType[i]
		case "tester":
			tester = &run.Tools.ByAgentType[i]
		}
	}
	if coder == nil || tester == nil {
		t.Fatalf("by-agent-type tools = %+v, want entries for coder and tester", run.Tools.ByAgentType)
	}

	if got := toolNames(coder.Tools); !rmEqualStrings(got, []string{"Bash", "Read", ""}) {
		t.Fatalf("coder tools = %q, want Bash, Read, unnamed", got)
	}
	read := requireTool(t, coder.Tools, "Read")
	if read.Calls != 2 || read.Errors != 1 || read.Unpaired != 0 {
		t.Errorf("coder Read = %+v, want 2 calls, 1 error, none unpaired", read)
	}
	// p1 ran 1s, p2 ran 2s while overlapping p1.
	assertDurations(t, "coder Read", read.Durations, 2, 3*time.Second, time.Second, 2*time.Second, 1500*time.Millisecond)
	assertDurations(t, "coder Bash", requireTool(t, coder.Tools, "Bash").Durations, 1, 3*time.Second, 3*time.Second, 3*time.Second, 3*time.Second)
	open := requireTool(t, tester.Tools, "Read")
	if open.Calls != 1 || open.Unpaired != 1 || open.Durations.Count != 0 {
		t.Errorf("tester Read = %+v, want 1 unpaired call without duration", open)
	}
}

func TestTools_PairedCallWithEndBeforeStart_HasNoDuration(t *testing.T) {
	in := oneAgentStream(testRunRef, "coder#1",
		rmInvStart("coder#1", "coder", rmAt(1)),
		rmToolStart("c1", "Read", rmAt(10)), rmToolEnd("c1", domain.ToolCallSuccess, rmAt(5)))

	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	read := requireTool(t, requireRunReport(t, report, testRunID).Tools.Run, "Read")
	if read.Calls != 1 || read.Unpaired != 0 || read.Durations.Count != 0 {
		t.Errorf("Read = %+v, want one paired call with no usable duration", read)
	}
}

func TestTools_RunWithoutToolCalls_HasNoToolFigures(t *testing.T) {
	in := oneAgentStream(testRunRef, "coder#1", rmInvStart("coder#1", "coder", rmAt(1)))

	report := analysis.Analyze(in, viewsPricing(t), testSource(), nil)

	tools := requireRunReport(t, report, testRunID).Tools
	if got := requireRunReport(t, report, testRunID).ByAgentType; len(got) != 1 {
		t.Fatalf("by-agent-type = %+v, want the one coder group (views not computed)", got)
	}
	if len(tools.Run) != 0 || len(tools.Orchestrator) != 0 {
		t.Errorf("tools = %+v, want no figures without tool calls", tools)
	}
}
