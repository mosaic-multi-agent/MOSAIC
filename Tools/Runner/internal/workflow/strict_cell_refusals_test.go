package workflow_test

// Strict parsing refuses routing-cell and fill-in defects (T5, T6, T9, T12,
// T14, P9, P11) and the 1.x cell notation. Every refusal names the workflow,
// the rule where one exists, and the 1-based row or the token.

import (
	"testing"
)

// targetRows renders agent-a (row 1), agent-b (row 2), agent-c (row 3) with
// the given On Success and On Findings cells on row 2.
func targetRows(onSuccess, onFindings string) string {
	return table7(
		plain7("PLANNING", "agent-a"),
		row7("PLANNING", "agent-b", "FALSE", onSuccess, onFindings),
		plain7("REVIEW", "agent-c"),
	)
}

func TestParseStrict_TargetMatchingNoSubagentCell_IsRefusedByT9(t *testing.T) {
	cases := map[string]string{
		"On Success":                  targetRows("agent-z", "-"),
		"On Findings":                 targetRows("NEXT", "agent-z"),
		"fork with one bad entry":     targetRows("agent-a, agent-z", "-"),
		"fork with the bad entry 1st": targetRows("agent-z, agent-a", "-"),
		"bare id of a qualified row": table7(
			plain7("PLANNING", "agent-a"),
			row7("RESEARCH", "codebase-research(architecture)", "FALSE", "NEXT", "-"),
			row7("REVIEW", "agent-c", "FALSE", "COMPLETE", "codebase-research"),
		),
		"qualified cell of a bare row": table7(
			plain7("PLANNING", "agent-a"),
			row7("RESEARCH", "codebase-research", "FALSE", "NEXT", "-"),
			row7("REVIEW", "agent-c", "FALSE", "codebase-research(architecture)", "-"),
		),
	}
	wantRow := map[string]int{
		"On Success": 2, "On Findings": 2, "fork with one bad entry": 2, "fork with the bad entry 1st": 2,
		"bare id of a qualified row": 3, "qualified cell of a bare row": 3,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			re := requireRefusal(t, err, refusalWant{rule: "T9", row: wantRow[name]})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_T9Refusal_NamesTheUnmatchedTarget(t *testing.T) {
	_, err := parseStrict(targetRows("agent-a, agent-z", "-"))

	requireRefusal(t, err, refusalWant{rule: "T9", row: 2, token: "agent-z"})
}

func TestParseStrict_WaitsForTargetMatchingNoSubagentCell_IsRefusedByT9(t *testing.T) {
	rows := joinRows("agent-b, agent-z")

	_, err := parseStrict(table8(rows...))

	requireRefusal(t, err, refusalWant{rule: "T9", row: 4, token: "agent-z"})
}

func TestParseStrict_ForkWhoseEntriesAllMatch_IsAccepted(t *testing.T) {
	mustParseStrict(t, targetRows("agent-a, agent-c", "-"))
}

func TestParseStrict_OnFindings_ProseOrAlternatives_IsRefusedByT6(t *testing.T) {
	cases := map[string]string{
		"prose with qualifier text": "agent-a (re-check)",
		"two targets":               "agent-a, agent-c",
		"alternatives":              "agent-a or agent-c",
		"sentence":                  "back to the planner",
	}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(targetRows("NEXT", cell))

			re := requireRefusal(t, err, refusalWant{rule: "T6", row: 2, token: cell})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_OnFindingsProseNextToAMatchingRow_IsStillRefused(t *testing.T) {
	// The agent id before the parenthesis names a real row; the suffix makes
	// the cell prose, which is no target.
	rows := table7(
		plain7("TESTING", "mosaictest-scripted"),
		row7("REVIEW", "test-runner", "FALSE", "COMPLETE", "mosaictest-scripted (re-check)"),
	)

	_, err := parseStrict(rows)

	requireRefusal(t, err, refusalWant{rule: "T6", row: 2, token: "mosaictest-scripted (re-check)"})
}

func TestParseStrict_OnSuccessMalformedTarget_IsRefusedByT6OrT9(t *testing.T) {
	cases := map[string]string{
		"prose":             "hand over to the reviewer",
		"target with space": "agent-c (final)",
		"uppercase id":      "Agent-C",
		"fork with prose":   "agent-a, the reviewer",
		"empty fork entry":  "agent-a,",
		"trailing comma":    "agent-a, agent-c,",
	}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(targetRows(cell, "-"))

			re := requireRefusal(t, err, refusalWant{anyOf: []string{"T5", "T6", "T9", "T12"}, row: 2})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_DuplicateTargetInOneCell_IsRefusedByT14(t *testing.T) {
	cases := map[string]string{
		"fork twice the same": targetRows("agent-a, agent-a", "-"),
		"fork third repeats":  targetRows("agent-a, agent-c, agent-a", "-"),
		"waits for twice":     table8(joinRows("agent-b, agent-b")...),
	}
	wantRow := map[string]int{"fork twice the same": 2, "fork third repeats": 2, "waits for twice": 4}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			re := requireRefusal(t, err, refusalWant{rule: "T14", row: wantRow[name]})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_FillInOfABlockTheRowIsNotIn_IsRefusedByP9(t *testing.T) {
	cases := map[string][][]string{
		"row outside every block": {
			{"PLANNING", "agent-a", "FALSE", "NEXT", "-", "Stage-{Stage}/Plan.md", "out.md"},
			{"EXECUTION.[Stage]", "agent-b", "FALSE", "COMPLETE", "-", "-", "out.md"},
		},
		"other block": {
			plain7("PLANNING", "agent-a"),
			{"EXECUTION.[Stage]", "agent-b", "FALSE", "COMPLETE", "-", "-", "{Other}/Progress.md"},
		},
		"in the output of a plain row": {
			{"PLANNING", "agent-a", "FALSE", "NEXT", "-", "-", "{Stage}/Notes.md"},
			{"EXECUTION.[Stage]", "agent-b", "FALSE", "COMPLETE", "-", "-", "out.md"},
		},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(table7(rows...) + strictBlocksTable)

			re := requireRefusal(t, err, refusalWant{rule: "P9"})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_FillInOutsideInputAndOutput_IsRefused(t *testing.T) {
	cases := map[string][]string{
		"Subagent":    {"EXECUTION.[Stage]", "build-{Stage}", "FALSE", "COMPLETE", "-", "-", "out.md"},
		"On Success":  {"EXECUTION.[Stage]", "agent-b", "FALSE", "agent-{Stage}", "-", "-", "out.md"},
		"On Findings": {"EXECUTION.[Stage]", "agent-b", "FALSE", "COMPLETE", "agent-{Stage}", "-", "out.md"},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			rows := [][]string{plain7("PLANNING", "agent-a"), row}

			_, err := parseStrict(table7(rows...) + strictBlocksTable)

			re := requireRefusal(t, err, refusalWant{anyOf: []string{"P11", "T6", "T9"}, row: 2})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_FillInInWaitsFor_IsRefusedByP11(t *testing.T) {
	rows := [][]string{
		row8("PLANNING", "agent-a", "FALSE", "agent-b, agent-c", "-", "-"),
		row8("RESEARCH", "agent-b", "FALSE", "agent-d", "-", "-"),
		row8("RESEARCH", "agent-c", "FALSE", "agent-d", "-", "-"),
		row8("REVIEW", "agent-d", "FALSE", "COMPLETE", "-", "agent-{Stage}, agent-c"),
	}

	_, err := parseStrict(table8(rows...))

	requireRefusal(t, err, refusalWant{anyOf: []string{"P11", "T6", "T9"}, row: 4})
}

// ---- 1.x cell notation ----

func TestParseStrict_OnSuccessDashOrEmpty_IsRefused(t *testing.T) {
	cases := map[string]string{"dash": "-", "empty": ""}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(threeRowsWith("On Success", cell))

			re := requireRefusal(t, err, refusalWant{anyOf: []string{"T5", "T12"}, row: 2})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_1xTargetNotation_IsRefusedNamingTheToken(t *testing.T) {
	cases := map[string]struct {
		content string
		token   string
	}{
		"lowercase next":             {targetRows("next", "-"), "next"},
		"capitalised Next":           {targetRows("Next", "-"), "Next"},
		"lowercase complete":         {targetRows("complete", "-"), "complete"},
		"star suffix on On Success":  {targetRows("agent-c*", "-"), "agent-c*"},
		"star suffix on On Findings": {targetRows("NEXT", "agent-a*"), "agent-a*"},
		"star suffix inside a fork":  {targetRows("agent-a, agent-c*", "-"), "agent-c*"},
		"star suffix on a Waits For": {table8(joinRows("agent-b, agent-c*")...), "agent-c*"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(tc.content)

			re := requireRefusal(t, err, refusalWant{anyOf: []string{"T6", "T9", ""}, token: tc.token})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_StarSuffixedTarget_IsNotMatchedToItsBareRow(t *testing.T) {
	// agent-c exists; "agent-c*" must not be read as agent-c.
	table, err := parseStrict(targetRows("agent-c*", "-"))

	if err == nil {
		t.Fatalf("a * suffix must be refused, got row 2 Success %+v", table.Rows[1].Success)
	}
}

func TestParseStrict_ForkEntryThatIsAReservedWordOrBlockEntry_IsRefused(t *testing.T) {
	cases := map[string]string{
		"NEXT after a target":    "agent-c, NEXT",
		"COMPLETE before target": "COMPLETE, agent-c",
		"block entry in a fork":  "[Stage], agent-c",
	}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			rows := [][]string{
				row7("PLANNING", "agent-a", "FALSE", cell, "-"),
				plain7("PLANNING", "agent-c"),
				{"EXECUTION.[Stage]", "agent-b", "FALSE", "COMPLETE", "-", "-", "out.md"},
			}

			_, err := parseStrict(table7(rows...) + strictBlocksTable)

			re := requireRefusal(t, err, refusalWant{anyOf: []string{"T5", "T6"}, row: 1})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_WaitsForMalformedCell_IsRefusedByT6(t *testing.T) {
	cases := map[string]string{
		"prose":                "agent-b and agent-c",
		"dash inside the list": "-, agent-b",
	}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(table8(joinRows(cell)...))

			re := requireRefusal(t, err, refusalWant{rule: "T6", row: 4})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_FillInOfAnInnerBlockOnARowInTheOuterBlockOnly_IsRefusedByP9(t *testing.T) {
	rows := [][]string{
		plain7("PLANNING", "agent-a"),
		{"PLANNING.[Domain]", "agent-b", "FALSE", "NEXT", "-", "{Stage}/Notes.md", "out.md"},
		{"EXECUTION.[Domain].[Stage]", "agent-c", "FALSE", "COMPLETE", "-", "-", "out.md"},
	}
	content := table7(rows...) +
		blocksTable("| Domain | SystemDesign.md | Domains |", "| Stage | {Domain}/Plan.md | Stages |")

	_, err := parseStrict(content)

	re := requireRefusal(t, err, refusalWant{rule: "P9"})
	requireRowOrToken(t, re)
}

// ---- Refusal items ----

func TestParseStrict_EveryCellRefusal_NamesWorkflowRuleAndRow(t *testing.T) {
	cases := map[string]struct {
		content string
		row     int
	}{
		"missing On Success column": {missingColumnTable("On Success"), 0},
		"unknown column":            {unknownColumnTable(), 0},
		"empty On Findings":         {threeRowsWith("On Findings", ""), 2},
		"duplicate Subagent":        {table7(plain7("PLANNING", "agent-a"), plain7("REVIEW", "agent-a")), 0},
		"malformed Subagent":        {threeRowsWith("Subagent", "Agent_B"), 2},
		"HITL True":                 {threeRowsWith("HITL", "True"), 2},
		"On Success dash":           {threeRowsWith("On Success", "-"), 2},
		"target matching no cell":   {targetRows("agent-z", "-"), 2},
		"prose On Findings":         {targetRows("NEXT", "agent-a (re-check)"), 2},
		"duplicate fork target":     {targetRows("agent-a, agent-a", "-"), 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(tc.content)

			re := requireRefusal(t, err, refusalWant{row: tc.row})
			if re.Rule == "" {
				t.Error("refusal must carry the sec. 10 rule ID")
			}
			if tc.row == 0 {
				requireRowOrToken(t, re)
			}
			if name == "duplicate Subagent" {
				requireRowIn(t, re, 1, 2) // the design does not say which occurrence is named
			}
		})
	}
}
