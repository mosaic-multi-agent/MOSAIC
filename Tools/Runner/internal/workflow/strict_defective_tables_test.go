package workflow_test

// A 2.16 table with a defect in one cell or table is refused by strict parsing
// with the refusal items of that defect (rule, 1-based row, token) and the
// workflow id, whatever else the region holds.

import "testing"

func TestParseStrict_CellLevelDefect_IsRefusedWithItsItems(t *testing.T) {
	cases := map[string]struct {
		content string
		rule    string
		row     int
		token   string
	}{
		"target matching no Subagent cell": {
			content: table7(plain7("PLANNING", "agent-a"), row7("REVIEW", "agent-b", "FALSE", "agent-z", "-")),
			rule:    "T9", row: 2, token: "agent-z",
		},
		"prose On Findings": {
			content: table7(plain7("TESTING", "mosaictest-scripted"),
				row7("REVIEW", "test-runner", "FALSE", "COMPLETE", "mosaictest-scripted (re-check)")),
			rule: "T6", row: 2, token: "mosaictest-scripted (re-check)",
		},
		"duplicated Subagent cell": {
			content: table7(plain7("PLANNING", "agent-a"), plain7("REVIEW", "agent-a")),
			rule:    "T8",
		},
		"duplicated Subagent cell among typed targets": {
			content: table7(
				row7("PLANNING", "agent-a", "FALSE", "agent-b", "-"),
				row7("REVIEW", "agent-b", "FALSE", "agent-a", "-"),
				row7("REVIEW", "agent-a", "FALSE", "COMPLETE", "-"),
			),
			rule: "T8",
		},
		"target matching no Subagent cell next to a Blocks table": {
			content: table7(
				row7("PLANNING", "agent-a", "FALSE", "[Stage]", "-"),
				row7("EXECUTION.[Stage]", "agent-b", "FALSE", "agent-z", "-"),
			) + strictBlocksTable,
			rule: "T9", row: 2, token: "agent-z",
		},
		"duplicated Subagent cell next to a Blocks table": {
			content: table7(
				row7("PLANNING", "agent-a", "FALSE", "[Stage]", "-"),
				row7("EXECUTION.[Stage]", "agent-a", "FALSE", "COMPLETE", "-"),
			) + strictBlocksTable,
			rule: "T8",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(tc.content)

			re := requireRefusal(t, err, refusalWant{rule: tc.rule, row: tc.row, token: tc.token})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_DefectiveTable_IsRefusedWithItsItems(t *testing.T) {
	cases := map[string]struct {
		content string
		rule    string
		row     int
		token   string
	}{
		"malformed group token": {
			content: strictRegion(
				[]string{
					"| PLANNING | agent-a | FALSE | NEXT | - | - | out.md |\n",
					"| EXECUTION.[Stage].T st | agent-b | FALSE | COMPLETE | - | - | out.md |\n",
				}, strictBlocksTable),
			rule: "P2", row: 2, token: "EXECUTION.[Stage].T st",
		},
		"groups table with a missing cell": {
			content: strictRegion(
				[]string{
					"| PLANNING | agent-a | FALSE | NEXT | - | - | out.md |\n",
					"| EXECUTION.[Stage].Test | agent-b | FALSE | COMPLETE | - | - | out.md |\n",
				}, strictBlocksTable,
				"\n**Groups:**\n\n| Block | Approach | Groups |\n|---|---|---|\n| Stage | | Test |\n"),
			rule: "P7",
		},
		"group token with a slash": {
			content: strictRegion(
				[]string{
					"| PLANNING | agent-a | FALSE | NEXT | - | - | out.md |\n",
					"| EXECUTION.[Stage].Te/st | agent-b | FALSE | COMPLETE | - | - | out.md |\n",
				}, strictBlocksTable),
			rule: "P2", row: 2, token: "Te/st",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(tc.content)

			re := requireRefusal(t, err, refusalWant{rule: tc.rule, row: tc.row, token: tc.token})
			requireRowOrToken(t, re)
		})
	}
}
