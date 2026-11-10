package compat_test

// Fixtures and assertion helpers for the WorkflowFormat 2.16 routing-shape and
// schedule-source admission tests (compat_parallel_test.go,
// compat_scopes_test.go, compat_schedule_source*_test.go, compat_cycles_test.go,
// compat_routing_shapes_test.go). Fixtures are inline: no test reads a workflow
// file from the catalog.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
)

// rrow is one routing row of a routing fixture. An empty onSuccess routes NEXT
// (COMPLETE on the last row); an empty onFindings, waitsFor or input is "-"; an
// empty output derives an output that fills in every block reference.
type rrow struct {
	phase      string
	agent      string // whole Subagent cell, mode qualifier included
	onSuccess  string
	onFindings string
	waitsFor   string
	input      string
	output     string
}

// routedRegion renders the routing table of rows, then each extra part
// verbatim. The Waits For column is present only when some row joins.
func routedRegion(rows []rrow, extra ...string) string {
	hasJoin := false
	for _, r := range rows {
		hasJoin = hasJoin || r.waitsFor != ""
	}
	var sb strings.Builder
	if hasJoin {
		sb.WriteString("| Phase | Subagent | HITL | On Success | On Findings | Waits For | Input | Output |\n")
		sb.WriteString("|-------|----------|:----:|------------|-------------|-----------|-------|--------|\n")
	} else {
		sb.WriteString(routingColumns)
	}
	for i, r := range rows {
		onSuccess := orDefault(r.onSuccess, "NEXT")
		if r.onSuccess == "" && i == len(rows)-1 {
			onSuccess = "COMPLETE"
		}
		out := wfRow{phase: r.phase, output: r.output}.outputCell(i)
		if hasJoin {
			fmt.Fprintf(&sb, "| %s | %s | FALSE | %s | %s | %s | %s | %s |\n",
				r.phase, r.agent, onSuccess, orDefault(r.onFindings, "-"),
				orDefault(r.waitsFor, "-"), orDefault(r.input, "-"), out)
			continue
		}
		fmt.Fprintf(&sb, "| %s | %s | FALSE | %s | %s | %s | %s |\n",
			r.phase, r.agent, onSuccess, orDefault(r.onFindings, "-"), orDefault(r.input, "-"), out)
	}
	sb.WriteString(strings.Join(extra, ""))
	return sb.String()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// parseRows parses rows plus tables with the strict 2.16 parser.
func parseRows(t *testing.T, rows []rrow, tables string) domain.RoutingTable {
	t.Helper()
	return mustParse216(t, routedRegion(rows, tables))
}

// ---- Shared table shapes ----

// stageBlockTable declares block Stage with source Plan.md.
func stageBlockTable() string { return blocksSection("| Stage | Plan.md | Stages |") }

// ungroupedStageRows: row 1 planner produces Plan.md, rows 2-3 are the ungrouped
// block Stage, row 4 follows the block.
func ungroupedStageRows() []rrow {
	return []rrow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW", agent: "test-runner"},
	}
}

// groupedStageRows: row 1 planner produces Plan.md, rows 2-3 are group Test and
// rows 4-5 group Implementation of block Stage, row 6 follows the block.
func groupedStageRows() []rrow {
	return []rrow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-a"},
		{phase: "EXECUTION.[Stage].Test", agent: "t-b"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "i-a"},
		{phase: "EXECUTION.[Stage].Implementation", agent: "i-b"},
		{phase: "REVIEW", agent: "test-runner"},
	}
}

func groupedStageTables() string {
	return stageBlockTable() + groupsSection("| Stage | TDD | Test, Implementation |")
}

// twoBlockRows: row 1 planner produces Plan.md and Audit.md, rows 2-3 are the
// ungrouped block Stage, rows 4-5 the ungrouped block Audit, row 6 the reporter.
func twoBlockRows() []rrow {
	return []rrow{
		{phase: "PLANNING", agent: "planner", output: "Plan.md, Audit.md"},
		{phase: "EXECUTION.[Stage]", agent: "impl-a"},
		{phase: "EXECUTION.[Stage]", agent: "impl-b"},
		{phase: "REVIEW.[Audit]", agent: "auditor"},
		{phase: "REVIEW.[Audit]", agent: "audit-fixer"},
		{phase: "REPORT", agent: "reporter"},
	}
}

