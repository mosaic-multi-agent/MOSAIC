package app_test

// deployed_tools_mcp_pairs_test.go verifies that whole-server and single-tool MCP names are
// parsed with the source harness's forms and written with the target harness's forms for
// every ordered pair of built-in harnesses, and that the two names that were once carried as
// plain text between OpenCode and GHCP CLI arrive in GHCP syntax.

import (
	"testing"

	"mosaic-deploy/internal/app"
)

// mcpHarness is one built-in harness with the forms of a server "srv" and its tool "lookup".
type mcpHarness struct {
	name        string
	rel         string
	wholeServer string
	singleTool  string
}

var mcpHarnesses = []mcpHarness{
	{"claude-code", harnessClaude, "mcp__srv__*", "mcp__srv__lookup"},
	{"opencode", harnessOpen, "srv_*", "srv_lookup"},
	{"ghcp-cli", harnessCLI, "srv/*", "srv/lookup"},
	{"vscode-ghcp", harnessVSCode, "srv/*", "srv/lookup"},
}

func TestConvertDeployedTools_WholeServerNameBetweenEveryPairOfHarnesses(t *testing.T) {
	for _, src := range mcpHarnesses {
		for _, tgt := range mcpHarnesses {
			if src.rel == tgt.rel {
				continue
			}
			t.Run(src.name+" to "+tgt.name, func(t *testing.T) {
				got := convertTools(t, src.rel, tgt.rel, []string{src.wholeServer})

				assertStrings(t, "TargetTools", []string{tgt.wholeServer}, got.TargetTools)
				assertStrings(t, "Dropped", nil, droppedNames(got))
				assertStrings(t, "Unmapped", nil, got.Unmapped)
				assertCustomTools(t, nil, got.CustomTools)
				assertRuleOf(t, got, src.wholeServer, app.ToolRuleMCP, "srv", "")
			})
		}
	}
}

func TestConvertDeployedTools_SingleToolNameBetweenEveryPairOfHarnesses(t *testing.T) {
	for _, src := range mcpHarnesses {
		for _, tgt := range mcpHarnesses {
			if src.rel == tgt.rel {
				continue
			}
			t.Run(src.name+" to "+tgt.name, func(t *testing.T) {
				got := convertTools(t, src.rel, tgt.rel, []string{src.singleTool})

				assertStrings(t, "TargetTools", []string{tgt.singleTool}, got.TargetTools)
				assertStrings(t, "Dropped", nil, droppedNames(got))
				assertStrings(t, "Unmapped", nil, got.Unmapped)
				if len(got.Ambiguous) != 0 {
					t.Errorf("a name with one possible split must not be ambiguous, got %+v", got.Ambiguous)
				}
				assertRuleOf(t, got, src.singleTool, app.ToolRuleMCP, "srv", "lookup")
			})
		}
	}
}

// assertRuleOf asserts the conversion entry of a source name is the given rule with the given
// server and tool parts.
func assertRuleOf(t *testing.T, c app.ConvertedTools, source string, rule app.ToolConversionRule, server, tool string) {
	t.Helper()
	cv := findConversion(t, c, source)
	if cv.Rule != rule || cv.Server != server || cv.Tool != tool {
		t.Errorf("conversion of %q: want rule %q server %q tool %q, got %+v", source, rule, server, tool, cv)
	}
}

func TestConvertDeployedTools_OpenCodeExampleNamesArriveInGhcpSyntax(t *testing.T) {
	// The two names from the OpenCode agents that GHCP CLI used to receive unchanged.
	got := convertTools(t, harnessOpen, harnessCLI,
		[]string{"contact-user*", "azure-devops_get_pull_request"}, withDeclaredServers("azure-devops"))
	assertStrings(t, "TargetTools", []string{"contact-user/*", "azure-devops/get_pull_request"}, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	if len(got.Ambiguous) != 0 {
		t.Errorf("a declared server resolves the name, got Ambiguous %+v", got.Ambiguous)
	}
}

func TestConvertDeployedTools_ServerNameCharactersAreNormalisedOnlyWhereTheTargetDemandsIt(t *testing.T) {
	// A server with a space cannot be written in Claude Code or OpenCode names; GHCP writes it as is.
	cases := []struct {
		src, tgt string
		in, want string
	}{
		{harnessCLI, harnessClaude, "my server/*", "mcp__my_server__*"},
		{harnessCLI, harnessOpen, "my server/*", "my_server_*"},
		{harnessOpen, harnessCLI, "my_server_*", "my_server/*"},
		{harnessCLI, harnessClaude, "my server/lookup", "mcp__my_server__lookup"},
		{harnessCLI, harnessOpen, "my server/lookup", "my_server_lookup"},
	}
	for _, tc := range cases {
		got := convertTools(t, tc.src, tc.tgt, []string{tc.in})
		assertStrings(t, tc.in+" to "+tc.tgt, []string{tc.want}, got.TargetTools)
	}
}

func TestConvertDeployedTools_OpenCodeWholeServerWithUnderscoreResolvesWithoutAQuestion(t *testing.T) {
	// Only single-tool names are ambiguous; the whole-server form is tried first.
	got := convertTools(t, harnessOpen, harnessCLI, []string{"a-b_c_*"})

	assertStrings(t, "TargetTools", []string{"a-b_c/*"}, got.TargetTools)
	assertStrings(t, "Dropped", nil, droppedNames(got))
	if len(got.Ambiguous) != 0 {
		t.Errorf("a whole-server name is never ambiguous, got %+v", got.Ambiguous)
	}
	assertRuleOf(t, got, "a-b_c_*", app.ToolRuleMCP, "a-b_c", "")
}

func TestConvertDeployedTools_NoNameIsCarriedAsPlainText(t *testing.T) {
	// Every MCP name of a mixed list leaves in the target's forms; none stays as written in the source.
	got := convertTools(t, harnessOpen, harnessCLI,
		[]string{"read", "alpha_*", "beta_search", "gamma*"})

	assertStrings(t, "TargetTools", []string{"alpha/*", "beta/search", "gamma/*"}, got.TargetTools)
	if len(got.Generic) == 0 || got.Generic[0] != "file_read" {
		t.Errorf("the mapped tool read must stay the first generic tool, got %v", got.Generic)
	}
	assertStrings(t, "Dropped", nil, droppedNames(got))
}
