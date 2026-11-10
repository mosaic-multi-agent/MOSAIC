package compat

import (
	"fmt"
	"sort"
	"strings"

	"mosaic-run/internal/domain"
)

// blockRange is the contiguous row range of one referenced block.
type blockRange struct {
	name  domain.BlockName
	start int // inclusive, zero-based
	end   int // exclusive
}

// admitBlocks admits a table parsed from WorkflowFormat 2.16 notation. It
// refuses a table holding a shape no parse could produce, nested blocks and
// violations of rules A1-A7, A11, A12 and (engine-routed modes) the cycle part
// of A15, and otherwise builds the per-block model. Forks and joins are
// admitted per mode.
func admitBlocks(table domain.RoutingTable, mode domain.ExecutionMode) (domain.AdmittedWorkflow, error) {
	if err := checkParsedShape(table); err != nil {
		return domain.AdmittedWorkflow{}, err
	}
	if err := checkParallelism(table, mode); err != nil {
		return domain.AdmittedWorkflow{}, err
	}
	if err := checkNoNesting(table); err != nil {
		return domain.AdmittedWorkflow{}, err
	}
	if err := checkBlockReferences(table); err != nil {
		return domain.AdmittedWorkflow{}, err
	}
	ranges, err := contiguousBlockRanges(table)
	if err != nil {
		return domain.AdmittedWorkflow{}, err
	}
	if err := checkBlocksHaveRows(table, ranges); err != nil {
		return domain.AdmittedWorkflow{}, err
	}

	blocks := make([]domain.AdmittedBlock, 0, len(ranges))
	for _, r := range ranges {
		b, err := buildBlock(table, r)
		if err != nil {
			return domain.AdmittedWorkflow{}, err
		}
		blocks = append(blocks, b)
	}
	if err := checkApproachTable(table, blocks); err != nil {
		return domain.AdmittedWorkflow{}, err
	}
	if err := checkScopes(table, blocks); err != nil {
		return domain.AdmittedWorkflow{}, err
	}
	if err := classifySchedules(table, blocks); err != nil {
		return domain.AdmittedWorkflow{}, err
	}
	if err := checkNoCycles(table, blocks, mode); err != nil {
		return domain.AdmittedWorkflow{}, err
	}

	return assemble(table, blocks), nil
}

// assemble builds the admitted workflow from the table and its blocks.
func assemble(table domain.RoutingTable, blocks []domain.AdmittedBlock) domain.AdmittedWorkflow {
	return domain.AdmittedWorkflow{Table: table, Blocks: blocks}
}

// checkParsedShape refuses a table holding a row that no parse of
// WorkflowFormat 2.16 notation could produce. The first such row in table order
// is reported.
func checkParsedShape(table domain.RoutingTable) error {
	declared := make(map[domain.BlockName]bool, len(table.Blocks))
	for _, d := range table.Blocks {
		declared[d.Name] = true
	}
	for i, row := range table.Rows {
		if defect := rowShapeDefect(row, len(table.Rows), declared); defect != "" {
			return refusal(table, "", domain.WorkflowRowFromIndex(i), "",
				fmt.Sprintf("row %d was not produced by the workflow parser: %s", i+1, defect))
		}
	}
	return nil
}

// rowShapeDefect returns the first defect of row, or "" when the row has the
// shape a 2.16 parse produces. nRows is the table's row count.
func rowShapeDefect(row domain.RoutingRow, nRows int, declared map[domain.BlockName]bool) string {
	switch row.Success.Kind {
	case "":
		return "its On Success entry has no kind"
	}
	switch row.Findings.Kind {
	case "":
		return "its On Findings entry has no kind"
	}
	if row.PhaseParsed.IsStaged != (len(row.PhaseParsed.Blocks) > 0) {
		return "its staged flag disagrees with its block references"
	}
	inRange := func(r domain.RowRef) bool { return r.Index >= 0 && r.Index < nRows }
	switch row.Success.Kind {
	case domain.SuccessTarget:
		if !inRange(row.Success.Target) {
			return fmt.Sprintf("its On Success target index %d is outside the table", row.Success.Target.Index)
		}
	case domain.SuccessFork:
		for _, f := range row.Success.Fork {
			if !inRange(f) {
				return fmt.Sprintf("its On Success fork target index %d is outside the table", f.Index)
			}
		}
	case domain.SuccessBlockEntry:
		if !declared[row.Success.Block] {
			return fmt.Sprintf("its On Success block entry names undeclared block %q", row.Success.Block)
		}
	}
	if row.Findings.Kind == domain.FindingsTarget && !inRange(row.Findings.Target) {
		return fmt.Sprintf("its On Findings target index %d is outside the table", row.Findings.Target.Index)
	}
	for _, w := range row.WaitsFor {
		if !inRange(w) {
			return fmt.Sprintf("its Waits For target index %d is outside the table", w.Index)
		}
	}
	return ""
}

