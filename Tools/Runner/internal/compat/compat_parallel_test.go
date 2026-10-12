package compat_test

// Mode-dependent admission of forks (a comma list in On Success) and joins (a
// Waits For value other than -) on WorkflowFormat 2.16 tables.
//
//   - Modes 2 and 3 (auto, auto-review) and the unset sentinel refuse both; the
//     refusal names the row holding the fork or the join and its cell.
//   - Mode 1 (orchestrated) admits both, and keeps refusing a fork branch that
//     ends its phase or its group without a join.
//
// The 1.x counterparts live in compat_mode_admission_test.go and stay unchanged.

import (
	"testing"

	"mosaic-run/internal/domain"
)

var orchestratedOnly = []domain.ExecutionMode{domain.ExecutionModeOrchestrated}

// forkJoinRunRows: row 1 forks to rows 2 and 3, both lead to the closing join
// at row 4. All four rows share one phase.
func forkJoinRunRows() []rrow {
	return []rrow{
		{phase: "PLANNING", agent: "agent-a", onSuccess: "agent-b, agent-c"},
		{phase: "PLANNING", agent: "agent-b", onSuccess: "agent-d"},
		{phase: "PLANNING", agent: "agent-c", onSuccess: "agent-d"},
		{phase: "PLANNING", agent: "agent-d", waitsFor: "agent-b, agent-c"},
	}
}

// forkJoinGroupRows: a fork inside group Test of block Stage, closed by a join
// that is the last row of the group.
func forkJoinGroupRows() []rrow {
	return []rrow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-a", onSuccess: "t-b, t-c"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-b", onSuccess: "t-d"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-c", onSuccess: "t-d"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-d", waitsFor: "t-b, t-c"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "i-a"},
		{phase: "REVIEW", agent: "test-runner"},
	}
}

// forkBranchEndsPhaseRows: row 3 is a fork branch and the last row of its
// phase, with no join after it.
func forkBranchEndsPhaseRows() []rrow {
	return []rrow{
		{phase: "PLANNING", agent: "agent-a", onSuccess: "agent-b, agent-c"},
		{phase: "PLANNING", agent: "agent-b", onSuccess: "COMPLETE"},
		{phase: "PLANNING", agent: "agent-c", onSuccess: "COMPLETE"},
	}
}

// forkBranchEndsGroupRows: row 4 is a fork branch and the last row of group
// Test, with group Implementation following and no join.
func forkBranchEndsGroupRows() []rrow {
	return []rrow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-a", onSuccess: "t-b, t-c"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-b", onSuccess: "COMPLETE"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-c", onSuccess: "COMPLETE"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "i-a", onSuccess: "COMPLETE"},
	}
}

// ---- Modes 2 and 3 refuse forks and joins ----

func TestAdmitParallel_ForkAndJoinInRunScope_RefusedInEngineRoutedModes(t *testing.T) {
	table := parseRows(t, forkJoinRunRows(), "")

	requireRefusedIn(t, table, engineRoutedModes,
		wantRoute{rules: []string{""}, rows: []int{1, 4}, tokens: []string{"agent-b", "agent-c"}})
}

func TestAdmitParallel_ForkOnly_RefusedInEngineRoutedModesAtTheForkRow(t *testing.T) {
	table := parseRows(t, forkBranchEndsPhaseRows(), "")

	requireRefusedIn(t, table, engineRoutedModes,
		wantRoute{rules: []string{""}, rows: []int{1}, tokens: []string{"agent-b"}})
}

func TestAdmitParallel_JoinOnly_RefusedInEngineRoutedModesAtTheJoinRow(t *testing.T) {
	rows := []rrow{
		{phase: "PLANNING", agent: "agent-a"},
		{phase: "PLANNING", agent: "agent-b"},
		{phase: "PLANNING", agent: "agent-c", waitsFor: "agent-a, agent-b"},
	}
	table := parseRows(t, rows, "")

	requireRefusedIn(t, table, engineRoutedModes,
		wantRoute{rules: []string{""}, rows: []int{3}, tokens: []string{"agent-a"}})
}