func twoBlockTables() string {
	return blocksSection("| Stage | Plan.md | Stages |", "| Audit | Audit.md | Items |")
}

// ---- Assertions ----

// wantRoute lists the items a routing-shape refusal must carry. Wording is
// never asserted.
type wantRoute struct {
	rules  []string // acceptable Rule values; []string{""} means "no rule ID"
	rows   []int    // acceptable 1-based routing rows; nil = not asserted
	tokens []string // Token must contain one of these; nil = not asserted
	// noToken requires an empty Token: a refusal that names only the row.
	noToken bool
}

// requireRefusedIn asserts Admit refuses table in every one of modes with a
// compat refusal that names the workflow and carries the wanted items.
func requireRefusedIn(t *testing.T, table domain.RoutingTable, modes []domain.ExecutionMode, want wantRoute) {
	t.Helper()
	for _, mode := range modes {
		t.Run("refused-"+modeName(mode), func(t *testing.T) {
			_, err := compat.Admit(table, mode)
			if err == nil {
				t.Fatal("Admit must refuse the workflow, got nil error")
			}
			re := asRefusalError(t, err)
			if re.Component != "compat" {
				t.Errorf("Component: want %q, got %q", "compat", re.Component)
			}
			if !strings.Contains(re.Resource, blocksWorkflowID) {
				t.Errorf("Resource must name workflow %q, got %q", blocksWorkflowID, re.Resource)
			}
			if !containsString(want.rules, re.Rule) {
				t.Errorf("Rule: want one of %q, got %q", want.rules, re.Rule)
			}
			if len(want.rows) > 0 && !containsRow(want.rows, re.Row) {
				t.Errorf("Row: want one of %v (1-based), got %d", want.rows, re.Row)
			}
			if len(want.tokens) > 0 && !containsAny(re.Token, want.tokens) {
				t.Errorf("Token: want text containing one of %q, got %q", want.tokens, re.Token)
			}
			if want.noToken && re.Token != "" {
				t.Errorf("Token: want empty, got %q", re.Token)
			}
			if re.Row == domain.NoWorkflowRow && re.Token == "" {
				t.Error("refusal must name the offending row or token; both are empty")
			}
			if re.Reason == "" {
				t.Error("Reason must describe the refusal")
			}
		})
	}
}

func containsRow(rows []int, got domain.WorkflowRow) bool {
	for _, r := range rows {
		if domain.WorkflowRow(r) == got {
			return true
		}
	}
	return false
}

// requireAdmittedIn asserts Admit admits table in every one of modes and
// returns the admitted workflow of the last mode.
func requireAdmittedIn(t *testing.T, table domain.RoutingTable, modes []domain.ExecutionMode) domain.AdmittedWorkflow {
	t.Helper()
	var aw domain.AdmittedWorkflow
	for _, mode := range modes {
		t.Run("admitted-"+modeName(mode), func(t *testing.T) {
			aw = admitIn(t, table, mode)
		})
	}
	return aw
}

// requireSeedModel asserts the schedule-source facts the admitted model records
// for block name: its seed class, producing rows and in-block writer rows
// (zero-based). A nil and an empty list are the same.
func requireSeedModel(t *testing.T, aw domain.AdmittedWorkflow, name domain.BlockName,
	seed domain.SeedClass, producing, writers []int) {
	t.Helper()
	b, ok := aw.BlockByName(name)
	if !ok {
		t.Fatalf("block %q is not in the admitted model", name)
	}
	if b.Seed != seed {
		t.Errorf("block %q Seed: want %q, got %q", name, seed, b.Seed)
	}
	if !sameInts(b.ProducingRows, producing) {
		t.Errorf("block %q ProducingRows: want %v, got %v", name, producing, b.ProducingRows)
	}
	if !sameInts(b.WriterRows, writers) {
		t.Errorf("block %q WriterRows: want %v, got %v", name, writers, b.WriterRows)
	}
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