// checkNoNesting refuses nested blocks, which the Runner does not support: a
// row with more than one block reference, or a block whose declared Source
// fills in another block.
func checkNoNesting(table domain.RoutingTable) error {
	for i, row := range table.Rows {
		if len(row.PhaseParsed.Blocks) > 1 {
			refs := make([]string, len(row.PhaseParsed.Blocks))
			for j, b := range row.PhaseParsed.Blocks {
				refs[j] = "[" + string(b) + "]"
			}
			return refusal(table, "", domain.WorkflowRowFromIndex(i), strings.Join(refs, "."),
				fmt.Sprintf("nested blocks are not supported: row %d carries %d block references", i+1, len(refs)))
		}
	}
	for _, d := range table.Blocks {
		if !strings.Contains(d.Source, "{") {
			continue
		}
		var row domain.WorkflowRow
		for i, r := range table.Rows {
			if len(r.PhaseParsed.Blocks) > 0 && r.PhaseParsed.Blocks[0] == d.Name {
				row = domain.WorkflowRowFromIndex(i)
				break
			}
		}
		return refusal(table, "", row, d.Source,
			fmt.Sprintf("nested blocks are not supported: block %q declares Source %q that fills in another block", d.Name, d.Source))
	}
	return nil
}

// checkBlockReferences implements the first half of A2: every block reference
// names a declared block.
func checkBlockReferences(table domain.RoutingTable) error {
	declared := make(map[domain.BlockName]bool, len(table.Blocks))
	for _, d := range table.Blocks {
		declared[d.Name] = true
	}
	for i, row := range table.Rows {
		for _, ref := range row.PhaseParsed.Blocks {
			if !declared[ref] {
				return refusal(table, "A2", domain.WorkflowRowFromIndex(i), string(ref),
					fmt.Sprintf("row %d references block %q, which the blocks table does not declare", i+1, ref))
			}
		}
	}
	return nil
}

// checkBlocksHaveRows implements the second half of A2: every declared block
// has at least one row. ranges holds the referenced blocks.
func checkBlocksHaveRows(table domain.RoutingTable, ranges []blockRange) error {
	used := make(map[domain.BlockName]bool, len(ranges))
	for _, r := range ranges {
		used[r.name] = true
	}
	for _, d := range table.Blocks {
		if !used[d.Name] {
			return refusal(table, "A2", domain.NoWorkflowRow, string(d.Name),
				fmt.Sprintf("block %q is declared but no row carries its reference", d.Name))
		}
	}
	return nil
}

// contiguousBlockRanges implements A6 and returns the row range of each
// referenced block, ordered by first row.
func contiguousBlockRanges(table domain.RoutingTable) ([]blockRange, error) {
	var ranges []blockRange
	index := map[domain.BlockName]int{}
	for i, row := range table.Rows {
		if len(row.PhaseParsed.Blocks) == 0 {
			continue
		}
		name := row.PhaseParsed.Blocks[0]
		at, seen := index[name]
		switch {
		case !seen:
			index[name] = len(ranges)
			ranges = append(ranges, blockRange{name: name, start: i, end: i + 1})
		case ranges[at].end == i:
			ranges[at].end = i + 1
		default:
			return nil, refusal(table, "A6", domain.WorkflowRowFromIndex(i), string(name),
				fmt.Sprintf("rows of block %q must be contiguous: row %d re-opens it after row %d", name, i+1, ranges[at].end))
		}
	}
	sort.SliceStable(ranges, func(a, b int) bool { return ranges[a].start < ranges[b].start })
	return ranges, nil
}

// buildBlock builds the admitted model of one block: its declaration, its
// groups (A3, A1) and its own approach rows.
func buildBlock(table domain.RoutingTable, r blockRange) (domain.AdmittedBlock, error) {
	var decl domain.BlockDecl
	for _, d := range table.Blocks {
		if d.Name == r.name {
			decl = d
			break
		}
	}
	groups, grouped, err := resolveBlockGroups(table, r)
	if err != nil {
		return domain.AdmittedBlock{}, err
	}
	return domain.AdmittedBlock{
		Name:       r.name,
		Source:     decl.Source,
		Section:    decl.Section,
		StartRow:   r.start,
		EndRow:     r.end,
		Grouped:    grouped,
		Groups:     groups,
		Approaches: table.ApproachTable.ForBlock(r.name),
	}, nil
}