func TestAdmitParallel_ForkInsideGroup_RefusedInEngineRoutedModes(t *testing.T) {
	table := parseRows(t, forkJoinGroupRows(), groupedStageTables())

	requireRefusedIn(t, table, engineRoutedModes,
		wantRoute{rules: []string{"B1"}, rows: []int{2, 5}, tokens: []string{"t-b", "t-c"}})
}

func TestAdmitParallel_JoinInsideGroup_RefusedInEngineRoutedModesAtTheJoinRow(t *testing.T) {
	rows := []rrow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-a"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-b", waitsFor: "t-a"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "i-a"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, groupedStageTables())

	requireRefusedIn(t, table, engineRoutedModes,
		wantRoute{rules: []string{"B1"}, rows: []int{3}, tokens: []string{"t-a"}})
}

// ---- Mode 1 admits forks and joins as today ----

func TestAdmitParallel_ForkAndJoinInRunScope_AdmittedInOrchestratedMode(t *testing.T) {
	table := parseRows(t, forkJoinRunRows(), "")

	requireAdmittedIn(t, table, orchestratedOnly)
}

func TestAdmitParallel_ThreeBranchForkWithAllEntriesMatching_AdmittedInOrchestratedMode(t *testing.T) {
	rows := []rrow{
		{phase: "PLANNING", agent: "agent-a", onSuccess: "agent-b, agent-c, agent-d"},
		{phase: "PLANNING", agent: "agent-b", onSuccess: "agent-e"},
		{phase: "PLANNING", agent: "agent-c", onSuccess: "agent-e"},
		{phase: "PLANNING", agent: "agent-d", onSuccess: "agent-e"},
		{phase: "PLANNING", agent: "agent-e", waitsFor: "agent-b, agent-c, agent-d"},
	}
	table := parseRows(t, rows, "")

	requireAdmittedIn(t, table, orchestratedOnly)
}

func TestAdmitParallel_JoinOnly_AdmittedInOrchestratedMode(t *testing.T) {
	rows := []rrow{
		{phase: "PLANNING", agent: "agent-a"},
		{phase: "PLANNING", agent: "agent-b"},
		{phase: "PLANNING", agent: "agent-c", waitsFor: "agent-a, agent-b"},
	}
	table := parseRows(t, rows, "")

	requireAdmittedIn(t, table, orchestratedOnly)
}

func TestAdmitParallel_ForkAndJoinInsideGroup_AdmittedInOrchestratedMode(t *testing.T) {
	table := parseRows(t, forkJoinGroupRows(), groupedStageTables())

	aw := requireAdmittedIn(t, table, orchestratedOnly)

	if len(aw.Blocks) != 1 || aw.Blocks[0].Name != "Stage" {
		t.Errorf("want the one block Stage in the admitted model, got %+v", aw.Blocks)
	}
}

// ---- Mode 1 keeps the fork-branch boundary check ----

// The fork branches close with COMPLETE, a legal B2 close, so the refusal comes
// from the Runner's own fork-branch boundary check, which carries no rule ID.
func TestAdmitParallel_ForkBranchEndsPhase_RefusedInOrchestratedModeAtTheBranchRow(t *testing.T) {
	table := parseRows(t, forkBranchEndsPhaseRows(), "")

	requireRefusedIn(t, table, orchestratedOnly,
		wantRoute{rules: []string{""}, rows: []int{3}, tokens: []string{"agent-c"}})
}

func TestAdmitParallel_ForkBranchEndsGroup_RefusedInOrchestratedModeAtTheBranchRow(t *testing.T) {
	table := parseRows(t, forkBranchEndsGroupRows(), groupedStageTables())

	requireRefusedIn(t, table, orchestratedOnly,
		wantRoute{rules: []string{""}, rows: []int{4}, tokens: []string{"t-c"}})
}

func TestAdmitParallel_ForkBranchEndsGroup_RefusedInEngineRoutedModesAtTheForkRow(t *testing.T) {
	table := parseRows(t, forkBranchEndsGroupRows(), groupedStageTables())

	requireRefusedIn(t, table, engineRoutedModes,
		wantRoute{rules: []string{"B1"}, rows: []int{2}, tokens: []string{"t-b"}})
}
