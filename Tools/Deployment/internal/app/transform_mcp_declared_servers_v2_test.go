package app_test

// transform_mcp_declared_servers_v2_test.go specifies how the OpenCode V2 project configuration
// decides the split of an ambiguous MCP name when the transform source is opencode-v2: the keys
// of the mcp.servers object of opencode.json and opencode.jsonc at the workspace root and in
// .opencode/ (all read as JSONC) are the declared servers; keys directly under mcp (the V1
// form) are not read for a V2 source; a missing, unreadable or invalid file contributes nothing
// and never fails the run; the configuration is only read. A V1 source keeps reading only the
// top-level mcp keys of the root files.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/interactiontest"
)

// trMCPV2Run is an OpenCode V2 to GHCP CLI transform run over the V2 MCP agent fixture.
func trMCPV2Run(t *testing.T, script *interactiontest.Stub) *trRun {
	t.Helper()
	r := trNew(t, script, hOpenV2, hCLI, ocWithRegistry(t))
	r.put(hOpenV2, trMCPAgentKey, trOpenCodeFixture(t, "opencode-v2-agents", trMCPAgentKey+".md"))
	return r
}

// trPutV2Config copies a V2 configuration fixture to the workspace-relative path.
func trPutV2Config(t *testing.T, r *trRun, fixture, rel string) {
	t.Helper()
	trWrite(t, r.ws, rel, trOpenCodeFixture(t, "opencode-v2-config", fixture))
}

func TestTransformMcpDeclaredV2_ServersUnderMcpServersDecideWithoutAskingTheUser(t *testing.T) {
	r := trMCPV2Run(t, nil)
	trPutV2Config(t, r, "declared-azure.json", "opencode.json")

	r.mustRun(r.request())

	if got := r.spy.splitSubjects(); slices.Contains(got, trSplitAzure) {
		t.Errorf("the declared V2 server must decide %s without a question, asked: %q", trSplitAzure, got)
	}
	out := trMCPOutput(r)
	if !strings.Contains(out, "azure-devops/get_pull_request") || strings.Contains(out, "azure-devops_get_pull_request") {
		t.Errorf("the declared server must resolve the split in the written agent:\n%s", out)
	}
}

func TestTransformMcpDeclaredV2_JsoncWithCommentsAndTrailingCommasIsRead(t *testing.T) {
	r := trMCPV2Run(t, nil)
	trPutV2Config(t, r, "declared-tracker.jsonc", "opencode.jsonc")

	r.mustRun(r.request())

	if got := r.spy.splitSubjects(); slices.Contains(got, trSplitTracker) {
		t.Errorf("the declared server tracker must decide %s without a question, asked: %q", trSplitTracker, got)
	}
	if out := trMCPOutput(r); !strings.Contains(out, "tracker/get_issue") {
		t.Errorf("the GHCP agent must carry tracker/get_issue:\n%s", out)
	}
}

func TestTransformMcpDeclaredV2_AllFourProjectLocationsAreReadAndCombined(t *testing.T) {
	locations := []struct{ name, fixture, rel string }{
		{"root json", "declared-azure.json", "opencode.json"},
		{"root jsonc", "declared-tracker.jsonc", "opencode.jsonc"},
		{"dot-opencode json", "declared-azure.json", ".opencode/opencode.json"},
		{"dot-opencode jsonc", "declared-tracker.jsonc", ".opencode/opencode.jsonc"},
	}
	for _, loc := range locations {
		t.Run(loc.name, func(t *testing.T) {
			r := trMCPV2Run(t, nil)
			trPutV2Config(t, r, loc.fixture, loc.rel)

			r.mustRun(r.request())

			declared := strings.Contains(loc.fixture, "azure")
			subject := trSplitTracker
			if declared {
				subject = trSplitAzure
			}
			if got := r.spy.splitSubjects(); slices.Contains(got, subject) {
				t.Errorf("a server declared in %s must decide %s without a question, asked: %q", loc.rel, subject, got)
			}
		})
	}
	t.Run("combined", func(t *testing.T) {
		r := trMCPV2Run(t, nil)
		trPutV2Config(t, r, "declared-azure.json", ".opencode/opencode.json")
		trPutV2Config(t, r, "declared-tracker.jsonc", "opencode.jsonc")

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
	})
}

