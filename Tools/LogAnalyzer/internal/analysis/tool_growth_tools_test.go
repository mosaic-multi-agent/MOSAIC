package analysis_test

// Tests for the growth figures of the Tools breakdown.
//
// Rules under test:
//   - Per tool: sample count, average (half-up), minimum and maximum of the
//     measured growth of the singular calls that requested only that tool,
//     at run, orchestrator and agent-type level.
//   - A tool without samples has absent average, minimum and maximum
//     ("not provided"), distinct from a zero sample.
//   - Left-out calls (negative, not measurable) are counted per tool and
//     level; a tool seen only through growth gets a row with no tool calls.
//   - Logs without requested tools show no growth anywhere.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// tgToolsScenario: the orchestrator and one coder agent each request
// read_file and glob on singular calls; only read_file has tool call entries.
//
//	orchestrator read_file 400, glob 150
//	coder        read_file 201, glob 79
func tgToolsScenario(t *testing.T) domain.RunReport {
	t.Helper()
	none := domain.NewRequestedTools(nil)
	return tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 100, tgOne("read_file")),
			tgCall("o2", 20, 1500, 50, tgOne("glob")),
			tgCall("o3", 30, 1700, 50, none),
		},
		orchTools: []domain.ToolCall{ctxTool("ot1", "read_file", 11, 12)},
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType,
			usage: []domain.InvocationUsage{
				tgCall("a1", 10, 2000, 100, tgOne("read_file")),
				tgCall("a2", 20, 2301, 20, tgOne("glob")),
				tgCall("a3", 30, 2400, 10, none),
			},
			tools: []domain.ToolCall{ctxTool("at1", "read_file", 11, 12)},
		}},
	}})
}

func tgAgentTypeTools(t *testing.T, run domain.RunReport, agentType string) []domain.ToolFigures {
	t.Helper()
	for _, a := range run.Tools.ByAgentType {
		if a.AgentType == agentType {
			return a.Tools
		}
	}
	t.Fatalf("no agent type %q among %+v", agentType, run.Tools.ByAgentType)
	return nil
}

func tgAssertGrowthFigures(t *testing.T, name string, g domain.ToolGrowth, samples int, avg, min, max int64) {
	t.Helper()
	if g.Samples != samples {
		t.Errorf("%s samples = %d, want %d", name, g.Samples, samples)
	}
	ctxAssertTokens(t, name+" average", g.Average, avg)
	ctxAssertTokens(t, name+" min", g.Min, min)
	ctxAssertTokens(t, name+" max", g.Max, max)
}

func TestToolGrowthFigures_Run_CombinesOrchestratorAndAgents(t *testing.T) {
	run := tgToolsScenario(t)

	read := requireTool(t, run.Tools.Run, "read_file")
	glob := requireTool(t, run.Tools.Run, "glob")

	// (400 + 201) / 2 = 300.5 rounds half-up to 301.
	tgAssertGrowthFigures(t, "read_file", read.Growth, 2, 301, 201, 400)
	// (150 + 79) / 2 = 114.5 rounds half-up to 115.
	tgAssertGrowthFigures(t, "glob", glob.Growth, 2, 115, 79, 150)
}

func TestToolGrowthFigures_Orchestrator_CountsOnlyTheOrchestrator(t *testing.T) {
	run := tgToolsScenario(t)

	read := requireTool(t, run.Tools.Orchestrator, "read_file")
	glob := requireTool(t, run.Tools.Orchestrator, "glob")

	tgAssertGrowthFigures(t, "read_file", read.Growth, 1, 400, 400, 400)
	tgAssertGrowthFigures(t, "glob", glob.Growth, 1, 150, 150, 150)
}

func TestToolGrowthFigures_AgentType_CountsOnlyThatTypesInvocations(t *testing.T) {
	run := tgToolsScenario(t)

	tools := tgAgentTypeTools(t, run, tgAgentType)

	tgAssertGrowthFigures(t, "read_file", requireTool(t, tools, "read_file").Growth, 1, 201, 201, 201)
	tgAssertGrowthFigures(t, "glob", requireTool(t, tools, "glob").Growth, 1, 79, 79, 79)
}

