package artifact_test

// Tests that table headers and cell counts that do not match the expected
// layout, and stray lines in a table with no data rows, are reported as damage
// with the table's section and the line of the offending header or row.

import (
	"strings"
	"testing"
)

// headerCase renames one required column in a table header.
type headerCase struct {
	name    string
	section string
	oldHdr  string // header line as the compliant document renders it
	newHdr  string // header line with a required column missing or renamed
}

var headerCases = []headerCase{
	{
		name: "ExecutionLog header without the Agent column", section: "ExecutionLog",
		oldHdr: "| Seq | Agent | Phase |",
		newHdr: "| Seq | Who | Phase |",
	},
	{
		name: "ExecutionLog header without the Seq column", section: "ExecutionLog",
		oldHdr: "| Seq | Agent | Phase |",
		newHdr: "| Number | Agent | Phase |",
	},
	{
		name: "Artifacts header with wrong columns", section: "Artifacts",
		oldHdr: "| Artifact | Created In | Created By |",
		newHdr: "| Artifact | Made In | Made By |",
	},
	{
		name: "WorkflowNotes header with wrong columns", section: "WorkflowNotes",
		oldHdr: "| Seq | Note |",
		newHdr: "| Seq | Remark |",
	},
}

func TestParse_HeaderNotMatchingTheExpectedColumns_IsDamageAtTheHeaderLine(t *testing.T) {
	for _, c := range headerCases {
		t.Run(c.name, func(t *testing.T) {
			doc := strings.Replace(newStrictDoc().String(), c.oldHdr, c.newHdr, 1)
			if !strings.Contains(doc, c.newHdr) {
				t.Fatal("test setup: header replacement did not apply")
			}

			damage := parseDamaged(t, doc)

			requireDamageProblem(t, damage, c.section, lineOf(t, doc, c.newHdr))
		})
	}
}

var extraCellCases = []damageCase{
	{
		name:    "ExecutionLog row with more cells than the header",
		mutate:  func(d *strictDoc) { d.logRows = append(d.logRows, rawLogRow("3", "worker#3", "extra-cell-marker")+" extra |") },
		section: "ExecutionLog", needle: "extra-cell-marker",
	},
	{
		name:    "Artifacts row with more cells than the header",
		mutate:  func(d *strictDoc) { d.registry = append(d.registry, "| Out.md | PLANNING | worker#1 | extra-cell-marker |") },
		section: "Artifacts", needle: "extra-cell-marker",
	},
	{
		name:    "WorkflowNotes row with more cells than the header",
		mutate:  func(d *strictDoc) { d.notes = append(d.notes, "| 2 | first | extra-cell-marker |") },
		section: "WorkflowNotes", needle: "extra-cell-marker",
	},
}

func TestParse_RowWithMoreCellsThanTheHeader_IsDamageAtTheRow(t *testing.T) {
	for _, c := range extraCellCases {
		t.Run(c.name, func(t *testing.T) {
			d := newStrictDoc()
			c.mutate(&d)
			doc := d.String()

			damage := parseDamaged(t, doc)

			requireDamageProblem(t, damage, c.section, c.wantLine(t, doc))
		})
	}
}

// A stray line directly after the header separator: the table has no data rows,
// so the line cannot be taken for a row that follows other rows.
func TestParse_StrayLineInATableWithNoRows_IsDamage(t *testing.T) {
	cases := []struct {
		section string
		mutate  func(d *strictDoc)
	}{
		{"ExecutionLog", func(d *strictDoc) { d.logRows, d.logTail, d.lastAgent, d.globalSeq = nil, []string{"stray prose line"}, "null", 0 }},
		{"Artifacts", func(d *strictDoc) { d.registry, d.registryTail = nil, []string{"stray prose line"} }},
		{"WorkflowNotes", func(d *strictDoc) { d.notes, d.notesTail = nil, []string{"stray prose line"} }},
	}
	for _, c := range cases {
		t.Run(c.section, func(t *testing.T) {
			d := newStrictDoc()
			c.mutate(&d)
			doc := d.String()

			damage := parseDamaged(t, doc)

			requireDamageProblem(t, damage, c.section, lineOf(t, doc, "stray prose line"))
		})
	}
}
