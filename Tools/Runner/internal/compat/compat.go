// Package compat implements workflow admission validation. It enforces that
// only supported workflow shapes are allowed to run and builds the per-block
// admitted model the engine and session read.
//
// The single entry point is Admit. A table gets the WorkflowFormat 2.16
// structural checks in blocks.go and groups.go: block references (A2),
// contiguous blocks (A6), groups per block (A1, A3) and the approach table per
// block (A4, A5). Nested blocks, which the Runner does not support, are
// refused by name.
//
// Matching of block, group and approach names is case-sensitive throughout.
package compat

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// Admit validates that the routing table is inside the supported workflow subset
// and builds its admitted model.
//
// Returns an AdmittedWorkflow on success. Returns *domain.RefusalError on
// failure, with Component set to "compat", Resource naming the workflow, and
// Rule, Row (1-based) and Token set where the refusal has them.
//
// The admitted model is AdmittedWorkflow.Blocks, one entry per declared block.
//
// Duplicate agent identifiers in different rows are allowed.
func Admit(table domain.RoutingTable, mode domain.ExecutionMode) (domain.AdmittedWorkflow, error) {
	return admitBlocks(table, mode)
}

// refusal builds a compat refusal for table's workflow. row is the 1-based
// offending row (domain.NoWorkflowRow when none); rule and token are empty when
// the refusal has none.
func refusal(table domain.RoutingTable, rule string, row domain.WorkflowRow, token, reason string) error {
	return &domain.RefusalError{
		Component: "compat",
		Resource:  fmt.Sprintf("workflow %q", string(table.Info.ID)),
		Reason:    reason,
		Rule:      rule,
		Row:       row,
		Token:     token,
	}
}
