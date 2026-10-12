package workflow_test

// Shared fixtures and assertions for the strict WorkflowFormat 2.16 parsing
// tests (strict_*_test.go). The transitional entry point's parse helper is in
// workflow_legacy_helpers_test.go.

import (
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/workflow"
)

// strictID is the workflow id every strict-parse refusal must name.
const strictID = "strict-wf"

// strictHeader is the 7-column routing table header of a 2.16 source workflow.
const strictHeader = "| Phase | Subagent | HITL | On Success | On Findings | Input | Output |\n" +
	"|-------|----------|:----:|------------|-------------|-------|--------|\n"

// blocksHeader and groupsHeader are the reserved headings that introduce the
// **Blocks:** and **Groups:** tables.
const blocksHeader = "\n**Blocks:**\n\n"
const groupsHeader = "\n**Groups:**\n\n"

// blocksColumns and groupsColumns are the valid column header and separator
// lines of the two tables.
const blocksColumns = "| Block | Source | Section |\n|-------|--------|---------|\n"
const groupsColumns = "| Block | Approach | Groups |\n|-------|----------|--------|\n"

// strictBlocksTable is a valid blocks table declaring block Stage.
const strictBlocksTable = blocksHeader + blocksColumns + "| Stage | Plan.md | Stages |\n"

// strictGroupsTable is a valid approach table for block Stage.
const strictGroupsTable = groupsHeader + groupsColumns + "| Stage | TDD | Test, Implementation |\n"

// blocksTable renders a blocks table (heading, columns, then the given rows).
func blocksTable(rows ...string) string {
	return blocksHeader + blocksColumns + strings.Join(rows, "\n") + "\n"
}

// groupsTable renders a groups table (heading, columns, then the given rows).
func groupsTable(rows ...string) string {
	return groupsHeader + groupsColumns + strings.Join(rows, "\n") + "\n"
}

func strictInfo() domain.WorkflowInfo {
	return domain.WorkflowInfo{ID: strictID, Version: "2.16"}
}

// strictRow renders one routing row with fixed routing cells.
func strictRow(phase, agent string) string {
	return "| " + phase + " | " + agent + " | FALSE | NEXT | - | - | out.md |\n"
}

// strictRowIO renders one routing row with explicit Input and Output cells.
func strictRowIO(phase, agent, input, output string) string {
	return "| " + phase + " | " + agent + " | FALSE | NEXT | - | " + input + " | " + output + " |\n"
}

// strictRegion assembles a region: the routing table of the given rows, then
// each extra part (tables, headings, notes) verbatim.
func strictRegion(rows []string, extra ...string) string {
	return strictHeader + strings.Join(rows, "") + strings.Join(extra, "")
}

// strictPhases builds one row per phase cell with distinct agent ids.
func strictPhases(phases ...string) []string {
	rows := make([]string, len(phases))
	for i, p := range phases {
		rows[i] = strictRow(p, "agent-"+string(rune('a'+i)))
	}
	return rows
}

func parseStrict(content string) (domain.RoutingTable, error) {
	return workflow.Parse([]byte(content), strictInfo())
}

func mustParseStrict(t *testing.T, content string) domain.RoutingTable {
	t.Helper()
	table, err := parseStrict(content)
	if err != nil {
		t.Fatalf("strict Parse: unexpected refusal: %v", err)
	}
	return table
}

// refusalWant lists the items a refusal must carry. Empty/zero fields are not
// asserted. Wording is never asserted.
type refusalWant struct {
	rule  string   // exact rule ID
	anyOf []string // the rule ID is one of these (when rule is "")
	row   int      // 1-based routing row; 0 = not asserted
	token string   // text that must be related to the refusal's Token
}

// requireRefusal asserts err is a *domain.RefusalError of component "workflow"
// that names the workflow id and carries the wanted rule, row and token.
func requireRefusal(t *testing.T, err error, want refusalWant) *domain.RefusalError {
	t.Helper()
	if err == nil {
		t.Fatal("expected a refusal, got nil error")
	}
	re := asRefusalError(t, err)
	if re.Component != "workflow" {
		t.Errorf("Component: want %q, got %q", "workflow", re.Component)
	}
	if !strings.Contains(re.Resource, strictID) {
		t.Errorf("Resource must name workflow %q, got %q", strictID, re.Resource)
	}
	if want.rule != "" && re.Rule != want.rule {
		t.Errorf("Rule: want %q, got %q", want.rule, re.Rule)
	}
	if len(want.anyOf) > 0 && !containsString(want.anyOf, re.Rule) {
		t.Errorf("Rule: want one of %v, got %q", want.anyOf, re.Rule)
	}
	if want.row != 0 && re.Row != domain.WorkflowRow(want.row) {
		t.Errorf("Row: want %d (1-based), got %d", want.row, re.Row)
	}
	if want.token != "" {
		if re.Token == "" {
			t.Errorf("Token must name the offending token (%q), got empty", want.token)
		} else if !strings.Contains(re.Token, want.token) && !strings.Contains(want.token, re.Token) {
			t.Errorf("Token %q is not related to the offending text %q", re.Token, want.token)
		}
	}
	return re
}

// requireRowOrToken asserts the refusal names its offending row or token.
func requireRowOrToken(t *testing.T, re *domain.RefusalError) {
	t.Helper()
	if re.Row == domain.NoWorkflowRow && re.Token == "" {
		t.Error("refusal must name the offending row or token; both are empty")
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// blockNames converts a row's block references to plain strings.
func blockNames(blocks []domain.BlockName) []string {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = string(b)
	}
	return out
}

func equalStrings(a, b []string) bool {
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
