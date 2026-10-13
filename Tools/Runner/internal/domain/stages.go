package domain

// GroupName is a workflow-defined execution group token, taken verbatim from
// the Phase column's group segment and from the **Groups:** table.
// The empty value means "no group declared".
type GroupName string

// Approach is a workflow-defined, opaque token read verbatim from the plan
// artifact's Approach column and matched against the workflow's **Groups:**
// table. The runner defines no Approach constants.
// The zero value "" means "no approach read" and is only legal when the
// workflow declares no groups.
type Approach string

// ApproachSequence is one row of the **Groups:** table: an approach token
// and the ordered group sequence it selects.
type ApproachSequence struct {
	Block    BlockName // the **Groups:** Block cell
	Approach Approach
	Groups   []GroupName // ordered; non-empty for a valid table row
}

// ApproachTable is a workflow's **Groups:** table in declaration order.
// The zero value means the workflow declares no approach table.
type ApproachTable struct {
	Rows []ApproachSequence
}

// Present reports whether the workflow declares an approach table.
func (t ApproachTable) Present() bool {
	return len(t.Rows) > 0
}

// Sequence returns the ordered group sequence for the given approach token,
// and false when no row matches. Matching is exact and case-sensitive.
func (t ApproachTable) Sequence(a Approach) ([]GroupName, bool) {
	for _, row := range t.Rows {
		if row.Approach == a {
			return row.Groups, true
		}
	}
	return nil, false
}

// ForBlock returns the rows keyed to block b, in table order.
func (t ApproachTable) ForBlock(b BlockName) ApproachTable {
	var rows []ApproachSequence
	for _, row := range t.Rows {
		if row.Block == b {
			rows = append(rows, row)
		}
	}
	return ApproachTable{Rows: rows}
}

// Approaches returns every declared approach token in table order.
func (t ApproachTable) Approaches() []Approach {
	out := make([]Approach, len(t.Rows))
	for i, row := range t.Rows {
		out[i] = row.Approach
	}
	return out
}

// GroupNames returns every group named anywhere in the table, deduplicated,
// in order of first appearance.
func (t ApproachTable) GroupNames() []GroupName {
	seen := make(map[GroupName]bool)
	var out []GroupName
	for _, row := range t.Rows {
		for _, g := range row.Groups {
			if !seen[g] {
				seen[g] = true
				out = append(out, g)
			}
		}
	}
	return out
}

// ExecutionGroup is a contiguous range of routing table rows that form
// one group of a block.
type ExecutionGroup struct {
	Name     GroupName // workflow-defined; "" for the implicit no-groups range
	StartRow int       // inclusive, zero-based row index into RoutingTable.Rows
	EndRow   int       // exclusive, zero-based row index
}

// AdmittedWorkflow is the output of compat admission: a routing table that has
// been validated as inside the supported subset, with its blocks resolved.
type AdmittedWorkflow struct {
	Table RoutingTable

	// Blocks is the per-block admitted model, in table order. Rows outside
	// every block belong to the run scope.
	Blocks []AdmittedBlock
}
