package workflow

import (
	"strings"

	"mosaic-common/mdtable"
	"mosaic-run/internal/domain"
)

// tableRefusal builds a strict-parse refusal for the workflow.
func tableRefusal(workflowID, rule string, row int, token, reason string) *domain.RefusalError {
	return &domain.RefusalError{
		Component: "workflow",
		Resource:  workflowID,
		Reason:    reason,
		Rule:      rule,
		Row:       domain.WorkflowRow(row),
		Token:     token,
	}
}

// hasExactColumns reports whether the table header holds exactly the named
// columns, in any order.
func hasExactColumns(t mdtable.Table, names ...string) bool {
	if len(t.Header) != len(names) {
		return false
	}
	for _, n := range names {
		if t.Column(n) < 0 {
			return false
		}
	}
	return true
}

// emptyCellRow returns the first data row with an empty cell, as its text.
func emptyCellRow(t mdtable.Table) (string, bool) {
	for _, row := range t.Rows {
		for _, cell := range row {
			if cell == "" {
				return "| " + strings.Join(row, " | ") + " |", true
			}
		}
	}
	return "", false
}

// parseBlocksTable decodes the **Blocks:** table (P4, P5). rows are the parsed
// routing rows: they define which blocks enclose a declared block.
func parseBlocksTable(t mdtable.Table, rows []domain.RoutingRow, workflowID string) ([]domain.BlockDecl, error) {
	header := strings.Join(t.Header, " | ")
	if !hasExactColumns(t, "Block", "Source", "Section") {
		return nil, tableRefusal(workflowID, "P4", 0, header,
			"blocks table must have exactly the Block, Source and Section columns")
	}
	if len(t.Rows) == 0 {
		return nil, tableRefusal(workflowID, "P4", 0, header, "blocks table has no data rows")
	}
	if row, bad := emptyCellRow(t); bad {
		return nil, tableRefusal(workflowID, "P4", 0, row, "blocks table has an empty cell")
	}
	blockCol, sourceCol, sectionCol := t.Column("Block"), t.Column("Source"), t.Column("Section")
	declared := make(map[domain.BlockName]bool)
	decls := make([]domain.BlockDecl, 0, len(t.Rows))
	for _, raw := range t.Rows {
		decl := domain.BlockDecl{
			Name:    domain.BlockName(raw[blockCol]),
			Source:  raw[sourceCol],
			Section: raw[sectionCol],
		}
		if err := checkBlockDecl(decl, declared, rows, workflowID); err != nil {
			return nil, err
		}
		declared[decl.Name] = true
		decls = append(decls, decl)
	}
	return decls, nil
}

// checkBlockDecl applies the P5 rules to one blocks-table row.
func checkBlockDecl(decl domain.BlockDecl, declared map[domain.BlockName]bool,
	rows []domain.RoutingRow, workflowID string) error {
	name := string(decl.Name)
	if !validBlockName(name) || name == "HITL" || name == "Approach" {
		return tableRefusal(workflowID, "P5", 0, name,
			"block name must be letters only and not HITL or Approach")
	}
	if declared[decl.Name] {
		return tableRefusal(workflowID, "P5", 0, name, "block "+name+" is declared twice")
	}
	if strings.Contains(decl.Source, "*") {
		return tableRefusal(workflowID, "P5", 0, decl.Source, "block Source must not contain a wildcard")
	}
	enclosing := enclosingBlocks(decl.Name, rows)
	for _, fill := range fillIns(decl.Source) {
		if !enclosing[domain.BlockName(fill)] {
			return tableRefusal(workflowID, "P5", 0, "{"+fill+"}",
				"block Source fills in {"+fill+"}, which is not a block enclosing "+name)
		}
	}
	return nil
}

// enclosingBlocks returns the blocks written before block in the first row
// that carries it.
func enclosingBlocks(block domain.BlockName, rows []domain.RoutingRow) map[domain.BlockName]bool {
	out := make(map[domain.BlockName]bool)
	for _, row := range rows {
		for i, b := range row.PhaseParsed.Blocks {
			if b != block {
				continue
			}
			for _, outer := range row.PhaseParsed.Blocks[:i] {
				out[outer] = true
			}
			return out
		}
	}
	return out
}

// fillIns returns the names of every {Name} in s.
func fillIns(s string) []string {
	var out []string
	for {
		open := strings.Index(s, "{")
		if open < 0 {
			return out
		}
		end := strings.Index(s[open:], "}")
		if end < 0 {
			return out
		}
		out = append(out, s[open+1:open+end])
		s = s[open+end+1:]
	}
}
