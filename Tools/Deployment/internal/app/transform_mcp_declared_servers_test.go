package app_test

// transform_mcp_declared_servers_test.go specifies how the OpenCode MCP configuration decides
// the split of an ambiguous OpenCode name during a transform: the keys of the top-level mcp
// object of opencode.json and opencode.jsonc at the workspace root (both read as JSONC) are the
// declared servers; a missing, unreadable or invalid file contributes nothing and never fails
// the run; the configuration is only read, never written.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
)

func TestTransformMcpDeclared_ExampleNamesArriveInGhcpSyntax(t *testing.T) {
	r := trMCPRun(t, nil)
	trPutOpenCodeConfig(t, r, "declared-azure.json", "opencode.json")

	r.mustRun(r.request())

	out := trMCPOutput(r)
	for _, want := range []string{"contact-user/*", "azure-devops/get_pull_request"} {
		if !strings.Contains(out, want) {
			t.Errorf("the GHCP agent must carry %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"contact-user*", "azure-devops_get_pull_request", "blocked"} {
		if strings.Contains(out, bad) {
			t.Errorf("the GHCP agent must not carry %q:\n%s", bad, out)
		}
	}
}

func TestTransformMcpDeclared_DeclaredServerDecidesWithoutAskingTheUser(t *testing.T) {
	r := trMCPRun(t, nil)
	trPutOpenCodeConfig(t, r, "declared-azure.json", "opencode.json")

	r.mustRun(r.request())

	if got := r.spy.splitSubjects(); slices.Contains(got, trSplitAzure) {
		t.Errorf("the declared server must decide %s without a question, asked: %q", trSplitAzure, got)
	}
	out := trMCPOutput(r)
	if !strings.Contains(out, "azure-devops/get_pull_request") || strings.Contains(out, "azure-devops_get_pull_request") {
		t.Errorf("the declared server must resolve the split in the written agent:\n%s", out)
	}
}

func TestTransformMcpDeclared_JsoncWithCommentsAndTrailingCommasIsRead(t *testing.T) {
	r := trMCPRun(t, nil)
	trPutOpenCodeConfig(t, r, "declared-tracker.jsonc", "opencode.jsonc")

	r.mustRun(r.request())

	if got := r.spy.splitSubjects(); slices.Contains(got, trSplitTracker) {
		t.Errorf("the declared server tracker must decide %s without a question, asked: %q", trSplitTracker, got)
	}
	if out := trMCPOutput(r); !strings.Contains(out, "tracker/get_issue") {
		t.Errorf("the GHCP agent must carry tracker/get_issue:\n%s", out)
	}
}

func TestTransformMcpDeclared_ServersOfBothConfigurationFilesAreCombined(t *testing.T) {
	r := trMCPRun(t, nil)
	trPutOpenCodeConfig(t, r, "declared-azure.json", "opencode.json")
	trPutOpenCodeConfig(t, r, "declared-tracker.jsonc", "opencode.jsonc")

	res := r.mustRun(r.request())

	if got := r.spy.splitSubjects(); len(got) != 0 {
		t.Errorf("both names are declared; no question may be asked, asked: %q", got)
	}
	if got := trUnmapped(t, res); len(got) != 0 {
		t.Errorf("UnmappedTools = %q, want none", got)
	}
	out := trMCPOutput(r)
	for _, want := range []string{"tracker/get_issue", "azure-devops/get_pull_request"} {
		if !strings.Contains(out, want) {
			t.Errorf("servers of both files must be combined; the GHCP agent must carry %q:\n%s", want, out)
		}
	}
}

func TestTransformMcpDeclared_MissingInvalidOrMcpLessConfigurationFallsThroughToTheQuestion(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
	}{
		{"missing", ""},
		{"invalid json", "invalid.json"},
		{"no mcp section", "no-mcp-section.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := trMCPRun(t, nil)
			if tc.fixture != "" {
				trPutOpenCodeConfig(t, r, tc.fixture, "opencode.json")
			}

			r.mustRun(r.request())

			if got, want := trSortedCopy(r.spy.splitSubjects()), trSortedCopy([]string{trSplitTracker, trSplitAzure}); !slices.Equal(got, want) {
				t.Errorf("split questions = %q, want %q", got, want)
			}
			if !r.exists(r.dest(trMCPAgentKey)) {
				t.Error("the agent must still be written after an unreadable configuration")
			}
		})
	}
}

func TestTransformMcpDeclared_UnreadableConfigurationNeverFailsTheRun(t *testing.T) {
	r := trMCPRun(t, nil)
	// A directory where the file should be cannot be read as a file.
	if err := os.MkdirAll(filepath.Join(r.ws, "opencode.json"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	r.mustRun(r.request())

	if got := r.spy.splitSubjects(); !slices.Contains(got, trSplitTracker) {
		t.Errorf("an unreadable configuration contributes nothing; the question must be asked, asked: %q", got)
	}
}

func TestTransformMcpDeclared_ConfigurationIsReadOnly(t *testing.T) {
	r := trMCPRun(t, nil)
	trPutOpenCodeConfig(t, r, "declared-azure.json", "opencode.json")
	want := trOpenCodeFixture(t, "opencode-config", "declared-azure.json")

	r.mustRun(r.request())

	if got := r.read("opencode.json"); got != want {
		t.Errorf("opencode.json was modified:\n%s", got)
	}
	if r.exists("opencode.jsonc") {
		t.Error("opencode.jsonc must not be created")
	}
}

func TestTransformMcpDeclared_OnlyAnOpenCodeSourceReadsTheConfiguration(t *testing.T) {
	// A Claude Code source has no ambiguous names; the OpenCode configuration is irrelevant.
	r := trNew(t, nil, hClaude, hCLI)
	r.seed("subagent")
	trWrite(t, r.ws, "opencode.json", trOpenCodeFixture(t, "opencode-config", "declared-azure.json"))

	res := r.mustRun(r.request())

	if got := r.spy.splitSubjects(); len(got) != 0 {
		t.Errorf("no split question may be asked for a Claude Code source, asked: %q", got)
	}
	if f := trFile(t, res, "subagent"); f.Status != app.StatusTransformed {
		t.Errorf("status = %q, want transformed", f.Status)
	}
}
