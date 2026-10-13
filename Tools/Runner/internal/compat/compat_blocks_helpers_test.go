package compat_test

// Fixtures and assertion helpers for the WorkflowFormat 2.16 block-structure
// admission tests (compat_blocks_*_test.go). Fixtures are inline: no test reads
// a workflow file from the catalog.

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/workflow"
)

// blocksWorkflowID is the id every refusal of these fixtures must name.
const blocksWorkflowID = "block-wf"

const routingColumns = "| Phase | Subagent | HITL | On Success | On Findings | Input | Output |\n" +
	"|-------|----------|:----:|------------|-------------|-------|--------|\n"

const routingColumnsWithRow = "| Row | Phase | Subagent | HITL | On Success | On Findings | Input | Output |\n" +
	"|-----|-------|----------|:----:|------------|-------------|-------|--------|\n"

// wfRow is one routing row of a fixture. Every row routes NEXT except the last,
// which is COMPLETE; Input and On Findings are "-".
type wfRow struct {
	phase  string
	agent  string // whole Subagent cell, mode qualifier included
	output string // "" derives an output that fills in every block reference of the phase
}

// outputCell returns the row's Output cell.
func (r wfRow) outputCell(i int) string {
	if r.output != "" {
		return r.output
	}
	out := fmt.Sprintf("out-%d", i)
	for _, seg := range strings.Split(r.phase, ".") {
		if strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") {
			out += "-{" + strings.Trim(seg, "[]") + "}"
		}
	}
	return out + ".md"
}

// region renders the routing table of rows, then each extra part verbatim.
func region(rows []wfRow, withRowColumn bool, extra ...string) string {
	var sb strings.Builder
	if withRowColumn {
		sb.WriteString(routingColumnsWithRow)
	} else {
		sb.WriteString(routingColumns)
	}
	for i, r := range rows {
		onSuccess := "NEXT"
		if i == len(rows)-1 {
			onSuccess = "COMPLETE"
		}
		if withRowColumn {
			fmt.Fprintf(&sb, "| %d ", i+1)
		}
		fmt.Fprintf(&sb, "| %s | %s | FALSE | %s | - | - | %s |\n", r.phase, r.agent, onSuccess, r.outputCell(i))
	}
	sb.WriteString(strings.Join(extra, ""))
	return sb.String()
}

// blocksSection renders the **Blocks:** table with the given data rows.
func blocksSection(rows ...string) string {
	return "\n**Blocks:**\n\n| Block | Source | Section |\n|-------|--------|---------|\n" +
		strings.Join(rows, "\n") + "\n"
}

// groupsSection renders the **Groups:** table with the given data rows.
func groupsSection(rows ...string) string {
	return "\n**Groups:**\n\n| Block | Approach | Groups |\n|-------|----------|--------|\n" +
		strings.Join(rows, "\n") + "\n"
}

// mustParse216 parses content with the strict 2.16 parser and fails the test
// when it is refused.
func mustParse216(t *testing.T, content string) domain.RoutingTable {
	t.Helper()
	info := domain.WorkflowInfo{ID: blocksWorkflowID, Version: "2.16"}
	table, err := workflow.Parse([]byte(content), info)
	if err != nil {
		t.Fatalf("strict Parse of the fixture: %v", err)
	}
	return table
}

// admitIn admits table in mode and fails the test on a refusal.
func admitIn(t *testing.T, table domain.RoutingTable, mode domain.ExecutionMode) domain.AdmittedWorkflow {
	t.Helper()
	aw, err := compat.Admit(table, mode)
	if err != nil {
		t.Fatalf("Admit in mode %q: unexpected refusal: %v", modeName(mode), err)
	}
	return aw
}

// forEachMode runs fn as a subtest per run mode, the unset sentinel included.
func forEachMode(t *testing.T, fn func(t *testing.T, mode domain.ExecutionMode)) {
	t.Helper()
	for _, mode := range allModes {
		t.Run(modeName(mode), func(t *testing.T) { fn(t, mode) })
	}
}

