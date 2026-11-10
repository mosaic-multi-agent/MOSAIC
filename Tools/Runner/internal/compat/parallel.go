package compat

import (
	"fmt"
	"strings"

	"mosaic-run/internal/domain"
)

// checkParallelism applies the per-mode fork and join rules to a 2.16 table.
//
// Engine-routed modes (auto, auto-review and the unset sentinel) dispatch one
// row at a time, so a fork (a comma list in On Success) and a join (a Waits For
// value other than "-") are refused. A fork or join on a block row also breaks
// B1; a run-scope one has no format rule ID. In orchestrated mode the
// consultant dispatches one agent at a time, so forks and joins are admitted,
// subject to the fork-branch boundary check.
func checkParallelism(table domain.RoutingTable, mode domain.ExecutionMode) error {
	if mode == domain.ExecutionModeOrchestrated {
		return checkForkBranchEnds(table)
	}
	for i, row := range table.Rows {
		rule := ""
		if len(row.PhaseParsed.Blocks) > 0 {
			rule = "B1"
		}
		if row.Success.Kind == domain.SuccessFork {
			return refusal(table, rule, domain.WorkflowRowFromIndex(i), row.Success.Raw,
				fmt.Sprintf("parallel dispatch is not supported: row %d forks with On Success %q", i+1, row.Success.Raw))
		}
		if len(row.WaitsFor) > 0 {
			cells := make([]string, len(row.WaitsFor))
			for j, w := range row.WaitsFor {
				cells[j] = w.Cell
			}
			joined := strings.Join(cells, ", ")
			return refusal(table, rule, domain.WorkflowRowFromIndex(i), joined,
				fmt.Sprintf("parallel dispatch is not supported: row %d joins with Waits For %q", i+1, joined))
		}
	}
	return nil
}

// checkForkBranchEnds refuses a fork branch row that is the last row of its
// phase (which also covers the last row of an execution group), because a
// branch must be followed by a join row.
func checkForkBranchEnds(table domain.RoutingTable) error {
	rows := table.Rows
	for i, row := range rows {
		if row.Success.Kind != domain.SuccessFork {
			continue
		}
		branches := make(map[int]bool, len(row.Success.Fork))
		for _, f := range row.Success.Fork {
			branches[f.Index] = true
		}
		for j := i + 1; j < len(rows) && rows[j].Phase == row.Phase; j++ {
			endsPhase := j+1 == len(rows) || rows[j+1].Phase != row.Phase
			if endsPhase && branches[j] {
				return refusal(table, "", domain.WorkflowRowFromIndex(j), rows[j].Agent,
					fmt.Sprintf("fork branch %q at row %d is the last row of its phase %q: a fork branch must be followed by a join row",
						rows[j].Agent, j+1, rows[j].Phase))
			}
		}
	}
	return nil
}
