package analysis_test

// Tests for the harness of each tool row (RunReport.Tools).
//
// Rules under test:
//   - A tool row lists the distinct non-empty harnesses of the actors that
//     called the tool, sorted, using the same actor attribution as the
//     wall-time split: the actor's first usage harness, else the run's only
//     harness, else unknown.
//   - A tool called by actors of different harnesses lists each harness once.
//   - A tool whose actors have no resolvable harness lists none.
//   - A tool counted as user-interaction for a harness by the wall-time split
//     lists that harness in its row.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// toolHarnessRun prices the spec and returns the run report.
func toolHarnessRun(t *testing.T, spec wtSpec) domain.RunReport {
	t.Helper()
	return ctxPrice(t, wtAggregate(spec), domain.DefaultAnalysisSettings())
}

func assertToolHarnesses(t *testing.T, tools []domain.ToolFigures, name string, want ...domain.Harness) {
	t.Helper()
	got := requireTool(t, tools, name).Harnesses
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tool %q harnesses = %v, want %v", name, got, want)
	}
}

func TestToolHarness_SingleHarnessRun_EveryRowResolvesToThatHarness(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("o1", "Read", 1, 2)}
	spec.agents = []wtAgent{{id: "coder#1", start: 10, end: 50, tools: []domain.ToolCall{wtTool("a1", "Edit", 11, 12)}}}

	run := toolHarnessRun(t, spec)

	assertToolHarnesses(t, run.Tools.Run, "Read", domain.HarnessClaudeCode)
	assertToolHarnesses(t, run.Tools.Run, "Edit", domain.HarnessClaudeCode)
	assertToolHarnesses(t, run.Tools.Orchestrator, "Read", domain.HarnessClaudeCode)
	assertToolHarnesses(t, run.Tools.ByAgentType[0].Tools, "Edit", domain.HarnessClaudeCode)
}

func TestToolHarness_ActorUsageHarnessWinsOverTheRunsOnlyHarness(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.agents = []wtAgent{{id: "coder#1", start: 10, end: 50, harness: domain.HarnessGHCPCLI,
		tools: []domain.ToolCall{wtTool("a1", "create", 11, 12)}}}

	run := toolHarnessRun(t, spec)

	assertToolHarnesses(t, run.Tools.Run, "create", domain.HarnessGHCPCLI)
}

func TestToolHarness_MixedRun_ToolCalledUnderBothHarnessesListsBothSorted(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.harnesses = []domain.Harness{domain.HarnessClaudeCode, domain.HarnessGHCPCLI}
	spec.orchHarness = domain.HarnessClaudeCode
	spec.orchTools = []domain.ToolCall{wtTool("o1", "Read", 1, 2), wtTool("o2", "Grep", 3, 4)}
	spec.agents = []wtAgent{{id: "coder#1", start: 10, end: 50, harness: domain.HarnessGHCPCLI,
		tools: []domain.ToolCall{wtTool("a1", "Read", 11, 12), wtTool("a2", "create", 13, 14)}}}

	run := toolHarnessRun(t, spec)

	assertToolHarnesses(t, run.Tools.Run, "Read", domain.HarnessClaudeCode, domain.HarnessGHCPCLI)
	assertToolHarnesses(t, run.Tools.Run, "Grep", domain.HarnessClaudeCode)
	assertToolHarnesses(t, run.Tools.Run, "create", domain.HarnessGHCPCLI)
}

func TestToolHarness_SeveralActorsOfOneHarnessCallingTheSameToolListTheHarnessOnce(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchHarness = domain.HarnessClaudeCode
	spec.orchTools = []domain.ToolCall{wtTool("o1", "Read", 1, 2)}
	spec.agents = []wtAgent{
		{id: "coder#1", start: 10, end: 50, harness: domain.HarnessClaudeCode, tools: []domain.ToolCall{wtTool("a1", "Read", 11, 12)}},
		{id: "tester#1", start: 55, end: 90, harness: domain.HarnessClaudeCode, tools: []domain.ToolCall{wtTool("a2", "Read", 56, 57)}},
	}

	run := toolHarnessRun(t, spec)

	assertToolHarnesses(t, run.Tools.Run, "Read", domain.HarnessClaudeCode)
}

func TestToolHarness_MixedRun_SectionRowsListOnlyTheirOwnActorsHarness(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.harnesses = []domain.Harness{domain.HarnessClaudeCode, domain.HarnessGHCPCLI}
	spec.orchHarness = domain.HarnessClaudeCode
	spec.orchTools = []domain.ToolCall{wtTool("o1", "Read", 1, 2)}
	spec.agents = []wtAgent{{id: "coder#1", start: 10, end: 50, harness: domain.HarnessGHCPCLI,
		tools: []domain.ToolCall{wtTool("a1", "Read", 11, 12)}}}

	run := toolHarnessRun(t, spec)

	assertToolHarnesses(t, run.Tools.Orchestrator, "Read", domain.HarnessClaudeCode)
	assertToolHarnesses(t, run.Tools.ByAgentType[0].Tools, "Read", domain.HarnessGHCPCLI)
}

func TestToolHarness_MixedRunActorWithoutUsageHarness_IsUnknownAndListsNone(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.harnesses = []domain.Harness{domain.HarnessClaudeCode, domain.HarnessGHCPCLI}
	spec.agents = []wtAgent{{id: "coder#1", start: 10, end: 50, tools: []domain.ToolCall{wtTool("a1", "Mystery", 11, 12)}}}

	run := toolHarnessRun(t, spec)

	assertToolHarnesses(t, run.Tools.Run, "Mystery")
	if len(run.WallTime.CountedTools) != 0 {
		t.Errorf("counted tools = %v, want none for an actor of unknown harness", run.WallTime.CountedTools)
	}
}

func TestToolHarness_UnnamedToolRowStillCarriesTheHarness(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.orchTools = []domain.ToolCall{wtTool("o1", "", 1, 2)}

	run := toolHarnessRun(t, spec)

	assertToolHarnesses(t, run.Tools.Run, "", domain.HarnessClaudeCode)
}

func TestToolHarness_MatchesTheHarnessTheWallTimeSplitCountsTheToolUnder(t *testing.T) {
	spec := newTestWallSpec(0, 100)
	spec.harnesses = []domain.Harness{domain.HarnessClaudeCode, domain.HarnessGHCPCLI}
	spec.orchHarness = domain.HarnessClaudeCode
	spec.orchTools = []domain.ToolCall{wtTool("o1", "AskUserQuestion", 1, 5)}
	spec.agents = []wtAgent{{id: "coder#1", start: 10, end: 50, harness: domain.HarnessGHCPCLI,
		tools: []domain.ToolCall{wtTool("a1", "ask_user", 11, 15)}}}

	run := toolHarnessRun(t, spec)

	if len(run.WallTime.CountedTools) != 2 {
		t.Fatalf("counted tools = %v, want AskUserQuestion and ask_user", run.WallTime.CountedTools)
	}
	for _, counted := range run.WallTime.CountedTools {
		assertToolHarnesses(t, run.Tools.Run, counted.Tool, counted.Harness)
	}
}
