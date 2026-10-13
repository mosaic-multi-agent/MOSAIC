package workflow_test

// Strict refusals of the **Blocks:** table (P4, P5) and the **Groups:** table
// (P6, P7, P8).

import "testing"

// blocksRegion builds a region whose blocks table is the given raw text.
func blocksRegion(tableText string, rows ...string) string {
	if rows == nil {
		rows = strictPhases("EXECUTION.[Stage]")
	}
	return strictRegion(rows, blocksHeader+tableText)
}

// groupsRegion builds a grouped region with a valid blocks table and the given
// raw approach table text.
func groupsRegion(tableText string) string {
	return strictRegion(groupedStageRows(), strictBlocksTable, groupsHeader+tableText)
}

func TestParseStrict_P4_BlocksTableShape_IsRefused(t *testing.T) {
	cases := map[string]string{
		"missing Section column": blocksRegion("| Block | Source |\n|---|---|\n| Stage | Plan.md |\n"),
		"missing Source column":  blocksRegion("| Block | Section |\n|---|---|\n| Stage | Stages |\n"),
		"extra column": blocksRegion("| Block | Source | Section | Notes |\n|---|---|---|---|\n" +
			"| Stage | Plan.md | Stages | x |\n"),
		"no data rows": blocksRegion("| Block | Source | Section |\n|---|---|---|\n"),
		"empty Source cell": blocksRegion("| Block | Source | Section |\n|---|---|---|\n" +
			"| Stage | | Stages |\n"),
		"empty Section cell": blocksRegion("| Block | Source | Section |\n|---|---|---|\n" +
			"| Stage | Plan.md | |\n"),
		"empty Block cell": blocksRegion("| Block | Source | Section |\n|---|---|---|\n" +
			"| | Plan.md | Stages |\n"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			re := requireRefusal(t, err, refusalWant{rule: "P4"})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_P5_BlockDeclaration_IsRefused(t *testing.T) {
	header := "| Block | Source | Section |\n|---|---|---|\n"
	cases := map[string]struct {
		table string
		token string
	}{
		"declared twice": {
			header + "| Stage | Plan.md | Stages |\n| Stage | Other.md | More |\n", "Stage"},
		"name with digit":        {header + "| Stage2 | Plan.md | Stages |\n", "Stage2"},
		"name with hyphen":       {header + "| My-Stage | Plan.md | Stages |\n", "My-Stage"},
		"name with underscore":   {header + "| My_Stage | Plan.md | Stages |\n", "My_Stage"},
		"reserved name HITL":     {header + "| HITL | Plan.md | Stages |\n", "HITL"},
		"reserved name Approach": {header + "| Approach | Plan.md | Stages |\n", "Approach"},
		"wildcard in source":     {header + "| Stage | Stage-*/Plan.md | Stages |\n", "Stage-*/Plan.md"},
		"wildcard star only":     {header + "| Stage | *.md | Stages |\n", "*.md"},
		"fill-in of itself in a block that is not nested": {
			header + "| Stage | {Stage}/Plan.md | Stages |\n", "{Stage}"},
		"fill-in of a block that does not enclose it": {
			header + "| Stage | {Product}/Plan.md | Stages |\n| Product | Req.md | Products |\n", "{Product}"},
		"fill-in of a block that is not declared at all": {
			header + "| Stage | {Foo}/Plan.md | Stages |\n", "{Foo}"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rows := strictPhases("EXECUTION.[Stage]", "REVIEW.[Product]")
			_, err := parseStrict(blocksRegion(tc.table, rows...))

			re := requireRefusal(t, err, refusalWant{rule: "P5", token: tc.token})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_P6_GroupsTableShape_IsRefused(t *testing.T) {
	cases := map[string]string{
		"1.x two-column form":   groupsRegion("| Approach | Groups |\n|---|---|\n| TDD | Test, Implementation |\n"),
		"missing Groups column": groupsRegion("| Block | Approach |\n|---|---|\n| Stage | TDD |\n"),
		"extra column": groupsRegion("| Block | Approach | Groups | Notes |\n|---|---|---|---|\n" +
			"| Stage | TDD | Test | x |\n"),
		"no data rows": groupsRegion("| Block | Approach | Groups |\n|---|---|---|\n"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			re := requireRefusal(t, err, refusalWant{rule: "P6"})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_P7_GroupsTableCells_AreRefused(t *testing.T) {
	header := "| Block | Approach | Groups |\n|---|---|---|\n"
	// token is the offending text that must be named; "" where the defect is an
	// empty cell, which has no text to name (the row or token must still be set).
	cases := map[string]struct{ table, token string }{
		"empty Block cell":              {header + "| | TDD | Test |\n", ""},
		"empty Approach cell":           {header + "| Stage | | Test |\n", ""},
		"empty Groups cell":             {header + "| Stage | TDD | |\n", ""},
		"empty token inside Groups":     {header + "| Stage | TDD | Test, , Implementation |\n", ""},
		"trailing comma in Groups":      {header + "| Stage | TDD | Test, |\n", ""},
		"approach with underscore":      {header + "| Stage | TDD_x | Test |\n", "TDD_x"},
		"approach starting with hyphen": {header + "| Stage | -TDD | Test |\n", "-TDD"},
		"approach with whitespace":      {header + "| Stage | T DD | Test |\n", "T DD"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(groupsRegion(tc.table))

			re := requireRefusal(t, err, refusalWant{rule: "P7", token: tc.token})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_P8_GroupsTableRepeats_AreRefused(t *testing.T) {
	header := "| Block | Approach | Groups |\n|---|---|---|\n"
	cases := map[string]struct{ table, token string }{
		"Block and Approach pair repeats": {header +
			"| Stage | TDD | Test, Implementation |\n| Stage | TDD | Implementation, Test |\n", "TDD"},
		"group repeats within one cell": {header + "| Stage | TDD | Test, Implementation, Test |\n", "Test"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(groupsRegion(tc.table))

			re := requireRefusal(t, err, refusalWant{rule: "P8", token: tc.token})
			requireRowOrToken(t, re)
		})
	}
}
