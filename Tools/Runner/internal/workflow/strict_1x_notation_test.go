package workflow_test

// Strict parsing refuses WorkflowFormat 1.x structural notation: [StageNumber]
// block references, {StageNumber} fill-ins, the **Execution Groups:** heading
// and a group before the block reference.

import "testing"

func TestParseStrict_LegacyBlockReference_IsRefused(t *testing.T) {
	cases := []string{
		"EXECUTION.[StageNumber]",
		"EXECUTION.Test.[StageNumber]",
		"EXECUTION.Implementation.[StageNumber]",
	}
	for _, phase := range cases {
		t.Run(phase, func(t *testing.T) {
			rows := []string{strictRow("PLANNING", "agent-a"), strictRow(phase, "agent-b")}

			_, err := parseStrict(strictRegion(rows))

			re := requireRefusal(t, err, refusalWant{row: 2, token: phase})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_LegacyBlockReference_IsRefusedEvenWhenAllOtherCellsAre216(t *testing.T) {
	// The only 1.x notation is the block reference: a strict parser that
	// accepts any bracketed name as a block would accept this table.
	rows := strictPhases("PLANNING", "EXECUTION.[StageNumber]", "REVIEW")

	_, err := parseStrict(strictRegion(rows))

	if err == nil {
		t.Fatal("strict Parse must refuse the [StageNumber] block reference")
	}
	requireRefusal(t, err, refusalWant{row: 2})
}

func TestParseStrict_LegacyBlockReferenceNextToABlocksTable_IsRefusedNotRescued(t *testing.T) {
	// A 2.16 blocks table beside a [StageNumber] row (1.x), otherwise valid:
	// mixed notation is refused, never read as 1.x.
	rows := strictPhases("PLANNING", "EXECUTION.[StageNumber]")

	_, err := parseStrict(strictRegion(rows, strictBlocksTable))

	re := requireRefusal(t, err, refusalWant{row: 2})
	requireRowOrToken(t, re)
}

func TestParseStrict_GroupBeforeBlockReference_IsRefused(t *testing.T) {
	// Even with block Stage declared, a group written before the reference is
	// 1.x order.
	rows := []string{strictRow("PLANNING", "agent-a"), strictRow("EXECUTION.Test.[Stage]", "agent-b")}

	_, err := parseStrict(strictRegion(rows, strictBlocksTable, strictGroupsTable))

	requireRefusal(t, err, refusalWant{anyOf: []string{"P1", "P10"}, row: 2, token: "EXECUTION.Test.[Stage]"})
}

func TestParseStrict_StageNumberFillIn_IsRefused(t *testing.T) {
	cases := map[string][]string{
		"input of a plain row": {
			strictRowIO("PLANNING", "agent-a", "Stage-{StageNumber}/Plan.md", "out.md"),
		},
		"output of a plain row": {
			strictRowIO("PLANNING", "agent-a", "-", "Stage-{StageNumber}/Notes.md"),
		},
		"input of a block row": {
			strictRowIO("EXECUTION.[Stage]", "agent-a", "Stage-{StageNumber}/Plan.md", "out.md"),
		},
		"second row": {
			strictRowIO("PLANNING", "agent-a", "-", "out.md"),
			strictRowIO("EXECUTION.[Stage]", "agent-b", "Plan.md", "Stage-{StageNumber}/PlanProgress.md"),
		},
	}
	wantRow := map[string]int{"input of a plain row": 1, "output of a plain row": 1, "input of a block row": 1, "second row": 2}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(strictRegion(rows, strictBlocksTable))

			re := requireRefusal(t, err, refusalWant{row: wantRow[name], token: "{StageNumber}"})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_StageNumberFillIn_InBlockSource_IsRefused(t *testing.T) {
	rows := strictPhases("PLANNING", "EXECUTION.[Stage]")
	content := strictRegion(rows, blocksTable("| Stage | Stage-{StageNumber}/Plan.md | Stages |"))

	_, err := parseStrict(content)

	re := requireRefusal(t, err, refusalWant{token: "{StageNumber}"})
	requireRowOrToken(t, re)
}

func TestParseStrict_ExecutionGroupsHeading_IsRefused(t *testing.T) {
	// Every case names the heading as the offending token: with a table under
	// it, a refusal through T1 (further table outside the reserved headings)
	// alone would name the table, not the heading.
	cases := map[string]string{
		"with a 2.16 routing table": strictRegion(groupedStageRows(), strictBlocksTable,
			"\n**Execution Groups:**\n\n| Approach | Groups |\n|---|---|\n| TDD | Test, Implementation |\n"),
		"instead of the Groups heading": strictRegion(groupedStageRows(), strictBlocksTable,
			"\n**Execution Groups:**\n\n| Block | Approach | Groups |\n|---|---|---|\n| Stage | TDD | Test, Implementation |\n"),
		"heading only": strictRegion(groupedStageRows(), strictBlocksTable, "\n**Execution Groups:**\n"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			table, err := parseStrict(content)

			if err == nil {
				t.Fatalf("strict Parse must refuse the 1.x heading; got ApproachTable %+v", table.ApproachTable.Rows)
			}
			re := requireRefusal(t, err, refusalWant{token: "Execution Groups"})
			requireRowOrToken(t, re)
		})
	}
}
