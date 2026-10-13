package compat

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// resolveBlockGroups partitions the rows of block r into its groups. The bool
// result reports whether the block is grouped.
//
// A3: either every own row of the block carries a group or none does. A1: a
// group's rows are contiguous within the block. An ungrouped block holds one
// unnamed group spanning its rows.
func resolveBlockGroups(table domain.RoutingTable, r blockRange) ([]domain.ExecutionGroup, bool, error) {
	grouped := false
	for i := r.start; i < r.end; i++ {
		if table.Rows[i].PhaseParsed.Group != "" {
			grouped = true
			break
		}
	}
	if !grouped {
		return []domain.ExecutionGroup{{Name: "", StartRow: r.start, EndRow: r.end}}, false, nil
	}

	for i := r.start; i < r.end; i++ {
		if table.Rows[i].PhaseParsed.Group == "" {
			return nil, false, refusal(table, "A3", domain.WorkflowRowFromIndex(i), string(r.name),
				fmt.Sprintf("block %q mixes grouped and ungrouped rows: row %d declares no group", r.name, i+1))
		}
	}

	var groups []domain.ExecutionGroup
	ended := map[domain.GroupName]bool{}
	for i := r.start; i < r.end; i++ {
		g := table.Rows[i].PhaseParsed.Group
		last := len(groups) - 1
		if last >= 0 && groups[last].Name == g {
			groups[last].EndRow = i + 1
			continue
		}
		if last >= 0 {
			ended[groups[last].Name] = true
		}
		if ended[g] {
			return nil, false, refusal(table, "A1", domain.WorkflowRowFromIndex(i), string(g),
				fmt.Sprintf("rows of group %q in block %q must be contiguous: row %d re-opens it", g, r.name, i+1))
		}
		groups = append(groups, domain.ExecutionGroup{Name: g, StartRow: i, EndRow: i + 1})
	}
	return groups, true, nil
}

// checkApproachTable implements A4 and A5 per block.
//
// A4: every grouped block has at least one approach-table row, and every
// approach-table row names a grouped block. A5: every group an approach row
// names exists in its block, and every group of a block is named by some
// approach row of that block.
func checkApproachTable(table domain.RoutingTable, blocks []domain.AdmittedBlock) error {
	byName := make(map[domain.BlockName]domain.AdmittedBlock, len(blocks))
	for _, b := range blocks {
		byName[b.Name] = b
	}

	for _, row := range table.ApproachTable.Rows {
		if b, ok := byName[row.Block]; !ok || !b.Grouped {
			token := string(row.Block)
			if token == "" {
				token = string(row.Approach)
			}
			return refusal(table, "A4", domain.NoWorkflowRow, token,
				fmt.Sprintf("approach %q names block %q, which is not a grouped block", row.Approach, row.Block))
		}
	}
	for _, b := range blocks {
		if b.Grouped && !b.Approaches.Present() {
			return refusal(table, "A4", domain.WorkflowRowFromIndex(b.StartRow), string(b.Name),
				fmt.Sprintf("grouped block %q has no approach-table row", b.Name))
		}
	}

	for _, b := range blocks {
		if !b.Grouped {
			continue
		}
		inBlock := make(map[domain.GroupName]bool, len(b.Groups))
		for _, g := range b.Groups {
			inBlock[g.Name] = true
		}
		named := make(map[domain.GroupName]bool)
		for _, ap := range b.Approaches.Rows {
			for _, g := range ap.Groups {
				if !inBlock[g] {
					return refusal(table, "A5", domain.NoWorkflowRow, string(g),
						fmt.Sprintf("approach %q of block %q names group %q, which no row of the block declares", ap.Approach, b.Name, g))
				}
				named[g] = true
			}
		}
		for _, g := range b.Groups {
			if !named[g.Name] {
				return refusal(table, "A5", domain.WorkflowRowFromIndex(g.StartRow), string(g.Name),
					fmt.Sprintf("group %q of block %q is named by no approach-table row", g.Name, b.Name))
			}
		}
	}
	return nil
}
