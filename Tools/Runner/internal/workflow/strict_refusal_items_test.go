package workflow_test

// Every strict refusal names the WorkflowFormat sec. 10 rule ID where one
// exists, the offending row or token, and the workflow id. Rows are named by
// their 1-based position. Presence is asserted, never exact wording.

import (
	"errors"
	"testing"

	"mosaic-run/internal/domain"
)

func TestParseStrict_Refusals_AllNameRuleRowOrTokenAndWorkflow(t *testing.T) {
	cases := []struct {
		name    string
		content string
		rule    string
	}{
		{"P1", strictRegion(strictPhases("PLANNING", "EXECUTION.Test")), "P1"},
		{"P2", strictRegion(strictPhases("PLANNING", "EXECUTION.[Stage].T st")), "P2"},
		{"P10", strictRegion(strictPhases("PLANNING", "EXECUTION.[Stage].[Stage]")), "P10"},
		{"P12", strictRegion(strictPhases("PLANNING", "execution")), "P12"},
		{"T3", strictEmptyRouting, "T3"},
		{"T1", strictBlocksTable[1:] + "\n" + strictRegion(strictPhases("PLANNING")), "T1"},
		{"P3", strictRegion(strictPhases("PLANNING"), "\n**Blocks:**\n\nProse.\n"), "P3"},
		{"P4", blocksRegion("| Block | Source |\n|---|---|\n| Stage | Plan.md |\n"), "P4"},
		{"P5", blocksRegion("| Block | Source | Section |\n|---|---|---|\n| HITL | Plan.md | Stages |\n"), "P5"},
		{"P6", groupsRegion("| Approach | Groups |\n|---|---|\n| TDD | Test |\n"), "P6"},
		{"P7", groupsRegion("| Block | Approach | Groups |\n|---|---|---|\n| Stage | | Test |\n"), "P7"},
		{"P8", groupsRegion("| Block | Approach | Groups |\n|---|---|---|\n| Stage | TDD | Test, Test |\n"), "P8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseStrict(tc.content)

			re := requireRefusal(t, err, refusalWant{rule: tc.rule})
			requireRowOrToken(t, re)
		})
	}
}

func TestParseStrict_RowRefusals_NameTheRoutingRowPosition(t *testing.T) {
	// The same defective cell at positions 1, 2 and 3: the refusal follows it.
	for pos := 1; pos <= 3; pos++ {
		phases := []string{"PLANNING", "RESEARCH", "DESIGN"}
		phases[pos-1] = "EXECUTION.Test"

		_, err := parseStrict(strictRegion(strictPhases(phases...)))

		requireRefusal(t, err, refusalWant{rule: "P1", row: pos, token: "EXECUTION.Test"})
	}
}

func TestParseStrict_Refusal_IsADomainRefusalErrorReachableThroughWrapping(t *testing.T) {
	_, err := parseStrict(strictRegion(strictPhases("EXECUTION.Test")))

	var re *domain.RefusalError
	if !errors.As(err, &re) {
		t.Fatalf("want *domain.RefusalError, got %T: %v", err, err)
	}
	if re.Rule != "P1" {
		t.Errorf("Rule: want P1, got %q", re.Rule)
	}
}
