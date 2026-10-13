package compat_test

// Structural refusals of the WorkflowFormat 2.16 block structure (rules A1-A6
// and nested blocks), each asserted in every run mode for the rule ID, the
// 1-based row or token and the workflow id. A grouped workflow is never
// admitted as a bare block.

import (
	"testing"

	"mosaic-run/internal/domain"
)

const noRule = ""

func twoGroupApproach() string {
	return groupsSection("| Stage | TDD | Test, Implementation |")
}

func TestAdmitBlocks_A1_GroupRowsNotContiguous_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-fixer"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |") + twoGroupApproach()
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A1"}, row: 4, tokens: []string{"Test"}})
}

func TestAdmitBlocks_A2_BlockReferenceNamesUndeclaredBlock_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Audit.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "reviewer"},
		{phase: "REVIEW.[Audit]", agent: "auditor"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Audit | Audit.md | Items |")))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A2"}, row: 2, tokens: []string{"Stage"}})
}

func TestAdmitBlocks_A2_BlockReferenceWithNoBlocksTable_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
	}
	table := mustParse216(t, region(rows, false))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A2"}, row: 2, tokens: []string{"Stage"}})
}

func TestAdmitBlocks_A2_DeclaredBlockWithNoRows_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "reviewer"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |")
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A2"}, tokens: []string{"Audit"}})
}

func TestAdmitBlocks_A3_GroupedBlockWithAnUngroupedOwnRow_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "finisher"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |") + twoGroupApproach()
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A3"}, row: 4})
}

func TestAdmitBlocks_A3_UngroupedOwnRowFirst_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "finisher"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |") + twoGroupApproach()
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A3"}, row: 2})
}

func TestAdmitBlocks_A2_BlockReferenceDiffersInCaseFromDeclaredBlock_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[stage]", agent: "implementer"},
		{phase: "EXECUTION.[stage]", agent: "reviewer"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A2"}, tokens: []string{"stage", "Stage"}})
}

func TestAdmitBlocks_A4_GroupedBlockWithoutApproachRow_IsRefusedNotAdmittedAsBare(t *testing.T) {
	// The rows carry groups but the workflow declares no approach table: the
	// block must not be admitted as a bare (ungrouped) block.
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A4"}, tokens: []string{"Stage"}})
}

func TestAdmitBlocks_A4_GroupedBlockWithApproachRowsOfAnotherBlockOnly_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "REVIEW.[Audit].Scan", agent: "auditor"},
		{phase: "REVIEW.[Audit].Fix", agent: "audit-fixer"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |") +
		groupsSection("| Audit | Full | Scan, Fix |")
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A4"}, tokens: []string{"Stage"}})
}

func TestAdmitBlocks_A4_ApproachRowNamesUngroupedBlock_IsRefused(t *testing.T) {
	// An approach row for an ungrouped block breaks A4 (the block is not
	// grouped) and necessarily A5 as well (it names groups the block lacks);
	// either rule ID names the defect.
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "REVIEW.[Audit]", agent: "auditor"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |") + groupsSection(
		"| Stage | TDD | Test, Implementation |",
		"| Audit | Full | Scan |",
	)
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A4", "A5"}})
}

func TestAdmitBlocks_A5_ApproachRowNamesGroupTheBlockLacks_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |") +
		groupsSection("| Stage | TDD | Test, Implementation, Ghost |")
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A5"}, tokens: []string{"Ghost"}})
}

func TestAdmitBlocks_A5_GroupNamedByNoApproachRow_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
		{phase: "EXECUTION.[Stage].Cleanup", agent: "cleaner"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |") + twoGroupApproach()
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A5"}, tokens: []string{"Cleanup"}})
}

func TestAdmitBlocks_A5_GroupNameIsCaseSensitive_IsRefused(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].test", agent: "test-writer"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "implementer"},
	}
	tables := blocksSection("| Stage | Plan.md | Stages |") + twoGroupApproach()
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A5"}, tokens: []string{"test", "Test"}})
}

func TestAdmitBlocks_A6_BlockRowsNotContiguous_IsRefused(t *testing.T) {
	cases := []struct {
		name string
		rows []wfRow
		row  int
	}{
		{
			name: "run-scope row between two rows of one block",
			rows: []wfRow{
				{phase: "PLANNING", agent: "planner", output: "Plan.md"},
				{phase: "EXECUTION.[Stage]", agent: "implementer"},
				{phase: "INTEGRATION", agent: "integrator"},
				{phase: "EXECUTION.[Stage]", agent: "reviewer"},
			},
			row: 4,
		},
		{
			name: "rows of two blocks interleaved",
			rows: []wfRow{
				{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
				{phase: "EXECUTION.[Stage]", agent: "implementer"},
				{phase: "REVIEW.[Audit]", agent: "auditor"},
				{phase: "EXECUTION.[Stage]", agent: "reviewer"},
				{phase: "REVIEW.[Audit]", agent: "audit-fixer"},
			},
			row: 4,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tables := blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |")
			table := mustParse216(t, region(tc.rows, false, tables))

			requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{"A6"}, row: tc.row, tokens: []string{"Stage"}})
		})
	}
}

func TestAdmitBlocks_NestedBlocks_RowWithTwoBlockReferencesIsRefusedByName(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Domains.md, Plan.md"},
		{phase: "EXECUTION.[Domain].[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Domain].[Stage]", agent: "reviewer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	tables := blocksSection("| Domain | Domains.md | Domains |", "| Stage | Plan.md | Stages |")
	table := mustParse216(t, region(rows, false, tables))

	// Nested blocks have no 2.16 rule ID: the refusal names the row.
	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{noRule}, row: 2})
}

