package workflow_test

// Region rules T1 and T3, and the reserved-heading rule P3, under strict
// parsing.

import (
	"strings"
	"testing"
)

const strictEmptyRouting = "| Phase | Subagent | HITL | On Success | On Findings | Input | Output |\n" +
	"|-------|----------|:----:|------------|-------------|-------|--------|\n"

func TestParseStrict_T1_TableBeforeRoutingTable_IsRefused(t *testing.T) {
	cases := map[string]string{
		"blocks table first": strictBlocksTable[1:] + "\n" + strictRegion(strictPhases("EXECUTION.[Stage]")),
		"unrelated table first": "| Term | Meaning |\n|---|---|\n| a | b |\n\n" +
			strictRegion(strictPhases("PLANNING")),
		"approach table first": strictGroupsTable[1:] + "\n" + strictRegion(strictPhases("EXECUTION.[Stage].Test")),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			re := requireRefusal(t, err, refusalWant{rule: "T1"})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_T1_FurtherTableOutsideReservedHeadings_IsRefused(t *testing.T) {
	extraTable := "| Note | Detail |\n|---|---|\n| x | y |\n"
	cases := map[string]string{
		"directly after routing table": strictRegion(strictPhases("PLANNING"), "\n", extraTable),
		"under notes heading":          strictRegion(strictPhases("PLANNING"), "\n**Notes:**\n\n", extraTable),
		"after blocks table":           strictRegion(strictPhases("EXECUTION.[Stage]"), strictBlocksTable, "\n", extraTable),
		"after groups table": strictRegion(groupedStageRows(), strictBlocksTable, strictGroupsTable,
			"\n**Notes:**\n\n", extraTable),
		"under the 1.x heading": strictRegion(strictPhases("PLANNING"),
			"\n**Execution Groups:**\n\n| Approach | Groups |\n|---|---|\n| TDD | Test |\n"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			re := requireRefusal(t, err, refusalWant{})
			if name != "under the 1.x heading" && re.Rule != "T1" {
				t.Errorf("Rule: want T1, got %q", re.Rule)
			}
		})
	}
}

func TestParseStrict_T1_FirstTableWithSomeRequiredColumnsIsNotAT1Matter(t *testing.T) {
	// A first table that has a Phase column but lacks Subagent is a routing
	// table with a missing column (T2), not "another table before it".
	content := "| Phase | HITL | Input | Output |\n|---|---|---|---|\n| PLANNING | TRUE | - | Plan.md |\n"

	_, err := parseStrict(content)

	re := requireRefusal(t, err, refusalWant{})
	if re.Rule == "T1" {
		t.Error("Rule: a first table with only some required columns must not be reported as T1")
	}
}

func TestParseStrict_T3_RoutingTableWithoutDataRows_IsRefusedNamingTheRule(t *testing.T) {
	_, err := parseStrict(strictEmptyRouting)

	re := requireRefusal(t, err, refusalWant{rule: "T3"})
	if !strings.Contains(re.Reason, "no data rows") {
		t.Errorf("Reason must keep saying the routing table has no data rows, got %q", re.Reason)
	}
}

func TestParseStrict_T3_EmptyRoutingTableBeforeReservedTables_IsRefusedAsT3(t *testing.T) {
	_, err := parseStrict(strictEmptyRouting + strictBlocksTable)

	requireRefusal(t, err, refusalWant{rule: "T3"})
}

func TestParseStrict_P3_ReservedHeadingTwice_IsRefused(t *testing.T) {
	cases := map[string]string{
		"blocks heading twice": strictRegion(strictPhases("EXECUTION.[Stage]"), strictBlocksTable, strictBlocksTable),
		"groups heading twice": strictRegion(groupedStageRows(), strictBlocksTable, strictGroupsTable, strictGroupsTable),
		"second heading has no table": strictRegion(strictPhases("EXECUTION.[Stage]"), strictBlocksTable,
			"\n**Blocks:**\n\nNothing here.\n"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			re := requireRefusal(t, err, refusalWant{rule: "P3"})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_P3_ReservedHeadingWithoutTable_IsRefused(t *testing.T) {
	cases := map[string]string{
		"blocks heading, prose follows": strictRegion(strictPhases("EXECUTION.[Stage]"),
			"\n**Blocks:**\n\nThis is not a table.\n"),
		"blocks heading at end of region": strictRegion(strictPhases("EXECUTION.[Stage]"), "\n**Blocks:**\n"),
		"groups heading, prose follows": strictRegion(groupedStageRows(), strictBlocksTable,
			"\n**Groups:**\n\nThis is not a table.\n"),
		"groups heading then notes heading": strictRegion(groupedStageRows(), strictBlocksTable,
			"\n**Groups:**\n\n**Notes:**\n- x\n"),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			re := requireRefusal(t, err, refusalWant{rule: "P3"})
			requireRowOrToken(t, re)
		})
	}
}
