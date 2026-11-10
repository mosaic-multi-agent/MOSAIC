package workflow_test

// Row parsing of the quick-fix shape (two PLANNING rows, one ungrouped Stage
// block row, one REVIEW row) by the strict parser.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/workflow"
)

func TestParse_QuickFix_RowCount(t *testing.T) {
	table := mustParseQuickFix(t)

	if len(table.Rows) != 4 {
		t.Errorf("want 4 rows, got %d", len(table.Rows))
	}
}

func TestParse_QuickFix_RowIndices_ZeroBasedConsecutive(t *testing.T) {
	// Indices must be 0, 1, 2, 3 in declaration order.
	table := mustParseQuickFix(t)

	for i, row := range table.Rows {
		if row.Index != i {
			t.Errorf("Rows[%d].Index: want %d, got %d", i, i, row.Index)
		}
	}
}

func TestParse_QuickFix_HITL_False_ForFalseCell(t *testing.T) {
	// Row 1 (plan-review) has FALSE in the HITL column -> HITL must be false.
	table := mustParseQuickFix(t)

	if table.Rows[1].HITL {
		t.Errorf("Rows[1].HITL: want false (FALSE plan-review), got true")
	}
}

func TestParse_QuickFix_HITL_True_ForTrueCell(t *testing.T) {
	// Row 0 (planner-tdd-soft) has TRUE in the HITL column -> HITL must be true.
	table := mustParseQuickFix(t)

	if !table.Rows[0].HITL {
		t.Errorf("Rows[0].HITL: want true (TRUE planner-tdd-soft), got false")
	}
}

func TestParse_QuickFix_PlainRow_NotInABlock(t *testing.T) {
	// PLANNING rows carry no block reference.
	table := mustParseQuickFix(t)

	if table.Rows[0].PhaseParsed.IsStaged {
		t.Errorf("Rows[0] (PLANNING): PhaseParsed.IsStaged must be false for a row outside every block")
	}
	if got := table.Rows[0].PhaseParsed.Blocks; len(got) != 0 {
		t.Errorf("Rows[0].PhaseParsed.Blocks: want none, got %v", got)
	}
}

func TestParse_QuickFix_PlainRow_PhaseName(t *testing.T) {
	// For a PLANNING row, PhaseParsed.Name must be "PLANNING".
	table := mustParseQuickFix(t)

	got := table.Rows[0].PhaseParsed.Name
	if got != "PLANNING" {
		t.Errorf("Rows[0].PhaseParsed.Name: want %q, got %q", "PLANNING", got)
	}
}

func TestParse_QuickFix_PlainRow_Phase_LiteralPreserved(t *testing.T) {
	// The Phase field must carry the literal text from the table cell.
	table := mustParseQuickFix(t)

	got := table.Rows[0].Phase
	if got != "PLANNING" {
		t.Errorf("Rows[0].Phase: want %q, got %q", "PLANNING", got)
	}
}

func TestParse_QuickFix_BlockRow_IsInTheStageBlock(t *testing.T) {
	// Row 2 (EXECUTION.[Stage]) must carry the block reference Stage.
	table := mustParseQuickFix(t)

	row := table.Rows[2]
	if !row.PhaseParsed.IsStaged {
		t.Errorf("Rows[2] (EXECUTION.[Stage]): PhaseParsed.IsStaged must be true")
	}
	if b, ok := row.Block(); !ok || b != "Stage" {
		t.Errorf("Rows[2].Block(): want (Stage, true), got (%q, %v)", b, ok)
	}
}

func TestParse_QuickFix_BlockRow_PhaseName(t *testing.T) {
	// The PhaseParsed.Name for an EXECUTION.[Stage] row must be "EXECUTION".
	table := mustParseQuickFix(t)

	got := table.Rows[2].PhaseParsed.Name
	if got != "EXECUTION" {
		t.Errorf("Rows[2].PhaseParsed.Name: want %q, got %q", "EXECUTION", got)
	}
}

func TestParse_QuickFix_BlockRow_Phase_LiteralPreserved(t *testing.T) {
	// The Phase field must preserve the literal "EXECUTION.[Stage]" string.
	table := mustParseQuickFix(t)

	got := table.Rows[2].Phase
	if got != "EXECUTION.[Stage]" {
		t.Errorf("Rows[2].Phase: want %q, got %q", "EXECUTION.[Stage]", got)
	}
}

func TestParse_QuickFix_GlobPattern_PreservedVerbatim_OutputArtifact(t *testing.T) {
	// Row 0 output: "Plan.md, Stage-*/Plan.md, Stage-*/PlanProgress.md".
	// The Stage-*/... glob patterns must survive verbatim in OutputArtifacts.
	table := mustParseQuickFix(t)

	artifacts := table.Rows[0].OutputArtifacts
	if !containsArtifact(artifacts, "Stage-*/Plan.md") {
		t.Errorf("Rows[0].OutputArtifacts: want %q to be present verbatim, got %v", "Stage-*/Plan.md", artifacts)
	}
}

