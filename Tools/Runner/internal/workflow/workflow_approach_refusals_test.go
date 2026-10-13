package workflow_test

// Refusals of a malformed approach table on the **Groups:** heading: each case
// is a grouped region whose Groups table breaks one rule. Every refusal names
// the workflow, the rule ID and the offending row or token.

import "testing"

// approachRefusalRegion builds a grouped Stage region (blocks table valid)
// whose Groups section is the given raw text, heading included.
func approachRefusalRegion(groupsSection string) string {
	return strictRegion(groupedStageRows(), strictBlocksTable, groupsSection)
}

func TestParse_ApproachTable_Malformed_IsRefusedWithRuleAndOffender(t *testing.T) {
	header := groupsHeader + groupsColumns
	cases := []struct {
		name    string
		section string
		rule    string
	}{
		{"heading present but no table follows", groupsHeader + "This is not a table.\n", "P3"},
		{"missing Approach column", groupsHeader + "| Block | Groups |\n|---|---|\n| Stage | Test, Implementation |\n", "P6"},
		{"missing Groups column", groupsHeader + "| Block | Approach |\n|---|---|\n| Stage | TDD |\n", "P6"},
		{"zero data rows", header, "P6"},
		{"empty Approach cell", header + "| Stage | | Test |\n", "P7"},
		{"empty Groups cell", header + "| Stage | TDD | |\n", "P7"},
		{"empty token inside a Groups cell", header + "| Stage | TDD | Test, , Implementation |\n", "P7"},
		{"duplicate Approach token", header +
			"| Stage | TDD | Test, Implementation |\n| Stage | TDD | Implementation, Test |\n", "P8"},
		{"duplicate group within one Groups cell", header + "| Stage | TDD | Test, Implementation, Test |\n", "P8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseStrict(approachRefusalRegion(tc.section))

			re := requireRefusal(t, err, refusalWant{rule: tc.rule})
			requireRowOrToken(t, re)
		})
	}
}
