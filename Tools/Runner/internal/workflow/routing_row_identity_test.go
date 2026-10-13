package workflow_test

// Row identity accessors on parsed rows: Block() and BareAgentID() give a
// parsed row exactly the values its fields hold. The transitional 1.x path's
// counterpart is in workflow_legacy_parse_test.go.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// checkRowIdentityMatchesFields asserts the accessors of every row of table
// give what the row's fields hold.
func checkRowIdentityMatchesFields(t *testing.T, table domain.RoutingTable) {
	t.Helper()
	for _, row := range table.Rows {
		wantBlock, wantOK := domain.BlockName(""), false
		if len(row.PhaseParsed.Blocks) > 0 {
			wantBlock, wantOK = row.PhaseParsed.Blocks[0], true
		}
		gotBlock, gotOK := row.Block()
		if gotBlock != wantBlock || gotOK != wantOK {
			t.Errorf("row %d Block(): want (%q, %v), got (%q, %v)", row.Index, wantBlock, wantOK, gotBlock, gotOK)
		}
		if got := row.BareAgentID(); got != row.AgentID {
			t.Errorf("row %d BareAgentID(): want field AgentID %q, got %q", row.Index, row.AgentID, got)
		}
		if row.AgentID == "" {
			t.Errorf("row %d: parser left AgentID empty", row.Index)
		}
	}
}

func TestRowIdentity_StrictParse_BlockAndNonBlockRows(t *testing.T) {
	// Arrange
	content := strictRegion(
		[]string{
			strictRow("PLANNING", "planner"),
			strictRow("EXECUTION.[Stage]", "build-review(tests)"),
		},
		strictBlocksTable,
	)

	// Act
	table := mustParseStrict(t, content)

	// Assert
	if len(table.Rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(table.Rows))
	}
	checkRowIdentityMatchesFields(t, table)
	if b, ok := table.Rows[0].Block(); ok || b != "" {
		t.Errorf("non-block row Block(): want (\"\", false), got (%q, %v)", b, ok)
	}
	if b, ok := table.Rows[1].Block(); !ok || b != "Stage" {
		t.Errorf("block row Block(): want (Stage, true), got (%q, %v)", b, ok)
	}
	if got := table.Rows[1].BareAgentID(); got != "build-review" {
		t.Errorf("qualified row BareAgentID(): want build-review, got %q", got)
	}
	if got := table.Rows[0].BareAgentID(); got != "planner" {
		t.Errorf("plain row BareAgentID(): want planner, got %q", got)
	}
}
