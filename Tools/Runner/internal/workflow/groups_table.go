package workflow

import (
	"strings"

	"mosaic-common/mdtable"
	"mosaic-run/internal/domain"
)

// parseGroupsTable decodes the **Groups:** approach table (P6, P7, P8, and P2
// for the group tokens inside a Groups cell).
func parseGroupsTable(t mdtable.Table, workflowID string) ([]domain.ApproachSequence, error) {
	header := strings.Join(t.Header, " | ")
	if !hasExactColumns(t, "Block", "Approach", "Groups") {
		return nil, tableRefusal(workflowID, "P6", 0, header,
			"approach table must have exactly the Block, Approach and Groups columns")
	}
	if len(t.Rows) == 0 {
		return nil, tableRefusal(workflowID, "P6", 0, header, "approach table has no data rows")
	}
	blockCol, approachCol, groupsCol := t.Column("Block"), t.Column("Approach"), t.Column("Groups")
	type pair struct {
		block    domain.BlockName
		approach domain.Approach
	}
	seen := make(map[pair]bool)
	out := make([]domain.ApproachSequence, 0, len(t.Rows))
	for _, raw := range t.Rows {
		rowText := "| " + strings.Join(raw, " | ") + " |"
		block, approach, cell := raw[blockCol], raw[approachCol], raw[groupsCol]
		if block == "" || approach == "" || cell == "" {
			return nil, tableRefusal(workflowID, "P7", 0, rowText, "approach table has an empty cell")
		}
		if !validGroupToken(approach) {
			return nil, tableRefusal(workflowID, "P7", 0, approach,
				"approach token may contain only letters, digits and - and must start with a letter or digit")
		}
		groups, err := parseGroupsCell(cell, workflowID)
		if err != nil {
			return nil, err
		}
		key := pair{domain.BlockName(block), domain.Approach(approach)}
		if seen[key] {
			return nil, tableRefusal(workflowID, "P8", 0, approach,
				"approach "+approach+" is declared twice for block "+block)
		}
		seen[key] = true
		out = append(out, domain.ApproachSequence{Block: key.block, Approach: key.approach, Groups: groups})
	}
	return out, nil
}

// parseGroupsCell splits a comma-separated, ordered Groups cell.
func parseGroupsCell(cell, workflowID string) ([]domain.GroupName, error) {
	parts := strings.Split(cell, ",")
	seen := make(map[string]bool, len(parts))
	groups := make([]domain.GroupName, 0, len(parts))
	for _, p := range parts {
		token := strings.TrimSpace(p)
		switch {
		case token == "":
			return nil, tableRefusal(workflowID, "P7", 0, cell, "Groups cell holds an empty group token")
		case !validGroupToken(token):
			return nil, tableRefusal(workflowID, "P2", 0, token,
				"a group token may contain only letters, digits and - and must start with a letter or digit")
		case seen[token]:
			return nil, tableRefusal(workflowID, "P8", 0, token, "group "+token+" repeats within one Groups cell")
		}
		seen[token] = true
		groups = append(groups, domain.GroupName(token))
	}
	return groups, nil
}
