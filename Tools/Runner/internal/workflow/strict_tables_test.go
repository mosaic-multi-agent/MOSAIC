package workflow_test

// Strict parsing of the **Blocks:** table (Block | Source | Section) and the
// **Groups:** approach table (Block | Approach | Groups) that follow the
// routing table inside the region.

import (
	"reflect"
	"testing"

	"mosaic-run/internal/domain"
)

// groupedStageRows is the routing rows of a grouped Stage block.
func groupedStageRows() []string {
	return strictPhases("PLANNING", "EXECUTION.[Stage].Test", "EXECUTION.[Stage].Implementation", "REVIEW")
}

func TestParseStrict_GroupedTable_ParsesRowsBlocksGroupsAndApproachRows(t *testing.T) {
	// Arrange: a complete 2.16 grouped table.
	content := strictRegion(groupedStageRows(),
		blocksTable("| Stage | Plan.md | Stages |"),
		groupsTable(
			"| Stage | TDD | Test, Implementation |",
			"| Stage | Implementation-First | Implementation, Test |",
		),
		"\n**Notes:**\n- nothing\n",
	)

	// Act
	table := mustParseStrict(t, content)

	// Assert: rows keep block and group.
	wantGroups := []domain.GroupName{"", "Test", "Implementation", ""}
	for i, row := range table.Rows {
		if row.PhaseParsed.Group != wantGroups[i] {
			t.Errorf("Rows[%d].Group: want %q, got %q", i, wantGroups[i], row.PhaseParsed.Group)
		}
	}
	// Assert: declared blocks.
	wantBlocks := []domain.BlockDecl{{Name: "Stage", Source: "Plan.md", Section: "Stages"}}
	if !reflect.DeepEqual(table.Blocks, wantBlocks) {
		t.Errorf("Blocks: want %+v, got %+v", wantBlocks, table.Blocks)
	}
	// Assert: approach rows keyed by block, in table order, groups ordered.
	wantApproach := []domain.ApproachSequence{
		{Block: "Stage", Approach: "TDD", Groups: []domain.GroupName{"Test", "Implementation"}},
		{Block: "Stage", Approach: "Implementation-First", Groups: []domain.GroupName{"Implementation", "Test"}},
	}
	if !reflect.DeepEqual(table.ApproachTable.Rows, wantApproach) {
		t.Errorf("ApproachTable.Rows: want %+v, got %+v", wantApproach, table.ApproachTable.Rows)
	}
}