func TestTransformMcpDeclaredV2_TheV1FormUnderMcpIsNotReadForAV2Source(t *testing.T) {
	r := trMCPV2Run(t, nil)
	trPutV2Config(t, r, "v1-form.json", "opencode.json")

	r.mustRun(r.request())

	if got, want := trSortedCopy(r.spy.splitSubjects()), trSortedCopy([]string{trSplitTracker, trSplitAzure}); !slices.Equal(got, want) {
		t.Errorf("split questions = %q, want %q: keys directly under mcp declare nothing for a V2 source", got, want)
	}
}

func TestTransformMcpDeclaredV2_MissingInvalidOrServerLessConfigurationFallsThroughToTheQuestion(t *testing.T) {
	cases := []struct{ name, fixture string }{
		{"missing", ""},
		{"invalid json", "invalid.json"},
		{"mcp is not an object", "mcp-not-object.json"},
		{"servers is not an object", "servers-not-object.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := trMCPV2Run(t, nil)
			if tc.fixture != "" {
				trPutV2Config(t, r, tc.fixture, "opencode.json")
				trPutV2Config(t, r, tc.fixture, ".opencode/opencode.jsonc")
			}

			r.mustRun(r.request())

			if got, want := trSortedCopy(r.spy.splitSubjects()), trSortedCopy([]string{trSplitTracker, trSplitAzure}); !slices.Equal(got, want) {
				t.Errorf("split questions = %q, want %q", got, want)
			}
			if !r.exists(r.dest(trMCPAgentKey)) {
				t.Error("the agent must still be written after an unusable configuration")
			}
		})
	}
}

func TestTransformMcpDeclaredV2_UnreadableConfigurationNeverFailsTheRun(t *testing.T) {
	r := trMCPV2Run(t, nil)
	// A directory where the file should be cannot be read as a file.
	if err := os.MkdirAll(filepath.Join(r.ws, ".opencode", "opencode.json"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	trPutV2Config(t, r, "declared-azure.json", "opencode.json")

	r.mustRun(r.request())

	if got := r.spy.splitSubjects(); slices.Contains(got, trSplitAzure) {
		t.Errorf("the readable file must still decide %s, asked: %q", trSplitAzure, got)
	}
	if !r.exists(r.dest(trMCPAgentKey)) {
		t.Error("the agent must still be written")
	}
}

func TestTransformMcpDeclaredV2_ConfigurationIsOnlyReadNeverWritten(t *testing.T) {
	r := trMCPV2Run(t, nil)
	trPutV2Config(t, r, "declared-azure.json", "opencode.json")
	trPutV2Config(t, r, "declared-tracker.jsonc", ".opencode/opencode.jsonc")
	want := map[string]string{
		"opencode.json":            r.read("opencode.json"),
		".opencode/opencode.jsonc": r.read(".opencode/opencode.jsonc"),
	}

	r.mustRun(r.request())

	for rel, content := range want {
		if got := r.read(rel); got != content {
			t.Errorf("%s changed: got %q, want %q", rel, got, content)
		}
	}
}

func TestTransformMcpDeclaredV2_MCPSplitPreAnswerAppliesToAV2Source(t *testing.T) {
	r := trMCPV2Run(t, nil)
	req := r.request()
	req.MCPSplits = map[string]string{trSplitAzure: "azure-devops"}

	r.mustRun(req)

	if got := r.spy.splitSubjects(); slices.Contains(got, trSplitAzure) {
		t.Errorf("the pre-answer must settle %s without a question, asked: %q", trSplitAzure, got)
	}
	if out := trMCPOutput(r); !strings.Contains(out, "azure-devops/get_pull_request") {
		t.Errorf("the pre-answer must resolve the split in the written agent:\n%s", out)
	}
}

func TestTransformMcpDeclaredV2_AV1SourceStillReadsOnlyTheTopLevelMcpKeysOfTheRootFiles(t *testing.T) {
	cases := []struct {
		name, fixture, rel string
		declared           bool
	}{
		{"V1 form at the root", "v1-form.json", "opencode.json", true},
		{"V1 form in .opencode", "v1-form.json", ".opencode/opencode.json", false},
		{"V2 form at the root", "declared-azure.json", "opencode.json", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := trMCPRun(t, nil)
			trPutV2Config(t, r, tc.fixture, tc.rel)

			r.mustRun(r.request())

			asked := slices.Contains(r.spy.splitSubjects(), trSplitAzure)
			if tc.declared && asked {
				t.Errorf("a V1 source must read %s and decide %s without a question", tc.rel, trSplitAzure)
			}
			if !tc.declared && !asked {
				t.Errorf("a V1 source must not read %s; %s must be asked", tc.rel, trSplitAzure)
			}
		})
	}
}