// requireBlockModel asserts every field of got that the admitted model of one
// block defines, comparing approach rows by content.
func requireBlockModel(t *testing.T, got, want domain.AdmittedBlock) {
	t.Helper()
	if got.Name != want.Name {
		t.Errorf("Name: want %q, got %q", want.Name, got.Name)
	}
	if got.Source != want.Source || got.Section != want.Section {
		t.Errorf("Source/Section: want %q/%q, got %q/%q", want.Source, want.Section, got.Source, got.Section)
	}
	if got.StartRow != want.StartRow || got.EndRow != want.EndRow {
		t.Errorf("rows: want [%d,%d), got [%d,%d)", want.StartRow, want.EndRow, got.StartRow, got.EndRow)
	}
	if got.Grouped != want.Grouped {
		t.Errorf("Grouped: want %v, got %v", want.Grouped, got.Grouped)
	}
	if !reflect.DeepEqual(got.Groups, want.Groups) {
		t.Errorf("Groups: want %+v, got %+v", want.Groups, got.Groups)
	}
	if len(got.Approaches.Rows) != len(want.Approaches.Rows) ||
		(len(want.Approaches.Rows) > 0 && !reflect.DeepEqual(got.Approaches.Rows, want.Approaches.Rows)) {
		t.Errorf("Approaches: want %+v, got %+v", want.Approaches.Rows, got.Approaches.Rows)
	}
	// The seed classification has its own tests (compat_schedule_source_test.go);
	// it is compared here only when the expectation names one.
	if want.Seed != domain.SeedUnclassified && got.Seed != want.Seed {
		t.Errorf("Seed: want %q, got %q", want.Seed, got.Seed)
	}
}

// requireBlocks asserts aw.Blocks holds exactly want, in order.
func requireBlocks(t *testing.T, aw domain.AdmittedWorkflow, want ...domain.AdmittedBlock) {
	t.Helper()
	if len(aw.Blocks) != len(want) {
		t.Fatalf("Blocks: want %d block(s), got %d: %+v", len(want), len(aw.Blocks), aw.Blocks)
	}
	for i := range want {
		t.Run(fmt.Sprintf("block-%s", want[i].Name), func(t *testing.T) {
			requireBlockModel(t, aw.Blocks[i], want[i])
		})
	}
}

// requireOneBlockRange asserts the one block of the workflow starts after pre
// rows, ends where the rows after it begin (postStart) and are postEnd rows in
// total, and holds the given groups.
func requireOneBlockRange(t *testing.T, aw domain.AdmittedWorkflow, pre, postStart, postEnd int, groups []domain.ExecutionGroup, declared bool) {
	t.Helper()
	if len(aw.Blocks) != 1 {
		t.Fatalf("Blocks: want exactly one block, got %d: %+v", len(aw.Blocks), aw.Blocks)
	}
	b := aw.Blocks[0]
	if b.Grouped != declared {
		t.Errorf("Grouped: want %v, got %v", declared, b.Grouped)
	}
	if !reflect.DeepEqual(b.Groups, groups) {
		t.Errorf("Groups: want %+v, got %+v", groups, b.Groups)
	}
	if b.StartRow != pre {
		t.Errorf("StartRow: want %d rows before the block, got %d", pre, b.StartRow)
	}
	if b.EndRow != postStart {
		t.Errorf("EndRow: want %d, got %d", postStart, b.EndRow)
	}
	if got := len(aw.Table.Rows); got != postEnd {
		t.Errorf("rows in the table: want %d, got %d", postEnd, got)
	}
}

// wantRefusal lists the items an admission refusal must carry. Wording is
// never asserted.
type wantRefusal struct {
	rules  []string // acceptable Rule values; []string{""} means "no rule ID"
	row    int      // 1-based routing row; 0 = not asserted
	tokens []string // Token must contain one of these; nil = not asserted
}

// requireRefusedInEveryMode asserts Admit refuses table in every run mode with
// a compat refusal that names the workflow and carries the wanted items.
func requireRefusedInEveryMode(t *testing.T, table domain.RoutingTable, want wantRefusal) {
	t.Helper()
	forEachMode(t, func(t *testing.T, mode domain.ExecutionMode) {
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
		if want.row != 0 && re.Row != domain.WorkflowRow(want.row) {
			t.Errorf("Row: want %d (1-based), got %d", want.row, re.Row)
		}
		if len(want.tokens) > 0 && !containsAny(re.Token, want.tokens) {
			t.Errorf("Token: want text containing one of %q, got %q", want.tokens, re.Token)
		}
		if re.Row == domain.NoWorkflowRow && re.Token == "" {
			t.Error("refusal must name the offending row or token; both are empty")
		}
		if re.Reason == "" {
			t.Error("Reason must describe the refusal")
		}
	})
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