func TestParseStrict_BlocksTable_DeclaresSourceAndSectionVerbatim(t *testing.T) {
	cases := []struct {
		name string
		rows []string
		want []domain.BlockDecl
	}{
		{
			name: "non-Plan.md source",
			rows: []string{"| Stage | AuditPlan.md | Findings |"},
			want: []domain.BlockDecl{{Name: "Stage", Source: "AuditPlan.md", Section: "Findings"}},
		},
		{
			name: "several blocks keep declaration order",
			rows: []string{
				"| Product | Requirements.md | Products |",
				"| Stage | Plan.md | Stages |",
				"| Correction | KBProgress.md | Corrections |",
			},
			want: []domain.BlockDecl{
				{Name: "Product", Source: "Requirements.md", Section: "Products"},
				{Name: "Stage", Source: "Plan.md", Section: "Stages"},
				{Name: "Correction", Source: "KBProgress.md", Section: "Corrections"},
			},
		},
		{
			name: "whitespace around cells is trimmed",
			rows: []string{"|   Stage   |  Plan.md  |   Stages   |"},
			want: []domain.BlockDecl{{Name: "Stage", Source: "Plan.md", Section: "Stages"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := strictRegion(strictPhases("EXECUTION.[Stage]"), blocksTable(tc.rows...))

			table := mustParseStrict(t, content)

			if !reflect.DeepEqual(table.Blocks, tc.want) {
				t.Errorf("Blocks: want %+v, got %+v", tc.want, table.Blocks)
			}
		})
	}
}

func TestParseStrict_BlocksTable_InnerBlockMayFillInEnclosingBlock(t *testing.T) {
	rows := strictPhases("PLANNING.[Domain]", "EXECUTION.[Domain].[Stage].Test", "EXECUTION.[Domain].[Stage].Implementation")
	content := strictRegion(rows,
		blocksTable("| Domain | SystemDesign.md | Domains |", "| Stage | {Domain}/Plan.md | Stages |"),
		groupsTable("| Stage | TDD | Test, Implementation |"),
	)

	table := mustParseStrict(t, content)

	want := []domain.BlockDecl{
		{Name: "Domain", Source: "SystemDesign.md", Section: "Domains"},
		{Name: "Stage", Source: "{Domain}/Plan.md", Section: "Stages"},
	}
	if !reflect.DeepEqual(table.Blocks, want) {
		t.Errorf("Blocks: want %+v, got %+v", want, table.Blocks)
	}
	if got := blockNames(table.Rows[1].PhaseParsed.Blocks); !equalStrings(got, []string{"Domain", "Stage"}) {
		t.Errorf("nested row Blocks: want [Domain Stage], got %v", got)
	}
}

func TestParseStrict_GroupsTable_SameApproachTokenForDifferentBlocksIsKept(t *testing.T) {
	rows := strictPhases("EXECUTION.[Stage].Test", "EXECUTION.[Stage].Impl", "REVIEW.[Audit].Scan", "REVIEW.[Audit].Fix")
	content := strictRegion(rows,
		blocksTable("| Stage | Plan.md | Stages |", "| Audit | AuditPlan.md | Items |"),
		groupsTable(
			"| Stage | Full | Test, Impl |",
			"| Audit | Full | Fix, Scan |",
		),
	)

	table := mustParseStrict(t, content)

	want := []domain.ApproachSequence{
		{Block: "Stage", Approach: "Full", Groups: []domain.GroupName{"Test", "Impl"}},
		{Block: "Audit", Approach: "Full", Groups: []domain.GroupName{"Fix", "Scan"}},
	}
	if !reflect.DeepEqual(table.ApproachTable.Rows, want) {
		t.Errorf("ApproachTable.Rows: want %+v, got %+v", want, table.ApproachTable.Rows)
	}
}

func TestParseStrict_GroupsTable_GroupsOrderIsTheCellOrderWithTrimmedTokens(t *testing.T) {
	content := strictRegion(groupedStageRows(),
		blocksTable("| Stage | Plan.md | Stages |"),
		groupsTable("| Stage | Odd-One | Implementation ,Test |"),
	)

	table := mustParseStrict(t, content)

	if len(table.ApproachTable.Rows) != 1 {
		t.Fatalf("want 1 approach row, got %d", len(table.ApproachTable.Rows))
	}
	got := table.ApproachTable.Rows[0].Groups
	want := []domain.GroupName{"Implementation", "Test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Groups: want %v in cell order, got %v", want, got)
	}
}

func TestParseStrict_ReservedHeadings_AreFoundByHeadingNotByPosition(t *testing.T) {
	// The Groups table may precede the Blocks table, and the heading may be
	// followed by its table without a blank line.
	groups := "\n**Groups:**\n| Block | Approach | Groups |\n|---|---|---|\n| Stage | TDD | Test, Implementation |\n"
	blocks := "\n**Blocks:**\n| Block | Source | Section |\n|---|---|---|\n| Stage | Plan.md | Stages |\n"

	table := mustParseStrict(t, strictRegion(groupedStageRows(), groups, blocks))

	if len(table.Blocks) != 1 || table.Blocks[0].Name != "Stage" {
		t.Errorf("Blocks: want the declared Stage block, got %+v", table.Blocks)
	}
	if len(table.ApproachTable.Rows) != 1 || table.ApproachTable.Rows[0].Block != "Stage" {
		t.Errorf("ApproachTable.Rows: want one Stage row, got %+v", table.ApproachTable.Rows)
	}
}

func TestParseStrict_NoBlocksNoGroups_LeavesBothEmpty(t *testing.T) {
	table := mustParseStrict(t, strictRegion(strictPhases("PLANNING", "REVIEW")))

	if len(table.Blocks) != 0 {
		t.Errorf("Blocks: want none declared, got %+v", table.Blocks)
	}
	if table.ApproachTable.Present() {
		t.Errorf("ApproachTable: want absent, got %+v", table.ApproachTable.Rows)
	}
}
