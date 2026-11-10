package workflow_test

// Parsing of the approach table on the **Groups:** heading (Block | Approach |
// Groups) and the ApproachTable helper methods. The refusals of a malformed
// table are in workflow_approach_refusals_test.go.

import (
	"testing"

	"mosaic-run/internal/domain"
)

// approachRegion renders a region of one grouped Stage block holding the given
// groups (one row each), followed by the blocks table and a Groups table of
// the given data rows.
func approachRegion(groups []string, approachRows ...string) string {
	rows := make([]string, len(groups))
	for i, g := range groups {
		rows[i] = strictRow("EXECUTION.[Stage]."+g, "agent-"+string(rune('a'+i)))
	}
	return strictRegion(rows, strictBlocksTable, groupsTable(approachRows...))
}

// twoApproachTable is the two-group, two-approach table the tests below share.
func twoApproachTable(t *testing.T) domain.RoutingTable {
	t.Helper()
	return mustParseStrict(t, approachRegion([]string{"Test", "Implementation"},
		"| Stage | TDD | Test, Implementation |",
		"| Stage | Implementation-First | Implementation, Test |",
	))
}

func requireSequence(t *testing.T, table domain.RoutingTable, approach domain.Approach, want ...domain.GroupName) {
	t.Helper()
	groups, ok := table.ApproachTable.Sequence(approach)
	if !ok {
		t.Fatalf("Sequence(%q): want ok=true, got false", approach)
	}
	if len(groups) != len(want) {
		t.Fatalf("Sequence(%q): want %v, got %v", approach, want, groups)
	}
	for i, g := range groups {
		if g != want[i] {
			t.Errorf("Sequence(%q)[%d]: want %q, got %q", approach, i, want[i], g)
		}
	}
}

func TestParse_ApproachTable_Present_WhenHeadingAndTableExist(t *testing.T) {
	// A workflow with the **Groups:** heading and a valid table must report
	// ApproachTable.Present() == true after parsing.
	table := twoApproachTable(t)

	if !table.ApproachTable.Present() {
		t.Error("ApproachTable.Present(): want true when heading and table are present, got false")
	}
}

func TestParse_ApproachTable_Absent_WhenNoHeading(t *testing.T) {
	// A workflow without the **Groups:** heading must parse without error and
	// report ApproachTable.Present() == false.
	table := mustParseQuickFix(t)

	if table.ApproachTable.Present() {
		t.Error("ApproachTable.Present(): want false for a workflow with no heading, got true")
	}
}

func TestParse_ApproachTable_RowCount(t *testing.T) {
	table := twoApproachTable(t)

	if got := len(table.ApproachTable.Rows); got != 2 {
		t.Errorf("len(ApproachTable.Rows): want 2, got %d", got)
	}
}

func TestParse_ApproachTable_ApproachTokensAreVerbatimAndKeyedByBlock(t *testing.T) {
	table := twoApproachTable(t)
	if len(table.ApproachTable.Rows) != 2 {
		t.Fatalf("want 2 approach rows, got %d", len(table.ApproachTable.Rows))
	}

	for i, want := range []domain.Approach{"TDD", "Implementation-First"} {
		row := table.ApproachTable.Rows[i]
		if row.Approach != want {
			t.Errorf("ApproachTable.Rows[%d].Approach: want %q (verbatim, case-preserved), got %q", i, want, row.Approach)
		}
		if row.Block != "Stage" {
			t.Errorf("ApproachTable.Rows[%d].Block: want Stage, got %q", i, row.Block)
		}
	}
}

func TestParse_ApproachTable_GroupOrderPreserved_TDD(t *testing.T) {
	// Sequence("TDD") must return ["Test", "Implementation"] - the order from
	// the table cell, not alphabetical or any other derived order.
	requireSequence(t, twoApproachTable(t), "TDD", "Test", "Implementation")
}

func TestParse_ApproachTable_GroupOrderPreserved_ImplementationFirst(t *testing.T) {
	// The inverted order compared to TDD proves order is cell-driven.
	requireSequence(t, twoApproachTable(t), "Implementation-First", "Implementation", "Test")
}

