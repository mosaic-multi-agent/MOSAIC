package app_test

// transform_mcp_split_helpers_test.go holds the shared builders of the MCP name transform
// tests: an OpenCode workspace seeded with an agent that grants MCP names, tool-local OpenCode
// MCP configuration fixtures, and readers of the split questions a run asked.

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

const (
	// trSplitTracker has two possible splits; trSplitAzure has three.
	trSplitTracker = "tracker_get_issue"
	trSplitAzure   = "azure-devops_get_pull_request"
)

// trMCPAgentKey is the key of the OpenCode agent fixture.
const trMCPAgentKey = "mcp-agent"

// splitQuestions returns the QMCPNameSplit questions asked, in order.
func (s *trSpy) splitQuestions() []domain.ChoiceQuestion {
	var out []domain.ChoiceQuestion
	for _, q := range s.choices {
		if q.ID == domain.QMCPNameSplit {
			out = append(out, q)
		}
	}
	return out
}

// splitSubjects returns the subjects of the split questions asked, in order.
func (s *trSpy) splitSubjects() []string {
	var out []string
	for _, q := range s.splitQuestions() {
		out = append(out, q.Subject)
	}
	return out
}

// trMCPRun is an OpenCode to GHCP CLI transform run over the MCP agent fixture.
func trMCPRun(t *testing.T, script *interactiontest.Stub) *trRun {
	t.Helper()
	r := trNew(t, script, hOpen, hCLI)
	r.put(hOpen, trMCPAgentKey, trOpenCodeFixture(t, "opencode-agents", trMCPAgentKey+".md"))
	return r
}

// trOpenCodeFixture reads a fixture below testdata/transform.
func trOpenCodeFixture(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "transform", dir, name))
	if err != nil {
		t.Fatalf("read fixture %s/%s: %v", dir, name, err)
	}
	return string(b)
}

// trPutOpenCodeConfig copies an OpenCode configuration fixture to the workspace root under the
// given file name (opencode.json or opencode.jsonc).
func trPutOpenCodeConfig(t *testing.T, r *trRun, fixture, fileName string) {
	t.Helper()
	trWrite(t, r.ws, fileName, trOpenCodeFixture(t, "opencode-config", fixture))
}

// trMCPOutput returns the written GHCP CLI agent of the fixture.
func trMCPOutput(r *trRun) string { return r.read(r.dest(trMCPAgentKey)) }

// trUnmapped returns the unmapped tools of the fixture agent's outcome.
func trUnmapped(t *testing.T, res app.TransformHarnessResult) []string {
	t.Helper()
	return trFile(t, res, trMCPAgentKey).UnmappedTools
}

// trSortedCopy returns a sorted copy.
func trSortedCopy(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}
