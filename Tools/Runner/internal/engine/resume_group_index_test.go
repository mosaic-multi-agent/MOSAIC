package engine_test

// ResumeInfo.GroupIndex indexes the groups of the block the position is in
// (first-appearance order within that block), not one workflow-wide list.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// twoGroupedBlocksRows: block Stage has groups Test, Implementation; block
// Audit has groups Fix, Check (Fix appears first in its rows).
func twoGroupedBlocksRows() []scopeRow {
	return []scopeRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "REVIEW.[Audit].Fix", agent: "fixer"},
		{phase: "REVIEW.[Audit].Check", agent: "checker"},
		{phase: "REPORT", agent: "reporter"},
	}
}

func twoGroupedBlocksTables() string {
	return scopeBlocks(stageBlockRow, "| Audit | Audit.md | Items |") + scopeGroups(
		"| Stage | TDD | Test, Implementation |",
		"| Audit | Standard | Fix, Check |",
	)
}

func twoGroupedBlocksSchedules() domain.BlockSchedules {
	return joinSchedules(
		scheduleOf("Stage", idsOf("TDD", "1")...),
		scheduleOf("Audit", idsOf("Standard", "x1")...),
	)
}

// requireResumeGroup asserts ResumeInfo.GroupIndex.
func requireResumeGroup(t *testing.T, info domain.ResumeInfo, want int) {
	t.Helper()
	if info.GroupIndex != want {
		t.Errorf("GroupIndex: want %d, got %d", want, info.GroupIndex)
	}
}

type groupResumeCase struct {
	name      string
	agent     string
	phase     string
	stage     string
	row       int // 1-based recorded row
	wantRow   int
	wantBlock domain.BlockName
	wantIter  domain.IterationID
	wantGroup int
}

func runGroupResumeCases(t *testing.T, cases []groupResumeCase, interrupted bool) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aw := mustAdmitScope(t, twoGroupedBlocksRows(), twoGroupedBlocksTables(), domain.ExecutionModeAutoReview)
			log := (&runLog{}).
				workflowStep("planner", "", domain.StatusSUCCESS, 1).lastInPhase("PLANNING").
				workflowStep(tc.agent, tc.stage, domain.StatusSUCCESS, tc.row).lastInPhase(tc.phase)

			info, err := resumeLogIDs(aw, twoGroupedBlocksSchedules(), log, interrupted)

			requireResumeBlock(t, info, err, tc.wantRow, interrupted, tc.wantBlock, tc.wantIter)
			requireResumeGroup(t, info, tc.wantGroup)
		})
	}
}

func TestResumeGroupIndex_InterruptedRow_IndexesGroupsOfItsOwnBlock(t *testing.T) {
	runGroupResumeCases(t, []groupResumeCase{
		{"first group of first block", "test-writer", "EXECUTION", "1.Test", 2, 1, "Stage", "1", 0},
		{"second group of first block", "implementer", "EXECUTION", "1.Implementation", 3, 2, "Stage", "1", 1},
		{"first group of second block", "fixer", "REVIEW", "x1.Fix", 4, 3, "Audit", "x1", 0},
		{"second group of second block", "checker", "REVIEW", "x1.Check", 5, 4, "Audit", "x1", 1},
	}, true)
}

func TestResumeGroupIndex_AdvancedRow_IndexesGroupsOfItsOwnBlock(t *testing.T) {
	runGroupResumeCases(t, []groupResumeCase{
		{"into second group of first block", "test-writer", "EXECUTION", "1.Test", 2, 2, "Stage", "1", 1},
		{"into second group of second block", "fixer", "REVIEW", "x1.Fix", 4, 4, "Audit", "x1", 1},
	}, false)
}
