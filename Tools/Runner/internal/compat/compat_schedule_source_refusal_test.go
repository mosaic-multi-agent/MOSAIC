package compat_test

// Refusals of WorkflowFormat 2.16 block schedule sources (A7): a Source that is
// neither a seed nor output by a row outside its block that runs before the
// block is entered. Each refusal names rule A7 and the Source token.

import (
	"testing"
)

func a7Refusal(source string) wantRoute {
	return wantRoute{rules: []string{"A7"}, tokens: []string{source}}
}

func TestAdmitScheduleSource_SourceOutputOnlyByRowInsideBlock_IsRefused(t *testing.T) {
	rows := []rrow{
		{phase: "PLANNING", agent: "planner"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	requireRefusedIn(t, table, allModes, a7Refusal("Plan.md"))
}

func TestAdmitScheduleSource_SourceOutputOnlyByRowAfterBlock_IsRefused(t *testing.T) {
	rows := ungroupedStageRows()
	rows[0].output = "" // the planner no longer produces Plan.md
	rows[3].output = "Plan.md"
	table := parseRows(t, rows, stageBlockTable())

	requireRefusedIn(t, table, allModes, a7Refusal("Plan.md"))
}

func TestAdmitScheduleSource_SourceNeitherSeedNorOutputByAnyRow_IsRefusedOnA216Table(t *testing.T) {
	rows := ungroupedStageRows()
	rows[0].output = "" // no row outputs Plan.md; the first row neither lists it nor begins the block
	table := parseRows(t, rows, stageBlockTable())

	requireRefusedIn(t, table, allModes, a7Refusal("Plan.md"))
}

func TestAdmitScheduleSource_SourceNameWithoutPlanMdOutputByNoRow_IsRefused(t *testing.T) {
	rows := ungroupedStageRows() // no row outputs Backlog.md
	table := parseRows(t, rows, blocksSection("| Stage | Backlog.md | Items |"))

	requireRefusedIn(t, table, allModes, a7Refusal("Backlog.md"))
}

func TestAdmitScheduleSource_WildcardOutputNotMatchingAcrossPathSegments_IsRefused(t *testing.T) {
	rows := ungroupedStageRows()
	rows[0].output = "Plans/*.md" // * never stands for text containing a slash
	table := parseRows(t, rows, blocksSection("| Stage | Plans/Sub/Plan.md | Stages |"))

	requireRefusedIn(t, table, allModes, a7Refusal("Plans/Sub/Plan.md"))
}

func TestAdmitScheduleSource_SourceOutputOnlyByRowOfALaterBlock_IsRefused(t *testing.T) {
	rows := twoBlockRows()
	rows[0].output = "Audit.md" // the planner produces only the Audit source
	rows[3].output = "Plan.md"  // auditor, a row of the block that runs after Stage
	table := parseRows(t, rows, twoBlockTables())

	requireRefusedIn(t, table, allModes, a7Refusal("Plan.md"))
}

func TestAdmitScheduleSource_SourceOutputOnlyByRowReachedOnlyThroughOnFindings_IsRefused(t *testing.T) {
	// Row 1 jumps over the producer at row 2, which only row 3's On Findings
	// names. On Findings routes do not count as running before the block.
	rows := []rrow{
		{phase: "RESEARCH", agent: "agent-a", onSuccess: "agent-c"},
		{phase: "RESEARCH", agent: "producer", output: "Plan.md"},
		{phase: "PLANNING", agent: "agent-c", onFindings: "producer"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	requireRefusedIn(t, table, allModes, a7Refusal("Plan.md"))
}

func TestAdmitScheduleSource_SourceListedOnlyInInputOfLaterRow_IsRefused(t *testing.T) {
	// Only the first row's Inputs seed a workflow. A later run-scope row listing
	// Plan.md as an Input, with no row outputting it, does not make it a seed.
	rows := []rrow{
		{phase: "PLANNING", agent: "researcher"},
		{phase: "PLANNING", agent: "planner", input: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	requireRefusedIn(t, table, allModes, a7Refusal("Plan.md"))
}