func TestAdmitBlocks_NestedBlocks_ParsedSourceWithFillInIsRefusedByName(t *testing.T) {
	// A nested row whose inner block declares a Source with the enclosing
	// block's fill-in: the shape the parser produces for nested blocks.
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Domains.md"},
		{phase: "EXECUTION.[Domain].[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Domain].[Stage]", agent: "reviewer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	tables := blocksSection("| Domain | Domains.md | Domains |", "| Stage | {Domain}/Plan.md | Stages |")
	table := mustParse216(t, region(rows, false, tables))

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{noRule}, row: 2})
}

func TestAdmitBlocks_NestedBlocks_HandBuiltSourceWithFillInIsRefusedByName(t *testing.T) {
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "reviewer"},
	}
	table := mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))
	table.Blocks = append([]domain.BlockDecl(nil), table.Blocks...)
	table.Blocks[0].Source = "{Domain}/Plan.md"

	requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{noRule}, tokens: []string{"{Domain}/Plan.md"}})
}

// ---- Tables that no parse could produce (non-legacy, hand-modified) ----

// rowMutation changes the row at index idx of a copied table.
type rowMutation struct {
	idx int
	fn  func(r *domain.RoutingRow)
}

// fourRowTable parses a 2.16 table of four rows: planning, two block rows,
// review. Row indexes 0..3; a RowRef.Index of 4 is one past the end.
func fourRowTable(t *testing.T) domain.RoutingTable {
	t.Helper()
	rows := []wfRow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "implementer"},
		{phase: "EXECUTION.[Stage]", agent: "reviewer"},
		{phase: "REVIEW", agent: "test-runner"},
	}
	return mustParse216(t, region(rows, false, blocksSection("| Stage | Plan.md | Stages |")))
}

func successTo(idx int) domain.SuccessEntry {
	return domain.SuccessEntry{Kind: domain.SuccessTarget, Target: domain.RowRef{Cell: "x", Index: idx}}
}

func findingsTo(idx int) domain.FindingsEntry {
	return domain.FindingsEntry{Kind: domain.FindingsTarget, Target: domain.RowRef{Cell: "x", Index: idx}}
}

func forkTo(idx ...int) domain.SuccessEntry {
	refs := make([]domain.RowRef, len(idx))
	for i, n := range idx {
		refs[i] = domain.RowRef{Cell: "a", Index: n}
	}
	return domain.SuccessEntry{Kind: domain.SuccessFork, Fork: refs}
}

func TestAdmitBlocks_TableNoParseCouldProduce_IsRefusedNamingTheRow(t *testing.T) {
	base := fourRowTable(t)
	cases := []struct {
		name    string
		muts    []rowMutation
		wantRow int // 1-based row of the first offending row in table order
	}{
		{"zero success kind", []rowMutation{{1, func(r *domain.RoutingRow) { r.Success = domain.SuccessEntry{} }}}, 2},
		{"zero findings kind", []rowMutation{{2, func(r *domain.RoutingRow) { r.Findings = domain.FindingsEntry{} }}}, 3},
		{"IsStaged disagrees with Blocks", []rowMutation{{2, func(r *domain.RoutingRow) { r.PhaseParsed.IsStaged = false }}}, 3},
		{"IsStaged set on a row with no block", []rowMutation{{0, func(r *domain.RoutingRow) { r.PhaseParsed.IsStaged = true }}}, 1},
		{"success target out of range", []rowMutation{{1, func(r *domain.RoutingRow) { r.Success = successTo(99) }}}, 2},
		{"success target one past the last row", []rowMutation{{2, func(r *domain.RoutingRow) { r.Success = successTo(4) }}}, 3},
		{"findings target out of range", []rowMutation{{3, func(r *domain.RoutingRow) { r.Findings = findingsTo(50) }}}, 4},
		{"findings target one past the last row", []rowMutation{{0, func(r *domain.RoutingRow) { r.Findings = findingsTo(4) }}}, 1},
		{"fork element out of range", []rowMutation{{1, func(r *domain.RoutingRow) { r.Success = forkTo(2, 50) }}}, 2},
		{"fork element one past the last row", []rowMutation{{2, func(r *domain.RoutingRow) { r.Success = forkTo(1, 4) }}}, 3},
		{"waits for negative", []rowMutation{{3, func(r *domain.RoutingRow) { r.WaitsFor = []domain.RowRef{{Cell: "x", Index: -1}} }}}, 4},
		{"waits for one past the last row", []rowMutation{{1, func(r *domain.RoutingRow) { r.WaitsFor = []domain.RowRef{{Cell: "x", Index: 4}} }}}, 2},
		{"block entry names undeclared block", []rowMutation{{2, func(r *domain.RoutingRow) {
			r.Success = domain.SuccessEntry{Kind: domain.SuccessBlockEntry, Block: "Ghost"}
		}}}, 3},
		{"two defective rows: the earlier one is reported", []rowMutation{
			{3, func(r *domain.RoutingRow) { r.Success = domain.SuccessEntry{} }},
			{1, func(r *domain.RoutingRow) { r.Findings = domain.FindingsEntry{} }},
		}, 2},
		{"two defective rows of different kinds: the earlier one is reported", []rowMutation{
			{2, func(r *domain.RoutingRow) { r.Success = successTo(99) }},
			{0, func(r *domain.RoutingRow) { r.PhaseParsed.IsStaged = true }},
		}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := base
			table.Rows = append([]domain.RoutingRow(nil), base.Rows...)
			for _, m := range tc.muts {
				m.fn(&table.Rows[m.idx])
			}

			requireRefusedInEveryMode(t, table, wantRefusal{rules: []string{noRule}, row: tc.wantRow})
		})
	}
}
