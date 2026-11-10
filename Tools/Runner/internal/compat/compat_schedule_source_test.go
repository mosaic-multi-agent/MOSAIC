package compat_test

// Admission of WorkflowFormat 2.16 block schedule sources (A7) and the seed
// classification recorded in the per-block admitted model: the classification
// ("seeded" or "produced"), the rows outside the block that produce the Source
// before it is entered, and the in-block writer rows. Refusals are in
// compat_schedule_source_refusal_test.go.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestAdmitScheduleSource_ProducedByRunScopeRowBeforeBlock_RecordedAsProduced(t *testing.T) {
	table := parseRows(t, ungroupedStageRows(), stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0}, nil)
	})
}

func TestAdmitScheduleSource_FirstRowBeginsBlock_RecordedAsSeeded(t *testing.T) {
	rows := []rrow{
		{phase: "EXECUTION.[Stage]", agent: "implementation-tdd"},
		{phase: "EXECUTION.[Stage]", agent: "implementation-review"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedSeeded, nil, nil)
	})
}

func TestAdmitScheduleSource_SourceInFirstRowInputs_RecordedAsSeeded(t *testing.T) {
	rows := []rrow{
		{phase: "RESEARCH", agent: "researcher", input: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedSeeded, nil, nil)
	})
}

func TestAdmitScheduleSource_SeededSourceAlsoOutputBeforeBlock_StaysSeededWithNoProducingRow(t *testing.T) {
	rows := []rrow{
		{phase: "RESEARCH", agent: "researcher", input: "Plan.md"},
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedSeeded, nil, nil)
	})
}

func TestAdmitScheduleSource_SourceNameNotEndingInPlanMd_IsAdmittedAsProduced(t *testing.T) {
	rows := ungroupedStageRows()
	rows[0].output = "AuditPlan.md, Stage-*/AuditPlan.md"
	table := parseRows(t, rows, blocksSection("| Stage | AuditPlan.md | Stages |"))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0}, nil)
	})
}

func TestAdmitScheduleSource_SourceNameNotEndingInPlanMdInFirstRowInputs_IsAdmittedAsSeeded(t *testing.T) {
	rows := []rrow{
		{phase: "RESEARCH", agent: "researcher", input: "Backlog.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, blocksSection("| Stage | Backlog.md | Items |"))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedSeeded, nil, nil)
	})
}

func TestAdmitScheduleSource_SourceNameWithoutPlanMdOutputByRowBeforeBlock_RecordedAsProduced(t *testing.T) {
	rows := ungroupedStageRows()
	rows[0].output = "Backlog.md"
	table := parseRows(t, rows, blocksSection("| Stage | Backlog.md | Items |"))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0}, nil)
	})
}

func TestAdmitScheduleSource_ProducerPositionedBelowBlockButRoutedBeforeEntry_RecordedAsProduced(t *testing.T) {
	// Row 1 jumps over the block to the producer at row 5, which enters the
	// block; the block exits to row 4, which completes. The producer runs before
	// the block is entered although it sits below it in the table.
	rows := []rrow{
		{phase: "PLANNING", agent: "agent-a", onSuccess: "producer"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW", agent: "test-runner", onSuccess: "COMPLETE"},
		{phase: "REVIEW", agent: "producer", onSuccess: "[Stage]", output: "Plan.md"},
	}
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{4}, nil)
	})
}

func TestAdmitScheduleSource_WildcardOutputMatchingWithinOneSegment_IsAdmittedAsProduced(t *testing.T) {
	rows := ungroupedStageRows()
	rows[0].output = "Plans/*.md"
	table := parseRows(t, rows, blocksSection("| Stage | Plans/Plan.md | Stages |"))

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0}, nil)
	})
}

func TestAdmitScheduleSource_ProducedBlockWhoseOwnRowAlsoOutputsSource_RecordsProducerAndWriter(t *testing.T) {
	rows := ungroupedStageRows()
	rows[2].output = "Plan.md"
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0}, []int{2})
	})
}

func TestAdmitScheduleSource_SeededBlockWhoseOwnRowOutputsSource_RecordsWriterOnly(t *testing.T) {
	rows := []rrow{
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b", output: "Plan.md"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedSeeded, nil, []int{1})
	})
}

func TestAdmitScheduleSource_SourceOutputByRowOfAnotherBlockBeforeEntry_RecordsThatRowAsProducer(t *testing.T) {
	rows := twoBlockRows()
	rows[0].output = "Plan.md"
	rows[2].output = "Audit.md" // impl-b, the last row of block Stage
	table := parseRows(t, rows, twoBlockTables())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0}, nil)
		requireSeedModel(t, aw, "Audit", domain.SeedProduced, []int{2}, nil)
	})
}

func TestAdmitScheduleSource_SeveralRowsProduceSource_RecordsEveryProducerInTableOrder(t *testing.T) {
	rows := append([]rrow{{phase: "RESEARCH", agent: "researcher", output: "Plan.md"}}, ungroupedStageRows()...)
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0, 1}, nil)
	})
}

func TestAdmitScheduleSource_SeveralBlockRowsOutputSource_RecordsEveryWriterInTableOrder(t *testing.T) {
	rows := ungroupedStageRows()
	rows[1].output = "Plan.md"
	rows[2].output = "Plan.md"
	table := parseRows(t, rows, stageBlockTable())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		requireSeedModel(t, aw, "Stage", domain.SeedProduced, []int{0}, []int{1, 2})
	})
}

func TestAdmitScheduleSource_SeveralBlocks_NoBlockIsLeftUnclassified(t *testing.T) {
	table := parseRows(t, twoBlockRows(), twoBlockTables())

	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
		aw := admitIn(t, table, mode)

		for _, b := range aw.Blocks {
			if b.Seed == domain.SeedUnclassified {
				t.Errorf("block %q is left unclassified", b.Name)
			}
		}
	})
}

func TestAdmitScheduleSource_BlocksOfA216Table_AreClassifiedAsProduced(t *testing.T) {
	sibling := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "REVIEW.[Audit]", agent: "auditor"},
	}
	tables := map[string]domain.RoutingTable{
		"grouped": mustParse216(t, region(brownfield40Rows(), false, brownfield40Tables())),
		"siblings": mustParse216(t, region(sibling, false,
			blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |"))),
	}

	for name, table := range tables {
		t.Run(name, func(t *testing.T) {
			forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
				aw := admitIn(t, table, mode)

				if len(aw.Blocks) == 0 {
					t.Fatal("want at least one admitted block")
				}
				for _, b := range aw.Blocks {
					// The planner row outputs every Source, and the first row neither
					// lists a Source nor begins a block.
					if b.Seed != domain.SeedProduced {
						t.Errorf("block %q: Seed want %q, got %q", b.Name, domain.SeedProduced, b.Seed)
					}
				}
			})
		})
	}
}
