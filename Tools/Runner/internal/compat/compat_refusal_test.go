package compat_test

// The RefusalError contract of compat.Admit for a WorkflowFormat 2.16 refusal:
// the component is "compat", the resource names the workflow, and the reason is
// non-empty. The 1.x admission conditions are in compat_legacy_refusal_test.go.

import (
	"strings"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
)

// refusedTable parses a 2.16 table that Admit refuses (a row references block
// Stage, which no blocks table declares) and returns the refusal Admit gives.
func refusedTable(t *testing.T) *domain.RefusalError {
	t.Helper()
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
	}
	table := mustParse216(t, region(rows, false))

	_, err := compat.Admit(table, domain.ExecutionModeAuto)

	if err == nil {
		t.Fatal("Admit must refuse a block reference that no blocks table declares")
	}
	return asRefusalError(t, err)
}

func TestAdmit_RefusalError_ComponentIsCompat(t *testing.T) {
	re := refusedTable(t)

	if re.Component != "compat" {
		t.Errorf("RefusalError.Component: want %q, got %q", "compat", re.Component)
	}
}

func TestAdmit_RefusalError_ResourceNamesWorkflow(t *testing.T) {
	re := refusedTable(t)

	if re.Resource == "" {
		t.Fatal("RefusalError.Resource must name the workflow, not be empty")
	}
	if !strings.Contains(re.Resource, blocksWorkflowID) {
		t.Errorf("RefusalError.Resource must contain the workflow ID %q, got %q", blocksWorkflowID, re.Resource)
	}
}

func TestAdmit_RefusalError_ReasonIsNonEmpty(t *testing.T) {
	re := refusedTable(t)

	if re.Reason == "" {
		t.Error("RefusalError.Reason must not be empty")
	}
}