func TestToolGrowthFigures_AgentTypes_AreKeptApart(t *testing.T) {
	none := domain.NewRequestedTools(nil)
	other := domain.AgentInstanceID("tester#1")
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{agents: []ctxAgent{
		{id: ctxAgentID, typ: "coder", usage: []domain.InvocationUsage{
			tgCall("a1", 10, 2000, 100, tgOne("read_file")), tgCall("a2", 20, 2300, 10, none),
		}, tools: []domain.ToolCall{ctxTool("at1", "read_file", 11, 12)}},
		{id: other, typ: "tester", usage: []domain.InvocationUsage{
			tgCall("b1", 10, 5000, 100, tgOne("read_file")), tgCall("b2", 20, 6000, 10, none),
		}, tools: []domain.ToolCall{ctxTool("bt1", "read_file", 11, 12)}},
	}}})

	coder := requireTool(t, tgAgentTypeTools(t, run, "coder"), "read_file")
	tester := requireTool(t, tgAgentTypeTools(t, run, "tester"), "read_file")

	tgAssertGrowthFigures(t, "coder", coder.Growth, 1, 200, 200, 200)
	tgAssertGrowthFigures(t, "tester", tester.Growth, 1, 900, 900, 900)
}

func TestToolGrowthFigures_ToolSeenOnlyThroughGrowth_GetsARowWithoutToolCalls(t *testing.T) {
	run := tgToolsScenario(t)

	glob := requireTool(t, run.Tools.Run, "glob")

	if glob.Calls != 0 || glob.Errors != 0 || glob.Unpaired != 0 {
		t.Errorf("glob = %d calls, %d errors, %d unpaired, want a growth-only row of zeros", glob.Calls, glob.Errors, glob.Unpaired)
	}
	if got := toolNames(run.Tools.Run); !rmEqualStrings(got, []string{"glob", "read_file"}) {
		t.Errorf("run tools = %q, want glob and read_file sorted by name", got)
	}
}

func TestToolGrowthFigures_ToolWithCallsButNoSamples_IsNotProvidedAndNotZero(t *testing.T) {
	none := domain.NewRequestedTools(nil)
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 100, none),
			tgCall("o2", 20, 1500, 50, none),
		},
		orchTools: []domain.ToolCall{ctxTool("ot1", "Bash", 11, 12)},
	}})

	bash := requireTool(t, run.Tools.Run, "Bash")

	if bash.Calls != 1 {
		t.Fatalf("Bash calls = %d, want 1", bash.Calls)
	}
	if bash.Growth.IsProvided() || bash.Growth.Samples != 0 {
		t.Errorf("Bash growth = %+v, want no samples", bash.Growth)
	}
	for name, v := range map[string]domain.TokenCount{"average": bash.Growth.Average, "min": bash.Growth.Min, "max": bash.Growth.Max} {
		if v.IsPresent() {
			t.Errorf("Bash growth %s is present, want not provided (distinct from zero)", name)
		}
	}
}

func TestToolGrowthFigures_ZeroSample_IsProvidedAsZero(t *testing.T) {
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 500, tgOne("read_file")),
			tgCall("o2", 20, 1500, 50, domain.NewRequestedTools(nil)),
		},
	}})

	read := requireTool(t, run.Tools.Run, "read_file")

	if !read.Growth.IsProvided() {
		t.Fatalf("growth = %+v, want one zero sample", read.Growth)
	}
	tgAssertGrowthFigures(t, "read_file", read.Growth, 1, 0, 0, 0)
}

