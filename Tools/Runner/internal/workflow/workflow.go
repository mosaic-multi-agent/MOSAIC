// Package workflow parses a routing table from a workflow region's raw content
// into a flat, ordered, index-identified model. It uses mdtable to parse the
// pipe-delimited markdown tables and produces a RoutingTable value whose rows
// preserve declaration order.
//
// Parse is the strict WorkflowFormat 2.16 parser of a region's structure: the
// Phase cells (blocks, groups, nesting), the routing table, and the
// **Blocks:** and **Groups:** tables. It does not interpret the tables: no
// compat checks, no path resolution. Artifact paths are preserved verbatim.
// The parser accepts no 1.x notation: every such token is refused.
//
// Every refusal is a *domain.RefusalError naming the workflow and the
// condition; strict refusals also carry the sec. 10 rule ID where one exists,
// the 1-based routing row and the offending token.
package workflow

import (
	"strings"

	"mosaic-common/mdtable"
	"mosaic-run/internal/domain"
)

// Parse turns a workflow region's raw content into a flat, ordered,
// row-identified routing table model, strictly by WorkflowFormat 2.16. It
// never accepts 1.x notation.
//
// The content is the raw bytes inside the <Workflow type="core" name="{id}">
// region. The info carries the identifier and version extracted by orchfile.
//
// Required columns: Phase, Subagent, HITL, On Success, On Findings, Input,
// Output. Allowed besides them: Waits For and the deployed Row column. Row is
// validated, not stored: when present, each data row must hold its 1-based
// position in the table.
//
// Returns a *domain.RefusalError if:
//   - the first table of the region is not a routing table, or the region
//     holds a table besides the routing, Blocks and Groups tables (T1)
//   - a required column is missing (T2), another column is present (T11), or
//     the routing table has no data rows (T3)
//   - a Row column is present and a row's value is not its 1-based position (T4)
//   - a Phase cell is malformed (P1, P2, P10, P12) or uses 1.x notation
//   - a cell is empty (T12), a Subagent cell is malformed (T6) or repeated
//     (T8), a HITL cell is not TRUE or FALSE (T7)
//   - an On Success, On Findings or Waits For cell is malformed (T5, T6), a
//     target repeats in one cell (T14) or matches no Subagent cell (T9), or the
//     Waits For column is present without any join (T13)
//   - a fill-in stands in a target cell (P11) or names a block the row is not
//     in (P9)
//   - a reserved heading repeats or is not followed by a table (P3)
//   - the Blocks or Groups table is invalid (P4-P8)
//   - the region holds the 1.x **Execution Groups:** heading
func Parse(content []byte, info domain.WorkflowInfo) (domain.RoutingTable, error) {
	workflowID := string(info.ID)

	routing, _, routingEnd, err := mdtable.ParseAt(content, 0)
	if err != nil {
		return domain.RoutingTable{}, tableRefusal(workflowID, "T1", 0, "",
			"no parseable routing table found")
	}
	if !firstTableIsRouting(routing) {
		return domain.RoutingTable{}, tableRefusal(workflowID, "T1", 0, strings.Join(routing.Header, " | "),
			"the first table of the region is not a routing table")
	}
	if hasExecutionGroupsHeading(content) {
		return domain.RoutingTable{}, tableRefusal(workflowID, "", 0, retiredExecutionGroupsHeading,
			"the 1.x heading is not WorkflowFormat 2.16 notation; use **Groups:**")
	}
	reserved, err := locateReservedTables(content, routingEnd, workflowID)
	if err != nil {
		return domain.RoutingTable{}, err
	}

	rows, err := parseRoutingRows(routing, workflowID)
	if err != nil {
		return domain.RoutingTable{}, err
	}
	if err := checkNoOtherTables(content, routingEnd, reserved, workflowID); err != nil {
		return domain.RoutingTable{}, err
	}

	table := domain.RoutingTable{Info: info, Rows: rows}
	if reserved.blocks != nil {
		if table.Blocks, err = parseBlocksTable(*reserved.blocks, rows, workflowID); err != nil {
			return domain.RoutingTable{}, err
		}
	}
	if reserved.groups != nil {
		approach, err := parseGroupsTable(*reserved.groups, workflowID)
		if err != nil {
			return domain.RoutingTable{}, err
		}
		table.ApproachTable = domain.ApproachTable{Rows: approach}
	}
	return table, nil
}

// parseRoutingRows validates the routing table's columns and Row column and
// decodes its data rows, resolving every routing cell to typed entries.
func parseRoutingRows(t mdtable.Table, workflowID string) ([]domain.RoutingRow, error) {
	if err := checkColumns(t, workflowID); err != nil {
		return nil, err
	}
	if len(t.Rows) == 0 {
		return nil, tableRefusal(workflowID, "T3", 0, strings.Join(t.Header, " | "), "routing table has no data rows")
	}
	if err := validateRowColumn(t, workflowID); err != nil {
		return nil, err
	}
	cols := locateColumns(t)
	rows, err := parseRowIdentities(t, cols, workflowID)
	if err != nil {
		return nil, err
	}
	if err := resolveRoutingCells(t, cols, rows, workflowID); err != nil {
		return nil, err
	}
	return rows, nil
}

// checkColumns refuses a routing table that lacks a required column (T2) or
// holds a column that is neither required, Waits For nor the deployed Row
// column (T11).
func checkColumns(t mdtable.Table, workflowID string) error {
	for _, col := range strictRequiredColumns {
		if t.Column(col) < 0 {
			return tableRefusal(workflowID, "T2", 0, col, "required column missing: "+col)
		}
	}
	for _, col := range t.Header {
		if col == waitsForColumn || col == rowColumnName || containsName(strictRequiredColumns, col) {
			continue
		}
		return tableRefusal(workflowID, "T11", 0, col, "unknown column: "+col)
	}
	return nil
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// splitArtifacts splits a comma-separated artifact path cell into individual
// trimmed paths. Returns nil when the cell is "-" or empty (no artifacts).
func splitArtifacts(cell string) []string {
	if cell == "-" || cell == "" {
		return nil
	}
	parts := strings.Split(cell, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
