package workflow_test

// Refusals of a region with no routing table, a table missing one required
// column, and a table with no data rows, and the shape every such refusal has:
// component "workflow", a resource naming the workflow, and the offending
// column or rule.

import (
	"testing"
)

func TestParse_NoTable_ReturnsRefusalError(t *testing.T) {
	_, err := parseStrict("No routing table here.\n")

	re := requireRefusal(t, err, refusalWant{})
	if re.Reason == "" {
		t.Error("RefusalError.Reason must say that the region has no routing table")
	}
}

func TestParse_MissingRequiredColumn_IsRefusedForEachColumn(t *testing.T) {
	// Each required column removed from an otherwise valid table gives a
	// refusal that names the workflow, the rule and the missing column.
	for _, column := range cols7 {
		t.Run(column, func(t *testing.T) {
			_, err := parseStrict(missingColumnTable(column))

			re := requireRefusal(t, err, refusalWant{rule: "T2", token: column})
			requireRowOrToken(t, re)
		})
	}
}

func TestParse_EmptyTable_ZeroDataRows_ReturnsRefusalError(t *testing.T) {
	// A table with a valid header and separator but no data rows must be refused.
	_, err := parseStrict(strictEmptyRouting)

	requireRefusal(t, err, refusalWant{rule: "T3"})
}

func TestParse_RefusalError_ComponentIsWorkflowAndResourceNamesIt(t *testing.T) {
	// Every refusal from workflow.Parse must name "workflow" as the component and
	// identify the workflow so the user can find the problematic file; the
	// requireRefusal assertions check both.
	cases := map[string]string{
		"no table":       "No table.\n",
		"missing column": missingColumnTable("Phase"),
		"no data rows":   strictEmptyRouting,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStrict(content)

			requireRefusal(t, err, refusalWant{})
		})
	}
}
