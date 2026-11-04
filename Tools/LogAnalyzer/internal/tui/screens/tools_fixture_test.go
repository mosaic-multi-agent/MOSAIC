package screens_test

// Fixture for the tools view. Tool names are unique per section so a row can be
// found by its tool name.

import (
	"fmt"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

func durationStats(count int, total, min, max, avg time.Duration) domain.DurationStats {
	return domain.DurationStats{Count: count, Total: total, Min: min, Max: max, Average: avg}
}

// toolsRun has a run list (Read, Bash, one unnamed unpaired call), an
// orchestrator list (Grep) and two agent types (builder with Edit, unknown with
// Write). The base run carries money so the tests can prove V4 does not show it.
func toolsRun() domain.RunReport {
	run := overviewRun()
	run.Tools = domain.ToolsBreakdown{
		Run: []domain.ToolFigures{
			{Tool: "Bash", Calls: 6, Errors: 0, Unpaired: 2, Durations: durationStats(4, 10*time.Second, time.Second, 6*time.Second, 2500*time.Millisecond)},
			{Tool: "Read", Calls: 11, Errors: 3, Unpaired: 0, Durations: durationStats(11, 44*time.Second, ms(1250), ms(7500), 4*time.Second)},
			{Tool: "", Calls: 3, Errors: 0, Unpaired: 2},
		},
		Orchestrator: []domain.ToolFigures{
			{Tool: "Grep", Calls: 4, Errors: 1, Unpaired: 0, Durations: durationStats(4, 6*time.Second, ms(500), ms(3500), ms(1500))},
		},
		ByAgentType: []domain.AgentTypeTools{
			{AgentType: "builder", Tools: []domain.ToolFigures{
				{Tool: "Edit", Calls: 9, Errors: 0, Unpaired: 0, Durations: durationStats(9, 27*time.Second, 2*time.Second, 4*time.Second, 3*time.Second)},
			}},
			{AgentType: "", Tools: []domain.ToolFigures{
				{Tool: "Write", Calls: 5, Errors: 2, Unpaired: 1, Durations: durationStats(4, 8*time.Second, ms(900), ms(3200), 2*time.Second)},
			}},
		},
	}
	return run
}

// longMCPTool is an MCP tool name of exactly 41 characters.
const longMCPTool = "mcp__contact-user__ask_user_questions_ext"

// runWithTools is a run whose only tool section is the given run-level list.
func runWithTools(tools ...domain.ToolFigures) domain.RunReport {
	run := overviewRun()
	run.Tools = domain.ToolsBreakdown{Run: tools}
	return run
}

// shareRun has a chatty tool with little total time and a quiet one with much
// total time, so a calls share and a duration share rank them oppositely.
func shareRun() domain.RunReport {
	return runWithTools(
		domain.ToolFigures{Tool: "ToolA", Calls: 10, Durations: durationStats(10, time.Second, ms(100), ms(100), ms(100))},
		domain.ToolFigures{Tool: "ToolB", Calls: 2, Durations: durationStats(2, 20*time.Second, 10*time.Second, 10*time.Second, 10*time.Second)},
	)
}

// errorsRun has a first row (most calls) without errors and a second row with errors.
func errorsRun() domain.RunReport {
	return runWithTools(
		domain.ToolFigures{Tool: "Alpha", Calls: 20, Durations: durationStats(20, 20*time.Second, time.Second, time.Second, time.Second)},
		domain.ToolFigures{Tool: "Beta", Calls: 5, Errors: 4, Durations: durationStats(5, 5*time.Second, time.Second, time.Second, time.Second)},
		domain.ToolFigures{Tool: "Gamma", Calls: 3, Durations: durationStats(3, 3*time.Second, time.Second, time.Second, time.Second)},
	)
}

// manyToolsRun has n run-level tools tool-01 ...
func manyToolsRun(n int) domain.RunReport {
	run := overviewRun()
	for i := 1; i <= n; i++ {
		run.Tools.Run = append(run.Tools.Run, domain.ToolFigures{
			Tool: fmt.Sprintf("tool-%02d", i), Calls: i,
			Durations: durationStats(i, time.Duration(i)*time.Second, time.Second, time.Second, time.Second),
		})
	}
	return run
}