func TestToolGrowthFigures_OnlyLeftOutCalls_GivesARowWithCountsAndNoFigures(t *testing.T) {
	partial := tgCall("o2", 20, 1500, 50, tgOne("glob"))
	partial.Usage.CacheRead = domain.AbsentTokens()
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 500, tgOne("read_file")), // next 1200: negative
			tgCall("o1b", 15, 1200, 50, tgOne("read_file")), // pairs with o2: next context partial
			partial,
			tgCall("o3", 30, 2500, 10, domain.NewRequestedTools(nil)),
		},
	}})

	read := requireTool(t, run.Tools.Orchestrator, "read_file")
	glob := requireTool(t, run.Tools.Orchestrator, "glob")

	if read.Growth.Negative != 1 || read.Growth.NotMeasurable != 1 || read.Growth.Samples != 0 {
		t.Errorf("read_file growth = %+v, want 1 negative, 1 not measurable, no samples", read.Growth)
	}
	if read.Growth.IsProvided() || read.Growth.Average.IsPresent() {
		t.Errorf("read_file growth = %+v, want average not provided", read.Growth)
	}
	if glob.Growth.NotMeasurable != 1 || glob.Growth.Samples != 0 {
		t.Errorf("glob growth = %+v, want 1 not measurable (own call lacks cache read)", glob.Growth)
	}
}

func TestToolGrowthFigures_LeftOutCounts_AreKeptPerLevel(t *testing.T) {
	none := domain.NewRequestedTools(nil)
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 500, tgOne("read_file")), tgCall("o2", 20, 1200, 10, none), // negative
		},
		agents: []ctxAgent{{id: ctxAgentID, typ: tgAgentType, usage: []domain.InvocationUsage{
			tgCall("a1", 10, 2000, 100, tgOne("read_file")), tgCall("a2", 20, 2600, 10, none), // 500
		}}},
	}})

	runRead := requireTool(t, run.Tools.Run, "read_file")
	orchRead := requireTool(t, run.Tools.Orchestrator, "read_file")
	agentRead := requireTool(t, tgAgentTypeTools(t, run, tgAgentType), "read_file")

	if runRead.Growth.Samples != 1 || runRead.Growth.Negative != 1 {
		t.Errorf("run growth = %+v, want 1 sample and 1 negative", runRead.Growth)
	}
	if orchRead.Growth.Samples != 0 || orchRead.Growth.Negative != 1 {
		t.Errorf("orchestrator growth = %+v, want 0 samples and 1 negative", orchRead.Growth)
	}
	if agentRead.Growth.Samples != 1 || agentRead.Growth.Negative != 0 {
		t.Errorf("agent type growth = %+v, want 1 sample and 0 negative", agentRead.Growth)
	}
}

func TestToolGrowthFigures_CallsThatAreNotSingular_ContributeNothing(t *testing.T) {
	none := domain.NewRequestedTools(nil)
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 100, domain.NewRequestedTools(map[string]int{"read_file": 2})),
			tgCall("o2", 20, 1500, 100, domain.NewRequestedTools(map[string]int{"read_file": 1, "glob": 1})),
			tgCall("o3", 30, 2000, 100, none),
		},
	}})

	if len(run.Tools.Run) != 0 {
		t.Errorf("run tools = %q, want none: multi-tool calls are never split", toolNames(run.Tools.Run))
	}
}

func TestToolGrowthFigures_LogsWithoutRequestedTools_ShowNoGrowth(t *testing.T) {
	run := tgPrice(t, tgRunSpec{ctxRunSpec: ctxRunSpec{
		orchUsage: []domain.InvocationUsage{
			tgCall("o1", 10, 1000, 100, domain.RequestedTools{}),
			tgCall("o2", 20, 1500, 50, domain.RequestedTools{}),
		},
		orchTools: []domain.ToolCall{ctxTool("ot1", "Read", 11, 12)},
	}})

	read := requireTool(t, run.Tools.Run, "Read")

	if read.Growth != (domain.ToolGrowth{}) {
		t.Errorf("Read growth = %+v, want the not provided zero value", read.Growth)
	}
	for _, c := range run.Context.Orchestrator.Calls {
		if c.Growth.State != domain.GrowthNotSingular {
			t.Errorf("call %s growth = %+v, want not singular without requested tools", c.RecordID, c.Growth)
		}
	}
}