func TestParse_QuickFix_FillIn_PreservedVerbatim_InputArtifact(t *testing.T) {
	// Row 2 input: "Stage-{Stage}/Plan.md, Stage-{Stage}/PlanProgress.md".
	// The Stage-{Stage}/... fill-in must survive verbatim.
	table := mustParseQuickFix(t)

	artifacts := table.Rows[2].InputArtifacts
	if !containsArtifact(artifacts, "Stage-{Stage}/Plan.md") {
		t.Errorf("Rows[2].InputArtifacts: want %q to be present verbatim, got %v", "Stage-{Stage}/Plan.md", artifacts)
	}
}

func TestParse_QuickFix_CommaSeparatedArtifacts_SplitAndTrimmed(t *testing.T) {
	// Row 0 output has three comma-separated paths -> OutputArtifacts must have 3 elements.
	table := mustParseQuickFix(t)

	got := len(table.Rows[0].OutputArtifacts)
	if got != 3 {
		t.Errorf("Rows[0].OutputArtifacts: want 3 elements, got %d: %v", got, table.Rows[0].OutputArtifacts)
	}
}

func TestParse_QuickFix_DashInput_ProducesEmptyArtifacts(t *testing.T) {
	// Row 0 has Input = "-" which means no input artifacts.
	// InputArtifacts must be empty (nil or zero-length slice).
	table := mustParseQuickFix(t)

	if len(table.Rows[0].InputArtifacts) != 0 {
		t.Errorf("Rows[0].InputArtifacts: want empty (Input is \"-\"), got %v", table.Rows[0].InputArtifacts)
	}
}

func TestParse_QuickFix_OnSuccess_NamedTargetResolvesToItsRow(t *testing.T) {
	// Row 0 On Success = "plan-review" -> a target entry at row index 1.
	table := mustParseQuickFix(t)

	got := table.Rows[0].Success
	if got.Kind != domain.SuccessTarget || got.Target.Index != 1 || got.Target.Cell != "plan-review" {
		t.Errorf("Rows[0].Success: want target plan-review at index 1, got %+v", got)
	}
}

func TestParse_QuickFix_OnSuccess_BlockReferenceIsABlockEntry(t *testing.T) {
	// Row 1 On Success = "[Stage]" -> a block-entry entry naming Stage.
	table := mustParseQuickFix(t)

	got := table.Rows[1].Success
	if got.Kind != domain.SuccessBlockEntry || got.Block != "Stage" {
		t.Errorf("Rows[1].Success: want block entry of Stage, got %+v", got)
	}
}

func TestParse_QuickFix_OnFindings_Dash_IsNone(t *testing.T) {
	// Row 0 On Findings = "-" -> no fix target.
	table := mustParseQuickFix(t)

	if got := table.Rows[0].Findings.Kind; got != domain.FindingsNone {
		t.Errorf("Rows[0].Findings.Kind: want none for dash, got %q", got)
	}
}

func TestParse_QuickFix_OnFindings_NamedTargetResolvesToItsRow(t *testing.T) {
	// Row 1 On Findings = "planner-tdd-soft" -> a target entry at row index 0.
	table := mustParseQuickFix(t)

	got := table.Rows[1].Findings
	if got.Kind != domain.FindingsTarget || got.Target.Index != 0 || got.Target.Cell != "planner-tdd-soft" {
		t.Errorf("Rows[1].Findings: want target planner-tdd-soft at index 0, got %+v", got)
	}
}

func TestParse_QuickFix_AgentIdentifier_PreservedVerbatim(t *testing.T) {
	// The Subagent column value must be stored verbatim in Agent.
	table := mustParseQuickFix(t)

	got := table.Rows[0].Agent
	if got != "planner-tdd-soft" {
		t.Errorf("Rows[0].Agent: want %q, got %q", "planner-tdd-soft", got)
	}
}

func TestParse_QuickFix_WorkflowInfo_CarriedIntoTable(t *testing.T) {
	// The WorkflowInfo passed to Parse must appear unchanged in RoutingTable.Info.
	info := domain.WorkflowInfo{ID: "quick-fix", Version: "4.0"}
	table, err := workflow.Parse([]byte(quickFixContent), info)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if table.Info.ID != info.ID {
		t.Errorf("RoutingTable.Info.ID: want %q, got %q", info.ID, table.Info.ID)
	}
	if table.Info.Version != info.Version {
		t.Errorf("RoutingTable.Info.Version: want %q, got %q", info.Version, table.Info.Version)
	}
}