func TestParse_ApproachTable_Sequence_UnknownApproach_ReturnsFalse(t *testing.T) {
	table := twoApproachTable(t)

	_, ok := table.ApproachTable.Sequence("Nonexistent")

	if ok {
		t.Error(`ApproachTable.Sequence("Nonexistent"): want ok=false for undeclared approach, got true`)
	}
}

func TestParse_ApproachTable_GroupOmittedFromSomeRows(t *testing.T) {
	// When an approach row lists only one group, Sequence must return a
	// single-element slice - absent groups are not padded or defaulted.
	table := mustParseStrict(t, approachRegion([]string{"Test", "Implementation"},
		"| Stage | Implementation-Only | Implementation |",
		"| Stage | Tests-Only | Test |",
	))

	requireSequence(t, table, "Implementation-Only", "Implementation")
}

func TestParse_ApproachTable_MoreThanTwoGroups(t *testing.T) {
	// An approach row listing three groups must decode all three in order.
	table := mustParseStrict(t, approachRegion([]string{"Alpha", "Beta", "Gamma"},
		"| Stage | AllGroups | Alpha, Beta, Gamma |",
	))

	requireSequence(t, table, "AllGroups", "Alpha", "Beta", "Gamma")
}

func TestParse_ApproachTable_RowOrderPreserved(t *testing.T) {
	// Rows in ApproachTable must appear in declaration order.
	table := mustParseStrict(t, approachRegion([]string{"Test", "Implementation"},
		"| Stage | Implementation-First | Implementation, Test |",
		"| Stage | TDD | Test, Implementation |",
	))

	got := table.ApproachTable.Approaches()
	want := []domain.Approach{"Implementation-First", "TDD"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Approaches(): want %v in declaration order, got %v", want, got)
	}
}

func TestParse_ApproachTable_NearMissHeading_NotTreatedAsReservedHeading(t *testing.T) {
	// A line that contains the heading text but is not an exact trimmed line
	// match must not activate approach-table parsing. The table that follows it
	// is then a table outside the reserved headings, which strict parsing
	// refuses (T1) instead of decoding it as an approach table.
	content := strictRegion(groupedStageRows(), strictBlocksTable,
		"\n**Groups:** and more text\n\n| Block | Approach | Groups |\n|---|---|---|\n| Stage | TDD | Test, Implementation |\n")

	_, err := parseStrict(content)

	re := requireRefusal(t, err, refusalWant{rule: "T1"})
	requireRowOrToken(t, re)
}

// ---- ApproachTable helper methods (domain.ApproachTable.Approaches and GroupNames) ----

func TestApproachTable_Approaches_ReturnsTokensInTableOrder(t *testing.T) {
	// Approaches() must return all declared approach tokens in declaration order.
	at := domain.ApproachTable{
		Rows: []domain.ApproachSequence{
			{Approach: "TDD", Groups: []domain.GroupName{"Test", "Implementation"}},
			{Approach: "Implementation-First", Groups: []domain.GroupName{"Implementation", "Test"}},
		},
	}

	got := at.Approaches()

	want := []domain.Approach{"TDD", "Implementation-First"}
	if len(got) != len(want) {
		t.Fatalf("Approaches(): want %v, got %v", want, got)
	}
	for i, a := range got {
		if a != want[i] {
			t.Errorf("Approaches()[%d]: want %q, got %q", i, want[i], a)
		}
	}
}

func TestApproachTable_GroupNames_DeduplicatesInFirstAppearanceOrder(t *testing.T) {
	// GroupNames() must return every group named anywhere in the table, deduplicated,
	// in order of first appearance. When the same group appears in multiple rows,
	// it must appear exactly once at its first-appearance position.
	at := domain.ApproachTable{
		Rows: []domain.ApproachSequence{
			{Approach: "TDD", Groups: []domain.GroupName{"Test", "Implementation"}},
			{Approach: "Implementation-First", Groups: []domain.GroupName{"Implementation", "Test"}},
		},
	}

	got := at.GroupNames()

	// "Test" first appears in row 0, position 0.
	// "Implementation" first appears in row 0, position 1.
	// Row 1 repeats both in reverse order - neither must appear again.
	want := []domain.GroupName{"Test", "Implementation"}
	if len(got) != len(want) {
		t.Fatalf("GroupNames(): want %v, got %v", want, got)
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("GroupNames()[%d]: want %q, got %q", i, want[i], g)
		}
	}
}
