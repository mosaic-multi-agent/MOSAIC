package analysis_test

// Tests for naming tool calls that have an end but no start.
//
// Rules under test:
//   - An end-only call takes its tool name from the end's tool_name; with no
//     name on the end it stays unnamed.
//   - A paired call keeps the name of its start, even when the end names
//     another tool.
//   - The tools view counts an end-only call under the end's name, so it joins
//     the calls of that tool; a nameless end-only call stays in the unnamed group.
//   - No finding is added for an end-only call.

import (
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

func rmToolEndNamed(callID, name string, status domain.ToolCallStatus, sec int) domain.Event {
	ev := rmToolEnd(callID, status, rmAt(sec))
	ev.ToolCallEnd.ToolName = name
	return ev
}

func toolFiguresOrNone(tools []domain.ToolFigures, name string) (domain.ToolFigures, bool) {
	for _, f := range tools {
		if f.Tool == name {
			return f, true
		}
	}
	return domain.ToolFigures{}, false
}

func TestRunModel_EndWithoutStart_TakesTheToolNameOfTheEnd(t *testing.T) {
	in := rmOrchestratorInput(rmToolEndNamed("c9", "runSubagent", domain.ToolCallError, 7))

	calls := rmActivity(t, in).OrchestratorToolCalls

	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	c := calls[0]
	if c.Pairing != domain.ToolCallEndOnly || c.ToolName != "runSubagent" {
		t.Errorf("call = %+v, want end-only named runSubagent", c)
	}
}

func TestRunModel_Subagent_EndWithoutStart_TakesTheToolNameOfTheEnd(t *testing.T) {
	in := analysis.Input{Streams: []analysis.Stream{
		{Ref: agentStream(testRunRef, "a#1"), Events: []domain.Event{
			rmInvStart("a#1", "coder", rmAt(0)),
			rmToolEndNamed("c1", "Edit", domain.ToolCallSuccess, 3),
		}},
	}}

	calls := rmInvocation(t, rmActivity(t, in), "a#1").ToolCalls

	if len(calls) != 1 || calls[0].Pairing != domain.ToolCallEndOnly || calls[0].ToolName != "Edit" {
		t.Errorf("calls = %+v, want one end-only call named Edit", calls)
	}
}

func TestRunModel_PairedCall_KeepsTheStartsToolName(t *testing.T) {
	in := rmOrchestratorInput(
		rmToolStart("c1", "Read", rmAt(1)),
		rmToolEndNamed("c1", "Other", domain.ToolCallSuccess, 3))

	calls := rmActivity(t, in).OrchestratorToolCalls

	if len(calls) != 1 || calls[0].Pairing != domain.ToolCallPaired || calls[0].ToolName != "Read" {
		t.Errorf("calls = %+v, want one paired call named Read", calls)
	}
}

func TestToolsView_EndOnlyCall_CountsUnderTheEndsToolName(t *testing.T) {
	in := rmOrchestratorInput(
		rmToolStart("c1", "Read", rmAt(1)),
		rmToolEnd("c1", domain.ToolCallSuccess, rmAt(2)),
		rmToolEndNamed("c2", "Read", domain.ToolCallError, 5))

	run := requireRunReport(t, analysis.Analyze(in, domain.NewPricingTable(nil), domain.DefaultAnalysisSettings(), testSource(), nil), testRunID)

	read, ok := toolFiguresOrNone(run.Tools.Run, "Read")
	if !ok {
		t.Fatalf("no Read among %v", toolNames(run.Tools.Run))
	}
	if read.Calls != 2 || read.Errors != 1 || read.Unpaired != 1 {
		t.Errorf("Read = %d calls, %d errors, %d unpaired, want 2, 1, 1", read.Calls, read.Errors, read.Unpaired)
	}
	if _, unnamed := toolFiguresOrNone(run.Tools.Run, ""); unnamed {
		t.Error("the end-only call must not be counted under the unnamed tool")
	}
}

func TestToolsView_EndOnlyCallWithoutAName_StaysInTheUnnamedGroup(t *testing.T) {
	in := rmOrchestratorInput(rmToolEnd("c9", domain.ToolCallSuccess, rmAt(5)))

	run := requireRunReport(t, analysis.Analyze(in, domain.NewPricingTable(nil), domain.DefaultAnalysisSettings(), testSource(), nil), testRunID)

	unnamed, ok := toolFiguresOrNone(run.Tools.Run, "")
	if !ok || unnamed.Calls != 1 || unnamed.Unpaired != 1 {
		t.Errorf("unnamed = %+v (found %v), want 1 call, 1 unpaired", unnamed, ok)
	}
}

func TestToolsView_EndOnlyCallWithAName_AddsNoFinding(t *testing.T) {
	in := rmOrchestratorInput(rmToolEndNamed("c2", "Read", domain.ToolCallError, 5))
	without := rmOrchestratorInput(rmToolEnd("c2", domain.ToolCallError, rmAt(5)))
	table, settings := domain.NewPricingTable(nil), domain.DefaultAnalysisSettings()

	named := analysis.Analyze(in, table, settings, testSource(), nil)
	plain := analysis.Analyze(without, table, settings, testSource(), nil)

	if len(named.Quality.Findings) != len(plain.Quality.Findings) {
		t.Errorf("findings with a named end = %d, without a name = %d, want equal",
			len(named.Quality.Findings), len(plain.Quality.Findings))
	}
}
